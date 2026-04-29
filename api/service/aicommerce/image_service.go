package aicommerce

import (
	"context"
	"encoding/json"
	"fmt"
	"geekai/service/aicommerce/prompt"
	"geekai/service/aicommerce/provider"
	"geekai/store/model"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type ImageService struct {
	db         *gorm.DB
	rdb        *redis.Client
	cfg        Config
	siliconFlow *provider.SiliconFlow
	tongyi     *provider.Tongyi
	promptRepo *prompt.Repository
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
func (s *ImageService) Copywrite(ctx context.Context, productName, hint string) (string, error) {
	return s.tongyi.GenerateCopywrite(ctx, productName, hint)
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
