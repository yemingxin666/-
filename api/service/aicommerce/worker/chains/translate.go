package chains

import (
	"context"
	"fmt"
	"geekai/service/aicommerce"
	"geekai/service/aicommerce/provider"
	"geekai/store/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

// RunTranslate 图文翻译链：OCR → 翻译 → 重合成
func RunTranslate(
	ctx context.Context,
	db *gorm.DB,
	ocr *provider.BaiduOCR,
	trans *provider.BaiduTranslate,
	cfg aicommerce.Config,
	task *model.AiImageTask,
) error {
	input := task.InputJSON
	assetNos, _ := extractStringSlice(input, "reference_assets")
	targetLang, _ := input["language"].(string)
	if targetLang == "" {
		targetLang = "en"
	}

	if len(assetNos) == 0 {
		return fmt.Errorf("translate: no image provided")
	}

	var srcAsset model.AiImageAsset
	if err := db.Where("asset_no = ? AND user_id = ?", assetNos[0], task.UserId).First(&srcAsset).Error; err != nil {
		return fmt.Errorf("source asset not found: %w", err)
	}

	updateProgress(db, task, 15)

	// 1. OCR 识别文字及坐标
	words, err := ocr.Recognize(ctx, signedURL(srcAsset.OssKey, cfg))
	if err != nil {
		return fmt.Errorf("OCR: %w", err)
	}

	updateProgress(db, task, 40)

	// 2. 批量翻译所有文字
	ocrData := make([]map[string]interface{}, 0, len(words))
	for _, w := range words {
		translated, err := trans.Translate(ctx, w.Words, "auto", langCode(targetLang))
		if err != nil {
			translated = w.Words // 翻译失败保留原文
		}
		ocrData = append(ocrData, map[string]interface{}{
			"original":   w.Words,
			"translated": translated,
			"location":   w.Location,
		})
	}

	updateProgress(db, task, 75)

	// 3. 将翻译结果存入资产元数据（实际 Canvas 重合成需前端或单独服务处理）
	taskIDCopy := task.Id
	asset := model.AiImageAsset{
		AssetNo:   fmt.Sprintf("tr_%d_%d", task.Id, time.Now().UnixNano()),
		TaskId:    &taskIDCopy,
		UserId:    task.UserId,
		Kind:      model.AssetKindGenerated,
		OssBucket: cfg.OSSBucket,
		OssKey:    srcAsset.OssKey, // 原图，前端根据 metadata_json 重合成文字
		MimeType:  srcAsset.MimeType,
		Width:     srcAsset.Width,
		Height:    srcAsset.Height,
		MetadataJSON: model.JSONMap{
			"translate_data": ocrData,
			"target_lang":    targetLang,
		},
		CreatedAt: time.Now(),
	}
	return db.Create(&asset).Error
}

// langCode 将语言名称转换为百度翻译语言代码
func langCode(lang string) string {
	m := map[string]string{
		"zh-CN": "zh", "en-US": "en", "ja-JP": "jp",
		"ko-KR": "kor", "en": "en", "zh": "zh",
	}
	if code, ok := m[strings.ToLower(lang)]; ok {
		return code
	}
	return "en"
}
