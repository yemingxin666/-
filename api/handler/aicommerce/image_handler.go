package aicommerce

import (
	"geekai/core"
	"geekai/core/middleware"
	"geekai/service/aicommerce"
	"geekai/store/model"
	"geekai/utils/resp"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ImageHandler struct {
	app     *core.AppServer
	db      *gorm.DB
	service *aicommerce.ImageService
}

func NewImageHandler(app *core.AppServer, db *gorm.DB, svc *aicommerce.ImageService) *ImageHandler {
	return &ImageHandler{app: app, db: db, service: svc}
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

// UploadAsset 上传参考图片
func (h *ImageHandler) UploadAsset(c *gin.Context) {
	userID := h.getLoginUserID(c)
	if userID == 0 {
		resp.ERROR(c, "未登录")
		return
	}
	// 简化实现：实际需集成 GeeKAI 现有 OSS 上传逻辑
	resp.SUCCESS(c, gin.H{"asset_no": "todo", "message": "接入 GeeKAI OSS uploader"})
}

// GetTask 轮询任务状态
func (h *ImageHandler) GetTask(c *gin.Context) {
	userID := h.getLoginUserID(c)
	taskNo := c.Param("task_no")

	task, assets, err := h.service.GetTask(c.Request.Context(), userID, taskNo)
	if err != nil {
		resp.ERROR(c, "任务不存在")
		return
	}

	assetURLs := make([]string, 0, len(assets))
	for _, a := range assets {
		assetURLs = append(assetURLs, a.OssKey)
	}

	resp.SUCCESS(c, gin.H{
		"task_no":  task.TaskNo,
		"module":   task.Module,
		"status":   task.Status,
		"progress": task.Progress,
		"outputs":  assetURLs,
		"error":    task.ErrorMessage,
	})
}

// TaskEvents SSE 实时进度推送
func (h *ImageHandler) TaskEvents(c *gin.Context) {
	userID := h.getLoginUserID(c)
	taskNo := c.Param("task_no")

	// 验证任务归属
	if _, _, err := h.service.GetTask(c.Request.Context(), userID, taskNo); err != nil {
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
		ProductName string `json:"product_name"`
		Hint        string `json:"hint"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		resp.ERROR(c, "参数错误")
		return
	}
	result, err := h.service.Copywrite(c.Request.Context(), body.ProductName, body.Hint)
	if err != nil {
		resp.ERROR(c, "代写失败: "+err.Error())
		return
	}
	resp.SUCCESS(c, gin.H{"content": result})
}

// ListModels 返回启用的 AI 模型列表（供用户端选择，不含 ApiKey）
func (h *ImageHandler) ListModels(c *gin.Context) {
	var models []model.AiModel
	if err := h.db.Where("status = ?", "active").Order("sort_order ASC, id ASC").Find(&models).Error; err != nil {
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

func (h *ImageHandler) getLoginUserID(c *gin.Context) uint {
	v, exists := c.Get("LoginUserId")
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
