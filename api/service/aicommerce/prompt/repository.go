package prompt

import (
	"geekai/store/model"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindTemplate 按 module+image_type 查询激活模板，image_type 不存在时降级到 generic
func (r *Repository) FindTemplate(module, imageType string) (*model.AiPromptTemplate, error) {
	for _, imgType := range []string{imageType, "generic"} {
		var tmpl model.AiPromptTemplate
		err := r.db.Where(
			"module = ? AND image_type = ? AND status = ?",
			module, imgType, model.TemplateStatusActive,
		).Order("version DESC").First(&tmpl).Error
		if err == nil {
			return &tmpl, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// GetPriceByModel 查询模型单价（不区分模块，向后兼容）
func (r *Repository) GetPriceByModel(modelName string) (int, error) {
	return r.GetPriceByModelModule(modelName, "")
}

// moduleDefaultPrice 模块级兜底单价
var moduleDefaultPrice = map[string]int{
	"main_image":    6,
	"detail_page":   6,
	"clone":         7,
	"white_bg":      4,
	"translate":     4,
	"ratio_convert": 6,
	"edit":          6,
}

// GetPriceByModelModule 查询模型+模块单价。
// 优先精确匹配 (model, module)，未命中则 fallback 到 (model, "all")，再未命中返回模块默认值。
func (r *Repository) GetPriceByModelModule(modelName, module string) (int, error) {
	if module != "" && module != "all" {
		var cfg model.AiModelPriceConfig
		err := r.db.Where("model = ? AND module = ? AND status = ?", modelName, module, "active").First(&cfg).Error
		if err == nil {
			return cfg.CreditPerImage, nil
		}
	}
	var cfg model.AiModelPriceConfig
	err := r.db.Where("model = ? AND module = ? AND status = ?", modelName, "all", "active").First(&cfg).Error
	if err == nil {
		return cfg.CreditPerImage, nil
	}
	if d, ok := moduleDefaultPrice[module]; ok {
		return d, nil
	}
	return 10, nil
}
