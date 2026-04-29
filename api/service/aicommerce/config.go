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
		QueueName:           "ai_commerce_tasks",
		WorkerConcurrency:   3,
		AssetURLTTL:         3600,
	}
}
