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

// FindTemplate 5级 Fallback 模板查询
// Level 1: 精确匹配 module+imageType+platform+language+ratio
// Level 2: ratio → any
// Level 3: language → zh-CN
// Level 4: platform → generic
// Level 5: imageType → generic
func (r *Repository) FindTemplate(module, imageType, platform, language, ratio string) (*model.AiPromptTemplate, error) {
	levels := []struct{ platform, language, ratio string }{
		{platform, language, ratio},
		{platform, language, "any"},
		{platform, "zh-CN", "any"},
		{"generic", "zh-CN", "any"},
		{"generic", "zh-CN", "any"}, // Level 5 用 imageType=generic
	}

	for i, lvl := range levels {
		imgType := imageType
		if i == 4 {
			imgType = "generic"
		}
		var tmpl model.AiPromptTemplate
		err := r.db.Where(
			"module = ? AND image_type = ? AND platform = ? AND language = ? AND ratio = ? AND status = ?",
			module, imgType, lvl.platform, lvl.language, lvl.ratio, model.TemplateStatusActive,
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
