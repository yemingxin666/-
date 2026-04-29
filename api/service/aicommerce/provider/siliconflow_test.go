package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"geekai/service/aicommerce/provider"
)

// mockServer 创建可控响应的 mock HTTP server
func mockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestSiliconFlow_TextToImage_Success(t *testing.T) {
	expected := provider.GenerateResult{
		Images: []struct {
			URL       string `json:"url"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			TimingsMs int    `json:"timings_ms"`
		}{{URL: "https://example.com/img.png", Width: 1024, Height: 1024}},
	}
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	})

	sf := provider.NewSiliconFlow(srv.URL, "test-key")
	result, err := sf.TextToImage(context.Background(), provider.TextToImageReq{
		Model:     "kolors",
		Prompt:    "a red shoe",
		ImageSize: "1024x1024",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Images) == 0 {
		t.Fatal("expected at least one image")
	}
	if result.Images[0].URL != expected.Images[0].URL {
		t.Errorf("got URL %q, want %q", result.Images[0].URL, expected.Images[0].URL)
	}
}

// TestSiliconFlow_Retry_SucceedsAfterTransientFailures 验证指数退避重试：前 2 次 5xx，第 3 次成功
func TestSiliconFlow_Retry_SucceedsAfterTransientFailures(t *testing.T) {
	var calls int32
	success := provider.GenerateResult{
		Images: []struct {
			URL       string `json:"url"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			TimingsMs int    `json:"timings_ms"`
		}{{URL: "https://example.com/retry.png"}},
	}
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(success)
	})

	sf := provider.NewSiliconFlow(srv.URL, "test-key")
	result, err := sf.TextToImage(context.Background(), provider.TextToImageReq{
		Model:     "kolors",
		Prompt:    "test",
		ImageSize: "1024x1024",
	})
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if result.Images[0].URL != success.Images[0].URL {
		t.Errorf("unexpected URL: %s", result.Images[0].URL)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

// TestSiliconFlow_Retry_ExhaustsAll 验证超过最大重试次数时返回错误
func TestSiliconFlow_Retry_ExhaustsAll(t *testing.T) {
	var calls int32
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, `{"error":"overloaded"}`, http.StatusTooManyRequests)
	})

	sf := provider.NewSiliconFlow(srv.URL, "test-key")
	_, err := sf.TextToImage(context.Background(), provider.TextToImageReq{
		Model:     "kolors",
		Prompt:    "fail",
		ImageSize: "1024x1024",
	})
	if err == nil {
		t.Fatal("expected error after max retries, got nil")
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Errorf("expected 3 retry attempts, got %d", calls)
	}
}

// TestSiliconFlow_ContextCancellation 验证 context 取消时立即退出
func TestSiliconFlow_ContextCancellation(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "err", http.StatusInternalServerError)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	sf := provider.NewSiliconFlow(srv.URL, "test-key")
	_, err := sf.TextToImage(ctx, provider.TextToImageReq{
		Model:     "kolors",
		Prompt:    "test",
		ImageSize: "1024x1024",
	})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestRatioToSize(t *testing.T) {
	cases := []struct{ ratio, want string }{
		{"1:1", "1024x1024"},
		{"16:9", "1280x720"},
		{"9:16", "720x1280"},
		{"4:3", "1024x768"},
		{"3:4", "768x1024"},
		{"unknown", "1024x1024"},
	}
	for _, c := range cases {
		got := provider.RatioToSize(c.ratio)
		if got != c.want {
			t.Errorf("RatioToSize(%q) = %q, want %q", c.ratio, got, c.want)
		}
	}
}
