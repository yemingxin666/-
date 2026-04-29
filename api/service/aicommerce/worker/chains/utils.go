package chains

import (
	"fmt"
	"geekai/service/aicommerce"
	"geekai/service/oss"
	"geekai/store/model"

	"gorm.io/gorm"
)

func updateProgress(db *gorm.DB, task *model.AiImageTask, progress int) {
	db.Model(task).Update("progress", progress)
}

func signedURL(ossKey string, cfg aicommerce.Config) string {
	return fmt.Sprintf("https://%s.oss-cn-hangzhou.aliyuncs.com/%s", cfg.OSSBucket, ossKey)
}

func extractStringSlice(m model.JSONMap, key string) ([]string, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	raw, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result, true
}

// ossUploadURL 从外部 URL 下载图片并上传到 OSS，返回 ossKey
func ossUploadURL(uploader oss.Uploader, imgURL string) (string, error) {
	ossKey, err := uploader.PutUrlFile(imgURL, "png", false)
	if err != nil {
		return "", fmt.Errorf("upload image to OSS: %w", err)
	}
	return ossKey, nil
}
