package worker

import (
	"context"
	"encoding/json"
	"geekai/service/aicommerce"
	"geekai/store/model"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type OrphanReconciler struct {
	db        *gorm.DB
	rdb       *redis.Client
	cfg       aicommerce.Config
	interval  time.Duration
	staleAge  time.Duration
	batchSize int
}

func NewOrphanReconciler(db *gorm.DB, rdb *redis.Client, cfg aicommerce.Config) *OrphanReconciler {
	return &OrphanReconciler{
		db:        db,
		rdb:       rdb,
		cfg:       cfg,
		interval:  60 * time.Second,
		staleAge:  2 * time.Minute,
		batchSize: 100,
	}
}

func (r *OrphanReconciler) Run(ctx context.Context) {
	logger.Infof("OrphanReconciler started, interval=%s, stale_age=%s", r.interval, r.staleAge)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.reconcile(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

func (r *OrphanReconciler) reconcile(ctx context.Context) {
	r.requeueStaleQueued(ctx)
	r.failStalePending(ctx)
}

func (r *OrphanReconciler) requeueStaleQueued(ctx context.Context) {
	cutoff := time.Now().Add(-r.staleAge)
	var tasks []model.AiImageTask
	if err := r.db.WithContext(ctx).
		Where("status = ? AND started_at IS NULL AND updated_at < ? AND deleted_at IS NULL",
			model.TaskStatusQueued, cutoff).
		Limit(r.batchSize).
		Find(&tasks).Error; err != nil {
		logger.Errorf("OrphanReconciler: query stale queued failed: %v", err)
		return
	}

	for _, task := range tasks {
		payload, _ := json.Marshal(map[string]interface{}{
			"task_id": task.Id,
			"task_no": task.TaskNo,
		})
		if err := r.rdb.LPush(ctx, r.cfg.QueueName, payload).Err(); err != nil {
			logger.Errorf("OrphanReconciler: re-enqueue failed task_no=%s: %v", task.TaskNo, err)
			continue
		}
		// touch updated_at to throttle re-enqueue (next scan skips until stale again)
		r.db.WithContext(ctx).
			Model(&model.AiImageTask{}).
			Where("id = ? AND status = ? AND started_at IS NULL", task.Id, model.TaskStatusQueued).
			Update("updated_at", time.Now())
		logger.Infof("OrphanReconciler: re-enqueued stale queued task_no=%s", task.TaskNo)
	}
}

func (r *OrphanReconciler) failStalePending(ctx context.Context) {
	cutoff := time.Now().Add(-r.staleAge)
	var tasks []model.AiImageTask
	if err := r.db.WithContext(ctx).
		Where("status = ? AND created_at < ? AND deleted_at IS NULL",
			model.TaskStatusPending, cutoff).
		Limit(r.batchSize).
		Find(&tasks).Error; err != nil {
		logger.Errorf("OrphanReconciler: query stale pending failed: %v", err)
		return
	}

	for _, task := range tasks {
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&model.AiImageTask{}).
				Where("id = ? AND status = ?", task.Id, model.TaskStatusPending).
				Updates(map[string]interface{}{
					"status":        model.TaskStatusFailed,
					"error_message": "任务提交中断，系统已自动退款",
					"finished_at":   time.Now(),
					"updated_at":    time.Now(),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			if task.CreditCost <= 0 {
				return nil
			}
			return tx.Model(&model.User{}).
				Where("id = ?", task.UserId).
				UpdateColumn("power", gorm.Expr("power + ?", task.CreditCost)).Error
		})
		if err != nil {
			logger.Errorf("OrphanReconciler: fail+refund failed task_no=%s: %v", task.TaskNo, err)
		} else {
			logger.Infof("OrphanReconciler: failed stale pending task_no=%s, refunded %d", task.TaskNo, task.CreditCost)
		}
	}
}
