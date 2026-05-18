package chains

import (
	"context"
	"fmt"
	logger2 "geekai/logger"
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
		id, err := createPhaseAsset(db, task, translateImageType(i), PhaseRendering)
		if err != nil {
			return err
		}
		placeholderIDs[i] = id
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

	if succeeded < total {
		unitPrice, quantity := extractBillingSnapshot(task)
		if unitPrice > 0 && quantity == total && unitPrice*quantity == task.CreditCost {
			if err := refundPartialTranslate(db, task, total, succeeded, unitPrice); err != nil {
				logger2.GetLogger().Errorf("[translate] task=%d partial refund failed: %v", task.Id, err)
				_ = db.Model(task).Update("error_message",
					fmt.Sprintf("translate partial refund failed: %v", err)).Error
			}
		} else {
			logger2.GetLogger().Warnf(
				"[translate] task=%d billing snapshot mismatch: unit=%d qty=%d total=%d credit_cost=%d, skip partial refund",
				task.Id, unitPrice, quantity, total, task.CreditCost)
		}
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

func refundPartialTranslate(db *gorm.DB, task *model.AiImageTask, total, succeeded, unitCost int) error {
	if task == nil || task.Id == 0 || total <= 0 || succeeded <= 0 || succeeded >= total || unitCost <= 0 {
		return nil
	}
	failed := total - succeeded
	refund := failed * unitCost
	finalCost := succeeded * unitCost
	expectedCost := total * unitCost
	return db.Transaction(func(tx *gorm.DB) error {
		taskResult := tx.Model(&model.AiImageTask{}).
			Where("id = ? AND status = ? AND credit_cost = ?", task.Id, model.TaskStatusRunning, expectedCost).
			Update("credit_cost", finalCost)
		if taskResult.Error != nil {
			return fmt.Errorf("update translate credit_cost: %w", taskResult.Error)
		}
		if taskResult.RowsAffected == 0 {
			return fmt.Errorf("credit_cost already changed (expected %d), skip refund", expectedCost)
		}
		userResult := tx.Model(&model.User{}).
			Where("id = ?", task.UserId).
			UpdateColumn("power", gorm.Expr("power + ?", refund))
		if userResult.Error != nil {
			return fmt.Errorf("refund translate credits: %w", userResult.Error)
		}
		if userResult.RowsAffected == 0 {
			return fmt.Errorf("user %d not found, cannot refund", task.UserId)
		}
		task.CreditCost = finalCost
		return nil
	})
}
