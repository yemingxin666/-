package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/alimt"
)

// AliyunTranslate 阿里云机器翻译图片翻译客户端（TranslateImage API）。
// 一站式完成 OCR → 翻译 → 文字重渲染，返回已翻译的成品图 URL。
type AliyunTranslate struct {
	accessKeyID     string
	accessKeySecret string
	region          string
	client          *alimt.Client
}

func NewAliyunTranslate(accessKeyID, accessKeySecret string, region ...string) *AliyunTranslate {
	r := "cn-hangzhou"
	if len(region) > 0 && strings.TrimSpace(region[0]) != "" {
		r = strings.TrimSpace(region[0])
	}
	t := &AliyunTranslate{
		accessKeyID:     strings.TrimSpace(accessKeyID),
		accessKeySecret: strings.TrimSpace(accessKeySecret),
		region:          r,
	}
	if t.accessKeyID != "" && t.accessKeySecret != "" {
		if c, err := alimt.NewClientWithAccessKey(r, t.accessKeyID, t.accessKeySecret); err == nil {
			t.client = c
		}
	}
	return t
}

// TranslateImage 调用阿里云 TranslateImage API，返回翻译后图片的临时 URL。
// 调用方需要在 URL 过期前下载并上传到自有 OSS。
func (t *AliyunTranslate) TranslateImage(ctx context.Context, imageURL, srcLang, tgtLang string) (string, error) {
	if t == nil || t.client == nil {
		return "", fmt.Errorf("aliyun translate not configured: 请在 config.toml [AiCommerce] 节填写 aliyun_translate_access_key_id / _secret")
	}
	if strings.TrimSpace(imageURL) == "" {
		return "", fmt.Errorf("aliyun translate: empty image url")
	}

	req := alimt.CreateTranslateImageRequest()
	req.Scheme = "https"
	req.ImageUrl = imageURL
	req.SourceLanguage = alimtLangCode(srcLang)
	req.TargetLanguage = alimtLangCode(tgtLang)
	req.Field = "e-commerce"
	req.SetReadTimeout(120 * time.Second)
	req.SetConnectTimeout(10 * time.Second)

	done := make(chan struct{})
	var (
		resp *alimt.TranslateImageResponse
		err  error
	)
	go func() {
		resp, err = t.client.TranslateImage(req)
		close(done)
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-done:
	}

	if err != nil {
		return "", fmt.Errorf("aliyun translate image: %w", err)
	}
	if resp.Code != 200 {
		return "", fmt.Errorf("aliyun translate image: code=%d msg=%s requestId=%s", resp.Code, resp.Message, resp.RequestId)
	}

	finalURL := resp.Data.FinalImageUrl
	if finalURL == "" {
		return "", fmt.Errorf("aliyun translate image: empty FinalImageUrl, requestId=%s", resp.RequestId)
	}
	return finalURL, nil
}

// alimtLangCode 将前端语言代码映射为阿里云机器翻译语言代码
func alimtLangCode(lang string) string {
	m := map[string]string{
		"zh": "zh", "en": "en", "ja": "ja", "ko": "ko",
		"de": "de", "fr": "fr", "es": "es", "pt": "pt",
		"ru": "ru", "it": "it", "ar": "ar", "th": "th",
		"vi": "vi", "auto": "auto",
	}
	if code, ok := m[strings.ToLower(lang)]; ok {
		return code
	}
	return lang
}
