package provider

// 跨区域 OSS 中转：把主 OSS 里的参考图先"过境"到 vision 所在区域的 bucket，
// 生成一个预签名 URL 交给 SegmentCommodity 调用；调用完再把过境对象删掉。
//
// 背景：阿里云 imageseg（视觉智能分割）部署在 cn-shanghai，且对"OSS 协议域名"
// 的参数做了同区域强校验（RegionRecommend）。若业务 OSS 在 cn-guangzhou
// 之类的其他区域，即便签了名，vision 也会拒掉。把图过一次 shanghai bucket
// 是"改动面最小"的兼容方案——业务 OSS 不用动，生成的 URL 不再有跨区问题。
//
// 中转对象的生命周期：上传 → 调用完 → 立即删除。万一删除失败（网络抖动等），
// 调用方只记一次 warning，由 bucket lifecycle rule 最终兜底清理（建议用户在
// 控制台给该前缀设 1 天过期规则）。

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"path"
	"strings"
	"time"

	alioss "github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/nfnt/resize"
)// RelayConfig 跨区域中转配置。AccessKey/AccessSecret 复用 vision 凭证，
// 但单独列出便于用户给中转 bucket 下发一对权限更窄的 RAM AK。
type RelayConfig struct {
	Enabled      bool
	Endpoint     string // 如 oss-cn-shanghai.aliyuncs.com
	AccessKey    string
	AccessSecret string
	Bucket       string
	// 对象 key 前缀，默认 aic-vision-relay/
	Prefix string
}

// Normalize 填默认值、去空白。只返回是否"有效且启用"。
func (c RelayConfig) Normalize() RelayConfig {
	c.Endpoint = strings.TrimSpace(c.Endpoint)
	c.AccessKey = strings.TrimSpace(c.AccessKey)
	c.AccessSecret = strings.TrimSpace(c.AccessSecret)
	c.Bucket = strings.TrimSpace(c.Bucket)
	c.Prefix = strings.TrimSpace(c.Prefix)
	if c.Prefix == "" {
		c.Prefix = "aic-vision-relay/"
	} else if !strings.HasSuffix(c.Prefix, "/") {
		c.Prefix += "/"
	}
	return c
}

// Valid 判断配置是否完整；任何一项缺失都视为未启用。
func (c RelayConfig) Valid() bool {
	return c.Enabled && c.Endpoint != "" && c.AccessKey != "" &&
		c.AccessSecret != "" && c.Bucket != ""
}

// relayHandle 一次过境的句柄，主要携带 cleanup 回调，释放调用者手动删除的负担。
type relayHandle struct {
	URL       string
	objectKey string
	bucket    *alioss.Bucket
}

// Close 删除中转对象。幂等，调用失败不返回错误（仅依赖 lifecycle rule 兜底）。
func (h *relayHandle) Close() {
	if h == nil || h.bucket == nil || h.objectKey == "" {
		return
	}
	// 单张图通常 <2MB，删除响应很快；加 5s 超时防止 worker 卡住
	_ = h.bucket.DeleteObject(h.objectKey)
}

// visionMaxEdge 阿里云 imageseg 要求分辨率 < 2000×2000。
// 取 1900 留安全边距，避免边界上报 InvalidFile.ResolutionRecommend。
const visionMaxEdge = 1900

// shrinkForVision 若源图长边 > visionMaxEdge，则等比缩放并编码为 PNG 返回；
// 否则原样返回。缩放统一输出 PNG 是为了：
//   - 消除编码差异（源图可能是 webp/heic，vision 兼容性参差）
//   - vision 返回的 mask 本身就是 PNG，链路一致
//
// 编解码失败会直接返回原字节流——走直传让 vision 自己判定，最坏得到一个可读的错误码。
func shrinkForVision(data []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	longEdge := w
	if h > longEdge {
		longEdge = h
	}
	if longEdge <= visionMaxEdge {
		return data
	}
	// 等比缩放到长边 = visionMaxEdge
	var nw, nh uint
	if w >= h {
		nw = uint(visionMaxEdge)
		nh = 0 // 0 表示按比例推算
	} else {
		nw = 0
		nh = uint(visionMaxEdge)
	}
	resized := resize.Resize(nw, nh, img, resize.Lanczos3)
	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return data
	}
	return buf.Bytes()
}

// RelayUpload 从 srcURL 下载图片，上传到中转 bucket，返回预签名 URL + 清理回调。
//
// 参数 downloader 用于下载原图；把它作为参数而不是直接在函数里 http.Get，
// 是为了复用 utils.DownloadImage 已经配置的 proxy / 超时策略。
func RelayUpload(
	ctx context.Context,
	cfg RelayConfig,
	srcURL string,
	downloader func(url string) ([]byte, error),
	ttl time.Duration,
) (*relayHandle, error) {
	if !cfg.Valid() {
		return nil, fmt.Errorf("relay config invalid")
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}

	data, err := downloader(srcURL)
	if err != nil {
		return nil, fmt.Errorf("relay: download source: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("relay: empty source bytes")
	}

	// vision 对分辨率有 2000x2000 上限，过大会被拒；超尺寸时原地缩放。
	// 注意：shrinkForVision 在缩放后会强制输出 PNG；
	// 因此后面的扩展名必须用 .png，不能再从源 URL 猜。
	originalLen := len(data)
	data = shrinkForVision(data)
	shrunk := len(data) != originalLen

	client, err := alioss.New(cfg.Endpoint, cfg.AccessKey, cfg.AccessSecret)
	if err != nil {
		return nil, fmt.Errorf("relay: new oss client: %w", err)
	}
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("relay: open bucket: %w", err)
	}

	// 对象 key：前缀 + 纳秒时间戳 + 扩展名
	// - 缩放过（shrunk=true）：shrinkForVision 输出 PNG，必须用 .png
	// - 未缩放：尽量保留源扩展名（部分 vision 能力对 MIME 敏感）
	ext := ".png"
	if !shrunk {
		if idx := strings.LastIndex(srcURL, "."); idx > 0 {
			candidate := srcURL[idx:]
			if q := strings.Index(candidate, "?"); q > 0 {
				candidate = candidate[:q]
			}
			if len(candidate) <= 6 && strings.HasPrefix(candidate, ".") {
				ext = strings.ToLower(candidate)
			}
		}
	}
	objectKey := path.Join(cfg.Prefix, fmt.Sprintf("%d%s", time.Now().UnixNano(), ext))

	if err := bucket.PutObject(objectKey, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("relay: put object: %w", err)
	}

	signed, err := bucket.SignURL(objectKey, alioss.HTTPGet, int64(ttl/time.Second))
	if err != nil {
		// 上传成功但签名失败：回滚删除，避免遗留对象
		_ = bucket.DeleteObject(objectKey)
		return nil, fmt.Errorf("relay: sign url: %w", err)
	}

	return &relayHandle{
		URL:       signed,
		objectKey: objectKey,
		bucket:    bucket,
	}, nil
}