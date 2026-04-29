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

type Tongyi struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewTongyi(baseURL, apiKey, model string) *Tongyi {
	return &Tongyi{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// GenerateCopywrite 根据商品品类和核心描述生成结构化卖点文案
func (t *Tongyi) GenerateCopywrite(ctx context.Context, productName, hint string) (string, error) {
	systemPrompt := `你是一位专业的电商文案撰写师。
请根据用户提供的商品信息，生成结构化的商品卖点文案，格式如下：
【商品品类】xxx
【核心卖点】xxx
【补充描述】xxx
文案需简洁、突出价值、适合电商主图展示。`

	userContent := fmt.Sprintf("商品名称：%s\n补充信息：%s", productName, hint)

	req := chatReq{
		Model: t.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
	}

	data, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+t.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tongyi: status %d: %s", resp.StatusCode, body)
	}

	var result chatResp
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("tongyi: empty response")
	}
	return result.Choices[0].Message.Content, nil
}
