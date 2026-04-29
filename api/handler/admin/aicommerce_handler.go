package admin

import (
	"geekai/core"
	"geekai/core/middleware"
	"geekai/core/types"
	"geekai/handler"
	"geekai/service/aicommerce/prompt"
	"geekai/store/model"
	"geekai/utils/resp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AiCommerceHandler struct {
	handler.BaseHandler
}

func NewAiCommerceHandler(app *core.AppServer, db *gorm.DB) *AiCommerceHandler {
	return &AiCommerceHandler{BaseHandler: handler.BaseHandler{App: app, DB: db}}
}

func (h *AiCommerceHandler) RegisterRoutes() {
	group := h.App.Engine.Group("/api/admin/ai-commerce/")
	group.Use(middleware.AdminAuthMiddleware(h.App.Config.AdminSession.SecretKey, h.App.Redis))
	{
		// Prompt 模板管理
		group.GET("templates", h.ListTemplates)
		group.POST("templates/save", h.SaveTemplate)
		group.POST("templates/preview", h.PreviewTemplate)
		group.GET("templates/remove", h.RemoveTemplate)
		group.POST("templates/status", h.SetTemplateStatus)

		// 模型积分定价管理
		group.GET("prices", h.ListPrices)
		group.POST("prices/save", h.SavePrice)
		group.GET("prices/remove", h.RemovePrice)

		// AI 模型管理
		group.GET("models", h.ListModels)
		group.POST("models/save", h.SaveModel)
		group.GET("models/remove", h.RemoveModel)

		// 任务审计
		group.GET("tasks", h.ListTasks)
	}
}

// ListTemplates 模板列表
func (h *AiCommerceHandler) ListTemplates(c *gin.Context) {
	module := c.Query("module")
	status := c.Query("status")
	platform := c.Query("platform")

	query := h.DB.Model(&model.AiPromptTemplate{})
	if module != "" {
		query = query.Where("module = ?", module)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if platform != "" {
		query = query.Where("platform = ?", platform)
	}

	var items []model.AiPromptTemplate
	if err := query.Order("module ASC, image_type ASC").Find(&items).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c, items)
}

// SaveTemplate 新建/更新模板
func (h *AiCommerceHandler) SaveTemplate(c *gin.Context) {
	var data model.AiPromptTemplate
	if err := c.ShouldBindJSON(&data); err != nil {
		resp.ERROR(c, types.InvalidArgs)
		return
	}

	now := time.Now()
	if data.Id == 0 {
		data.CreatedAt = now
		data.UpdatedAt = now
		if data.Status == "" {
			data.Status = model.TemplateStatusDraft
		}
		if err := h.DB.Create(&data).Error; err != nil {
			resp.ERROR(c, err.Error())
			return
		}
	} else {
		data.UpdatedAt = now
		if err := h.DB.Save(&data).Error; err != nil {
			resp.ERROR(c, err.Error())
			return
		}
	}
	resp.SUCCESS(c, data)
}

// PreviewTemplate 实时渲染预览
func (h *AiCommerceHandler) PreviewTemplate(c *gin.Context) {
	var body struct {
		UserTemplate     string `json:"user_template"`
		NegativeTemplate string `json:"negative_template"`
		ProductName      string `json:"product_name"`
		SellingPoints    string `json:"selling_points"`
		ImageType        string `json:"image_type"`
		Platform         string `json:"platform"`
		Language         string `json:"language"`
		Ratio            string `json:"ratio"`
		StyleDesc        string `json:"style_desc"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		resp.ERROR(c, types.InvalidArgs)
		return
	}

	vars := prompt.Vars{
		ProductName:         body.ProductName,
		SellingPoints:       body.SellingPoints,
		ImageTypeDesc:       prompt.ImageTypeDesc(body.ImageType),
		Platform:            body.Platform,
		PlatformRules:       prompt.PlatformRules(body.Platform),
		Language:            body.Language,
		Ratio:               body.Ratio,
		StyleDesc:           body.StyleDesc,
		ReferenceImageCount: 0,
	}

	result, err := prompt.Render(body.UserTemplate, body.NegativeTemplate, vars)
	if err != nil {
		resp.ERROR(c, "渲染失败: "+err.Error())
		return
	}
	resp.SUCCESS(c, gin.H{
		"positive": result.PositivePrompt,
		"negative": result.NegativePrompt,
	})
}

// RemoveTemplate 删除模板
func (h *AiCommerceHandler) RemoveTemplate(c *gin.Context) {
	id := h.GetInt(c, "id", 0)
	if id <= 0 {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	if err := h.DB.Where("id = ?", id).Delete(&model.AiPromptTemplate{}).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c)
}

// SetTemplateStatus 发布/归档模板
func (h *AiCommerceHandler) SetTemplateStatus(c *gin.Context) {
	var data struct {
		Id     uint   `json:"id"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	validStatuses := map[string]bool{
		model.TemplateStatusDraft:    true,
		model.TemplateStatusActive:   true,
		model.TemplateStatusArchived: true,
	}
	if !validStatuses[data.Status] {
		resp.ERROR(c, "无效的状态值")
		return
	}
	if err := h.DB.Model(&model.AiPromptTemplate{}).Where("id = ?", data.Id).
		Update("status", data.Status).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c)
}

