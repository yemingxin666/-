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

// GetTask 查询任务状态（含资产列表）
func (s *ImageService) GetTask(ctx context.Context, userID uint, taskNo string) (*model.AiImageTask, []model.AiImageAsset, error) {
	var task model.AiImageTask
	if err := s.db.Where("task_no = ? AND user_id = ? AND deleted_at IS NULL", taskNo, userID).First(&task).Error; err != nil {
		return nil, nil, err
	}
	var assets []model.AiImageAsset
	s.db.Where("task_id = ? AND kind = ? AND deleted_at IS NULL", task.Id, model.AssetKindGenerated).
		Order("created_at").Find(&assets)
	return &task, assets, nil
}

// ListGallery 历史图库分页查询
func (s *ImageService) ListGallery(ctx context.Context, userID uint, module string, page, pageSize int) ([]model.AiImageTask, int64, error) {
	query := s.db.Model(&model.AiImageTask{}).
		Where("user_id = ? AND status = ? AND deleted_at IS NULL", userID, model.TaskStatusSucceeded)
	if module != "" {
		query = query.Where("module = ?", module)
	}
	var total int64
	query.Count(&total)

	var tasks []model.AiImageTask
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error
	return tasks, total, err
}

// Copywrite AI 代写卖点
func (s *ImageService) Copywrite(ctx context.Context, userID uint, req CopywriteReq) (content string, err error) {
	assetNos := deduplicateAssetNos(req.AssetNos)
	if len(assetNos) == 0 {
		return "", fmt.Errorf("请先上传参考图，AI代写需要分析商品图片")
	}
	if len(assetNos) > maxCopywriteImageCount {
		return "", fmt.Errorf("参考图最多支持3张")
	}

	creditCost, priceErr := s.promptRepo.GetPriceByModel("vision-copywrite")
	if priceErr != nil || creditCost <= 0 {
		creditCost = 8
	}
	if err = s.deductCredit(ctx, userID, creditCost); err != nil {
		return "", fmt.Errorf("积分不足: %w", err)
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
		return "", err
	}

	visionModel, err := s.resolveCopywriteVisionModel(ctx)
	if err != nil {
		return "", err
	}

	client := provider.NewOpenAIVisionCopywriter(visionModel.ApiEndpoint, visionModel.ApiKey, visionModel.Name)
	content, err = client.GenerateCopywrite(ctx, req.ProductName, req.Hint, imageURLs)
	if err != nil {
		return "", fmt.Errorf("视觉代写失败: %w", err)
	}

	return content, nil
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
