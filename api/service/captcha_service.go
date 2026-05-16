package service

// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// * Copyright 2023 The Geek-AI Authors. All rights reserved.
// * Use of this source code is governed by a Apache-2.0 license
// * that can be found in the LICENSE file.
// * @Author yangjian102621@163.com
// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"geekai/core/types"
	"image"
	"image/draw"
	"image/png"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/wenlng/go-captcha-assets/resources/images"
	"github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/slide"
)

const (
	captchaRedisKeyPrefix = "captcha:slide:"
	captchaTTL            = 180 * time.Second
	captchaTolerance      = 5
	captchaImageWidth     = 310
	captchaImageHeight    = 155
)

var captchaGetDelScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if value then
	redis.call("DEL", KEYS[1])
end
return value
`)

type CaptchaService struct {
	config       types.CaptchaConfig
	redisClient  *redis.Client
	slideCaptcha slide.Captcha
}

func NewCaptchaService(captchaConfig types.CaptchaConfig, redisClient *redis.Client) (*CaptchaService, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("redis client is nil")
	}

	slideCaptcha, err := newSlideCaptcha()
	if err != nil {
		return nil, fmt.Errorf("init slide captcha: %w", err)
	}

	return &CaptchaService{
		config:       normalizeCaptchaConfig(captchaConfig),
		redisClient:  redisClient,
		slideCaptcha: slideCaptcha,
	}, nil
}

func (s *CaptchaService) UpdateConfig(config types.CaptchaConfig) {
	s.config = normalizeCaptchaConfig(config)
}

func (s *CaptchaService) GetConfig() types.CaptchaConfig {
	return s.config
}

func (s *CaptchaService) SlideGet() (any, error) {
	captData, err := s.slideCaptcha.Generate()
	if err != nil {
		return nil, fmt.Errorf("generate slide captcha: %w", err)
	}

	blockData := captData.GetData()
	if blockData == nil {
		return nil, fmt.Errorf("generate slide captcha data is empty")
	}

	bgBase64, err := captData.GetMasterImage().ToBase64Data()
	if err != nil {
		return nil, fmt.Errorf("encode slide captcha background: %w", err)
	}
	bgImg := "data:image/jpeg;base64," + bgBase64

	// 将 tile 图合成到全高透明画布上，使前端 top:0 时能正确显示
	tileImg := captData.GetTileImage().Get()
	bkImg, err := composeTileOnCanvas(tileImg, blockData.Y, captchaImageHeight)
	if err != nil {
		return nil, fmt.Errorf("compose slide captcha tile: %w", err)
	}

	key := uuid.NewString()
	if err := s.redisClient.Set(context.Background(), captchaAnswerKey(key), strconv.Itoa(blockData.X), captchaTTL).Err(); err != nil {
		return nil, fmt.Errorf("store slide captcha answer: %w", err)
	}

	return map[string]any{
		"bgImg": bgImg,
		"bkImg": bkImg,
		"key":   key,
	}, nil
}

func (s *CaptchaService) SlideCheck(key string, x int) bool {
	key = strings.TrimSpace(key)
	if key == "" || x <= 0 {
		return false
	}

	answer, err := captchaGetDelScript.Run(context.Background(), s.redisClient, []string{captchaAnswerKey(key)}).Text()
	if err != nil {
		return false
	}

	answerX, err := strconv.Atoi(answer)
	if err != nil {
		return false
	}

	return absInt(x-answerX) <= captchaTolerance
}

func newSlideCaptcha() (slide.Captcha, error) {
	builder := slide.NewBuilder(
		slide.WithImageSize(option.Size{
			Width:  captchaImageWidth,
			Height: captchaImageHeight,
		}),
		slide.WithEnableGraphVerticalRandom(false),
		slide.WithGenGraphNumber(1),
	)

	backgrounds, err := images.GetImages()
	if err != nil {
		return nil, fmt.Errorf("load slide captcha backgrounds: %w", err)
	}

	graphs, err := tiles.GetTiles()
	if err != nil {
		return nil, fmt.Errorf("load slide captcha tiles: %w", err)
	}
	if len(backgrounds) == 0 || len(graphs) == 0 {
		return nil, fmt.Errorf("slide captcha resources are empty")
	}

	graphImages := make([]*slide.GraphImage, 0, len(graphs))
	for _, graph := range graphs {
		graphImages = append(graphImages, &slide.GraphImage{
			OverlayImage: graph.OverlayImage,
			MaskImage:    graph.MaskImage,
			ShadowImage:  graph.ShadowImage,
		})
	}

	builder.SetResources(
		slide.WithBackgrounds(backgrounds),
		slide.WithGraphImages(graphImages),
	)

	return builder.Make(), nil
}

func normalizeCaptchaConfig(config types.CaptchaConfig) types.CaptchaConfig {
	config.Type = "slide"
	return config
}

func captchaAnswerKey(key string) string {
	return captchaRedisKeyPrefix + key
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func composeTileOnCanvas(tileImg image.Image, y int, canvasHeight int) (string, error) {
	tileBounds := tileImg.Bounds()
	canvas := image.NewRGBA(image.Rect(0, 0, tileBounds.Dx(), canvasHeight))
	draw.Draw(canvas, image.Rect(0, y, tileBounds.Dx(), y+tileBounds.Dy()), tileImg, tileBounds.Min, draw.Over)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
