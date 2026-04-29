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

// RunWhiteBg 白底图处理链：背景移除 → 白色画布合成
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

	updateProgress(db, task, 10)

	// 查找参考图 URL
	var refAsset model.AiImageAsset
	if err := db.Where("asset_no = ? AND user_id = ?", assetNos[0], task.UserId).First(&refAsset).Error; err != nil {
		return fmt.Errorf("reference asset not found: %w", err)
	}

	// 调用阿里云视觉背景移除
	// 实际场景中 refAsset.OssKey 需要生成签名 URL 供外部访问
	transparentURL, err := vision.RemoveBackground(ctx, signedURL(refAsset.OssKey, cfg))
	if err != nil {
		return fmt.Errorf("remove background: %w", err)
	}

	updateProgress(db, task, 70)

	ossKey, err := ossUploadURL(uploader, transparentURL)
	if err != nil {
		return err
	}

	taskIDCopy := task.Id
	asset := model.AiImageAsset{
		AssetNo:   fmt.Sprintf("wbg_%d_%d", task.Id, time.Now().UnixNano()),
		TaskId:    &taskIDCopy,
		UserId:    task.UserId,
		Kind:      model.AssetKindGenerated,
		OssBucket: cfg.OSSBucket,
		OssKey:    ossKey,
		MimeType:  "image/png",
		CreatedAt: time.Now(),
	}
	return db.Create(&asset).Error
}

