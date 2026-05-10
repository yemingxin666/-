package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geekai/service/aicommerce"
	"geekai/service/aicommerce/provider"
	"geekai/service/aicommerce/worker/chains"
	"geekai/service/oss"
	"geekai/store/model"
	"strings"
	"time"

	logger2 "geekai/logger"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

var logger = logger2.GetLogger()

type Dispatcher struct {
	db            *gorm.DB
	rdb           *redis.Client
	cfg           aicommerce.Config
	uploadManager *oss.UploaderManager
	tongyi        *provider.Tongyi
	vision        *provider.AliyunVision
	baiduOCR      *provider.BaiduOCR
	baiduTrans    *provider.BaiduTranslate
}

func NewDispatcher(db *gorm.DB, rdb *redis.Client, cfg aicommerce.Config, uploadManager *oss.UploaderManager) *Dispatcher {
	vision := provider.NewAliyunVision(cfg.AliyunVisionAccessKeyID, cfg.AliyunVisionAccessKeySecret, cfg.AliyunVisionRegion).
		WithRelay(provider.RelayConfig{
			Enabled:      cfg.AliyunVisionRelayEnabled,
			Endpoint:     cfg.AliyunVisionRelayEndpoint,
			AccessKey:    cfg.AliyunVisionRelayAccessKey,
			AccessSecret: cfg.AliyunVisionRelayAccessSecret,
			Bucket:       cfg.AliyunVisionRelayBucket,
			Prefix:       cfg.AliyunVisionRelayPrefix,
		})
	// 启动时打印 relay 状态，方便排查"配置没生效"这类问题
	if cfg.AliyunVisionRelayEnabled {
		if vision.RelayActive() {
			logger.Infof("[aicommerce] vision relay ACTIVE: bucket=%s endpoint=%s prefix=%s",
				cfg.AliyunVisionRelayBucket, cfg.AliyunVisionRelayEndpoint, cfg.AliyunVisionRelayPrefix)
		} else {
			logger.Warnf("[aicommerce] vision relay ENABLED but config INCOMPLETE (missing endpoint/bucket/ak)；将直连 vision，跨区域 OSS 仍会 InvalidImage.RegionRecommend")
		}
	} else {
		logger.Infof("[aicommerce] vision relay disabled；假定业务 OSS 与 vision region 同区域")
	}

	return &Dispatcher{
		db:            db,
		rdb:           rdb,
		cfg:           cfg,
		uploadManager: uploadManager,
		tongyi:        provider.NewTongyi(cfg.TongyiBaseURL, cfg.TongyiAPIKey, cfg.TongyiModel),
		vision:        vision,
		baiduOCR:      provider.NewBaiduOCR(cfg.BaiduOCRAppID, cfg.BaiduOCRAPIKey, cfg.BaiduOCRSecretKey),
		baiduTrans:    provider.NewBaiduTranslate(cfg.BaiduTranslateAppID, cfg.BaiduTranslateSecret),
	}
}

type queuePayload struct {
	TaskID uint   `json:"task_id"`
	TaskNo string `json:"task_no"`
}

// Run 启动 Worker，并发数由 cfg.WorkerConcurrency 控制
func (d *Dispatcher) Run(ctx context.Context) {
	sem := make(chan struct{}, d.cfg.WorkerConcurrency)
	logger.Infof("AI Commerce Worker started, concurrency=%d, queue=%s", d.cfg.WorkerConcurrency, d.cfg.QueueName)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, err := d.rdb.BRPop(ctx, 5*time.Second, d.cfg.QueueName).Result()
		if err != nil {
			continue
		}
		if len(result) < 2 {
			continue
		}

		var payload queuePayload
		if err := json.Unmarshal([]byte(result[1]), &payload); err != nil {
			logger.Errorf("invalid queue payload: %v", err)
			continue
		}

		sem <- struct{}{}
		go func(p queuePayload) {
			defer func() { <-sem }()
			d.execute(ctx, p.TaskID)
		}(payload)
	}
}

