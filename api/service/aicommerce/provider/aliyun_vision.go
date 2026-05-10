package provider

// 阿里云视觉智能「商品分割」(SegmentCommodity) —— 电商白底图核心能力。
//
// 之前的实现用自定义 Authorization: AccessKey id:secret 打 /green/segment/body，
// 这个端点属于"人像分割"，签名方式也不是阿里云 RPC 风格的 HMAC-SHA1，调不通。
// 现在改为直接用官方 alibaba-cloud-sdk-go 的 imageseg 客户端，
// 请求签名、异步轮询、endpoint 解析交给 SDK 处理。
//
// 跨区域兼容：imageseg 部署在 cn-shanghai，且对"OSS 域名参数"做了同区域校验
// （报错 InvalidImage.RegionRecommend）。若业务 OSS 在其他区域（如 cn-guangzhou），
// 通过注入 RelayConfig 启用"先把图过境到 shanghai 小 bucket"的中转策略。
//
// 服务参考：https://help.aliyun.com/zh/viapi/developer-reference/api-imageseg-2019-12-30-segmentcommodity

import (
	"context"
	"fmt"
	"geekai/utils"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/imageseg"
)

// AliyunVision 商品抠图客户端。region 决定请求端点（imageseg 官方仅开放 cn-shanghai）。
type AliyunVision struct {
	accessKeyID     string
	accessKeySecret string
	region          string
	client          *imageseg.Client
	relay           RelayConfig // 可选：跨区域中转配置
}

// NewAliyunVision 构造客户端。凭证为空时允许构造成功，调用阶段再报错，
// 避免凭证缺失直接导致进程启动失败（其它模块可能不依赖白底图）。
func NewAliyunVision(accessKeyID, accessKeySecret string, region ...string) *AliyunVision {
	r := "cn-shanghai"
	if len(region) > 0 && strings.TrimSpace(region[0]) != "" {
		r = strings.TrimSpace(region[0])
	}
	v := &AliyunVision{
		accessKeyID:     strings.TrimSpace(accessKeyID),
		accessKeySecret: strings.TrimSpace(accessKeySecret),
		region:          r,
	}
	if v.accessKeyID != "" && v.accessKeySecret != "" {
		// 构造失败通常意味着 SDK 本身异常，记到 logger 即可，运行期再报错
		if c, err := imageseg.NewClientWithAccessKey(r, v.accessKeyID, v.accessKeySecret); err == nil {
			v.client = c
		}
	}
	return v
}

// WithRelay 注入跨区域 OSS 中转配置。
// 凭证为空时会自动回退到 vision 自身的 AK/SK，简化配置。
func (a *AliyunVision) WithRelay(relay RelayConfig) *AliyunVision {
	if a == nil {
		return a
	}
	if relay.AccessKey == "" {
		relay.AccessKey = a.accessKeyID
	}
	if relay.AccessSecret == "" {
		relay.AccessSecret = a.accessKeySecret
	}
	a.relay = relay.Normalize()
	return a
}

// RelayActive 返回中转是否真正生效（配置完整且已启用）。
// 用于启动期日志，便于运维在 "enable=true 但字段没填全" 时快速发现。
func (a *AliyunVision) RelayActive() bool {
	if a == nil {
		return false
	}
	return a.relay.Valid()
}

// detectOSSRegionMismatch 从 URL 中尝试解析出 OSS region，判断是否与 vision 期望区域不一致。
//
// 支持识别两种常见形式：
//   - 虚拟托管：https://<bucket>.oss-cn-guangzhou.aliyuncs.com/<key>
//   - 路径型  ：https://oss-cn-guangzhou.aliyuncs.com/<bucket>/<key>
//
// 解析不出 region（例如自定义 CDN 域名、非 OSS 地址）时，返回 mismatch=false，
// 让请求照常发出——vision 自身会处理非 OSS URL。
func detectOSSRegionMismatch(imageURL, visionRegion string) (bool, string) {
	visionRegion = strings.TrimSpace(visionRegion)
	if visionRegion == "" {
		return false, ""
	}
	low := strings.ToLower(imageURL)
	// 去掉协议、query
	if i := strings.Index(low, "://"); i >= 0 {
		low = low[i+3:]
	}
	if i := strings.IndexAny(low, "?#/"); i >= 0 {
		low = low[:i]
	}
	if !strings.Contains(low, ".aliyuncs.com") {
		return false, ""
	}
	// 取最左段作为 host；形如 "<bucket>.oss-cn-xxx.aliyuncs.com" 或 "oss-cn-xxx.aliyuncs.com"
	// 找到 "oss-" 段提取 region
	idx := strings.Index(low, "oss-")
	if idx < 0 {
		return false, ""
	}
	seg := low[idx:]
	end := strings.Index(seg, ".aliyuncs.com")
	if end <= 0 {
		return false, ""
	}
	// seg[:end] 形如 "oss-cn-guangzhou" / "oss-accelerate" / "oss-cn-shanghai-internal"
	ossHost := seg[:end]
	// 把 "oss-accelerate" 这种全球加速域名当作不特定区域，不报错
	if ossHost == "oss-accelerate" || ossHost == "oss-accelerate-overseas" {
		return false, ""
	}
	// "oss-cn-guangzhou"→"cn-guangzhou"
	region := strings.TrimPrefix(ossHost, "oss-")
	// 去掉 "-internal" 后缀
	region = strings.TrimSuffix(region, "-internal")
	if region == "" || region == visionRegion {
		return false, region
	}
	return true, region
}

