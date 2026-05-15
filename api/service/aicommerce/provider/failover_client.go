package provider

import (
	"context"
	"errors"
	"fmt"
	logger2 "geekai/logger"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/openai/openai-go"
)

const defaultEndpointTimeout = 120 * time.Second

var failoverLogger = logger2.GetLogger()

type FailoverEndpoint struct {
	Label       string
	ApiEndpoint string
	Client      ImageClient
}

type FailoverImageClient struct {
	modelName       string
	endpoints       []FailoverEndpoint
	endpointTimeout time.Duration
}

func NewFailoverImageClient(modelName string, endpoints []FailoverEndpoint) *FailoverImageClient {
	copied := make([]FailoverEndpoint, len(endpoints))
	copy(copied, endpoints)
	return &FailoverImageClient{
		modelName:       modelName,
		endpoints:       copied,
		endpointTimeout: defaultEndpointTimeout,
	}
}

func (c *FailoverImageClient) TextToImage(ctx context.Context, req TextToImageReq) (*GenerateResult, error) {
	return c.execute(ctx, "text-to-image", func(callCtx context.Context, client ImageClient) (*GenerateResult, error) {
		return client.TextToImage(callCtx, req)
	})
}

func (c *FailoverImageClient) ImageToImage(ctx context.Context, req ImageToImageReq) (*GenerateResult, error) {
	return c.execute(ctx, "image-to-image", func(callCtx context.Context, client ImageClient) (*GenerateResult, error) {
		return client.ImageToImage(callCtx, req)
	})
}

func (c *FailoverImageClient) execute(
	ctx context.Context,
	operation string,
	call func(context.Context, ImageClient) (*GenerateResult, error),
) (*GenerateResult, error) {
	if len(c.endpoints) == 0 {
		return nil, fmt.Errorf("image model %q has no configured endpoints", c.modelName)
	}

	var lastErr error
	for i, ep := range c.endpoints {
		if ep.Client == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(ctx, c.budgetFor(ctx))
		result, err := call(callCtx, ep.Client)
		cancel()

		if err == nil {
			if i > 0 {
				failoverLogger.Infof("image failover succeeded model=%s op=%s endpoint=%s url=%s",
					c.modelName, operation, ep.Label, sanitizeURL(ep.ApiEndpoint))
			}
			return result, nil
		}

		lastErr = err
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, err
		}
		if parentErr := ctx.Err(); parentErr != nil {
			return nil, parentErr
		}
		if !isFailoverable(err) {
			return nil, fmt.Errorf("image %s failed on %s for model %q: %w", operation, ep.Label, c.modelName, err)
		}

		failoverLogger.Warnf("image endpoint failed model=%s op=%s endpoint=%s url=%s err=%v",
			c.modelName, operation, ep.Label, sanitizeURL(ep.ApiEndpoint), err)
	}

	if lastErr == nil {
		return nil, fmt.Errorf("image model %q has no usable endpoints", c.modelName)
	}
	return nil, fmt.Errorf("all image endpoints exhausted for model %q: %w", c.modelName, lastErr)
}

func (c *FailoverImageClient) budgetFor(ctx context.Context) time.Duration {
	timeout := c.endpointTimeout
	if timeout <= 0 {
		timeout = defaultEndpointTimeout
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return time.Nanosecond
		}
		if remaining < timeout {
			return remaining
		}
	}
	return timeout
}

var statusPattern = regexp.MustCompile(`status ([0-9]{3})`)

func isFailoverable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return isFailoverableStatus(apiErr.StatusCode)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	if m := statusPattern.FindStringSubmatch(err.Error()); len(m) == 2 {
		if code, e := strconv.Atoi(m[1]); e == nil {
			return isFailoverableStatus(code)
		}
	}
	return false
}

func isFailoverableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

func sanitizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "default"
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "invalid-url"
	}
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}