func (d *Dispatcher) execute(ctx context.Context, taskID uint) {
	var task model.AiImageTask
	if err := d.db.First(&task, taskID).Error; err != nil {
		logger.Errorf("task %d not found: %v", taskID, err)
		return
	}

	// compare-and-swap: queued → running，防重复执行
	result := d.db.Model(&task).Where("status = ?", model.TaskStatusQueued).
		Updates(map[string]interface{}{
			"status":     model.TaskStatusRunning,
			"started_at": time.Now(),
		})
	if result.RowsAffected == 0 {
		return // 已被其他 Worker 处理
	}

	var execErr error
	uploader := d.uploadManager.GetUploadHandler()

	switch task.Module {
	case model.ModuleMainImage, model.ModuleDetailPage:
		imgClient, err := d.resolveImageClient(ctx, &task)
		if err != nil {
			execErr = err
			break
		}
		execErr = chains.RunMainImage(ctx, d.db, d.rdb, imgClient, d.tongyi, uploader, d.cfg, &task)
	case model.ModuleWhiteBg:
		execErr = chains.RunWhiteBg(ctx, d.db, d.vision, uploader, d.cfg, &task)
	case model.ModuleClone:
		imgClient, err := d.resolveImageClient(ctx, &task)
		if err != nil {
			execErr = err
			break
		}
		execErr = chains.RunClone(ctx, d.db, imgClient, uploader, d.cfg, &task)
	case model.ModuleRatioConvert:
		imgClient, err := d.resolveImageClient(ctx, &task)
		if err != nil {
			execErr = err
			break
		}
		execErr = chains.RunRatioConvert(ctx, d.db, imgClient, uploader, d.cfg, &task)
	case model.ModuleTranslate:
		execErr = chains.RunTranslate(ctx, d.db, d.baiduOCR, d.baiduTrans, d.cfg, &task)
	case model.ModuleEdit:
		imgClient, err := d.resolveImageClient(ctx, &task)
		if err != nil {
			execErr = err
			break
		}
		execErr = chains.RunEdit(ctx, d.db, imgClient, uploader, d.cfg, &task)
	default:
		execErr = nil
	}

	now := time.Now()
	if execErr != nil {
		logger.Errorf("task %d failed: %v", taskID, execErr)
		d.db.Model(&task).Updates(map[string]interface{}{
			"status":        model.TaskStatusFailed,
			"error_message": execErr.Error(),
			"finished_at":   now,
		})
		// 退还算力
		d.db.Model(&model.User{}).Where("id = ?", task.UserId).
			UpdateColumn("power", gorm.Expr("power + ?", task.CreditCost))
	} else {
		d.db.Model(&task).Updates(map[string]interface{}{
			"status":      model.TaskStatusSucceeded,
			"progress":    100,
			"finished_at": now,
		})
	}

	// 发布进度事件到 Redis PubSub（供 SSE Handler 消费）
	d.rdb.Publish(ctx, "aic:task:"+task.TaskNo, func() string {
		if execErr != nil {
			return "failed"
		}
		return "completed"
	}())
}

func (d *Dispatcher) resolveImageClient(ctx context.Context, task *model.AiImageTask) (provider.ImageClient, error) {
	modelName := strings.TrimSpace(task.Model)
	if modelName == "" {
		modelName = d.cfg.SiliconFlowModel
	}
	if modelName == "" {
		return nil, fmt.Errorf("image model is empty")
	}

	var aiModel model.AiModel
	err := d.db.WithContext(ctx).
		Where("name = ? AND model_type = ? AND status = ?", modelName, "image", "active").
		First(&aiModel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("active image model %q not found in geekai_ai_models", modelName)
		}
		return nil, fmt.Errorf("load image model %q: %w", modelName, err)
	}
	if strings.TrimSpace(aiModel.ApiKey) == "" {
		return nil, fmt.Errorf("image model %q has empty api key", aiModel.Name)
	}
	requiredCap := requiredImageCapability(task.Module)
	if requiredCap != "" && strings.TrimSpace(aiModel.Capabilities) != "" {
		if !hasImageCapability(aiModel.Capabilities, requiredCap) {
			return nil, fmt.Errorf("image model %q does not support capability %q", aiModel.Name, requiredCap)
		}
	}

	task.Model = aiModel.Name
	task.Provider = aiModel.Provider
	if strings.EqualFold(strings.TrimSpace(aiModel.Name), "qwen-image-edit") {
		return provider.NewQwenEditClient(aiModel.ApiEndpoint, aiModel.ApiKey, aiModel.Name), nil
	}
	return provider.NewOpenAIImageClient(aiModel.ApiEndpoint, aiModel.ApiKey, aiModel.Name), nil
}

func requiredImageCapability(module string) string {
	switch module {
	case model.ModuleMainImage, model.ModuleDetailPage, model.ModuleClone, model.ModuleRatioConvert, model.ModuleEdit:
		return "img2img"
	default:
		return ""
	}
}

func hasImageCapability(capabilities, required string) bool {
	for _, capability := range strings.Split(capabilities, ",") {
		if strings.EqualFold(strings.TrimSpace(capability), required) {
			return true
		}
	}
	return false
}
