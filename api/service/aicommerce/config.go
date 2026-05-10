package aicommerce

// Config 电商生图模块配置（从环境变量或 config.toml 读取）
type Config struct {
	// 硅基流动文生图
	SiliconFlowBaseURL string `toml:"silicon_flow_base_url"`
	SiliconFlowAPIKey  string `toml:"silicon_flow_api_key"`
	SiliconFlowModel   string `toml:"silicon_flow_model"` // 默认 kolors

	// 通义千问卖点代写
	TongyiBaseURL string `toml:"tongyi_base_url"`
	TongyiAPIKey  string `toml:"tongyi_api_key"`
	TongyiModel   string `toml:"tongyi_model"` // 默认 qwen-turbo

	// 百度 OCR
	BaiduOCRAppID     string `toml:"baidu_ocr_app_id"`
	BaiduOCRAPIKey    string `toml:"baidu_ocr_api_key"`
	BaiduOCRSecretKey string `toml:"baidu_ocr_secret_key"`

	// 百度翻译
	BaiduTranslateAppID  string `toml:"baidu_translate_app_id"`
	BaiduTranslateSecret string `toml:"baidu_translate_secret"`

	// 阿里云视觉（背景移除）
	AliyunVisionAccessKeyID     string `toml:"aliyun_vision_access_key_id"`
	AliyunVisionAccessKeySecret string `toml:"aliyun_vision_access_key_secret"`
	// 阿里云视觉 SDK 区域 ID；imageseg 服务仅在少数区域提供，默认 cn-shanghai
	AliyunVisionRegion string `toml:"aliyun_vision_region"`

	// 阿里云视觉跨区域 OSS 中转。
	// 当主 OSS 与 vision region 不同（例如主 OSS 在 cn-guangzhou，vision 在 cn-shanghai），
	// vision API 会因"OSS URL 同区域校验"拒绝请求（InvalidImage.RegionRecommend）。
	// 此时需要在 vision region 建一个小 bucket，调用前把图过境一次，用完即删。
	//
	// 若留空（Enabled=false），chain 将直接用原 URL 调用 vision，
	// 适用于主 OSS 已经和 vision 同区域、或 bucket 绑定了非 OSS 域名的情况。
	AliyunVisionRelayEnabled      bool   `toml:"aliyun_vision_relay_enabled"`
	AliyunVisionRelayEndpoint     string `toml:"aliyun_vision_relay_endpoint"`     // 如 oss-cn-shanghai.aliyuncs.com
	AliyunVisionRelayAccessKey    string `toml:"aliyun_vision_relay_access_key"`   // 缺省回退到 AliyunVisionAccessKeyID
	AliyunVisionRelayAccessSecret string `toml:"aliyun_vision_relay_access_secret"` // 同上
	AliyunVisionRelayBucket       string `toml:"aliyun_vision_relay_bucket"`
	AliyunVisionRelayPrefix       string `toml:"aliyun_vision_relay_prefix"` // 默认 aic-vision-relay/

	// Redis 队列
	QueueName       string `toml:"queue_name"`        // 默认 ai_commerce_tasks
	WorkerConcurrency int  `toml:"worker_concurrency"` // 默认 3

	// OSS（复用 GeeKAI 现有 OSS 配置）
	OSSBucket string `toml:"oss_bucket"`

	// 资产 URL 签名有效期（秒）
	AssetURLTTL int `toml:"asset_url_ttl"` // 默认 3600
}

func DefaultConfig() Config {
	return Config{
		SiliconFlowBaseURL:  "https://api.siliconflow.cn/v1",
		SiliconFlowModel:    "kolors",
		TongyiBaseURL:       "https://dashscope.aliyuncs.com/compatible-mode/v1",
		TongyiModel:         "qwen-turbo",
		AliyunVisionRegion:  "cn-shanghai",
		QueueName:           "ai_commerce_tasks",
		WorkerConcurrency:   3,
		AssetURLTTL:         3600,
	}
}
