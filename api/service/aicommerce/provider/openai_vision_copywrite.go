package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type visionTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type visionImageURL struct {
	URL string `json:"url"`
}

type visionImagePart struct {
	Type     string         `json:"type"`
	ImageURL visionImageURL `json:"image_url"`
}

type visionChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type visionChatReq struct {
	Model    string              `json:"model"`
	Messages []visionChatMessage `json:"messages"`
}

type visionChatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type OpenAIVisionCopywriter struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewOpenAIVisionCopywriter(baseURL, apiKey, model string) *OpenAIVisionCopywriter {
	return &OpenAIVisionCopywriter{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		client:  &http.Client{Timeout: 45 * time.Second},
	}
}

func (c *OpenAIVisionCopywriter) GenerateCopywrite(ctx context.Context, productName, hint string, imageURLs []string) (string, error) {
	if c.baseURL == "" {
		return "", fmt.Errorf("vision copywrite baseURL is empty")
	}
	if c.apiKey == "" {
		return "", fmt.Errorf("vision copywrite apiKey is empty")
	}
	if c.model == "" {
		return "", fmt.Errorf("vision copywrite model is empty")
	}

	imageURLs = normalizeVisionImageURLs(imageURLs)
	if len(imageURLs) == 0 {
		return "", fmt.Errorf("vision copywrite requires at least one image")
	}
	if len(imageURLs) > 3 {
		return "", fmt.Errorf("vision copywrite supports at most 3 images")
	}

	systemPrompt := "你是专业的电商文案撰写师。请分析图片中的商品，严格按以下格式输出，不得添加任何额外内容、前言或总结：\n【商品品类】xxx\n【核心卖点】xxx\n【补充描述】xxx\n禁止虚构无法从图片中观察到的信息。"
	userText := fmt.Sprintf("商品名称：%s\n补充信息：%s", strings.TrimSpace(productName), strings.TrimSpace(hint))

	parts := make([]interface{}, 0, len(imageURLs)+1)
	parts = append(parts, visionTextPart{Type: "text", Text: userText})
	for _, imageURL := range imageURLs {
		parts = append(parts, visionImagePart{
			Type:     "image_url",
			ImageURL: visionImageURL{URL: imageURL},
		})
	}

	req := visionChatReq{
		Model: c.model,
		Messages: []visionChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: parts},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal vision copywrite request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read vision copywrite response: %w", err)
	}

	var result visionChatResp
	decodeErr := json.Unmarshal(body, &result)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("vision copywrite provider returned status %d", resp.StatusCode)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("decode vision copywrite response: %w", decodeErr)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("vision copywrite empty choices")
	}
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("vision copywrite empty content")
	}

	return content, nil
}

func normalizeVisionImageURLs(imageURLs []string) []string {
	result := make([]string, 0, len(imageURLs))
	for _, imageURL := range imageURLs {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			continue
		}
		result = append(result, imageURL)
	}
	return result
}
