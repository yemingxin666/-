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
	"strings"
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

		// 平台规范管理
		group.GET("platform-configs", h.ListPlatformConfigs)
		group.POST("platform-configs/save", h.SavePlatformConfig)
		group.GET("platform-configs/remove", h.RemovePlatformConfig)
		group.POST("platform-configs/status", h.SetPlatformConfigStatus)

		// 任务审计
		group.GET("tasks", h.ListTasks)
	}
}

// ListTemplates 模板列表
func (h *AiCommerceHandler) ListTemplates(c *gin.Context) {
	module := c.Query("module")
	status := c.Query("status")

	query := h.DB.Model(&model.AiPromptTemplate{})
	if module != "" {
		query = query.Where("module = ?", module)
	}
	if status != "" {
		query = query.Where("status = ?", status)
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
		if err := h.DB.Select(
			"template_key", "module", "image_type",
			"system_prompt", "user_template", "negative_template",
			"params_json", "version", "status", "updated_at",
		).Save(&data).Error; err != nil {
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
		PlatformRules:       prompt.PlatformRules(h.DB, body.Platform),
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
	if msg := validateBackupApiPairs(data); msg != "" {
		resp.ERROR(c, msg)
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

func validateBackupApiPairs(data model.AiModel) string {
	pairs := []struct {
		idx int
		ep  string
		key string
	}{
		{1, data.BackupApiEndpoint1, data.BackupApiKey1},
		{2, data.BackupApiEndpoint2, data.BackupApiKey2},
		{3, data.BackupApiEndpoint3, data.BackupApiKey3},
	}
	for _, p := range pairs {
		hasEp := strings.TrimSpace(p.ep) != ""
		hasKey := strings.TrimSpace(p.key) != ""
		if hasEp != hasKey {
			return "备用 API " + strconv.Itoa(p.idx) + " 的地址和密钥必须同时填写或同时留空"
		}
	}
	return ""
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

// ListPlatformConfigs 平台配置列表
func (h *AiCommerceHandler) ListPlatformConfigs(c *gin.Context) {
	status := c.Query("status")
	keyword := c.Query("keyword")
	query := h.DB.Model(&model.AiPlatformConfig{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("value LIKE ? OR label LIKE ?", like, like)
	}
	var items []model.AiPlatformConfig
	if err := query.Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c, gin.H{"items": items})
}

// SavePlatformConfig 新建/更新平台配置
func (h *AiCommerceHandler) SavePlatformConfig(c *gin.Context) {
	var data model.AiPlatformConfig
	if err := c.ShouldBindJSON(&data); err != nil {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	if data.Value == "" || data.Label == "" || data.PromptStyle == "" {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	if data.DefaultLanguage == "" {
		data.DefaultLanguage = "zh-CN"
	}
	if data.DefaultRatio == "" {
		data.DefaultRatio = "1:1"
	}
	if data.Status == "" {
		data.Status = model.PlatformStatusActive
	}
	if data.PriorityImages == nil {
		data.PriorityImages = model.JSONMap{"must_have": []string{}, "recommended": []string{}, "optional": []string{}}
	}
	if data.Constraints == nil {
		data.Constraints = model.JSONMap{}
	}
	now := time.Now()
	if data.Id == 0 {
		data.CreatedAt = now
		data.UpdatedAt = now
		if err := h.DB.Create(&data).Error; err != nil {
			resp.ERROR(c, err.Error())
			return
		}
	} else {
		updates := map[string]interface{}{
			"label":            data.Label,
			"default_language": data.DefaultLanguage,
			"default_ratio":    data.DefaultRatio,
			"prompt_style":     data.PromptStyle,
			"priority_images":  data.PriorityImages,
			"constraints":      data.Constraints,
			"status":           data.Status,
			"sort_order":       data.SortOrder,
			"updated_at":       now,
		}
		if err := h.DB.Model(&model.AiPlatformConfig{}).Where("id = ?", data.Id).Updates(updates).Error; err != nil {
			resp.ERROR(c, err.Error())
			return
		}
	}
	prompt.ClearPlatformRulesCache()
	resp.SUCCESS(c, data)
}

// RemovePlatformConfig 删除平台配置
func (h *AiCommerceHandler) RemovePlatformConfig(c *gin.Context) {
	id := h.GetInt(c, "id", 0)
	if id <= 0 {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	var cfg model.AiPlatformConfig
	if err := h.DB.Where("id = ?", id).First(&cfg).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	if cfg.Value == "generic" {
		resp.ERROR(c, "通用平台配置不允许删除")
		return
	}
	if err := h.DB.Where("id = ?", id).Delete(&model.AiPlatformConfig{}).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	prompt.ClearPlatformRulesCache()
	resp.SUCCESS(c)
}

// SetPlatformConfigStatus 启用/禁用平台配置
func (h *AiCommerceHandler) SetPlatformConfigStatus(c *gin.Context) {
	var data struct {
		Id     uint   `json:"id"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	if data.Id == 0 || (data.Status != model.PlatformStatusActive && data.Status != model.PlatformStatusDisabled) {
		resp.ERROR(c, types.InvalidArgs)
		return
	}
	if err := h.DB.Model(&model.AiPlatformConfig{}).Where("id = ?", data.Id).Updates(map[string]interface{}{
		"status":     data.Status,
		"updated_at": time.Now(),
	}).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	prompt.ClearPlatformRulesCache()
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

	buildWhere := func(tx *gorm.DB) *gorm.DB {
		tx = tx.Where("deleted_at IS NULL")
		if userID != "" {
			tx = tx.Where("user_id = ?", userID)
		}
		if module != "" {
			tx = tx.Where("module = ?", module)
		}
		if status != "" {
			tx = tx.Where("status = ?", status)
		}
		if startDate != "" {
			tx = tx.Where("created_at >= ?", startDate)
		}
		if endDate != "" {
			tx = tx.Where("created_at <= ?", endDate+" 23:59:59")
		}
		return tx
	}

	var total int64
	buildWhere(h.DB.Model(&model.AiImageTask{})).Count(&total)

	var tasks []model.AiImageTask
	offset := (page - 1) * pageSize
	query := buildWhere(h.DB.Model(&model.AiImageTask{}))
	if err := query.Select("id, task_no, user_id, module, image_type, platform, language, ratio, status, progress, model, credit_cost, provider, error_code, error_message, started_at, finished_at, created_at, updated_at").
		Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error; err != nil {
		resp.ERROR(c, err.Error())
		return
	}
	resp.SUCCESS(c, gin.H{"items": tasks, "total": total, "page": page, "page_size": pageSize})
}
