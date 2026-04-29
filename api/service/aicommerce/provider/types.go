package provider

import "context"

// ImageClient unified interface for image generation and editing clients.
type ImageClient interface {
	TextToImage(ctx context.Context, req TextToImageReq) (*GenerateResult, error)
	ImageToImage(ctx context.Context, req ImageToImageReq) (*GenerateResult, error)
}
