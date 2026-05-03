package aicommerce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geekai/service/aicommerce/prompt"
	"geekai/service/aicommerce/provider"
	"geekai/store/model"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type ImageService struct {
	db          *gorm.DB
	rdb         *redis.Client
	cfg         Config
	siliconFlow *provider.SiliconFlow
	tongyi      *provider.Tongyi
	promptRepo  *prompt.Repository
}

func NewImageService(db *gorm.DB, rdb *redis.Client, cfg Config) *ImageService {
	return &ImageService{
		db:          db,
		rdb:         rdb,
		cfg:         cfg,
		siliconFlow: provider.NewSiliconFlow(cfg.SiliconFlowBaseURL, cfg.SiliconFlowAPIKey),
		tongyi:      provider.NewTongyi(cfg.TongyiBaseURL, cfg.TongyiAPIKey, cfg.TongyiModel),
		promptRepo:  prompt.NewRepository(db),
	}
}

// 模块常量（供 Handler 使用）
const (
	ModuleMainImage    = "main_image"
	ModuleDetailPage   = "detail_page"
	ModuleWhiteBg      = "white_bg"
	ModuleClone        = "clone"
	ModuleRatioConvert = "ratio_convert"
	ModuleTranslate    = "translate"
)

type CopywriteReq struct {
	ProductName string
	Hint        string
	AssetNos    []string
}

const maxCopywriteImageCount = 3

// GenerateReq 生图请求（所有模块共用）
type GenerateReq struct {
	Module          string   `json:"module"`
	ProductName     string   `json:"product_name"`
	SellingPoints   string   `json:"selling_points"`
	ImageType       string   `json:"image_type"`
	Platform        string   `json:"platform"`
	Language        string   `json:"language"`
	Ratio           string   `json:"ratio"`
	StyleDesc       string   `json:"style_desc"`
	ReferenceAssets []string `json:"reference_assets"` // asset_no 列表
	Model           string   `json:"model"`            // 指定模型，为空用默认
}

// SubmitTask 提交生图任务
func (s *ImageService) SubmitTask(ctx context.Context, userID uint, req GenerateReq) (*model.AiImageTask, error) {
	// 0. 预检：主图/详情页模块同步校验模板存在性，避免扣费后异步失败
	//    同时规范化 image_type，确保预检值与 worker 实际查询值一致
	if req.Module == ModuleMainImage || req.Module == ModuleDetailPage {
		var validTypes []string
		for _, raw := range strings.Split(req.ImageType, ",") {
			t := strings.TrimSpace(raw)
			if t != "" {
				validTypes = append(validTypes, t)
			}
		}
		if len(validTypes) == 0 {
			return nil, fmt.Errorf("图片类型不能为空")
		}
		for _, imageType := range validTypes {
			if _, err := s.promptRepo.FindTemplate(req.Module, imageType, req.Platform, req.Language, req.Ratio); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, fmt.Errorf("图片类型「%s」暂无生成模板，请联系管理员配置", imageType)
				}
				return nil, fmt.Errorf("模板查询失败，请稍后重试")
			}
		}
		// 写回规范化后的值，确保与 worker 查询一致
		req.ImageType = strings.Join(validTypes, ",")
	}

	// 1. 确定模型和积分
	modelName := req.Model
	if modelName == "" {
		modelName = s.cfg.SiliconFlowModel
	}
	creditCost, err := s.promptRepo.GetPriceByModel(modelName)
	if err != nil {
		return nil, err
	}

	// 2. 扣减算力（调用 GeeKAI 现有机制）
	if err := s.deductCredit(ctx, userID, creditCost); err != nil {
		return nil, fmt.Errorf("积分不足: %w", err)
	}

	// 3. 序列化请求参数
	inputBytes, _ := json.Marshal(req)
	var inputJSON model.JSONMap
	_ = json.Unmarshal(inputBytes, &inputJSON)

	// 4. 创建任务记录
	taskNo := generateTaskNo()
	task := &model.AiImageTask{
		TaskNo:     taskNo,
		UserId:     userID,
		Module:     req.Module,
		ImageType:  req.ImageType,
		Platform:   req.Platform,
		Language:   req.Language,
		Ratio:      req.Ratio,
		InputJSON:  inputJSON,
		Status:     model.TaskStatusPending,
		Model:      modelName,
		CreditCost: creditCost,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.db.Create(task).Error; err != nil {
		// 创建失败退还算力
		_ = s.refundCredit(ctx, userID, creditCost)
		return nil, err
	}

	// 5. 入队
	if err := s.enqueue(ctx, task.Id, taskNo); err != nil {
		// 入队失败退还算力并更新状态
		_ = s.refundCredit(ctx, userID, creditCost)
		s.db.Model(task).Update("status", model.TaskStatusFailed)
		return nil, err
	}
	s.db.Model(task).Update("status", model.TaskStatusQueued)
	task.Status = model.TaskStatusQueued
	return task, nil
}

