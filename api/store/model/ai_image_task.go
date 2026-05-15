package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type AiImageTask struct {
	Id            uint       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TaskNo        string     `gorm:"column:task_no;type:varchar(64);uniqueIndex;not null" json:"task_no"`
	UserId        uint       `gorm:"column:user_id;type:int(11);not null;index:idx_user_created,priority:1" json:"user_id"`
	Module        string     `gorm:"column:module;type:varchar(32);not null;index:idx_module" json:"module"`
	ImageType     string     `gorm:"column:image_type;type:varchar(64)" json:"image_type"`
	Platform      string     `gorm:"column:platform;type:varchar(32)" json:"platform"`
	Language      string     `gorm:"column:language;type:varchar(16)" json:"language"`
	Ratio         string     `gorm:"column:ratio;type:varchar(16)" json:"ratio"`
	InputJSON     JSONMap    `gorm:"column:input_json;type:json;not null" json:"input_json"`
	PromptJSON    JSONMap    `gorm:"column:prompt_json;type:json" json:"prompt_json"`
	Status        string     `gorm:"column:status;type:varchar(16);not null;default:pending;index:idx_status" json:"status"`
	Progress      int        `gorm:"column:progress;type:tinyint;not null;default:0" json:"progress"`
	Model         string     `gorm:"column:model;type:varchar(64)" json:"model"`
	CreditCost    int        `gorm:"column:credit_cost;type:int" json:"credit_cost"`
	CreditTxId    string     `gorm:"column:credit_tx_id;type:varchar(64)" json:"credit_tx_id"`
	Provider      string     `gorm:"column:provider;type:varchar(32)" json:"provider"`
	ProviderJobId string     `gorm:"column:provider_job_id;type:varchar(128)" json:"provider_job_id"`
	ErrorCode     string     `gorm:"column:error_code;type:varchar(64)" json:"error_code"`
	ErrorMessage  string     `gorm:"column:error_message;type:varchar(1024)" json:"error_message"`
	StartedAt     *time.Time `gorm:"column:started_at" json:"started_at"`
	FinishedAt    *time.Time `gorm:"column:finished_at" json:"finished_at"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null;index:idx_deleted_created,priority:2,sort:desc;index:idx_user_created,priority:2,sort:desc" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
	DeletedAt     *time.Time `gorm:"column:deleted_at;index:idx_deleted_created,priority:1" json:"deleted_at,omitempty"`
}

func (m *AiImageTask) TableName() string {
	return "geekai_ai_image_tasks"
}

// Task 状态常量
const (
	TaskStatusPending   = "pending"
	TaskStatusQueued    = "queued"
	TaskStatusRunning   = "running"
	TaskStatusSucceeded = "succeeded"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
)

// Module 类型常量
const (
	ModuleMainImage    = "main_image"
	ModuleDetailPage   = "detail_page"
	ModuleWhiteBg      = "white_bg"
	ModuleClone        = "clone"
	ModuleRatioConvert = "ratio_convert"
	ModuleTranslate    = "translate"
	ModuleEdit         = "edit"
)

// JSONMap 用于 JSON 类型字段的序列化
type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	}
	return json.Unmarshal(bytes, j)
}
