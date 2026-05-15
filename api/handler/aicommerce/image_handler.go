package aicommerce

import (
	"encoding/base64"
	"fmt"
	"geekai/core"
	"geekai/core/middleware"
	"geekai/core/types"
	logger2 "geekai/logger"
	"geekai/service/aicommerce"
	"geekai/service/oss"
	"geekai/store/model"
	"geekai/utils/resp"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var logger = logger2.GetLogger()

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

type ImageHandler struct {
	app      *core.AppServer
	db       *gorm.DB
	service  *aicommerce.ImageService
	uploader oss.Uploader
}

func NewImageHandler(app *core.AppServer, db *gorm.DB, svc *aicommerce.ImageService, mgr *oss.UploaderManager) *ImageHandler {
	return &ImageHandler{app: app, db: db, service: svc, uploader: mgr.GetUploadHandler()}
}

func (h *ImageHandler) RegisterRoutes() {
	group := h.app.Engine.Group("/api/ai-commerce")
	group.Use(middleware.UserAuthMiddleware(h.app.Config.Session.SecretKey, h.app.Redis))
	{
		group.POST("/assets", h.UploadAsset)
		group.POST("/main-images", h.GenerateImage(aicommerce.ModuleMainImage))
		group.POST("/detail-pages", h.GenerateImage(aicommerce.ModuleDetailPage))
		group.POST("/white-backgrounds", h.GenerateImage(aicommerce.ModuleWhiteBg))
		group.POST("/clone-designs", h.GenerateImage(aicommerce.ModuleClone))
		group.POST("/ratio-conversions", h.GenerateImage(aicommerce.ModuleRatioConvert))
		group.POST("/image-text-translations", h.GenerateImage(aicommerce.ModuleTranslate))
		group.POST("/edit", h.EditImage)
		group.GET("/tasks/:task_no", h.GetTask)
		group.GET("/tasks/:task_no/events", h.TaskEvents)
		group.DELETE("/tasks/:task_no", h.DeleteTask)
		group.DELETE("/assets/:asset_no", h.DeleteAsset)
		group.GET("/gallery", h.Gallery)
		// group.POST("/copywrite", h.Copywrite) // AI 识别图片并代写卖点：暂时下线（保留 handler 以便后续恢复）
		group.GET("/models", h.ListModels)
		group.GET("/platform-configs", h.ListPlatformConfigs)
	}
}

// GenerateImage 生成各模块图片的通用 Handler
func (h *ImageHandler) GenerateImage(module string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := h.getLoginUserID(c)
		if userID == 0 {
			resp.ERROR(c, "未登录")
			return
		}

		var req aicommerce.GenerateReq
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.ERROR(c, "参数错误: "+err.Error())
			return
		}
		req.Module = module

		task, err := h.service.SubmitTask(c.Request.Context(), userID, req)
		if err != nil {
			resp.ERROR(c, err.Error())
			return
		}
		resp.SUCCESS(c, gin.H{
			"task_no":     task.TaskNo,
			"status":      task.Status,
			"credit_cost": task.CreditCost,
		})
	}
}

// EditImage 基于历史图库某张图 + 用户 prompt 编辑生成新图
// 新建独立任务，作为新条目出现在历史图库；比例沿用原图
func (h *ImageHandler) EditImage(c *gin.Context) {
	userID := h.getLoginUserID(c)
	if userID == 0 {
		resp.ERROR(c, "未登录")
		return
	}

	var req aicommerce.EditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.ERROR(c, "参数错误: "+err.Error())
		return
	}

	task, err := h.service.SubmitEditTask(c.Request.Context(), userID, req)
	if err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c, gin.H{
		"task_no":     task.TaskNo,
		"status":      task.Status,
		"credit_cost": task.CreditCost,
	})
}

