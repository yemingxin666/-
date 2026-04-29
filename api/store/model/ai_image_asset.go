package model

import "time"

type AiImageAsset struct {
	Id           uint       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AssetNo      string     `gorm:"column:asset_no;type:varchar(64);uniqueIndex;not null" json:"asset_no"`
	TaskId       *uint      `gorm:"column:task_id;type:bigint" json:"task_id"`
	UserId       uint       `gorm:"column:user_id;type:int(11);not null" json:"user_id"`
	Kind         string     `gorm:"column:kind;type:varchar(32);not null" json:"kind"`
	OssBucket    string     `gorm:"column:oss_bucket;type:varchar(128);not null" json:"oss_bucket"`
	OssKey       string     `gorm:"column:oss_key;type:varchar(512);not null" json:"oss_key"`
	MimeType     string     `gorm:"column:mime_type;type:varchar(64);not null" json:"mime_type"`
	Width        int        `gorm:"column:width;type:int" json:"width"`
	Height       int        `gorm:"column:height;type:int" json:"height"`
	SizeBytes    int64      `gorm:"column:size_bytes;type:bigint" json:"size_bytes"`
	Sha256       string     `gorm:"column:sha256;type:char(64)" json:"sha256"`
	MetadataJSON JSONMap    `gorm:"column:metadata_json;type:json" json:"metadata_json"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	DeletedAt    *time.Time `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

func (m *AiImageAsset) TableName() string {
	return "geekai_ai_image_assets"
}

// Asset 类型常量
const (
	AssetKindReference    = "reference"
	AssetKindGenerated    = "generated"
	AssetKindIntermediate = "intermediate"
	AssetKindThumbnail    = "thumbnail"
)
