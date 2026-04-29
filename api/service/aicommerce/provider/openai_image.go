package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	modelNanoBanana = "nano-banana"
	modelGPTImage2  = "gpt-image-2"
)

type OpenAIImageClient struct {
	client openai.Client
	model  string
}

func NewOpenAIImageClient(baseURL, apiKey, modelName string) *OpenAIImageClient {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithRequestTimeout(120 * time.Second),
	}
	if strings.TrimSpace(baseURL) != "" {
		opts = append(opts, option.WithBaseURL(strings.TrimRight(baseURL, "/")))
	}

	return &OpenAIImageClient{
		client: openai.NewClient(opts...),
		model:  modelName,
	}
}

func (c *OpenAIImageClient) TextToImage(ctx context.Context, req TextToImageReq) (*GenerateResult, error) {
	return c.generate(ctx, req, nil)
}

func (c *OpenAIImageClient) ImageToImage(ctx context.Context, req ImageToImageReq) (*GenerateResult, error) {
	if strings.TrimSpace(req.ImageURL) == "" {
		return nil, fmt.Errorf("image-to-image source image is empty")
	}
	if req.Strength < 0 || req.Strength > 1 {
		return nil, fmt.Errorf("image-to-image strength must be between 0 and 1, got %v", req.Strength)
	}
	textReq := TextToImageReq{
		Prompt:         req.Prompt,
		NegativePrompt: req.NegativePrompt,
		ImageSize:      req.ImageSize,
		BatchSize:      1,
		GuidanceScale:  req.GuidanceScale,
	}

	extraOpts := []option.RequestOption{
		option.WithJSONSet("image_url", req.ImageURL),
	}
	if req.Strength > 0 {
		extraOpts = append(extraOpts, option.WithJSONSet("strength", req.Strength))
	}

	return c.generate(ctx, textReq, extraOpts)
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

	opts := make([]option.RequestOption, 0, len(extraOpts)+4)
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
	return strings.EqualFold(strings.TrimSpace(model), modelNanoBanana)
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