// UploadAsset 上传参考图片到 OSS，写入 DB，返回 asset_no
func (h *ImageHandler) UploadAsset(c *gin.Context) {
	userID := h.getLoginUserID(c)
	if userID == 0 {
		resp.ERROR(c, "未登录")
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		resp.ERROR(c, "读取文件失败: "+err.Error())
		return
	}

	const maxSize = 10 << 20 // 10MB
	if fh.Size > maxSize {
		resp.ERROR(c, "图片大小不能超过 10MB")
		return
	}

	src, err := fh.Open()
	if err != nil {
		resp.ERROR(c, "打开文件失败: "+err.Error())
		return
	}
	defer src.Close()

	data, err := io.ReadAll(io.LimitReader(src, maxSize))
	if err != nil {
		resp.ERROR(c, "读取文件内容失败: "+err.Error())
		return
	}

	mime := fh.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/jpeg"
	}

	// PutBase64 期望纯 base64 字符串（不含 data URL 前缀）
	ossKey, err := h.uploader.PutBase64(encodeBase64(data))
	if err != nil {
		resp.ERROR(c, "上传 OSS 失败: "+err.Error())
		return
	}

	// 写入 DB
	assetNo := fmt.Sprintf("ref_%d_%d", userID, time.Now().UnixNano())
	asset := model.AiImageAsset{
		AssetNo:  assetNo,
		UserId:   userID,
		Kind:     model.AssetKindReference,
		OssKey:   ossKey,
		MimeType: mime,
		SizeBytes: int64(len(data)),
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(&asset).Error; err != nil {
		resp.ERROR(c, "保存资产失败: "+err.Error())
		return
	}

	resp.SUCCESS(c, gin.H{"asset_no": assetNo})
}

// GetTask 轮询任务状态
func (h *ImageHandler) GetTask(c *gin.Context) {
	userID := h.getLoginUserID(c)
	taskNo := c.Param("task_no")

	result, err := h.service.GetTask(c.Request.Context(), userID, taskNo)
	if err != nil {
		resp.ERROR(c, "任务不存在")
		return
	}

	resp.SUCCESS(c, gin.H{
		"task_no":  result.Task.TaskNo,
		"module":   result.Task.Module,
		"status":   result.Task.Status,
		"progress": result.Progress,
		"outputs":  result.Outputs,
		"items":    result.Items,
		"error":    result.Task.ErrorMessage,
	})
}

// TaskEvents SSE 实时进度推送
func (h *ImageHandler) TaskEvents(c *gin.Context) {
	userID := h.getLoginUserID(c)
	taskNo := c.Param("task_no")

	// 验证任务归属
	if _, err := h.service.GetTask(c.Request.Context(), userID, taskNo); err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	sub := h.app.Redis.Subscribe(c.Request.Context(), "aic:task:"+taskNo)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.SSEvent("message", msg.Payload)
			c.Writer.Flush()
			if msg.Payload == "completed" || msg.Payload == "failed" {
				return
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}

// DeleteTask 软删除任务
func (h *ImageHandler) DeleteTask(c *gin.Context) {
	userID := h.getLoginUserID(c)
	taskNo := c.Param("task_no")

	var task model.AiImageTask
	if err := h.db.Where("task_no = ? AND user_id = ? AND deleted_at IS NULL", taskNo, userID).
		First(&task).Error; err != nil {
		resp.ERROR(c, "任务不存在或已删除")
		return
	}

	now := time.Now()
	if err := h.db.Model(&model.AiImageTask{}).Where("id = ?", task.Id).
		Update("deleted_at", &now).Error; err != nil {
		resp.ERROR(c, "删除失败: "+err.Error())
		return
	}

	// 查询关联 assets，删除 OSS 文件并软删 asset 记录
	var assets []model.AiImageAsset
	h.db.Where("task_id = ? AND oss_key != '' AND deleted_at IS NULL", task.Id).Find(&assets)
	for _, a := range assets {
		if err := h.uploader.Delete(a.OssKey); err != nil {
			logger.Warnf("DeleteTask: OSS delete failed, asset_no=%s oss_key=%s err=%v", a.AssetNo, a.OssKey, err)
		}
	}
	if len(assets) > 0 {
		h.db.Model(&model.AiImageAsset{}).
			Where("task_id = ? AND deleted_at IS NULL", task.Id).
			Update("deleted_at", &now)
	}

	resp.SUCCESS(c, nil)
}

// DeleteAsset 软删除单张资产；若该任务下所有资产均被删除，则级联软删除任务本身。
// 解决场景：历史图库一个任务多张图，仅删除其中一张时不应丢失其他图。
func (h *ImageHandler) DeleteAsset(c *gin.Context) {
	userID := h.getLoginUserID(c)
	assetNo := c.Param("asset_no")

	var asset model.AiImageAsset
	if err := h.db.Where("asset_no = ? AND user_id = ? AND deleted_at IS NULL", assetNo, userID).
		First(&asset).Error; err != nil {
		resp.ERROR(c, "资产不存在或已删除")
		return
	}

	// 删除 OSS 文件
	if asset.OssKey != "" {
		if err := h.uploader.Delete(asset.OssKey); err != nil {
			logger.Warnf("DeleteAsset: OSS delete failed, asset_no=%s oss_key=%s err=%v", asset.AssetNo, asset.OssKey, err)
		}
	}

	now := time.Now()
	if err := h.db.Model(&model.AiImageAsset{}).
		Where("id = ?", asset.Id).
		Update("deleted_at", &now).Error; err != nil {
		resp.ERROR(c, "删除失败: "+err.Error())
		return
	}

	// 级联：若任务下已无未删除的真实资产（OssKey != ""），软删整个任务，避免空任务残留列表
	if asset.TaskId != nil {
		var remaining int64
		h.db.Model(&model.AiImageAsset{}).
			Where("task_id = ? AND oss_key != '' AND deleted_at IS NULL", *asset.TaskId).
			Count(&remaining)
		if remaining == 0 {
			h.db.Model(&model.AiImageTask{}).
				Where("id = ? AND deleted_at IS NULL", *asset.TaskId).
				Update("deleted_at", &now)
		}
	}
	resp.SUCCESS(c, nil)
}

// Gallery 历史图库
func (h *ImageHandler) Gallery(c *gin.Context) {
	userID := h.getLoginUserID(c)
	module := c.Query("module")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize > 50 {
		pageSize = 50
	}

	tasks, total, err := h.service.ListGallery(c.Request.Context(), userID, module, page, pageSize)
	if err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c, gin.H{"items": tasks, "total": total})
}

