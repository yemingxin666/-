package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	modelNanoBanana = "nano-banana"
	modelGPTImage2  = "gpt-image-2"
	// openAIImagePerImageTimeout 每张参考图分摊的超时预算。
	// 上游 chat/completions 图生图模型耗时大致与 content blocks 中的图数线性相关。
	openAIImagePerImageTimeout = 4 * time.Minute
	// openAIImageMinTimeout 至少给出的请求超时（覆盖网络抖动）。
	openAIImageMinTimeout = 4 * time.Minute
)

type OpenAIImageClient struct {
	client     openai.Client
	httpClient *http.Client
	baseURL    string
	apiKey     string
	model      string
}

func NewOpenAIImageClient(baseURL, apiKey, modelName string) *OpenAIImageClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	// http.Client.Timeout 留空：超时由每次请求的 ctx WithTimeout 控制（按图数动态计算）
	return &OpenAIImageClient{
		client:     openai.NewClient(opts...),
		httpClient: &http.Client{},
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      modelName,
	}
}

func (c *OpenAIImageClient) TextToImage(ctx context.Context, req TextToImageReq) (*GenerateResult, error) {
	return c.generate(ctx, req, nil)
}

// ImageToImage 使用 chat/completions 接口实现图生图（参考图直接传 URL）
func (c *OpenAIImageClient) ImageToImage(ctx context.Context, req ImageToImageReq) (*GenerateResult, error) {
	if strings.TrimSpace(req.ImageURL) == "" {
		return nil, fmt.Errorf("image-to-image source image is empty")
	}

	imageSize := req.ImageSize
	if strings.TrimSpace(imageSize) == "" {
		imageSize = "1024x1024"
	}

	content := []map[string]interface{}{
		{"type": "text", "text": req.Prompt},
		{"type": "image_url", "image_url": map[string]string{"url": req.ImageURL}},
	}
	imageCount := 1
	// 追加额外参考图（克隆设计：产品图随风格图一同送入，由 prompt 中的角色标注约束模型）
	for _, extra := range req.ExtraImageURLs {
		if strings.TrimSpace(extra) == "" {
			continue
		}
		content = append(content, map[string]interface{}{
			"type":      "image_url",
			"image_url": map[string]string{"url": extra},
		})
		imageCount++
	}

	// 按图数动态分配超时：每张图 90s，至少 90s
	timeout := time.Duration(imageCount) * openAIImagePerImageTimeout
	if timeout < openAIImageMinTimeout {
		timeout = openAIImageMinTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body := map[string]interface{}{
		"model":    c.model,
		"stream":   false,
		"quality":  "1k",
		"messages": []map[string]interface{}{{"role": "user", "content": content}},
	}
	if isNanoBananaModel(c.model) {
		body["aspect_ratio"] = imageSizeToAspectRatio(imageSize)
	} else {
		body["size"] = imageSize
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai image generation: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(respBody))
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return nil, fmt.Errorf("openai image generation: status %d: %s", resp.StatusCode, snippet)
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}
	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("image generation returned empty content")
	}

	// 从 markdown 格式 ![image](url) 中提取图片 URL
	imgURL := extractMarkdownImageURL(chatResp.Choices[0].Message.Content)
	if imgURL == "" {
		return nil, fmt.Errorf("image generation returned no image URL in content: %s", chatResp.Choices[0].Message.Content)
	}

	width, height := imageSizeDimensions(imageSize)
	return &GenerateResult{
		Images: []struct {
			URL       string `json:"url"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			TimingsMs int    `json:"timings_ms"`
		}{{URL: imgURL, Width: width, Height: height}},
	}, nil
}

// extractMarkdownImageURL 从 ![image](url) 格式中提取 URL
func extractMarkdownImageURL(content string) string {
	content = strings.TrimSpace(content)
	start := strings.Index(content, "![")
	if start < 0 {
		return ""
	}
	urlStart := strings.Index(content[start:], "](")
	if urlStart < 0 {
		return ""
	}
	urlStart += start + 2
	urlEnd := strings.Index(content[urlStart:], ")")
	if urlEnd < 0 {
		return ""
	}
	return strings.TrimSpace(content[urlStart : urlStart+urlEnd])
}

func (c *OpenAIImageClient) generate(ctx context.Context, req TextToImageReq, extraOpts []option.RequestOption) (*GenerateResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("image prompt is empty")
	}
	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	imageSize := req.ImageSize
	if strings.TrimSpace(imageSize) == "" {
		imageSize = "1024x1024"
	}

	params := openai.ImageGenerateParams{
		Model:  openai.ImageModel(c.model),
		Prompt: req.Prompt,
		N:      openai.Int(int64(batchSize)),
	}

	opts := make([]option.RequestOption, 0, len(extraOpts)+5)
	opts = append(opts, option.WithRequestTimeout(openAIImagePerImageTimeout))
	if isNanoBananaModel(c.model) {
		opts = append(opts, option.WithJSONSet("aspect_ratio", imageSizeToAspectRatio(imageSize)))
	} else {
		params.Size = openai.ImageGenerateParamsSize(imageSize)
	}
	if requiresURLResponseFormat(c.model) {
		params.ResponseFormat = openai.ImageGenerateParamsResponseFormatURL
	}
	if req.NegativePrompt != "" {
		opts = append(opts, option.WithJSONSet("negative_prompt", req.NegativePrompt))
	}
	if req.GuidanceScale > 0 {
		opts = append(opts, option.WithJSONSet("guidance_scale", req.GuidanceScale))
	}
	opts = append(opts, extraOpts...)

	resp, err := c.client.Images.Generate(ctx, params, opts...)
	if err != nil {
		return nil, fmt.Errorf("openai image generation: %w", err)
	}

	width, height := imageSizeDimensions(imageSize)
	result := &GenerateResult{}
	for _, img := range resp.Data {
		if strings.TrimSpace(img.URL) == "" {
			continue
		}
		result.Images = append(result.Images, struct {
			URL       string `json:"url"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			TimingsMs int    `json:"timings_ms"`
		}{
			URL:    img.URL,
			Width:  width,
			Height: height,
		})
	}
	if len(result.Images) == 0 {
		return nil, fmt.Errorf("image generation returned no usable images")
	}

	return result, nil
}

func imageSizeDimensions(size string) (int, int) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0
	}
	return width, height
}

func isNanoBananaModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), modelNanoBanana)
}

func requiresURLResponseFormat(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case modelNanoBanana, modelGPTImage2:
		return true
	default:
		return false
	}
}

func imageSizeToAspectRatio(size string) string {
	size = strings.TrimSpace(size)
	if strings.Contains(size, ":") {
		return size
	}
	sizes := map[string]string{
		"1024x1024": "1:1",
		"1024x768":  "4:3",
		"768x1024":  "3:4",
		"1280x720":  "16:9",
		"720x1280":  "9:16",
		"1200x800":  "3:2",
		"800x1200":  "2:3",
		"1024x1280": "4:5",
		"1280x1024": "5:4",
		"1680x720":  "21:9",
	}
	if ratio, ok := sizes[size]; ok {
		return ratio
	}
	return "1:1"
}
