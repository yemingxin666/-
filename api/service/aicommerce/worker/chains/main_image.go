package chains

import (
	"context"
	"encoding/json"
	"fmt"
	"geekai/service/aicommerce"
	"geekai/service/aicommerce/prompt"
	"geekai/service/aicommerce/provider"
	"geekai/service/oss"
	"geekai/store/model"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// RunMainImage 主图/详情页生成链
func RunMainImage(
	ctx context.Context,
	db *gorm.DB,
	rdb *redis.Client,
	imgClient provider.ImageClient,
	tongyi *provider.Tongyi,
	uploader oss.Uploader,
	cfg aicommerce.Config,
	task *model.AiImageTask,
) error {
	input := task.InputJSON

	productName, _ := input["product_name"].(string)
	sellingPoints, _ := input["selling_points"].(string)
	styleDesc, _ := input["style_desc"].(string)

	// 1. 卖点代写（如果为空）
	if sellingPoints == "" {
		generated, err := tongyi.GenerateCopywrite(ctx, productName, "")
		if err == nil {
			sellingPoints = generated
		}
	}

	// 2. 查找 Prompt 模板
	repo := prompt.NewRepository(db)
	tmpl, err := repo.FindTemplate(task.Module, task.ImageType, task.Platform, task.Language, task.Ratio)
	if err != nil {
		return fmt.Errorf("prompt template not found: %w", err)
	}

	// 3. 渲染 Prompt
	vars := prompt.Vars{
		ProductName:   productName,
		SellingPoints: sellingPoints,
		ImageTypeDesc: prompt.ImageTypeDesc(task.ImageType),
		Platform:      task.Platform,
		PlatformRules: prompt.PlatformRules(task.Platform),
		Language:      task.Language,
		Ratio:         task.Ratio,
		StyleDesc:     styleDesc,
	}
	rendered, err := prompt.Render(tmpl.UserTemplate, tmpl.NegativeTemplate, vars)
	if err != nil {
		return fmt.Errorf("render prompt: %w", err)
	}

	// 保存 Prompt 快照
	promptSnapshot := model.JSONMap{
		"positive":    rendered.PositivePrompt,
		"negative":    rendered.NegativePrompt,
		"template_id": tmpl.Id,
	}
	db.Model(task).Update("prompt_json", promptSnapshot)

	// 更新进度
	updateProgress(db, task, 30)

	// 4. 调用图片生成模型
	genReq := provider.TextToImageReq{
		Model:          task.Model,
		Prompt:         rendered.PositivePrompt,
		NegativePrompt: rendered.NegativePrompt,
		ImageSize:      provider.RatioToSize(task.Ratio),
		BatchSize:      1,
		GuidanceScale:  7.5,
	}
	genResult, err := imgClient.TextToImage(ctx, genReq)
	if err != nil {
		return fmt.Errorf("generate image: %w", err)
	}

	updateProgress(db, task, 80)

	// 5. 上传 OSS 并保存 asset 记录
	for _, img := range genResult.Images {
		ossKey, err := ossUploadURL(uploader, img.URL)
		if err != nil {
			return err
		}
		asset := model.AiImageAsset{
			AssetNo:   fmt.Sprintf("gen_%d_%d", task.Id, time.Now().UnixNano()),
			TaskId:    &task.Id,
			UserId:    task.UserId,
			Kind:      model.AssetKindGenerated,
			OssBucket: cfg.OSSBucket,
			OssKey:    ossKey,
			MimeType:  "image/png",
			Width:     img.Width,
			Height:    img.Height,
			MetadataJSON: model.JSONMap{
				"seed":       genResult.Seed,
				"timings_ms": img.TimingsMs,
			},
			CreatedAt: time.Now(),
		}
		if err := db.Create(&asset).Error; err != nil {
			return fmt.Errorf("create generated asset: %w", err)
		}
	}

	// 发布进度到 Redis PubSub
	progressEvent, _ := json.Marshal(map[string]interface{}{"progress": 100})
	rdb.Publish(ctx, "aic:progress:"+task.TaskNo, progressEvent)

	return nil
}
