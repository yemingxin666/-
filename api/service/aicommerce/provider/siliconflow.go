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

type SiliconFlow struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewSiliconFlow(baseURL, apiKey string) *SiliconFlow {
	return &SiliconFlow{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type TextToImageReq struct {
	Model          string  `json:"model"`
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negative_prompt,omitempty"`
	ImageSize      string  `json:"image_size"`           // e.g. "1024x1024"
	BatchSize      int     `json:"batch_size,omitempty"` // 默认 1
	GuidanceScale  float64 `json:"guidance_scale,omitempty"`
	NumInferSteps  int     `json:"num_inference_steps,omitempty"`
	Seed           int64   `json:"seed,omitempty"`
}

type ImageToImageReq struct {
	Model          string  `json:"model"`
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negative_prompt,omitempty"`
	ImageURL       string  `json:"image_url"` // 参考图 URL
	ImageSize      string  `json:"image_size"`
	Strength       float64 `json:"strength,omitempty"` // 风格迁移强度 0-1
	GuidanceScale  float64 `json:"guidance_scale,omitempty"`
}

type GenerateResult struct {
	Images []struct {
		URL       string `json:"url"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		TimingsMs int    `json:"timings_ms"`
	} `json:"images"`
	Seed int64 `json:"seed"`
}

// TextToImage 文生图
func (s *SiliconFlow) TextToImage(ctx context.Context, req TextToImageReq) (*GenerateResult, error) {
	return s.postWithRetry(ctx, "/images/generations", req)
}

// ImageToImage 图生图（克隆设计）
func (s *SiliconFlow) ImageToImage(ctx context.Context, req ImageToImageReq) (*GenerateResult, error) {
	return s.postWithRetry(ctx, "/images/generations", req)
}

func (s *SiliconFlow) postWithRetry(ctx context.Context, path string, body interface{}) (*GenerateResult, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		result, err := s.post(ctx, path, body)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("siliconflow: max retries exceeded: %w", lastErr)
}

func (s *SiliconFlow) post(ctx context.Context, path string, body interface{}) (*GenerateResult, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("siliconflow: status %d: %s", resp.StatusCode, respBody)
	}

	var result GenerateResult
	return &result, json.Unmarshal(respBody, &result)
}

// RatioToSize 将比例字符串转换为 SiliconFlow 支持的分辨率
func RatioToSize(ratio string) string {
	sizes := map[string]string{
		"1:1":  "1024x1024",
		"4:3":  "1024x768",
		"3:4":  "768x1024",
		"16:9": "1280x720",
		"9:16": "720x1280",
		"3:2":  "1200x800",
		"2:3":  "800x1200",
		"4:5":  "1024x1280",
		"5:4":  "1280x1024",
		"21:9": "1680x720",
	}
	if s, ok := sizes[ratio]; ok {
		return s
	}
	return "1024x1024"
}
