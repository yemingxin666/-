package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// BaiduOCR 百度高精度 OCR
type BaiduOCR struct {
	appID     string
	apiKey    string
	secretKey string
	client    *http.Client
	token     string
	tokenExp  time.Time
	mu        sync.Mutex
}

func NewBaiduOCR(appID, apiKey, secretKey string) *BaiduOCR {
	return &BaiduOCR{
		appID:     appID,
		apiKey:    apiKey,
		secretKey: secretKey,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

type OCRWord struct {
	Words    string `json:"words"`
	Location struct {
		Left   int `json:"left"`
		Top    int `json:"top"`
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"location"`
}

type OCRResult struct {
	WordsResult []OCRWord `json:"words_result"`
}

// Recognize 识别图片中的文字及坐标
func (b *BaiduOCR) Recognize(ctx context.Context, imageURL string) ([]OCRWord, error) {
	token, err := b.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	params := url.Values{
		"url":                  {imageURL},
		"recognize_granularity": {"big"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://aip.baidubce.com/rest/2.0/ocr/v1/accurate?access_token="+token,
		bytes.NewBufferString(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result OCRResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.WordsResult, nil
}

func (b *BaiduOCR) getAccessToken(ctx context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.token != "" && time.Now().Before(b.tokenExp) {
		return b.token, nil
	}
	params := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {b.apiKey},
		"client_secret": {b.secretKey},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://aip.baidubce.com/oauth/2.0/token",
		bytes.NewBufferString(params.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}
	b.token = tokenResp.AccessToken
	b.tokenExp = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)
	return b.token, nil
}

// BaiduTranslate 百度翻译
type BaiduTranslate struct {
	appID  string
	secret string
	client *http.Client
}

func NewBaiduTranslate(appID, secret string) *BaiduTranslate {
	return &BaiduTranslate{
		appID:  appID,
		secret: secret,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type TranslateResult struct {
	TransResult []struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	} `json:"trans_result"`
}

// Translate 翻译文本，from/to 为语言代码（auto/zh/en/ja/ko等）
func (t *BaiduTranslate) Translate(ctx context.Context, text, from, to string) (string, error) {
	salt := fmt.Sprintf("%d", time.Now().UnixNano())
	sign := md5Sum(t.appID + text + salt + t.secret)

	params := url.Values{
		"q":     {text},
		"from":  {from},
		"to":    {to},
		"appid": {t.appID},
		"salt":  {salt},
		"sign":  {sign},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://fanyi-api.baidu.com/api/trans/vip/translate",
		bytes.NewBufferString(params.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result TranslateResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.TransResult) == 0 {
		return "", fmt.Errorf("baidu translate: empty result for %q", text)
	}
	return result.TransResult[0].Dst, nil
}
