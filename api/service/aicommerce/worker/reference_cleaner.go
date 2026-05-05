package worker

import (
	"context"
	"geekai/service/oss"
	"geekai/store/model"
	"time"

	"gorm.io/gorm"
)

// ReferenceCleaner 定时清理过期参考图（OSS + DB 硬删除）
type ReferenceCleaner struct {
	db       *gorm.DB
	uploader oss.Uploader
	interval time.Duration
	maxAge   time.Duration
}

func NewReferenceCleaner(db *gorm.DB, mgr *oss.UploaderManager) *ReferenceCleaner {
	return &ReferenceCleaner{
		db:       db,
		uploader: mgr.GetUploadHandler(),
		interval: 3 * 24 * time.Hour,
		maxAge:   3 * 24 * time.Hour,
	}
}

// Run 启动定时清理，每隔 interval 执行一次，ctx 取消时退出
func (c *ReferenceCleaner) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// 启动时立即执行一次
	c.clean()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.clean()
		}
	}
}

func (c *ReferenceCleaner) clean() {
	cutoff := time.Now().Add(-c.maxAge)

	var assets []model.AiImageAsset
	c.db.Where("kind = ? AND created_at < ? AND deleted_at IS NULL", model.AssetKindReference, cutoff).
		Find(&assets)

	if len(assets) == 0 {
		return
	}

	ids := make([]uint, 0, len(assets))
	for _, a := range assets {
		if err := c.uploader.Delete(a.OssKey); err != nil {
			logger.Warnf("reference cleaner: delete OSS %s: %v", a.OssKey, err)
		}
		ids = append(ids, a.Id)
	}

	c.db.Where("id IN ?", ids).Delete(&model.AiImageAsset{})
	logger.Infof("reference cleaner: deleted %d expired reference assets", len(ids))
}