// RemoveBackground 对单张图像调用 SegmentCommodity，返回透明背景 PNG 的临时访问 URL。
//
// 注意：
//   - imageURL 必须是外网可访问、未做防盗链的地址；私有 OSS 需传预签名 URL；
//   - 若 OSS 区域与 vision region 不同，会自动走 RelayConfig 过境中转；
//   - SegmentCommodity 同步模式下 Data.ImageURL 即为结果；返回地址仅保持 30 分钟，
//     所以上游拿到后要立刻下载落盘/落 OSS；
//   - 调用失败时返回的 error 带 RequestId，便于在阿里云控制台排查。
func (a *AliyunVision) RemoveBackground(ctx context.Context, imageURL string) (string, error) {
	if a == nil || a.client == nil {
		return "", fmt.Errorf("aliyun vision not configured: 请在 config.toml [AiCommerce] 节填写 aliyun_vision_access_key_id / _secret")
	}
	if strings.TrimSpace(imageURL) == "" {
		return "", fmt.Errorf("aliyun vision: empty image url")
	}

	// 预检：vision 对"OSS 协议 URL"做同区域强校验，跨区域会直接报 InvalidImage.RegionRecommend。
	// 如果检测到跨区且中转未启用，直接返回可操作的错误，避免去调一次必失败的 API。
	if mismatch, srcRegion := detectOSSRegionMismatch(imageURL, a.region); mismatch && !a.relay.Valid() {
		return "", fmt.Errorf(
			"aliyun vision: OSS 跨区域调用会被拒绝（源 bucket 在 %s，vision 在 %s）。"+
				"请在 config.toml [AiCommerce] 打开 aliyun_vision_relay_enabled 并填上 "+
				"aliyun_vision_relay_endpoint / aliyun_vision_relay_bucket（在 %s 新建一个小 bucket 即可）",
			srcRegion, a.region, a.region,
		)
	}

	// 如果启用中转，先把原图过境到 vision region 的 bucket 上
	var relayHandle *relayHandle
	targetURL := imageURL
	if a.relay.Valid() {
		h, err := RelayUpload(ctx, a.relay, imageURL,
			func(url string) ([]byte, error) { return utils.DownloadImage(url, "") },
			15*time.Minute,
		)
		if err != nil {
			return "", fmt.Errorf("aliyun vision: relay upload: %w", err)
		}
		relayHandle = h
		targetURL = h.URL
		defer relayHandle.Close()
	}

	req := imageseg.CreateSegmentCommodityRequest()
	req.Scheme = "https"
	req.ImageURL = targetURL
	// ReturnForm 语义：
	//   - 留空（默认）：返回带透明通道的 PNG（抠图，主体完整 + alpha 背景）
	//   - "mask"：返回黑白二值 mask，白=主体、黑=背景，不适合直接合成
	//   - "whiteBK"：返回已合好的白底图，会省掉本地 CompositeWhiteBg，但尺寸/构图不可控
	// 我们要透明 PNG 让 imageutil 本地按比例合白底，所以这里不传。
	// SDK 默认 10s read timeout，对象分割常见 5~15s，这里放宽到 60s
	req.SetReadTimeout(60 * time.Second)
	req.SetConnectTimeout(10 * time.Second)

	// context 取消时同步取消请求
	done := make(chan struct{})
	var (
		resp *imageseg.SegmentCommodityResponse
		err  error
	)
	go func() {
		resp, err = a.client.SegmentCommodity(req)
		close(done)
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-done:
	}

	if err != nil {
		return "", fmt.Errorf("aliyun vision: segment commodity: %w", err)
	}
	url := resp.Data.ImageURL
	if url == "" {
		url = resp.Data.ImageUrl
	}
	if url == "" {
		return "", fmt.Errorf("aliyun vision: empty image url, requestId=%s", resp.RequestId)
	}
	return url, nil
}
