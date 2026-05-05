package chains

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geekai/service/aicommerce"
	"geekai/service/aicommerce/prompt"
	"geekai/service/aicommerce/provider"
	"geekai/service/oss"
	"geekai/store/model"
	"strings"

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

	// 基础 Vars
	vars := prompt.Vars{
		ProductName:   productName,
		SellingPoints: sellingPoints,
		ImageTypeDesc: prompt.ImageTypeDesc(task.ImageType),
		Platform:      task.Platform,
		PlatformRules: prompt.PlatformRules(db, task.Platform),
		Language:      task.Language,
		Ratio:         task.Ratio,
		StyleDesc:     styleDesc,
	}

	// 1. Case A / Case B：判断前端是否已附带 AI 代写的 analysis
	var analysisMap map[string]interface{}
	if raw, ok := input["analysis"]; ok && raw != nil {
		analysisMap, _ = raw.(map[string]interface{})
	}

	if analysisMap != nil {
		// Case A：用户已点击"AI 代写卖点"，直接复用 analysis，不再调 AI
		if analysis := parseAnalysisFromMap(analysisMap); analysis != nil {
			vars = fillAnalysisVars(vars, analysis)
			if styleDesc == "" {
				vars.StyleDesc = getStyleDesc(analysis.RecommendedStyle)
			}
		}
	} else {
		// Case B：用户手动填写卖点，后端调用视觉代写
		assetNos, _ := extractStringSlice(input, "reference_assets")
		if len(assetNos) == 0 {
			return fmt.Errorf("请上传参考图后再生成")
		}

		imageURLs := resolveAssetURLs(db, task.UserId, assetNos, cfg, uploader)
		if len(imageURLs) == 0 {
			return fmt.Errorf("参考图解析失败，请重新上传")
		}

		visionClient, visionErr := buildVisionClient(db)
		if visionErr == nil {
			content, analysis, err := visionClient.GenerateCopywrite(ctx, productName, sellingPoints, imageURLs)
			if err == nil {
				if sellingPoints == "" {
					vars.SellingPoints = content
				}
				if analysis != nil {
					vars = fillAnalysisVars(vars, analysis)
					if styleDesc == "" {
						vars.StyleDesc = getStyleDesc(analysis.RecommendedStyle)
					}
				}
			}
		}
	}

	// 2. 拆分多选 image_type，逐个查模板、渲染、生图
	repo := prompt.NewRepository(db)
	imageTypes := splitImageTypes(task.ImageType)
	total := len(imageTypes)

	// 解析参考图 URL（循环外，避免重复查询）
	refAssetNos, _ := extractStringSlice(input, "reference_assets")
	refURLs := resolveAssetURLs(db, task.UserId, refAssetNos, cfg, uploader)

	var firstErr error
	succeededCount := 0

	for i, imageType := range imageTypes {
		typeVars := vars
		typeVars.ImageTypeDesc = prompt.ImageTypeDesc(imageType)

		// 阶段 1：rendering — 创建占位 asset，前端立即可见"进行中"
		phaseAssetID := createPhaseAsset(db, task, imageType, PhaseRendering)

		tmpl, err := repo.FindTemplate(task.Module, imageType)
		if err != nil {
			saveTypeError(db, task, imageType, fmt.Sprintf("模板未找到: %v", err), phaseAssetID)
			if firstErr == nil {
				firstErr = fmt.Errorf("prompt template not found for %s: %w", imageType, err)
			}
			continue
		}

		rendered, err := prompt.Render(tmpl.UserTemplate, tmpl.NegativeTemplate, typeVars)
		if err != nil {
			saveTypeError(db, task, imageType, fmt.Sprintf("渲染失败: %v", err), phaseAssetID)
			if firstErr == nil {
				firstErr = fmt.Errorf("render prompt for %s: %w", imageType, err)
			}
			continue
		}

		// 首张图额外保存任务级 Prompt 快照（整体回溯用）
		if i == 0 {
			db.Model(task).Update("prompt_json", model.JSONMap{
				"positive":    rendered.PositivePrompt,
				"negative":    rendered.NegativePrompt,
				"template_id": tmpl.Id,
			})
		}

		updateProgress(db, task, 30+i*50/total)

		// 阶段 2：generating — 调用 AI API
		updatePhaseAsset(db, phaseAssetID, PhaseGenerating)

		var genResult *provider.GenerateResult
		if len(refURLs) > 0 {
			genResult, err = imgClient.ImageToImage(ctx, provider.ImageToImageReq{
				Model:     task.Model,
				Prompt:    rendered.PositivePrompt,
				ImageURL:  refURLs[0],
				ImageSize: provider.RatioToSize(task.Ratio),
				Strength:  0.85,
			})
		} else {
			genResult, err = imgClient.TextToImage(ctx, provider.TextToImageReq{
				Model:          task.Model,
				Prompt:         rendered.PositivePrompt,
				NegativePrompt: rendered.NegativePrompt,
				ImageSize:      provider.RatioToSize(task.Ratio),
				BatchSize:      1,
			})
		}
		if err != nil {
			saveTypeError(db, task, imageType, fmt.Sprintf("生图失败: %v", err), phaseAssetID)
			if firstErr == nil {
				firstErr = fmt.Errorf("generate image for %s: %w", imageType, err)
			}
			continue
		}

		// 阶段 3：uploading — 上传 OSS
		if len(genResult.Images) == 0 {
			saveTypeError(db, task, imageType, "生图返回空结果", phaseAssetID)
			if firstErr == nil {
				firstErr = fmt.Errorf("generate image for %s returned no images", imageType)
			}
			continue
		}
		updatePhaseAsset(db, phaseAssetID, PhaseUploading)

		typeSucceeded := true
		for _, img := range genResult.Images {
			ossKey, err := ossUploadURL(uploader, img.URL)
			if err != nil {
				saveTypeError(db, task, imageType, fmt.Sprintf("上传失败: %v", err), phaseAssetID)
				if firstErr == nil {
					firstErr = err
				}
				typeSucceeded = false
				break
			}
			// 阶段 4：succeeded — 将占位 asset 升级为真实 asset（含本类型完整 prompt）
			if err := finalizePhaseAsset(db, phaseAssetID, ossKey, cfg.OSSBucket, "image/png",
				img.Width, img.Height, model.JSONMap{
					"seed":            genResult.Seed,
					"timings_ms":      img.TimingsMs,
					"image_type":      imageType,
					"positive_prompt": rendered.PositivePrompt,
					"negative_prompt": rendered.NegativePrompt,
					"template_id":     tmpl.Id,
				}); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("finalize asset: %w", err)
				}
				typeSucceeded = false
				break
			}
		}
		if typeSucceeded {
			succeededCount++
		}
	}

	// 部分成功：有至少一张生成成功，任务视为成功（失败类型已记录在 asset error 中）
	// 全部失败：返回第一个错误，dispatcher 会将任务标记为 failed
	if succeededCount == 0 && firstErr != nil {
		return firstErr
	}

	updateProgress(db, task, 80)

	// 发布进度到 Redis PubSub
	progressEvent, _ := json.Marshal(map[string]interface{}{"progress": 100})
	rdb.Publish(ctx, "aic:progress:"+task.TaskNo, progressEvent)

	return nil
}

