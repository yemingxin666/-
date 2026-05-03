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
	systemPrompt := `你是一位拥有15年以上电商经验的顶级文案策划师。

请根据用户提供的商品信息，提炼3条最有商业价值的卖点，优先级：材质 > 版型/外观 > 设计感 > 舒适性 > 使用场景。

严格按以下格式输出，不得添加任何额外内容、前言或总结：
【商品品类】填写具体品类名称
【核心卖点】3条卖点，每条格式为「标题：说明」，换行分隔
【补充描述】30字以内一句话总结，突出核心价值，适合电商主图展示`

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