// ImageTaskItem 单个图片类型的生成状态
type ImageTaskItem struct {
	ImageType string  `json:"image_type"`
	Label     string  `json:"label"`
	Status    string  `json:"status"` // pending | running | succeeded | failed
	Phase     string  `json:"phase"`  // rendering | generating | uploading | succeeded | failed
	Progress  int     `json:"progress"`
	URL       *string `json:"url"`
}

// ImageTaskResult GetTask 完整响应
type ImageTaskResult struct {
	Task     *model.AiImageTask
	Outputs  []string
	Items    []ImageTaskItem
	Progress int
}

// GetTask 查询任务状态（含结构化 items）
func (s *ImageService) GetTask(ctx context.Context, userID uint, taskNo string) (*ImageTaskResult, error) {
	var task model.AiImageTask
	if err := s.db.Where("task_no = ? AND user_id = ? AND deleted_at IS NULL", taskNo, userID).First(&task).Error; err != nil {
		return nil, err
	}
	var assets []model.AiImageAsset
	if err := s.db.Where("task_id = ? AND kind = ? AND deleted_at IS NULL", task.Id, model.AssetKindGenerated).
		Order("created_at ASC, id ASC").Find(&assets).Error; err != nil {
		return nil, err
	}
	items, outputs, progress := buildTaskItems(&task, assets)
	return &ImageTaskResult{Task: &task, Outputs: outputs, Items: items, Progress: progress}, nil
}

// phaseToProgress 将 phase 字段映射为进度百分比
func phaseToProgress(phase string) int {
	switch phase {
	case "rendering":
		return 20
	case "generating":
		return 50
	case "uploading":
		return 85
	case "succeeded":
		return 100
	default:
		return 0
	}
}

// imageTypeLabel 返回图片类型的中文标签
func imageTypeLabel(t string) string {
	labels := map[string]string{
		"traffic_cover": "引流封面", "core_selling_point": "核心卖点",
		"scene_immersion": "场景代入", "value_breakdown": "价值拆解",
		"competitor_comparison": "竞品对比", "detail_display": "细节展示",
		"effect_proof": "效果证明", "trust_building": "信任消疑", "final_push": "临门一脚",
		"hero_visual": "首屏主视觉", "core_selling": "核心卖点图",
		"usage_scene": "使用场景图", "multi_angle": "多视角图", "atmosphere": "场景氛围图",
		"product_detail": "商品细节图", "brand_story": "品牌故事图",
		"size_capacity": "尺寸容量尺码图", "effect_comparison": "效果对比图",
		"spec_reference": "详细规格参考图", "craft_process": "工艺制作图",
		"accessory_gift": "配件赠品图", "series_showcase": "系列展示图",
		"ingredient": "商品成分图", "after_sales": "售后保障图", "usage_guide": "使用建议图",
	}
	if l, ok := labels[t]; ok {
		return l
	}
	return t
}

// buildTaskItems 从 task + assets 推导每个类型的状态和进度
func buildTaskItems(task *model.AiImageTask, assets []model.AiImageAsset) ([]ImageTaskItem, []string, int) {
	// outputs 保持向后兼容（只含真实图片，跳过占位和 error asset）
	outputs := make([]string, 0, len(assets))
	for _, a := range assets {
		if a.OssKey != "" {
			outputs = append(outputs, a.OssKey)
		}
	}

	// assetByType: 每个 image_type 取最新一条 asset（包含占位和成功 asset）
	assetByType := make(map[string]*model.AiImageAsset, len(assets))
	for i := range assets {
		t, _ := assets[i].MetadataJSON["image_type"].(string)
		if t == "" {
			continue
		}
		// 后插入的覆盖先插入的（ORDER BY created_at ASC，所以最后一条是最新状态）
		assetByType[t] = &assets[i]
	}

	requestedTypes := splitImageTypes(task.ImageType)
	total := len(requestedTypes)
	if total == 0 {
		return nil, outputs, task.Progress
	}

	items := make([]ImageTaskItem, 0, total)
	succeededCount := 0
	hasRunning := false

	for _, t := range requestedTypes {
		item := ImageTaskItem{
			ImageType: t,
			Label:     imageTypeLabel(t),
		}
		if a, ok := assetByType[t]; ok {
			phase, _ := a.MetadataJSON["phase"].(string)
			item.Phase = phase
			item.Progress = phaseToProgress(phase)
			if a.OssKey != "" {
				// 真实图片已生成
				url := a.OssKey
				item.Status = "succeeded"
				item.URL = &url
				succeededCount++
			} else if phase == "failed" {
				item.Status = "failed"
			} else {
				// 占位 asset 存在，说明该类型正在某个阶段处理中
				item.Status = "running"
				hasRunning = true
			}
		} else if task.Status == string(model.TaskStatusRunning) && !hasRunning {
			// 还没轮到这个类型（worker 还在处理前面的类型）
			item.Status = "running"
			item.Phase = "pending"
			item.Progress = 0
			hasRunning = true
		} else {
			item.Status = "pending"
			item.Progress = 0
		}
		items = append(items, item)
	}

	progress := succeededCount * 100 / total
	return items, outputs, progress
}