// parseAnalysisFromMap 从 task.InputJSON 中恢复 CopywriteAnalysis 结构体
// InputJSON 经过 JSON 序列化后嵌套对象变为 map[string]interface{}，需要二次 marshal/unmarshal 还原
func parseAnalysisFromMap(m map[string]interface{}) *provider.CopywriteAnalysis {
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	var a provider.CopywriteAnalysis
	if err := json.Unmarshal(b, &a); err != nil {
		return nil
	}
	return &a
}

var styleDescMap = map[string]string{
	"default_shoot":     "标准电商商拍，干净明亮，重点突出商品",
	"lifestyle_mag":     "自然光，有氛围感和生活质感",
	"minimal_cold":      "极简留白，高反差，奢侈品质感",
	"energetic_hit":     "高饱和度，大字冲击，活力感强",
	"dark_quality":      "深色系，电影质感，戏剧性打光",
	"asymmetric_layout": "非对称布局，左侧大图突出主体，右侧细节图",
}

func getStyleDesc(recommendedStyle string) string {
	return styleDescMap[recommendedStyle]
}

// fillAnalysisVars 将 CopywriteAnalysis 的字段填入 prompt.Vars
func fillAnalysisVars(vars prompt.Vars, a *provider.CopywriteAnalysis) prompt.Vars {
	if a == nil {
		return vars
	}
	vars.ProductDescForPrompt = a.ProductDescriptionForPrompt
	vars.ProductType = a.ProductType
	vars.GarmentPosition = a.GarmentPosition
	vars.Color = a.Color
	vars.Material = a.Material
	vars.Style = a.Style
	vars.PrintDesign = a.PrintDesign
	vars.PrintDesignLock = a.PrintDesignLock
	vars.TargetAudience = a.TargetAudience
	vars.ProductStyle = a.ProductStyle
	vars.ProductNameZh = a.ProductNameZh

	// 结构化卖点（最多5条）
	for i, sp := range a.SellingPoints {
		switch i {
		case 0:
			vars.SP0Zh, vars.SP0En, vars.SP0ZhDesc, vars.SP0EnDesc = sp.Zh, sp.En, sp.ZhDesc, sp.EnDesc
		case 1:
			vars.SP1Zh, vars.SP1En, vars.SP1ZhDesc, vars.SP1EnDesc = sp.Zh, sp.En, sp.ZhDesc, sp.EnDesc
		case 2:
			vars.SP2Zh, vars.SP2En, vars.SP2ZhDesc, vars.SP2EnDesc = sp.Zh, sp.En, sp.ZhDesc, sp.EnDesc
		case 3:
			vars.SP3Zh, vars.SP3En, vars.SP3ZhDesc, vars.SP3EnDesc = sp.Zh, sp.En, sp.ZhDesc, sp.EnDesc
		case 4:
			vars.SP4Zh, vars.SP4En, vars.SP4ZhDesc, vars.SP4EnDesc = sp.Zh, sp.En, sp.ZhDesc, sp.EnDesc
		}
	}

	// 目标使用场景（最多3条，中英双语）
	zhScene := func(i int) string {
		if i < len(a.TargetScenesZh) {
			return a.TargetScenesZh[i]
		}
		return ""
	}
	enScene := func(i int) string {
		if i < len(a.TargetScenesEn) {
			return a.TargetScenesEn[i]
		}
		return ""
	}
	vars.Scene0Zh, vars.Scene0En = zhScene(0), enScene(0)
	vars.Scene1Zh, vars.Scene1En = zhScene(1), enScene(1)
	vars.Scene2Zh, vars.Scene2En = zhScene(2), enScene(2)

	return vars
}

