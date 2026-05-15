package chains

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logger2 "geekai/logger"
	"geekai/service/aicommerce"
	"geekai/service/aicommerce/prompt"
	"geekai/service/aicommerce/provider"
	"geekai/service/oss"
	"geekai/store/model"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// imageTypeTimeout 单个图片类型（= 1 张输出图）的生图调用超时预算。
// 主图/详情页链路每个 image_type 对应 1 次 ImageToImage/TextToImage 调用，
// 与克隆设计风格图保持一致：每张图 90s。
const imageTypeTimeout = 5 * time.Minute

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

		visionClients, visionErr := buildVisionClients(db)
		if visionErr == nil {
			var content string
			var analysis *provider.CopywriteAnalysis
			var lastVisionErr error
			for i, vc := range visionClients {
				visionCtx, visionCancel := context.WithTimeout(ctx, 30*time.Second)
				content, analysis, lastVisionErr = vc.GenerateCopywrite(visionCtx, productName, sellingPoints, imageURLs, task.ImageType)
				visionCancel()
				if lastVisionErr == nil {
					if i > 0 {
						logger2.GetLogger().Infof("task %d vision copywrite failover succeeded on endpoint %d", task.Id, i+1)
					}
					break
				}
				if errors.Is(lastVisionErr, context.Canceled) {
					break
				}
				logger2.GetLogger().Warnf("task %d vision copywrite endpoint %d failed: %v", task.Id, i+1, lastVisionErr)
			}
			if lastVisionErr != nil {
				logger2.GetLogger().Warnf("task %d vision copywrite all endpoints failed (non-blocking): %v", task.Id, lastVisionErr)
			} else {
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
		} else {
			logger2.GetLogger().Warnf("task %d build vision client failed (non-blocking): %v", task.Id, visionErr)
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

		// 按图片类型分配超时：每个 image_type → 1 张输出图 → 90s
		callCtx, cancel := context.WithTimeout(ctx, imageTypeTimeout)

		var genResult *provider.GenerateResult
		if len(refURLs) > 0 {
			genResult, err = imgClient.ImageToImage(callCtx, provider.ImageToImageReq{
				Model:     task.Model,
				Prompt:    rendered.PositivePrompt,
				ImageURL:  refURLs[0],
				ImageSize: provider.RatioToSize(task.Ratio),
				Strength:  0.85,
			})
		} else {
			genResult, err = imgClient.TextToImage(callCtx, provider.TextToImageReq{
				Model:          task.Model,
				Prompt:         rendered.PositivePrompt,
				NegativePrompt: rendered.NegativePrompt,
				ImageSize:      provider.RatioToSize(task.Ratio),
				BatchSize:      1,
			})
		}
		cancel()
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

	if succeededCount == 0 && firstErr != nil {
		return firstErr
	}

	if succeededCount > 0 && succeededCount < total {
		unitPrice, quantity := extractBillingSnapshot(task)
		if unitPrice > 0 && quantity == total && unitPrice*quantity == task.CreditCost {
			if err := refundPartialMainImage(db, task, total, succeededCount, unitPrice); err != nil {
				logger2.GetLogger().Errorf(
					"[main_image] task=%d user=%d partial refund failed: total=%d succeeded=%d unit=%d err=%v",
					task.Id, task.UserId, total, succeededCount, unitPrice, err)
				_ = db.Model(task).Update("error_message",
					fmt.Sprintf("partial refund failed: %v", err)).Error
			}
		} else if unitPrice > 0 {
			logger2.GetLogger().Warnf(
				"[main_image] task=%d billing snapshot mismatch: unit=%d qty=%d total_types=%d credit_cost=%d, skip partial refund",
				task.Id, unitPrice, quantity, total, task.CreditCost)
		}
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

	// 尺码表 JSON 注入（仅 size_capacity 类型 vision 才会返回非空）
	if jsonStr, ok := provider.NormalizeSizeChartJSON(a.SizeChart); ok {
		vars.SizeChartJSON = jsonStr
		vars.HasSizeChart = true
	} else {
		vars.SizeChartJSON = ""
		vars.HasSizeChart = false
	}

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

// buildVisionClients 从 DB 读取 chat 模型配置构造视觉代写客户端列表（支持 failover）
// 优先查找 gpt-4o，如果不存在则使用任意可用的 chat 模型（按 sort_order 排序）
func buildVisionClients(db *gorm.DB) ([]*provider.OpenAIVisionCopywriter, error) {
	var m model.AiModel
	err := db.Where("name = ? AND model_type = ? AND status = ?", "gpt-4o", "chat", "active").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = db.Where("model_type = ? AND status = ?", "chat", "active").
				Order("sort_order ASC, id ASC").
				First(&m).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, fmt.Errorf("未找到可用的视觉识别模型（model_type='chat', status='active'）")
				}
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	endpoints := m.GetEndpoints()
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("视觉模型 %s 无可用的 API 端点配置", m.Name)
	}
	clients := make([]*provider.OpenAIVisionCopywriter, 0, len(endpoints))
	for _, ep := range endpoints {
		if strings.TrimSpace(ep.ApiEndpoint) == "" {
			continue
		}
		modelName := ep.ModelName
		if modelName == "" {
			modelName = m.Name
		}
		clients = append(clients, provider.NewOpenAIVisionCopywriter(ep.ApiEndpoint, ep.ApiKey, modelName))
	}
	if len(clients) == 0 {
		return nil, fmt.Errorf("视觉模型 %s api_endpoint 未配置", m.Name)
	}
	return clients, nil
}

func extractBillingSnapshot(task *model.AiImageTask) (unitPrice, quantity int) {
	billing, ok := task.InputJSON["_billing"].(map[string]interface{})
	if !ok {
		return 0, 0
	}
	if v, ok := billing["unit_price"].(float64); ok && v > 0 {
		unitPrice = int(v)
	}
	if v, ok := billing["quantity"].(float64); ok && v > 0 {
		quantity = int(v)
	}
	return
}

func refundPartialMainImage(db *gorm.DB, task *model.AiImageTask, total, succeeded, unitCost int) error {
	if task == nil || task.Id == 0 || total <= 0 || succeeded <= 0 || succeeded >= total || unitCost <= 0 {
		return nil
	}
	failed := total - succeeded
	refund := failed * unitCost
	finalCost := succeeded * unitCost
	expectedCost := total * unitCost
	return db.Transaction(func(tx *gorm.DB) error {
		taskResult := tx.Model(&model.AiImageTask{}).
			Where("id = ? AND credit_cost = ?", task.Id, expectedCost).
			Update("credit_cost", finalCost)
		if taskResult.Error != nil {
			return fmt.Errorf("update main_image credit_cost: %w", taskResult.Error)
		}
		if taskResult.RowsAffected == 0 {
			return fmt.Errorf("credit_cost already changed (expected %d), skip refund", expectedCost)
		}
		userResult := tx.Model(&model.User{}).
			Where("id = ?", task.UserId).
			UpdateColumn("power", gorm.Expr("power + ?", refund))
		if userResult.Error != nil {
			return fmt.Errorf("refund main_image credits: %w", userResult.Error)
		}
		if userResult.RowsAffected == 0 {
			return fmt.Errorf("user %d not found, cannot refund", task.UserId)
		}
		task.CreditCost = finalCost
		return nil
	})
}