// Copywrite AI 代写卖点
func (h *ImageHandler) Copywrite(c *gin.Context) {
	userID := h.getLoginUserID(c)
	if userID == 0 {
		resp.ERROR(c, "未登录")
		return
	}

	var body struct {
		ProductName     string   `json:"product_name"`
		Hint            string   `json:"hint"`
		AssetNos        []string `json:"asset_nos"`
		ReferenceAssets []string `json:"reference_assets"`
		ImageType       string   `json:"image_type"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		resp.ERROR(c, "参数错误")
		return
	}

	assetNos := body.AssetNos
	if len(assetNos) == 0 {
		assetNos = body.ReferenceAssets
	}

	content, analysis, err := h.service.Copywrite(c.Request.Context(), userID, aicommerce.CopywriteReq{
		ProductName: body.ProductName,
		Hint:        body.Hint,
		AssetNos:    assetNos,
		ImageType:   body.ImageType,
	})
	if err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c, gin.H{"content": content, "analysis": analysis})
}

// ListModels 返回启用的 AI 模型列表（供用户端选择，不含 ApiKey）
func (h *ImageHandler) ListModels(c *gin.Context) {
	var models []model.AiModel
	if err := h.db.Where("status = ? AND model_type = ?", "active", "image").Order("sort_order ASC, id ASC").Find(&models).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	type publicModel struct {
		Name         string `json:"name"`
		DisplayName  string `json:"display_name"`
		Provider     string `json:"provider"`
		Description  string `json:"description"`
		Capabilities string `json:"capabilities"`
	}
	result := make([]publicModel, 0, len(models))
	for _, m := range models {
		result = append(result, publicModel{
			Name:         m.Name,
			DisplayName:  m.DisplayName,
			Provider:     m.Provider,
			Description:  m.Description,
			Capabilities: m.Capabilities,
		})
	}
	resp.SUCCESS(c, result)
}

// ListPlatformConfigs 返回启用的平台配置列表（供用户端动态加载）
func (h *ImageHandler) ListPlatformConfigs(c *gin.Context) {
	var items []model.AiPlatformConfig
	if err := h.db.Where("status = ?", model.PlatformStatusActive).
		Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	type publicConfig struct {
		Value           string        `json:"value"`
		Label           string        `json:"label"`
		DefaultLanguage string        `json:"default_language"`
		DefaultRatio    string        `json:"default_ratio"`
		PriorityImages  model.JSONMap `json:"priority_images"`
		Constraints     model.JSONMap `json:"constraints"`
		SortOrder       int           `json:"sort_order"`
	}
	result := make([]publicConfig, 0, len(items))
	for _, item := range items {
		result = append(result, publicConfig{
			Value:           item.Value,
			Label:           item.Label,
			DefaultLanguage: item.DefaultLanguage,
			DefaultRatio:    item.DefaultRatio,
			PriorityImages:  item.PriorityImages,
			Constraints:     item.Constraints,
			SortOrder:       item.SortOrder,
		})
	}
	resp.SUCCESS(c, gin.H{"items": result})
}

func (h *ImageHandler) getLoginUserID(c *gin.Context) uint {
	v, exists := c.Get(types.LoginUserID)
	if !exists {
		return 0
	}
	switch id := v.(type) {
	case uint:
		return id
	case float64:
		return uint(id)
	}
	return 0
}
