package chains

import (
	"bytes"
	"context"
	"fmt"
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

	var (
		firstErr  error
		succeeded int
	)

	for i, assetNo := range assetNos {
		refAsset, ok := refMap[assetNo]
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("reference asset %s not found", assetNo)
			}
			continue
		}

		if err := processOneWhiteBg(ctx, db, vision, uploader, cfg, task, refAsset); err != nil {
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

	// 按张计费：若部分张数失败，按失败张数退款并同步 credit_cost 落库
	// - dispatcher 成功分支不退款，所以这里的退款只在 chain 内完成
	// - 若 succeeded == total，不退款，保留原扣费
	if failed := total - succeeded; failed > 0 && task.CreditCost > 0 && total > 0 {
		unit := task.CreditCost / total
		refund := unit * failed
		if refund > 0 {
			db.Model(&model.User{}).Where("id = ?", task.UserId).
				UpdateColumn("power", gorm.Expr("power + ?", refund))
			db.Model(task).Update("credit_cost", task.CreditCost-refund)
			task.CreditCost = task.CreditCost - refund
		}
	}
	return nil
}

// processOneWhiteBg 处理单张参考图：抠图 → 下载 → 合底 → 上传 → 落库。
// 抽出单图流程便于循环复用，也方便后续改造（例如并发、部分失败追踪）。
func processOneWhiteBg(
	ctx context.Context,
	db *gorm.DB,
	vision *provider.AliyunVision,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
	refAsset model.AiImageAsset,
) error {
	// 1) 准备可供阿里云拉取的参考图 URL。
	//    OssKey 此处实际存的是完整 URL（见 UploadAsset handler），
	//    但私有 bucket 的 URL 不签名会 403，所以统一走 uploader.SignURL。
	srcURL, err := resolveReferenceURL(refAsset, uploader, cfg)
	if err != nil {
		return fmt.Errorf("resolve reference url for %s: %w", refAsset.AssetNo, err)
	}

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

	// 5) 上传字节流
	fileURL, err := uploader.PutBytes(whiteBgBytes, ".png")
	if err != nil {
		return fmt.Errorf("upload white bg for %s: %w", refAsset.AssetNo, err)
	}

	// 6) 落库。OssKey 字段存完整 URL，与其他 chain 保持一致。
	taskIDCopy := task.Id
	asset := model.AiImageAsset{
		AssetNo:   fmt.Sprintf("wbg_%d_%d", task.Id, time.Now().UnixNano()),
		TaskId:    &taskIDCopy,
		UserId:    task.UserId,
		Kind:      model.AssetKindGenerated,
		OssBucket: cfg.OSSBucket,
		OssKey:    fileURL,
		MimeType:  "image/png",
		Width:     width,
		Height:    height,
		MetadataJSON: model.JSONMap{
			"source_asset_no": refAsset.AssetNo,
			"ratio":           task.Ratio,
		},
		CreatedAt: time.Now(),
	}
	return db.Create(&asset).Error
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
