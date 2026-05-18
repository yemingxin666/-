package chains

import (
	"bytes"
	"context"
	"fmt"
	logger2 "geekai/logger"
	"geekai/service/aicommerce"
	"geekai/service/aicommerce/provider"
	"geekai/service/oss"
	"geekai/store/model"
	"geekai/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

// RunWhiteBg 白底图处理链：参考图逐张 抠图 → 白底画布合成 → OSS。
//
// 产物是电商标准白底主图（纯白底 + 产品主体居中 + 等比留白），不是透明 PNG。
// 合成逻辑见 imageutil.go。
//
// 容错策略：循环内单张失败不中断，记录到 firstErr。
// 任务层面只要至少一张成功就返回 nil（dispatcher 会判为 succeeded）；
// 全部失败时返回 firstErr，触发上层的失败态与积分退还。
func RunWhiteBg(
	ctx context.Context,
	db *gorm.DB,
	vision *provider.AliyunVision,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
) error {
	input := task.InputJSON
	assetNos, _ := extractStringSlice(input, "reference_assets")
	if len(assetNos) == 0 {
		return fmt.Errorf("white_bg: no reference image provided")
	}

	total := len(assetNos)
	updateProgress(db, task, 5)

	// 预取所有参考图记录，一次查询避免 N+1。
	// 使用 map 保证顺序用 assetNos 的原始顺序检索。
	var refAssets []model.AiImageAsset
	if err := db.Where("asset_no IN ? AND user_id = ?", assetNos, task.UserId).
		Find(&refAssets).Error; err != nil {
		return fmt.Errorf("load reference assets: %w", err)
	}
	refMap := make(map[string]model.AiImageAsset, len(refAssets))
	for _, a := range refAssets {
		refMap[a.AssetNo] = a
	}

	// 为每张参考图预创建占位 asset（phase=generating，oss_key 空）。
	// 这样 buildSyntheticItems 在轮询时就能看到 N 张"处理中"卡片，
	// 不再出现"只显示 1 个任务 → 完成后突然冒出 2 张图"的跳变。
	placeholderIDs := make([]uint, total)
	for i, assetNo := range assetNos {
		id, err := createWhiteBgPlaceholder(db, task, assetNo, i)
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
		refAsset, ok := refMap[assetNo]
		if !ok {
			saveTypeError(db, task, fmt.Sprintf("%s_%d", task.Module, i),
				fmt.Sprintf("reference asset %s not found", assetNo), placeholderIDs[i])
			if firstErr == nil {
				firstErr = fmt.Errorf("reference asset %s not found", assetNo)
			}
			continue
		}

		if err := processOneWhiteBg(ctx, db, vision, uploader, cfg, task, refAsset, placeholderIDs[i]); err != nil {
			saveTypeError(db, task, fmt.Sprintf("%s_%d", task.Module, i), err.Error(), placeholderIDs[i])
			if firstErr == nil {
				firstErr = err
			}
		} else {
			succeeded++
		}

		// 均分 10 ~ 95 的进度区间到每张图上，前端看到的进度更线性
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
		return fmt.Errorf("white_bg: no image processed successfully")
	}

	if succeeded < total {
		unitPrice, quantity := extractBillingSnapshot(task)
		if unitPrice > 0 && quantity == total && unitPrice*quantity == task.CreditCost {
			if err := refundPartialWhiteBg(db, task, total, succeeded, unitPrice); err != nil {
				logger2.GetLogger().Errorf("[white_bg] task=%d partial refund failed: %v", task.Id, err)
				_ = db.Model(task).Update("error_message",
					fmt.Sprintf("white_bg partial refund failed: %v", err)).Error
			}
		} else {
			logger2.GetLogger().Warnf(
				"[white_bg] task=%d billing snapshot mismatch: unit=%d qty=%d total=%d credit_cost=%d, skip partial refund",
				task.Id, unitPrice, quantity, total, task.CreditCost)
		}
	}
	return nil
}

// processOneWhiteBg 处理单张参考图：抠图 → 下载 → 合底 → 上传 → 升级占位 asset。
// placeholderID 是 RunWhiteBg 预先创建的占位 asset，这里负责把它 finalize
// 为带有 oss_key 的真实资产，保证前端能在该张处理完后立即看到图片。
func processOneWhiteBg(
	ctx context.Context,
	db *gorm.DB,
	vision *provider.AliyunVision,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
	refAsset model.AiImageAsset,
	placeholderID uint,
) error {
	// 1) 准备可供阿里云拉取的参考图 URL。
	//    OssKey 此处实际存的是完整 URL（见 UploadAsset handler），
	//    但私有 bucket 的 URL 不签名会 403，所以统一走 uploader.SignURL。
	srcURL, err := resolveReferenceURL(refAsset, uploader, cfg)
	if err != nil {
		return fmt.Errorf("resolve reference url for %s: %w", refAsset.AssetNo, err)
	}

	// 阶段标记：generating（抠图 + 下载）→ uploading（合底 + 上传）→ succeeded
	updatePhaseAsset(db, placeholderID, PhaseGenerating)

	// 2) 抠图：拿到透明背景 PNG 的外链
	transparentURL, err := vision.RemoveBackground(ctx, srcURL)
	if err != nil {
		return fmt.Errorf("remove background for %s: %w", refAsset.AssetNo, err)
	}

	// 3) 下载透明图到内存
	transparentBytes, err := utils.DownloadImage(transparentURL, "")
	if err != nil {
		return fmt.Errorf("download transparent image for %s: %w", refAsset.AssetNo, err)
	}

	// 4) 合成白底
	whiteBgBytes, width, height, err := CompositeWhiteBg(bytes.NewReader(transparentBytes), task.Ratio)
	if err != nil {
		return fmt.Errorf("composite white bg for %s: %w", refAsset.AssetNo, err)
	}

	updatePhaseAsset(db, placeholderID, PhaseUploading)

	// 5) 上传字节流
	fileURL, err := uploader.PutBytes(whiteBgBytes, ".png")
	if err != nil {
		return fmt.Errorf("upload white bg for %s: %w", refAsset.AssetNo, err)
	}

	// 6) finalize 占位 asset：写入 OssKey + 尺寸 + metadata
	return finalizePhaseAsset(db, placeholderID, fileURL, cfg.OSSBucket, "image/png", width, height,
		model.JSONMap{
			"source_asset_no": refAsset.AssetNo,
			"ratio":           task.Ratio,
		})
}

// createWhiteBgPlaceholder 为 white_bg 单张参考图创建占位 asset。
// 与主图 chain 的 createPhaseAsset 行为一致，区别是用 "<module>_<idx>" 作为
// metadata.image_type，让前端（即使未来改造为 typed items）也能稳定定位。
func createWhiteBgPlaceholder(db *gorm.DB, task *model.AiImageTask, sourceAssetNo string, idx int) (uint, error) {
	taskID := task.Id
	asset := model.AiImageAsset{
		AssetNo:   fmt.Sprintf("wbg_%d_%d_%d", task.Id, idx, time.Now().UnixNano()),
		TaskId:    &taskID,
		UserId:    task.UserId,
		Kind:      model.AssetKindGenerated,
		MimeType:  "image/png",
		CreatedAt: time.Now(),
		MetadataJSON: model.JSONMap{
			"image_type":      fmt.Sprintf("%s_%d", task.Module, idx),
			"phase":           PhaseRendering,
			"source_asset_no": sourceAssetNo,
		},
	}
	if err := db.Create(&asset).Error; err != nil {
		return 0, fmt.Errorf("create white_bg placeholder task=%d idx=%d: %w", taskID, idx, err)
	}
	return asset.Id, nil
}

// resolveReferenceURL 为 white_bg / 其它 chain 产生"阿里云可访问"的参考图 URL。
//
// 历史数据情况：
//   - 新数据：OssKey 直接是 `https://<bucket>.oss-xxx.aliyuncs.com/<key>` 形式的完整 URL
//   - 老数据：OssKey 可能只是裸 key（无协议），bucket 以 asset.OssBucket 或 cfg.OSSBucket 为准
//
// uploader.SignURL 只有阿里云实现会真的签名（见 service/oss/aliyun_oss.go），
// 其他存储引擎直接返回原 URL；如果参考图本来就是公网可读的，签名仍然安全。
func resolveReferenceURL(a model.AiImageAsset, uploader oss.Uploader, cfg aicommerce.Config) (string, error) {
	key := strings.TrimSpace(a.OssKey)
	if key == "" {
		return "", fmt.Errorf("asset %s has empty oss_key", a.AssetNo)
	}

	ttl := int64(cfg.AssetURLTTL)
	if ttl <= 0 {
		ttl = 3600
	}

	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		signed, err := uploader.SignURL(key, ttl)
		if err != nil {
			return key, nil // 签名失败时退回原 URL，公网可读的对象仍可用
		}
		return signed, nil
	}

	// 裸 key：需要拼 bucket。遗留代码里把端点写死成 hangzhou，
	// 但真实数据都是从 signedURL 迁移过来的，历史遗留基本只剩 hangzhou 一个案例；
	// 保留兼容分支。
	bucket := strings.TrimSpace(a.OssBucket)
	if bucket == "" {
		bucket = strings.TrimSpace(cfg.OSSBucket)
	}
	if bucket == "" {
		return "", fmt.Errorf("asset %s has no bucket and cfg.OSSBucket is empty", a.AssetNo)
	}
	raw := fmt.Sprintf("https://%s.oss-cn-hangzhou.aliyuncs.com/%s", bucket, strings.TrimLeft(key, "/"))
	signed, err := uploader.SignURL(raw, ttl)
	if err != nil {
		return raw, nil
	}
	return signed, nil
}

func refundPartialWhiteBg(db *gorm.DB, task *model.AiImageTask, total, succeeded, unitCost int) error {
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
			return fmt.Errorf("update white_bg credit_cost: %w", taskResult.Error)
		}
		if taskResult.RowsAffected == 0 {
			return fmt.Errorf("credit_cost already changed (expected %d), skip refund", expectedCost)
		}
		userResult := tx.Model(&model.User{}).
			Where("id = ?", task.UserId).
			UpdateColumn("power", gorm.Expr("power + ?", refund))
		if userResult.Error != nil {
			return fmt.Errorf("refund white_bg credits: %w", userResult.Error)
		}
		if userResult.RowsAffected == 0 {
			return fmt.Errorf("user %d not found, cannot refund", task.UserId)
		}
		task.CreditCost = finalCost
		return nil
	})
}