// GalleryTask 历史图库列表项（含输出图片 URL）
type GalleryTask struct {
	model.AiImageTask
	Outputs []string `json:"outputs"`
}

// ListGallery 历史图库分页查询
func (s *ImageService) ListGallery(ctx context.Context, userID uint, module string, page, pageSize int) ([]GalleryTask, int64, error) {
	query := s.db.Model(&model.AiImageTask{}).
		Where("user_id = ? AND status = ? AND deleted_at IS NULL", userID, model.TaskStatusSucceeded)
	if module != "" {
		query = query.Where("module = ?", module)
	}
	var total int64
	query.Count(&total)

	var tasks []model.AiImageTask
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	if len(tasks) == 0 {
		return nil, total, nil
	}

	// 批量查出所有 task 的已生成 assets
	taskIDs := make([]uint, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.Id
	}
	var assets []model.AiImageAsset
	if err := s.db.Where("task_id IN ? AND kind = ? AND oss_key != '' AND deleted_at IS NULL",
		taskIDs, model.AssetKindGenerated).
		Order("task_id ASC, created_at ASC").Find(&assets).Error; err != nil {
		return nil, 0, err
	}

	// 按 task_id 分组
	urlsByTaskID := make(map[uint][]string, len(tasks))
	for _, a := range assets {
		if a.TaskId != nil {
			urlsByTaskID[*a.TaskId] = append(urlsByTaskID[*a.TaskId], a.OssKey)
		}
	}

	result := make([]GalleryTask, len(tasks))
	for i, t := range tasks {
		result[i] = GalleryTask{
			AiImageTask: t,
			Outputs:     urlsByTaskID[t.Id],
		}
		if result[i].Outputs == nil {
			result[i].Outputs = []string{}
		}
	}
	return result, total, nil
}

// Copywrite AI 代写卖点
func (s *ImageService) Copywrite(ctx context.Context, userID uint, req CopywriteReq) (content string, analysis *provider.CopywriteAnalysis, err error) {
	assetNos := deduplicateAssetNos(req.AssetNos)
	if len(assetNos) == 0 {
		return "", nil, fmt.Errorf("请先上传参考图，AI代写需要分析商品图片")
	}
	if len(assetNos) > maxCopywriteImageCount {
		return "", nil, fmt.Errorf("参考图最多支持3张")
	}

	creditCost, priceErr := s.promptRepo.GetPriceByModel("vision-copywrite")
	if priceErr != nil || creditCost <= 0 {
		creditCost = 8
	}
	if err = s.deductCredit(ctx, userID, creditCost); err != nil {
		return "", nil, fmt.Errorf("积分不足: %w", err)
	}

	defer func() {
		if err == nil {
			return
		}
		if refundErr := s.refundCredit(context.Background(), userID, creditCost); refundErr != nil {
			err = fmt.Errorf("%w; 积分退回失败，请联系客服: %v", err, refundErr)
		}
	}()

	imageURLs, err := s.resolveCopywriteImageURLs(ctx, userID, assetNos)
	if err != nil {
		return "", nil, err
	}

	visionModel, err := s.resolveCopywriteVisionModel(ctx)
	if err != nil {
		return "", nil, err
	}

	client := provider.NewOpenAIVisionCopywriter(visionModel.ApiEndpoint, visionModel.ApiKey, visionModel.Name)
	content, analysis, err = client.GenerateCopywrite(ctx, req.ProductName, req.Hint, imageURLs)
	if err != nil {
		return "", nil, fmt.Errorf("视觉代写失败: %w", err)
	}

	return content, analysis, nil
}