// ListPrices 模型积分定价列表
func (h *AiCommerceHandler) ListPrices(c *gin.Context) {
	var items []model.AiModelPriceConfig
	if err := h.DB.Order("id ASC").Find(&items).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c, items)
}

// SavePrice 新建/更新定价
func (h *AiCommerceHandler) SavePrice(c *gin.Context) {
	var data model.AiModelPriceConfig
	if err := c.ShouldBindJSON(&data); err != nil {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	if data.Model == "" || data.CreditPerImage <= 0 {
		resp.ERROR(c, "模型名称和积分不能为空")
		return
	}
	if data.Status == "" {
		data.Status = "active"
	}
	var res *gorm.DB
	if data.Id == 0 {
		res = h.DB.Create(&data)
	} else {
		res = h.DB.Save(&data)
	}
	if res.Error != nil {
		resp.ERROR(c, res.Error.Error())
		return
	}
	resp.SUCCESS(c, data)
}

// RemovePrice 删除定价配置
func (h *AiCommerceHandler) RemovePrice(c *gin.Context) {
	id := h.GetInt(c, "id", 0)
	if id <= 0 {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	if err := h.DB.Where("id = ?", id).Delete(&model.AiModelPriceConfig{}).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c)
}

// ListModels AI 模型列表
func (h *AiCommerceHandler) ListModels(c *gin.Context) {
	var items []model.AiModel
	if err := h.DB.Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c, items)
}

// SaveModel 新建/更新 AI 模型
func (h *AiCommerceHandler) SaveModel(c *gin.Context) {
	var data model.AiModel
	if err := c.ShouldBindJSON(&data); err != nil {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	if data.Name == "" || data.DisplayName == "" || data.Provider == "" {
		resp.ERROR(c, "模型标识、显示名称和提供商不能为空")
		return
	}
	if data.Status == "" {
		data.Status = "active"
	}
	now := time.Now()
	var res *gorm.DB
	if data.Id == 0 {
		data.CreatedAt = now
		data.UpdatedAt = now
		res = h.DB.Create(&data)
	} else {
		data.UpdatedAt = now
		res = h.DB.Save(&data)
	}
	if res.Error != nil {
		resp.ERROR(c, res.Error.Error())
		return
	}
	resp.SUCCESS(c, data)
}

// RemoveModel 删除 AI 模型
func (h *AiCommerceHandler) RemoveModel(c *gin.Context) {
	id := h.GetInt(c, "id", 0)
	if id <= 0 {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	if err := h.DB.Where("id = ?", id).Delete(&model.AiModel{}).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c)
}

// ListTasks 任务审计列表（分页）
func (h *AiCommerceHandler) ListTasks(c *gin.Context) {
	userID := c.Query("user_id")
	module := c.Query("module")
	status := c.Query("status")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize > 100 {
		pageSize = 100
	}

	query := h.DB.Model(&model.AiImageTask{}).Where("deleted_at IS NULL")
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if module != "" {
		query = query.Where("module = ?", module)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if startDate != "" {
		query = query.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("created_at <= ?", endDate+" 23:59:59")
	}

	var total int64
	query.Count(&total)

	var tasks []model.AiImageTask
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c, gin.H{"items": tasks, "total": total, "page": page, "page_size": pageSize})
}
