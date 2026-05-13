package chains

import (
	"context"
	"fmt"
	"geekai/service/aicommerce"
	"geekai/service/aicommerce/provider"
	"geekai/service/oss"
	"geekai/store/model"
	"time"

	"gorm.io/gorm"
)

const translatePerImageTimeout = 3 * time.Minute

// RunTranslate 图文翻译链：调用阿里云 TranslateImage API（OCR → 翻译 → 文字重渲染一站式完成），
// 下载结果图并上传到自有 OSS。
func RunTranslate(
	ctx context.Context,
	db *gorm.DB,
	translator *provider.AliyunTranslate,
	uploader oss.Uploader,
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

	total := len(assetNos)
	updateProgress(db, task, 5)

	placeholderIDs := make([]uint, total)
	for i := range assetNos {
		placeholderIDs[i] = createPhaseAsset(db, task, translateImageType(i), PhaseRendering)
	}

	var (
		firstErr  error
		succeeded int
	)

	for i, assetNo := range assetNos {
		imageType := translateImageType(i)
		placeholderID := placeholderIDs[i]

		if err := processOneTranslate(
			ctx, db, translator, uploader, cfg, task,
			assetNo, targetLang, imageType, placeholderID,
		); err != nil {
			saveTypeError(db, task, imageType, err.Error(), placeholderID)
			if firstErr == nil {
				firstErr = err
			}
		} else {
			succeeded++
		}

		progress := 10 + int(float64(i+1)/float64(total)*85)
		if progress > 95 {
			progress = 95
		}
		updateProgress(db, task, progress)
	}

	if succeeded == 0 {
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("translate: no image translated successfully")
	}

	return nil
}

func processOneTranslate(
	ctx context.Context,
	db *gorm.DB,
	translator *provider.AliyunTranslate,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
	assetNo, targetLang, imageType string,
	placeholderID uint,
) error {
	srcURLs := resolveAssetURLs(db, task.UserId, []string{assetNo}, cfg, uploader)
	if len(srcURLs) == 0 {
		return fmt.Errorf("translate: source asset %s not found or inaccessible", assetNo)
	}

	updatePhaseAsset(db, placeholderID, PhaseGenerating)

	callCtx, cancel := context.WithTimeout(ctx, translatePerImageTimeout)
	defer cancel()

	finalURL, err := translator.TranslateImage(callCtx, srcURLs[0], "auto", targetLang)
	if err != nil {
		return fmt.Errorf("translate image %s: %w", assetNo, err)
	}

	updatePhaseAsset(db, placeholderID, PhaseUploading)

	ossKey, err := ossUploadURL(uploader, finalURL)
	if err != nil {
		return fmt.Errorf("upload translated image %s: %w", assetNo, err)
	}

	var srcAsset model.AiImageAsset
	db.Where("asset_no = ? AND user_id = ?", assetNo, task.UserId).First(&srcAsset)

	return finalizePhaseAsset(db, placeholderID, ossKey, cfg.OSSBucket, "image/png",
		srcAsset.Width, srcAsset.Height,
		model.JSONMap{
			"image_type":       imageType,
			"source_asset_no":  assetNo,
			"target_lang":      targetLang,
			"translate_engine": "aliyun_translate_image",
		})
}

func translateImageType(index int) string {
	return fmt.Sprintf("translate_%d", index)
}
