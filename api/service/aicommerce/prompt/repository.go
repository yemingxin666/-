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

// GetPriceByModel 查询模型单价
func (r *Repository) GetPriceByModel(modelName string) (int, error) {
	var cfg model.AiModelPriceConfig
	err := r.db.Where("model = ? AND status = ?", modelName, "active").First(&cfg).Error
	if err != nil {
		return 10, nil // 查不到时默认 10 算力
	}
	return cfg.CreditPerImage, nil
}
