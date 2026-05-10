package chains

import (
	"fmt"
	"geekai/service/aicommerce"
	"geekai/service/oss"
	"geekai/store/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

// splitImageTypes 拆分逗号分隔的图片类型，过滤空白，保证与 SubmitTask 预检语义一致
func splitImageTypes(imageType string) []string {
	parts := strings.Split(imageType, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	if len(result) == 0 {
		return []string{imageType}
	}
	return result
}

// Phase 常量：每个图片类型的生成阶段，前端映射为进度百分比
const (
	PhaseRendering  = "rendering"  // 查模板+渲染 prompt → 20%
	PhaseGenerating = "generating" // 调用 AI 生图 API → 50%
	PhaseUploading  = "uploading"  // 上传 OSS → 85%
	PhaseSucceeded  = "succeeded"  // 完成 → 100%
	PhaseFailed     = "failed"     // 失败
)

// createPhaseAsset 为某个 image_type 创建占位 asset，phase 字段标记当前阶段
// 返回 asset ID 供后续 updatePhaseAsset 使用
func createPhaseAsset(db *gorm.DB, task *model.AiImageTask, imageType, phase string) uint {
	taskID := task.Id
	asset := model.AiImageAsset{
		AssetNo:  fmt.Sprintf("phase_%d_%s_%d", taskID, imageType, time.Now().UnixNano()),
		TaskId:   &taskID,
		UserId:   task.UserId,
		Kind:     model.AssetKindGenerated,
		MetadataJSON: model.JSONMap{
			"image_type": imageType,
			"phase":      phase,
		},
		CreatedAt: time.Now(),
	}
	db.Create(&asset)
	return asset.Id
}

// updatePhaseAsset 更新占位 asset 的阶段
func updatePhaseAsset(db *gorm.DB, assetID uint, phase string) {
	db.Model(&model.AiImageAsset{}).Where("id = ?", assetID).
		Update("metadata_json", gorm.Expr("JSON_SET(metadata_json, '$.phase', ?)", phase))
}

// finalizePhaseAsset 将占位 asset 升级为真实 asset（写入 OssKey 等字段，phase 改为 succeeded）
func finalizePhaseAsset(db *gorm.DB, assetID uint, ossKey, bucket, mime string, width, height int, extraMeta model.JSONMap) error {
	extraMeta["phase"] = PhaseSucceeded
	return db.Model(&model.AiImageAsset{}).Where("id = ?", assetID).Updates(map[string]interface{}{
		"oss_key":       ossKey,
		"oss_bucket":    bucket,
		"mime_type":     mime,
		"width":         width,
		"height":        height,
		"metadata_json": extraMeta,
	}).Error
}

// saveTypeError 将单个图片类型的失败信息更新到占位 asset，前端可据此显示失败态
// 若 assetID == 0，则创建新 error asset（兼容模板查找失败的场景，此时还未创建占位）
func saveTypeError(db *gorm.DB, task *model.AiImageTask, imageType, errMsg string, assetID uint) {
	if assetID != 0 {
		db.Model(&model.AiImageAsset{}).Where("id = ?", assetID).
			Update("metadata_json", gorm.Expr("JSON_SET(metadata_json, '$.phase', ?, '$.error', ?)", PhaseFailed, errMsg))
		return
	}
	taskID := task.Id
	db.Create(&model.AiImageAsset{
		AssetNo:  fmt.Sprintf("err_%d_%d", taskID, time.Now().UnixNano()),
		TaskId:   &taskID,
		UserId:   task.UserId,
		Kind:     model.AssetKindGenerated,
		MetadataJSON: model.JSONMap{
			"image_type": imageType,
			"phase":      PhaseFailed,
			"error":      errMsg,
		},
		CreatedAt: time.Now(),
	})
}

func updateProgress(db *gorm.DB, task *model.AiImageTask, progress int) {
	db.Model(task).Update("progress", progress)
}

// Deprecated: 此函数仅拼接公网 URL，不做签名。私有 OSS bucket 下上游 API 拉取会 403。
// 新代码请使用 resolveAssetURLs(...) 获取带签名的可访问 URL。
// 保留是为了兼容老 chain（clone/white_bg/ratio_convert/translate），同样存在风险，应逐步迁移。
func signedURL(ossKey string, cfg aicommerce.Config) string {
	return fmt.Sprintf("https://%s.oss-cn-hangzhou.aliyuncs.com/%s", cfg.OSSBucket, ossKey)
}

func extractStringSlice(m model.JSONMap, key string) ([]string, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	raw, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result, true
}

// ossUploadURL 从外部 URL 下载图片并上传到 OSS，返回 ossKey
func ossUploadURL(uploader oss.Uploader, imgURL string) (string, error) {
	ossKey, err := uploader.PutUrlFile(imgURL, ".png", true)
	if err != nil {
		return "", fmt.Errorf("upload image to OSS: %w", err)
	}
	return ossKey, nil
}
