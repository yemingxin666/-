package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	modelQwenImageEdit     = "qwen-image-edit"
	maxReferenceImageBytes = 50 * 1024 * 1024
)

type QwenEditClient struct {
	client     openai.Client
	httpClient *http.Client
	model      string
}

func NewQwenEditClient(baseURL, apiKey, modelName string) *QwenEditClient {
	httpClient := &http.Client{Timeout: 120 * time.Second}
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(httpClient),
		option.WithRequestTimeout(120 * time.Second),
	}
	if strings.TrimSpace(baseURL) != "" {
		opts = append(opts, option.WithBaseURL(strings.TrimRight(baseURL, "/")))
	}

	return &QwenEditClient{
		client:     openai.NewClient(opts...),
		httpClient: httpClient,
		model:      modelName,
	}
}

func (c *QwenEditClient) TextToImage(ctx context.Context, req TextToImageReq) (*GenerateResult, error) {
	return nil, fmt.Errorf("%s does not support text-to-image generation", c.model)
}

func (c *QwenEditClient) ImageToImage(ctx context.Context, req ImageToImageReq) (*GenerateResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("image edit prompt is empty")
	}
	if strings.TrimSpace(req.ImageURL) == "" {
		return nil, fmt.Errorf("image edit source image is empty")
	}

	imageBytes, err := c.downloadReferenceImage(ctx, req.ImageURL)
	if err != nil {
		return nil, err
	}

	params := openai.ImageEditParams{
		Model:          openai.ImageModel(c.model),
		Prompt:         req.Prompt,
		Image:          openai.ImageEditParamsImageUnion{OfFile: bytes.NewReader(imageBytes)},
		ResponseFormat: openai.ImageEditParamsResponseFormatURL,
	}

	resp, err := c.client.Images.Edit(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("qwen image edit: %w", err)
	}

	width, height := imageSizeDimensions(req.ImageSize)
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
		return nil, fmt.Errorf("qwen image edit returned no usable image URLs")
	}

	return result, nil
}

func (c *QwenEditClient) downloadReferenceImage(ctx context.Context, imageURL string) ([]byte, error) {
	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		return nil, fmt.Errorf("invalid source image url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("source image url scheme must be http or https")
	}
	host := parsedURL.Hostname()
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, fmt.Errorf("resolve source image host: %w", err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return nil, fmt.Errorf("source image host resolves to a private/reserved address")
		}
		if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
			return nil, fmt.Errorf("source image host resolves to a private/reserved address")
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	httpClient := *c.httpClient
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("download source image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download source image: status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxReferenceImageBytes {
		return nil, fmt.Errorf("source image exceeds %d bytes", maxReferenceImageBytes)
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if contentType != "" && !strings.HasPrefix(contentType, "image/") && contentType != "application/octet-stream" {
		return nil, fmt.Errorf("source image content-type is not image: %s", contentType)
	}

	limited := io.LimitReader(resp.Body, maxReferenceImageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read source image: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("source image is empty")
	}
	if len(data) > maxReferenceImageBytes {
		return nil, fmt.Errorf("source image exceeds %d bytes", maxReferenceImageBytes)
	}
	return data, nil
}
