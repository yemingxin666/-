package worker

import (
	"context"
	"fmt"
	"geekai/store/model"
	"time"

	"gorm.io/gorm"
)

const (
	timeoutRefundMessage = "任务执行超时，系统已自动退款"
	panicRefundMessage   = "任务执行异常，系统已自动退款"
)

func failTaskAndRefundWithRetry(ctx context.Context, db *gorm.DB, taskID uint, expectedStatuses []string, errorMessage string) (bool, error) {
	changed, err := failTaskAndRefund(ctx, db, taskID, expectedStatuses, errorMessage)
	if err == nil {
		return changed, nil
	}
	logger.Warnf("failTaskAndRefund retry task_id=%d err=%v", taskID, err)
	time.Sleep(100 * time.Millisecond)
	return failTaskAndRefund(ctx, db, taskID, expectedStatuses, errorMessage)
}

func failTaskAndRefund(ctx context.Context, db *gorm.DB, taskID uint, expectedStatuses []string, errorMessage string) (bool, error) {
	var changed bool
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.AiImageTask
		if err := tx.First(&task, taskID).Error; err != nil {
			return fmt.Errorf("load task: %w", err)
		}

		now := time.Now()
		result := tx.Model(&model.AiImageTask{}).
			Where("id = ? AND status IN ? AND finished_at IS NULL", task.Id, expectedStatuses).
			Updates(map[string]interface{}{
				"status":        model.TaskStatusFailed,
				"error_message": errorMessage,
				"finished_at":   now,
				"updated_at":    now,
			})
		if result.Error != nil {
			return fmt.Errorf("mark failed: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}
		changed = true

		if task.CreditCost <= 0 {
			return nil
		}
		userResult := tx.Model(&model.User{}).
			Where("id = ?", task.UserId).
			UpdateColumn("power", gorm.Expr("power + ?", task.CreditCost))
		if userResult.Error != nil {
			return fmt.Errorf("refund power: %w", userResult.Error)
		}
		if userResult.RowsAffected == 0 {
			return fmt.Errorf("user %d not found, cannot refund", task.UserId)
		}
		return nil
	})
	return changed, err
}

func succeedTaskWithRetry(ctx context.Context, db *gorm.DB, taskID uint) (bool, error) {
	now := time.Now()
	values := map[string]interface{}{
		"status":      model.TaskStatusSucceeded,
		"progress":    100,
		"finished_at": now,
		"updated_at":  now,
	}

	changed, err := casUpdateRunning(ctx, db, taskID, values)
	if err == nil {
		return changed, nil
	}
	logger.Warnf("succeedTask retry task_id=%d err=%v", taskID, err)
	time.Sleep(100 * time.Millisecond)
	return casUpdateRunning(ctx, db, taskID, values)
}

func casUpdateRunning(ctx context.Context, db *gorm.DB, taskID uint, values map[string]interface{}) (bool, error) {
	result := db.WithContext(ctx).Model(&model.AiImageTask{}).
		Where("id = ? AND status = ? AND finished_at IS NULL", taskID, model.TaskStatusRunning).
		Updates(values)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
