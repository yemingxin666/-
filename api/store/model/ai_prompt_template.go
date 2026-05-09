package model

import "time"

type AiPromptTemplate struct {
	Id               uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TemplateKey      string    `gorm:"column:template_key;type:varchar(128);not null" json:"template_key"`
	Module           string    `gorm:"column:module;type:varchar(32);not null" json:"module"`
	ImageType        string    `gorm:"column:image_type;type:varchar(64);not null" json:"image_type"`
	SystemPrompt     string    `gorm:"column:system_prompt;type:mediumtext;not null" json:"system_prompt"`
	UserTemplate     string    `gorm:"column:user_template;type:mediumtext;not null" json:"user_template"`
	NegativeTemplate string    `gorm:"column:negative_template;type:mediumtext" json:"negative_template"`
	ParamsJSON       JSONMap   `gorm:"column:params_json;type:json" json:"params_json"`
	Version          int       `gorm:"column:version;type:int;not null;default:1" json:"version"`
	Status           string    `gorm:"column:status;type:varchar(16);not null;default:active" json:"status"`
	CreatedBy        uint      `gorm:"column:created_by;type:int" json:"created_by"`
	CreatedAt        time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (m *AiPromptTemplate) TableName() string {
	return "geekai_ai_prompt_templates"
}

// 模板状态常量
const (
	TemplateStatusDraft    = "draft"
	TemplateStatusActive   = "active"
	TemplateStatusArchived = "archived"
)

type AiModelPriceConfig struct {
	Id             uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Model          string `gorm:"column:model;type:varchar(64);uniqueIndex;not null" json:"model"`
	Module         string `gorm:"column:module;type:varchar(32);not null;default:all" json:"module"`
	CreditPerImage int    `gorm:"column:credit_per_image;type:int;not null" json:"credit_per_image"`
	Description    string `gorm:"column:description;type:varchar(255)" json:"description"`
	Status         string `gorm:"column:status;type:varchar(16);not null;default:active" json:"status"`
}

func (m *AiModelPriceConfig) TableName() string {
	return "geekai_ai_model_price_config"
}

// AiModel AI 模型配置
type AiModel struct {
	Id           uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"column:name;type:varchar(64);uniqueIndex;not null" json:"name"`
	DisplayName  string    `gorm:"column:display_name;type:varchar(128);not null" json:"display_name"`
	Provider     string    `gorm:"column:provider;type:varchar(64);not null" json:"provider"`
	ModelType    string    `gorm:"column:model_type;type:varchar(32);not null;default:image" json:"model_type"`
	ApiEndpoint  string    `gorm:"column:api_endpoint;type:varchar(512)" json:"api_endpoint"`
	ApiKey       string    `gorm:"column:api_key;type:varchar(512)" json:"api_key"`
	Capabilities string    `gorm:"column:capabilities;type:varchar(128);not null;default:''" json:"capabilities"`
	Description  string    `gorm:"column:description;type:varchar(512)" json:"description"`
	SortOrder    int       `gorm:"column:sort_order;type:int;not null;default:0" json:"sort_order"`
	Status       string    `gorm:"column:status;type:varchar(16);not null;default:active" json:"status"`
	CreatedAt    time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (m *AiModel) TableName() string {
	return "geekai_ai_models"
}
