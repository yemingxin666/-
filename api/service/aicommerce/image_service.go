package aicommerce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geekai/service/aicommerce/prompt"
	"geekai/service/aicommerce/provider"
	"geekai/service/oss"
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
	uploader    oss.Uploader
	siliconFlow *provider.SiliconFlow
	tongyi      *provider.Tongyi
	promptRepo  *prompt.Repository
}

func NewImageService(db *gorm.DB, rdb *redis.Client, cfg Config, uploader oss.Uploader) *ImageService {
	return &ImageService{
		db:          db,
		rdb:         rdb,
		cfg:         cfg,
		uploader:    uploader,
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
	ModuleEdit         = "edit"
)

type CopywriteReq struct {
	ProductName string
	Hint        string
	AssetNos    []string
	ImageType   string
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
			if _, err := s.promptRepo.FindTemplate(req.Module, imageType); err != nil {
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
	unitPrice, err := s.promptRepo.GetPriceByModel(modelName)
	if err != nil {
		return nil, err
	}
	// 白底图不走文生图模型，按 rembg 固定单价 × 参考图张数计费（每张扣 rembg 单价）
	// 其他模块按"一次任务一张图"收单张费用
	creditCost := unitPrice
	if req.Module == ModuleWhiteBg {
		rembgPrice, perr := s.promptRepo.GetPriceByModel("rembg")
		if perr != nil || rembgPrice <= 0 {
			rembgPrice = 5 // 兜底：与 migration 初始值一致
		}
		n := len(req.ReferenceAssets)
		if n == 0 {
			return nil, fmt.Errorf("请上传至少 1 张参考图")
		}
		creditCost = rembgPrice * n
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

// SubmitEditTask 提交"基于原图 + prompt 编辑"任务
// 不走模板系统：精简版 SubmitTask，只校验源 task/asset 归属并继承 ratio
func (s *ImageService) SubmitEditTask(ctx context.Context, userID uint, req EditReq) (*model.AiImageTask, error) {
	// 1. 参数清洗
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt 不能为空")
	}
	// prompt 长度上限：限制 rune 数（兼容中英文），防止超大请求 / 异常计费
	const maxPromptRunes = 1000
	if r := []rune(prompt); len(r) > maxPromptRunes {
		return nil, fmt.Errorf("prompt 过长，最多 %d 个字符", maxPromptRunes)
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		return nil, fmt.Errorf("请先选择生图模型")
	}

	// 2. 校验源 task 归属，且必须为已成功的任务（避免编辑失败/进行中任务的脏 asset）
	var srcTask model.AiImageTask
	if err := s.db.Where("task_no = ? AND user_id = ? AND status = ? AND deleted_at IS NULL",
		req.TaskNo, userID, model.TaskStatusSucceeded).
		First(&srcTask).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("原任务不存在或无权访问")
		}
		return nil, err
	}

	// 3. 校验源 asset 归属、属于该 task、且为已生成图片
	var srcAsset model.AiImageAsset
	if err := s.db.Where("asset_no = ? AND user_id = ? AND kind = ? AND deleted_at IS NULL",
		req.AssetNo, userID, model.AssetKindGenerated).First(&srcAsset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("原图片不存在或无权访问")
		}
		return nil, err
	}
	if srcAsset.TaskId == nil || *srcAsset.TaskId != srcTask.Id {
		return nil, fmt.Errorf("原图片与原任务不匹配")
	}

	// 4. 计费
	creditCost, err := s.promptRepo.GetPriceByModel(modelName)
	if err != nil {
		return nil, fmt.Errorf("模型不可用: %w", err)
	}
	if err := s.deductCredit(ctx, userID, creditCost); err != nil {
		return nil, fmt.Errorf("积分不足: %w", err)
	}

	// 5. 序列化 InputJSON：只保留编辑必需字段，便于 worker 取用
	inputBytes, _ := json.Marshal(map[string]interface{}{
		"prompt":           prompt,
		"source_task_no":   srcTask.TaskNo,
		"source_asset_no":  srcAsset.AssetNo,
		"origin_ratio":     srcTask.Ratio,
		"model":            modelName,
	})
	var inputJSON model.JSONMap
	_ = json.Unmarshal(inputBytes, &inputJSON)

	// 6. 创建任务（继承原图 ratio，模块标记为 edit）
	taskNo := generateTaskNo()
	ratio := srcTask.Ratio
	if ratio == "" {
		ratio = "1:1"
	}
	task := &model.AiImageTask{
		TaskNo:     taskNo,
		UserId:     userID,
		Module:     ModuleEdit,
		Ratio:      ratio,
		InputJSON:  inputJSON,
		Status:     model.TaskStatusPending,
		Model:      modelName,
		CreditCost: creditCost,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.db.Create(task).Error; err != nil {
		_ = s.refundCredit(ctx, userID, creditCost)
		return nil, err
	}

	// 7. 入队
	if err := s.enqueue(ctx, task.Id, taskNo); err != nil {
		_ = s.refundCredit(ctx, userID, creditCost)
		s.db.Model(task).Update("status", model.TaskStatusFailed)
		return nil, err
	}
	// 入队成功后，task 状态必须从 pending 切到 queued，否则 dispatcher 的 CAS
	// (queued→running) 会跳过本任务，造成扣费后永久卡住的孤儿任务
	if err := s.db.Model(task).Update("status", model.TaskStatusQueued).Error; err != nil {
		_ = s.refundCredit(ctx, userID, creditCost)
		s.db.Model(task).Update("status", model.TaskStatusFailed)
		return nil, fmt.Errorf("更新任务状态失败: %w", err)
	}
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
	AssetNo   string  `json:"asset_no,omitempty"` // 编辑功能需要：仅当资产已生成（OssKey != ""）时填充
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
	s.signTaskResult(outputs, items)
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
		// clone 等无 image_type 的模块：基于 assets 合成 items，让前端能拿到 asset_no
		items := buildSyntheticItems(task, assets)
		return items, outputs, task.Progress
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
				item.AssetNo = a.AssetNo
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

// buildSyntheticItems 为无 image_type 的模块（如 clone）按 assets 合成 items。
// 让前端能基于 item.asset_no 触发编辑功能，同时保持与 typed item 接口一致。
func buildSyntheticItems(task *model.AiImageTask, assets []model.AiImageAsset) []ImageTaskItem {
	items := make([]ImageTaskItem, 0, len(assets))
	idx := 0
	for _, a := range assets {
		if a.OssKey == "" {
			continue
		}
		url := a.OssKey
		items = append(items, ImageTaskItem{
			ImageType: fmt.Sprintf("%s_%d", task.Module, idx),
			Label:     fmt.Sprintf("%s %d", moduleLabel(task.Module), idx+1),
			Status:    "succeeded",
			Phase:     "succeeded",
			Progress:  100,
			URL:       &url,
			AssetNo:   a.AssetNo,
		})
		idx++
	}
	return items
}

// moduleLabel 模块中文名（与前端 moduleMap 保持一致）
func moduleLabel(m string) string {
	labels := map[string]string{
		"main_image": "主图设计", "detail_page": "详情页", "white_bg": "白底图",
		"clone": "克隆设计", "ratio_convert": "比例转换", "translate": "图文翻译", "edit": "图片编辑",
	}
	if l, ok := labels[m]; ok {
		return l
	}
	return m
}

// GalleryTask 历史图库列表项（含输出图片 URL + asset_no）
type GalleryTask struct {
	model.AiImageTask
	Outputs []OutputItem `json:"outputs"`
}

// OutputItem 历史图库单张输出图（携带 asset_no 以支持编辑功能）
type OutputItem struct {
	Url     string `json:"url"`
	AssetNo string `json:"asset_no"`
}

// EditReq 图片编辑请求（基于原图 + prompt 重新生成）
type EditReq struct {
	TaskNo  string `json:"task_no" binding:"required"`
	AssetNo string `json:"asset_no" binding:"required"`
	Prompt  string `json:"prompt" binding:"required"`
	Model   string `json:"model" binding:"required"` // 必填，前端传入用户选中的模型
}

// ListGallery 历史图库分页查询
// 同时返回成功 + 进行中（queued/running）任务，让用户提交后立即看到进度条目；
// failed 任务不展示，避免列表中堆积失败记录干扰浏览
func (s *ImageService) ListGallery(ctx context.Context, userID uint, module string, page, pageSize int) ([]GalleryTask, int64, error) {
	visibleStatuses := []string{model.TaskStatusSucceeded, model.TaskStatusQueued, model.TaskStatusRunning, model.TaskStatusPending}
	query := s.db.Model(&model.AiImageTask{}).
		Where("user_id = ? AND status IN ? AND deleted_at IS NULL", userID, visibleStatuses)
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

	// 按 task_id 分组：保留 asset_no + oss_key，便于后续编辑功能精确定位单图
	type rawOut struct {
		AssetNo string
		OssKey  string
	}
	rawByTaskID := make(map[uint][]rawOut, len(tasks))
	for _, a := range assets {
		if a.TaskId != nil {
			rawByTaskID[*a.TaskId] = append(rawByTaskID[*a.TaskId], rawOut{AssetNo: a.AssetNo, OssKey: a.OssKey})
		}
	}

	result := make([]GalleryTask, len(tasks))
	for i, t := range tasks {
		raws := rawByTaskID[t.Id]
		outputs := make([]OutputItem, 0, len(raws))
		for _, r := range raws {
			outputs = append(outputs, OutputItem{Url: s.signSingleURL(r.OssKey), AssetNo: r.AssetNo})
		}
		result[i] = GalleryTask{
			AiImageTask: t,
			Outputs:     outputs,
		}
	}
	return result, total, nil
}

// signTaskResult 对任务结果中的所有图片 URL 进行签名
func (s *ImageService) signTaskResult(outputs []string, items []ImageTaskItem) {
	s.signURLs(outputs)
	for i := range items {
		if items[i].URL != nil && *items[i].URL != "" {
			signed := s.signSingleURL(*items[i].URL)
			items[i].URL = &signed
		}
	}
}

// signURLs 原地批量签名 URL 切片
func (s *ImageService) signURLs(urls []string) {
	for i, u := range urls {
		urls[i] = s.signSingleURL(u)
	}
}

// signSingleURL 对单个 URL 签名；不支持签名的实现会直接返回原 URL
func (s *ImageService) signSingleURL(u string) string {
	if u == "" || s.uploader == nil {
		return u
	}
	signed, err := s.uploader.SignURL(u, int64(s.assetURLTTL()))
	if err != nil {
		return u
	}
	return signed
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
	content, analysis, err = client.GenerateCopywrite(ctx, req.ProductName, req.Hint, imageURLs, req.ImageType)
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
			signed, err := s.uploader.SignURL(key, int64(s.assetURLTTL()))
			if err != nil {
				return nil, fmt.Errorf("参考图签名失败 %s: %w", assetNo, err)
			}
			urls = append(urls, signed)
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

func (s *ImageService) assetURLTTL() int {
	if s.cfg.AssetURLTTL > 0 {
		return s.cfg.AssetURLTTL
	}
	return 3600
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
	// 空输入返回空切片；buildTaskItems 依赖 len==0 走 buildSyntheticItems 分支
	// （white_bg / clone 等模块没有 image_type，需要按生成资产合成条目）。
	// 原来这里有 `return []string{imageType}` 的兜底，会把空字符串包成 [""]，
	// 导致 white_bg 任务成功后 items 里出现一条假 pending 卡片残留。
	return result
}
