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

// RunRatioConvert 比例转换链：裁剪 or Outpaint
func RunRatioConvert(
	ctx context.Context,
	db *gorm.DB,
	imgClient provider.ImageClient,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
) error {
	input := task.InputJSON
	assetNos, _ := extractStringSlice(input, "reference_assets")
	mode, _ := input["mode"].(string) // crop|outpaint|auto

	if len(assetNos) == 0 {
		return fmt.Errorf("ratio_convert: no source image provided")
	}

	var srcAsset model.AiImageAsset
	if err := db.Where("asset_no = ? AND user_id = ?", assetNos[0], task.UserId).First(&srcAsset).Error; err != nil {
		return fmt.Errorf("source asset not found: %w", err)
	}

	updateProgress(db, task, 20)

	var resultURL string

	if mode == "crop" || (mode == "auto" && canCrop(srcAsset, task.Ratio)) {
		// 本地裁剪（简化：直接标记为完成，实际需要图像处理库）
		resultURL = signedURL(srcAsset.OssKey, cfg) + "?ratio=" + task.Ratio
	} else {
		// Outpaint 扩图
		genReq := provider.ImageToImageReq{
			Model:     task.Model,
			Prompt:    "extend the image background naturally, maintain original product, professional e-commerce photo",
			ImageURL:  signedURL(srcAsset.OssKey, cfg),
			ImageSize: provider.RatioToSize(task.Ratio),
			Strength:  0.3, // 低强度保持主体
		}
		result, err := imgClient.ImageToImage(ctx, genReq)
		if err != nil {
			return fmt.Errorf("outpaint: %w", err)
		}
		if len(result.Images) > 0 {
			resultURL = result.Images[0].URL
		}
	}
	if resultURL == "" {
		return fmt.Errorf("ratio_convert: no result image returned")
	}

	ossKey, err := ossUploadURL(uploader, resultURL)
	if err != nil {
		return err
	}

	updateProgress(db, task, 85)

	taskIDCopy := task.Id
	asset := model.AiImageAsset{
		AssetNo:   fmt.Sprintf("rc_%d_%d", task.Id, time.Now().UnixNano()),
		TaskId:    &taskIDCopy,
		UserId:    task.UserId,
		Kind:      model.AssetKindGenerated,
		OssBucket: cfg.OSSBucket,
		OssKey:    ossKey,
		MimeType:  "image/jpeg",
		CreatedAt: time.Now(),
	}
	return db.Create(&asset).Error
}

func canCrop(asset model.AiImageAsset, targetRatio string) bool {
	if asset.Width == 0 || asset.Height == 0 {
		return false
	}
	// 简化判断：目标比例宽高比小于等于源图宽高比时可裁剪
	ratioMap := map[string]float64{
		"1:1": 1.0, "4:3": 1.33, "3:4": 0.75,
		"16:9": 1.78, "9:16": 0.56, "3:2": 1.5,
		"2:3": 0.67, "21:9": 2.33,
	}
	target, ok := ratioMap[targetRatio]
	if !ok {
		return false
	}
	src := float64(asset.Width) / float64(asset.Height)
	return target <= src
}
