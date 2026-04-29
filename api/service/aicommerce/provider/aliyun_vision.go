package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AliyunVision 阿里云视觉智能 - 背景移除
type AliyunVision struct {
	accessKeyID     string
	accessKeySecret string
	client          *http.Client
}

func NewAliyunVision(accessKeyID, accessKeySecret string) *AliyunVision {
	return &AliyunVision{
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		client:          &http.Client{Timeout: 60 * time.Second},
	}
}

type segmentReq struct {
	URL string `json:"url"`
}

type segmentResp struct {
	Code      string `json:"Code"`
	Message   string `json:"Message"`
	RequestId string `json:"RequestId"`
	Data      struct {
		ImageURL string `json:"ImageURL"`
	} `json:"Data"`
}

// RemoveBackground 背景移除，返回透明背景图片 URL
func (a *AliyunVision) RemoveBackground(ctx context.Context, imageURL string) (string, error) {
	reqBody := segmentReq{URL: imageURL}
	data, _ := json.Marshal(reqBody)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://imageseg.cn-shanghai.aliyuncs.com/green/segment/body",
		bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// 简化版：生产环境需使用阿里云 SDK 签名
	// 此处通过 API Key 授权方式调用，实际部署时替换为正式签名
	httpReq.Header.Set("Authorization", fmt.Sprintf("AccessKey %s:%s", a.accessKeyID, a.accessKeySecret))

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("aliyun vision: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result segmentResp
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.Code != "200" {
		return "", fmt.Errorf("aliyun vision: %s", result.Message)
	}
	return result.Data.ImageURL, nil
}
