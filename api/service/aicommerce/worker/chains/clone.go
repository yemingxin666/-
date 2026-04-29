package chains

import (
	"context"
	"fmt"
	"geekai/service/aicommerce"
	"geekai/service/aicommerce/prompt"
	"geekai/service/aicommerce/provider"
	"geekai/service/oss"
	"geekai/store/model"
	"time"

	"gorm.io/gorm"
)

// RunClone 克隆设计链：参考图风格分析 → 图生图
func RunClone(
	ctx context.Context,
	db *gorm.DB,
	imgClient provider.ImageClient,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
) error {
	input := task.InputJSON
	productName, _ := input["product_name"].(string)
	styleDesc, _ := input["style_desc"].(string)
	sellingPoints, _ := input["selling_points"].(string)
	assetNos, _ := extractStringSlice(input, "reference_assets")

	if len(assetNos) == 0 {
		return fmt.Errorf("clone: no reference image provided")
	}

	// 查找参考图
	var refAsset model.AiImageAsset
	if err := db.Where("asset_no = ? AND user_id = ?", assetNos[0], task.UserId).First(&refAsset).Error; err != nil {
		return fmt.Errorf("reference asset not found: %w", err)
	}

	updateProgress(db, task, 20)

	// 构建风格迁移 Prompt
	stylePrompt := fmt.Sprintf(
		"E-commerce product image, %s, %s, selling points: %s, %s style, high quality, professional photography",
		productName, prompt.PlatformRules(task.Platform), sellingPoints, styleDesc,
	)

	genReq := provider.ImageToImageReq{
		Model:     task.Model,
		Prompt:    stylePrompt,
		ImageURL:  signedURL(refAsset.OssKey, cfg),
		ImageSize: provider.RatioToSize(task.Ratio),
		Strength:  0.7,
	}

	updateProgress(db, task, 40)

	result, err := imgClient.ImageToImage(ctx, genReq)
	if err != nil {
		return fmt.Errorf("clone image: %w", err)
	}
	if len(result.Images) == 0 {
		return fmt.Errorf("clone image: no images returned")
	}

	updateProgress(db, task, 85)

	for _, img := range result.Images {
		ossKey, err := ossUploadURL(uploader, img.URL)
		if err != nil {
			return err
		}
		taskIDCopy := task.Id
		asset := model.AiImageAsset{
			AssetNo:   fmt.Sprintf("cln_%d_%d", task.Id, time.Now().UnixNano()),
			TaskId:    &taskIDCopy,
			UserId:    task.UserId,
			Kind:      model.AssetKindGenerated,
			OssBucket: cfg.OSSBucket,
			OssKey:    ossKey,
			MimeType:  "image/png",
			Width:     img.Width,
			Height:    img.Height,
			CreatedAt: time.Now(),
		}
		if err := db.Create(&asset).Error; err != nil {
			return fmt.Errorf("create cloned asset: %w", err)
		}
	}
	return nil
}