func deduplicateAssetNos(assetNos []string) []string {
	seen := make(map[string]struct{}, len(assetNos))
	result := make([]string, 0, len(assetNos))
	for _, assetNo := range assetNos {
		assetNo = strings.TrimSpace(assetNo)
		if assetNo == "" {
			continue
		}
		if _, ok := seen[assetNo]; ok {
			continue
		}
		seen[assetNo] = struct{}{}
		result = append(result, assetNo)
	}
	return result
}

func (s *ImageService) resolveCopywriteImageURLs(ctx context.Context, userID uint, assetNos []string) ([]string, error) {
	urls := make([]string, 0, len(assetNos))

	var dbAssetNos []string
	for _, assetNo := range assetNos {
		// Base64 data URL 直接使用，无需查 DB
		if strings.HasPrefix(assetNo, "data:") {
			urls = append(urls, assetNo)
		} else {
			dbAssetNos = append(dbAssetNos, assetNo)
		}
	}

	if len(dbAssetNos) == 0 {
		return urls, nil
	}

	var assets []model.AiImageAsset
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND asset_no IN ? AND deleted_at IS NULL", userID, dbAssetNos).
		Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("查询参考图失败: %w", err)
	}

	assetByNo := make(map[string]model.AiImageAsset, len(assets))
	for _, asset := range assets {
		assetByNo[asset.AssetNo] = asset
	}

	for _, assetNo := range dbAssetNos {
		asset, ok := assetByNo[assetNo]
		if !ok {
			return nil, fmt.Errorf("参考图不存在或无权访问: %s", assetNo)
		}

		key := strings.TrimSpace(asset.OssKey)
		if key == "" {
			return nil, fmt.Errorf("参考图 OSS Key 为空: %s", assetNo)
		}

		if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
			urls = append(urls, key)
			continue
		}

		bucket := strings.TrimSpace(asset.OssBucket)
		if bucket == "" {
			bucket = strings.TrimSpace(s.cfg.OSSBucket)
		}
		if bucket == "" {
			return nil, fmt.Errorf("参考图 OSS bucket 未配置")
		}
		urls = append(urls, fmt.Sprintf("https://%s.oss-cn-hangzhou.aliyuncs.com/%s", bucket, strings.TrimLeft(key, "/")))
	}

	return urls, nil
}

func (s *ImageService) resolveCopywriteVisionModel(ctx context.Context) (*model.AiModel, error) {
	var m model.AiModel
	err := s.db.WithContext(ctx).
		Where("name = ? AND model_type = ? AND status = ?", "gpt-4o", "chat", "active").
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未找到可用的视觉模型配置(gpt-4o chat)，请在后台 geekai_ai_models 中配置")
		}
		return nil, err
	}
	if strings.TrimSpace(m.ApiKey) == "" {
		return nil, fmt.Errorf("视觉模型 gpt-4o 的 api_key 未配置")
	}
	if strings.TrimSpace(m.ApiEndpoint) == "" {
		return nil, fmt.Errorf("视觉模型 gpt-4o 的 api_endpoint 未配置")
	}
	return &m, nil
}

func (s *ImageService) enqueue(ctx context.Context, taskID uint, taskNo string) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"task_id": taskID,
		"task_no": taskNo,
	})
	return s.rdb.LPush(ctx, s.cfg.QueueName, payload).Err()
}

func (s *ImageService) deductCredit(ctx context.Context, userID uint, amount int) error {
	result := s.db.Model(&model.User{}).
		Where("id = ? AND power >= ?", userID, amount).
		UpdateColumn("power", gorm.Expr("power - ?", amount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("算力不足")
	}
	return nil
}

func (s *ImageService) refundCredit(_ context.Context, userID uint, amount int) error {
	return s.db.Model(&model.User{}).Where("id = ?", userID).
		UpdateColumn("power", gorm.Expr("power + ?", amount)).Error
}

func generateTaskNo() string {
	// 简单实现，生产环境替换为雪花ID
	return fmt.Sprintf("aic_%d", time.Now().UnixNano())
}

// splitImageTypes 拆分逗号分隔的图片类型（与 worker/chains 同逻辑，不跨包引用）
func splitImageTypes(imageType string) []string {
	parts := strings.Split(imageType, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	if len(result) == 0 {
		return []string{imageType}
	}
	return result
}
