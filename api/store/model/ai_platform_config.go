package model

import "time"

type AiPlatformConfig struct {
	Id              uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Value           string    `gorm:"column:value;type:varchar(32);uniqueIndex;not null" json:"value"`
	Label           string    `gorm:"column:label;type:varchar(64);not null" json:"label"`
	DefaultLanguage string    `gorm:"column:default_language;type:varchar(16);not null;default:zh-CN" json:"default_language"`
	DefaultRatio    string    `gorm:"column:default_ratio;type:varchar(16);not null;default:1:1" json:"default_ratio"`
	PromptStyle     string    `gorm:"column:prompt_style;type:mediumtext;not null" json:"prompt_style"`
	PriorityImages  JSONMap   `gorm:"column:priority_images;type:json" json:"priority_images"`
	Constraints     JSONMap   `gorm:"column:constraints;type:json" json:"constraints"`
	Status          string    `gorm:"column:status;type:varchar(16);index;not null;default:active" json:"status"`
	SortOrder       int       `gorm:"column:sort_order;type:int;not null;default:0" json:"sort_order"`
	CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (m *AiPlatformConfig) TableName() string {
	return "puningai_ai_platform_configs"
}

const (
	PlatformStatusActive   = "active"
	PlatformStatusDisabled = "disabled"
)