// resolveAssetURLs 将 asset_no 列表解析为可访问的图片 URL
func resolveAssetURLs(db *gorm.DB, userID uint, assetNos []string, cfg aicommerce.Config, uploader oss.Uploader) []string {
	var assets []model.AiImageAsset
	if err := db.Where("user_id = ? AND asset_no IN ? AND deleted_at IS NULL", userID, assetNos).
		Find(&assets).Error; err != nil {
		return nil
	}

	byNo := make(map[string]model.AiImageAsset, len(assets))
	for _, a := range assets {
		byNo[a.AssetNo] = a
	}

	ttl := int64(cfg.AssetURLTTL)
	if ttl <= 0 {
		ttl = 3600
	}

	urls := make([]string, 0, len(assetNos))
	for _, no := range assetNos {
		asset, ok := byNo[no]
		if !ok {
			continue
		}
		key := strings.TrimSpace(asset.OssKey)
		if key == "" {
			continue
		}
		if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
			signed, err := uploader.SignURL(key, ttl)
			if err != nil {
				continue
			}
			urls = append(urls, signed)
			continue
		}
		bucket := strings.TrimSpace(asset.OssBucket)
		if bucket == "" {
			bucket = strings.TrimSpace(cfg.OSSBucket)
		}
		if bucket == "" {
			continue
		}
		rawURL := fmt.Sprintf("https://%s.oss-cn-hangzhou.aliyuncs.com/%s", bucket, strings.TrimLeft(key, "/"))
		signed, err := uploader.SignURL(rawURL, ttl)
		if err != nil {
			continue
		}
		urls = append(urls, signed)
	}
	return urls
}

// buildVisionClient 从 DB 读取 gpt-4o 模型配置构造视觉代写客户端
func buildVisionClient(db *gorm.DB) (*provider.OpenAIVisionCopywriter, error) {
	var m model.AiModel
	err := db.Where("name = ? AND model_type = ? AND status = ?", "gpt-4o", "chat", "active").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("视觉模型 gpt-4o 未配置")
		}
		return nil, err
	}
	if strings.TrimSpace(m.ApiKey) == "" || strings.TrimSpace(m.ApiEndpoint) == "" {
		return nil, fmt.Errorf("视觉模型 gpt-4o api_key 或 api_endpoint 未配置")
	}
	return provider.NewOpenAIVisionCopywriter(m.ApiEndpoint, m.ApiKey, m.Name), nil
}
