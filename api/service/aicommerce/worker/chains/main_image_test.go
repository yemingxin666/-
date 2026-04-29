package chains_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"geekai/service/aicommerce/provider"
)

// testSiliconFlowServer 创建返回成功响应的 mock SiliconFlow server
func testSiliconFlowServer(t *testing.T) *httptest.Server {
	t.Helper()
	result := provider.GenerateResult{
		Images: []struct {
			URL       string `json:"url"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			TimingsMs int    `json:"timings_ms"`
		}{{URL: "https://cdn.example.com/generated.png", Width: 1024, Height: 1024}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// testTongyiServer 创建返回成功响应的 mock 通义千问 server
func testTongyiServer(t *testing.T) *httptest.Server {
	t.Helper()
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]interface{}{"content": "高品质真皮牛津鞋，舒适耐穿，时尚百搭"}},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSiliconFlow_MainImageChain_ProviderLevel 验证主图链 Provider 层正确调用
func TestSiliconFlow_MainImageChain_ProviderLevel(t *testing.T) {
	sfSrv := testSiliconFlowServer(t)

	sf := provider.NewSiliconFlow(sfSrv.URL, "test-key")
	result, err := sf.TextToImage(context.Background(), provider.TextToImageReq{
		Model:     "kolors",
		Prompt:    "高端真皮牛津鞋，白底展示，专业摄影",
		ImageSize: "1024x1024",
	})
	if err != nil {
		t.Fatalf("text-to-image failed: %v", err)
	}
	if len(result.Images) == 0 || result.Images[0].URL == "" {
		t.Fatal("expected non-empty image URL")
	}
	t.Logf("generated image URL: %s", result.Images[0].URL)
}

// TestSiliconFlow_CloneChain_ProviderLevel 验证克隆链图生图 Provider 层
func TestSiliconFlow_CloneChain_ProviderLevel(t *testing.T) {
	sfSrv := testSiliconFlowServer(t)

	sf := provider.NewSiliconFlow(sfSrv.URL, "test-key")
	result, err := sf.ImageToImage(context.Background(), provider.ImageToImageReq{
		Model:     "kolors",
		Prompt:    "极简白底风格",
		ImageURL:  "https://example.com/ref.jpg",
		ImageSize: "1024x1024",
		Strength:  0.7,
	})
	if err != nil {
		t.Fatalf("image-to-image failed: %v", err)
	}
	if len(result.Images) == 0 {
		t.Fatal("expected at least one image")
	}
}

// TestSiliconFlow_Retry_5xx 验证 Worker 在 Provider 返回 5xx 时的重试行为
func TestSiliconFlow_Retry_5xx(t *testing.T) {
	var callCount int32
	successResp := provider.GenerateResult{
		Images: []struct {
			URL       string `json:"url"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			TimingsMs int    `json:"timings_ms"`
		}{{URL: "https://cdn.example.com/after-retry.png"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n <= 2 {
			// 前两次返回 503
			http.Error(w, `{"error":"service overloaded"}`, http.StatusServiceUnavailable)
			return
		}
		// 第三次成功
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(successResp)
	}))
	t.Cleanup(srv.Close)

	sf := provider.NewSiliconFlow(srv.URL, "test-key")
	result, err := sf.TextToImage(context.Background(), provider.TextToImageReq{
		Model:     "kolors",
		Prompt:    "test retry",
		ImageSize: "1024x1024",
	})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if result.Images[0].URL != successResp.Images[0].URL {
		t.Errorf("unexpected image URL: %s", result.Images[0].URL)
	}

	calls := atomic.LoadInt32(&callCount)
	if calls != 3 {
		t.Errorf("expected exactly 3 HTTP calls (2 failures + 1 success), got %d", calls)
	}
}

// TestSiliconFlow_Timeout_AllFail 验证所有重试失败时错误正确传播
func TestSiliconFlow_Timeout_AllFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"gateway timeout"}`, http.StatusGatewayTimeout)
	}))
	t.Cleanup(srv.Close)

	sf := provider.NewSiliconFlow(srv.URL, "test-key")
	_, err := sf.TextToImage(context.Background(), provider.TextToImageReq{
		Model:     "kolors",
		Prompt:    "always fail",
		ImageSize: "1024x1024",
	})
	if err == nil {
		t.Fatal("expected error when all retries fail, got nil")
	}
}
