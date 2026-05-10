package chains

import (
	"context"
	"fmt"
	"geekai/service/aicommerce"
	"geekai/service/aicommerce/provider"
	"geekai/service/oss"
	"geekai/store/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

// RunEdit 基于原图 + prompt 的编辑链：
// 1. 从 task.InputJSON 取 prompt + source_asset_no
// 2. 查源 asset，组装 image_url
// 3. 调用 ImageToImage(image_url, prompt, ratio→1K image_size)
// 4. 上传结果 OSS，写入新 generated asset
func RunEdit(
	ctx context.Context,
	db *gorm.DB,
	imgClient provider.ImageClient,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
) error {
	input := task.InputJSON
	prompt, _ := input["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("edit: empty prompt in task %d", task.Id)
	}
	srcAssetNo, _ := input["source_asset_no"].(string)
	if strings.TrimSpace(srcAssetNo) == "" {
		return fmt.Errorf("edit: missing source_asset_no in task %d", task.Id)
	}

	// 查源 asset（必须归属当前用户、已生成、未删除）
	var srcAsset model.AiImageAsset
	if err := db.Where("asset_no = ? AND user_id = ? AND kind = ? AND deleted_at IS NULL",
		srcAssetNo, task.UserId, model.AssetKindGenerated).First(&srcAsset).Error; err != nil {
		return fmt.Errorf("edit: source asset %s not found: %w", srcAssetNo, err)
	}

	updateProgress(db, task, 20)

	// 解析为带签名、可被上游 API 公网访问的 URL（私有 bucket 必需）
	srcURLs := resolveAssetURLs(db, task.UserId, []string{srcAsset.AssetNo}, cfg, uploader)
	if len(srcURLs) == 0 {
		return fmt.Errorf("edit: failed to resolve signed URL for source asset %s", srcAsset.AssetNo)
	}

	// 调用 img2img
	result, err := imgClient.ImageToImage(ctx, provider.ImageToImageReq{
		Model:     task.Model,
		Prompt:    prompt,
		ImageURL:  srcURLs[0],
		ImageSize: provider.RatioToSize(task.Ratio),
		Strength:  0.7,
	})
	if err != nil {
		return fmt.Errorf("edit: imageToImage call failed: %w", err)
	}
	if len(result.Images) == 0 {
		return fmt.Errorf("edit: no image returned from provider")
	}

	updateProgress(db, task, 70)

	// 上传到 OSS
	ossKey, err := ossUploadURL(uploader, result.Images[0].URL)
	if err != nil {
		return err
	}

	updateProgress(db, task, 90)

	// 写入新 generated asset，挂在本任务下
	taskID := task.Id
	asset := model.AiImageAsset{
		AssetNo:   fmt.Sprintf("ed_%d_%d", task.Id, time.Now().UnixNano()),
		TaskId:    &taskID,
		UserId:    task.UserId,
		Kind:      model.AssetKindGenerated,
		OssBucket: cfg.OSSBucket,
		OssKey:    ossKey,
		MimeType:  "image/jpeg",
		MetadataJSON: model.JSONMap{
			"source_task_no":  input["source_task_no"],
			"source_asset_no": srcAssetNo,
			"prompt":          prompt,
		},
		CreatedAt: time.Now(),
	}
	return db.Create(&asset).Error
}
