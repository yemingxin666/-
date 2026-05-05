package aicommerce

import (
	"encoding/base64"
	"fmt"
	"geekai/core"
	"geekai/core/middleware"
	"geekai/core/types"
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
		group.GET("/tasks/:task_no", h.GetTask)
		group.GET("/tasks/:task_no/events", h.TaskEvents)
		group.DELETE("/tasks/:task_no", h.DeleteTask)
		group.GET("/gallery", h.Gallery)
		group.POST("/copywrite", h.Copywrite)
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

	result := h.db.Where("task_no = ? AND user_id = ?", taskNo, userID).
		UpdateColumn("deleted_at", "NOW()")
	if result.Error != nil || result.RowsAffected == 0 {
		resp.ERROR(c, "删除失败")
		return
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
		Id           uint   `json:"id"`
		Name         string `json:"name"`
		DisplayName  string `json:"display_name"`
		Provider     string `json:"provider"`
		ModelType    string `json:"model_type"`
		Description  string `json:"description"`
		Capabilities string `json:"capabilities"`
		SortOrder    int    `json:"sort_order"`
	}
	result := make([]publicModel, 0, len(models))
	for _, m := range models {
		result = append(result, publicModel{
			Id:           m.Id,
			Name:         m.Name,
			DisplayName:  m.DisplayName,
			Provider:     m.Provider,
			ModelType:    m.ModelType,
			Description:  m.Description,
			Capabilities: m.Capabilities,
			SortOrder:    m.SortOrder,
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
