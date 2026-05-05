package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type visionTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type visionImageURL struct {
	URL string `json:"url"`
}

type visionImagePart struct {
	Type     string         `json:"type"`
	ImageURL visionImageURL `json:"image_url"`
}

type visionChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type visionChatReq struct {
	Model          string              `json:"model"`
	Messages       []visionChatMessage `json:"messages"`
	ResponseFormat *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
}

type visionChatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// CopywriteSellingPoint 单条卖点结构
type CopywriteSellingPoint struct {
	Icon           string   `json:"icon"`
	Zh             string   `json:"zh"`
	En             string   `json:"en"`
	ZhDesc         string   `json:"zh_desc"`
	EnDesc         string   `json:"en_desc"`
	VisualKeywords []string `json:"visual_keywords"`
}

// CopywriteAnalysis AI 视觉分析结果（与 ecommerce-image-suite 对齐 + recommended_style + 双语场景）
type CopywriteAnalysis struct {
	ProductName                 string                  `json:"product_name"`
	ProductDescriptionForPrompt string                  `json:"product_description_for_prompt"`
	ProductType                 string                  `json:"product_type"`
	GarmentPosition             string                  `json:"garment_position"`
	VisualFeatures              []string                `json:"visual_features"`
	SellingPoints               []CopywriteSellingPoint `json:"selling_points"`
	TargetAudience              string                  `json:"target_audience"`
	TargetScenes                []string                `json:"target_scenes"`     // 兼容旧字段：中文场景列表
	TargetScenesZh              []string                `json:"target_scenes_zh"`  // 中文场景（≤6字/条，最多3条）
	TargetScenesEn              []string                `json:"target_scenes_en"`  // 英文场景（≤4 words/条，最多3条）
	ProductStyle                string                  `json:"product_style"`
	Color                       string                  `json:"color"`
	Material                    string                  `json:"material"`
	Style                       string                  `json:"style"`
	PrintDesign                 string                  `json:"print_design"`
	PrintDesignLock             string                  `json:"print_design_lock"`
	ProductNameZh               string                  `json:"product_name_zh"`
	RecommendedStyle            string                  `json:"recommended_style"`
}

type OpenAIVisionCopywriter struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewOpenAIVisionCopywriter(baseURL, apiKey, model string) *OpenAIVisionCopywriter {
	return &OpenAIVisionCopywriter{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		client:  &http.Client{Timeout: 45 * time.Second},
	}
}

const copywriteSystemPrompt = `你是一位拥有15年以上电商经验的顶级视觉分析师和爆款文案策划师。

请仔细观察图片中的商品，按以下 JSON 格式输出分析结果，只输出 JSON，不要输出其他内容：

{
  "product_name": "商品详细名称，包含品类、材质、款型等关键词",
  "product_description_for_prompt": "英文描述，用于图像生成Prompt，包含颜色/款型/印花/材质等视觉细节，50词以内",
  "product_type": "服装 | 3C数码 | 家居 | 美妆 | 食品 | 其他",
  "garment_position": "top | bottom | full-body | non-apparel（非服装统一填non-apparel）",
  "visual_features": ["视觉特征1", "视觉特征2"],
  "selling_points": [
    {"icon": "fabric|fit|design|comfort|quality|function|scene", "zh": "中文卖点标题≤6字", "en": "English title ≤4 words", "zh_desc": "中文说明≤15字", "en_desc": "English desc ≤12 words", "visual_keywords": ["keyword1", "keyword2"]}
  ],
  "target_audience": "目标人群描述",
  "target_scenes": ["使用场景1", "使用场景2"],
  "target_scenes_zh": ["中文场景1（≤6字，如：浪漫约会）", "中文场景2", "中文场景3"],
  "target_scenes_en": ["English scene 1 (≤4 words, e.g., Romantic Date Night)", "English scene 2", "English scene 3"],
  "product_style": "商品风格（如：法式浪漫 / 日系可爱 / 简约商务 / 运动休闲）",
  "color": "精确英文色值描述（如 pure white、lavender purple）",
  "material": "主要材质（若可识别）",
  "style": "版型描述（宽松oversized、修身等）",
  "print_design": "印花/设计描述（无则填none）",
  "print_design_lock": "精确约束短语，要求exact same print pattern, color and position must not change",
  "product_name_zh": "中文商品名简短版，用于文案叠加",
  "recommended_style": "根据以下规则选择生图风格，只填value：default_shoot | lifestyle_mag | minimal_cold | energetic_hit | dark_quality | asymmetric_layout"
}

selling_points 请提炼 3-5 条，优先级：材质 > 版型 > 设计感 > 舒适性 > 使用场景。从图片可见特征推断，不要凭空捏造。

target_scenes_zh 与 target_scenes_en 必须输出 2-3 条最匹配的真实使用场景（中英一一对应，按重要性排序），用于电商场景代入图。
- 内衣/睡衣 → 卧室、梳妆台、清晨阳光卧房 / Cozy Bedroom, Vanity Table, Morning Light
- 运动服 → 健身房、户外跑道、瑜伽馆 / Gym, Outdoor Track, Yoga Studio
- 礼服/连衣裙 → 浪漫餐厅、宴会厅、约会 / Romantic Dining, Banquet Hall, Date Night
- 商务装 → 写字楼、办公室、咖啡商谈 / Office, Boardroom, Cafe Meeting
- 童装 → 儿童乐园、家庭客厅、户外草地 / Playground, Living Room, Outdoor Lawn
- 家居用品 → 北欧客厅、餐厨场景 / Nordic Living, Modern Kitchen
- 美妆护肤 → 梳妆台、浴室 / Vanity, Bathroom
- 3C数码 → 极客桌面、咖啡办公 / Geek Desk, Cafe Workspace
- 食品饮品 → 餐桌、野餐 / Dining Table, Picnic
其他品类按目标人群与使用习惯自行推断，禁止套用与品类无关的场景。

recommended_style 选择规则：
- 运动/街头/活力 + 年轻客群 → energetic_hit
- 商务/高端/极简 + 成熟人群 → minimal_cold
- 数码/运动/夜场 + 男性为主 → dark_quality
- 生活类/家居/食品/有氛围感 → lifestyle_mag
- 多SKU/需突出细节对比 → asymmetric_layout
- 无明显风格特征或新手 → default_shoot`

var validStyles = map[string]bool{
	"default_shoot": true, "lifestyle_mag": true, "minimal_cold": true,
	"energetic_hit": true, "dark_quality": true, "asymmetric_layout": true,
}

func (c *OpenAIVisionCopywriter) GenerateCopywrite(ctx context.Context, productName, hint string, imageURLs []string) (string, *CopywriteAnalysis, error) {
	if c.baseURL == "" {
		return "", nil, fmt.Errorf("vision copywrite baseURL is empty")
	}
	if c.apiKey == "" {
		return "", nil, fmt.Errorf("vision copywrite apiKey is empty")
	}
	if c.model == "" {
		return "", nil, fmt.Errorf("vision copywrite model is empty")
	}

	imageURLs = normalizeVisionImageURLs(imageURLs)
	if len(imageURLs) == 0 {
		return "", nil, fmt.Errorf("vision copywrite requires at least one image")
	}
	if len(imageURLs) > 3 {
		return "", nil, fmt.Errorf("vision copywrite supports at most 3 images")
	}

	hint = strings.TrimSpace(hint)
	productName = strings.TrimSpace(productName)
	var userParts []string
	if productName != "" {
		userParts = append(userParts, "商品名称："+productName)
	}
	if hint != "" {
		userParts = append(userParts, "补充信息："+hint)
	}
	userText := strings.Join(userParts, "\n")

	parts := make([]interface{}, 0, len(imageURLs)+1)
	parts = append(parts, visionTextPart{Type: "text", Text: userText})
	for _, imageURL := range imageURLs {
		parts = append(parts, visionImagePart{
			Type:     "image_url",
			ImageURL: visionImageURL{URL: imageURL},
		})
	}

	req := visionChatReq{
		Model: c.model,
		Messages: []visionChatMessage{
			{Role: "system", Content: copywriteSystemPrompt},
			{Role: "user", Content: parts},
		},
		ResponseFormat: &struct {
			Type string `json:"type"`
		}{Type: "json_object"},
	}

	raw, err := c.doRequest(ctx, req)
	if err != nil {
		// 400 降级：去掉 response_format 重试一次
		if strings.Contains(err.Error(), "status 400") {
			req.ResponseFormat = nil
			raw, err = c.doRequest(ctx, req)
		}
		if err != nil {
			return "", nil, err
		}
	}

	analysis, err := parseAndNormalize(raw)
	if err != nil {
		return raw, nil, nil
	}

	content := formatCopywriteContent(analysis)
	return content, analysis, nil
}

func (c *OpenAIVisionCopywriter) doRequest(ctx context.Context, req visionChatReq) (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal vision copywrite request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read vision copywrite response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return "", fmt.Errorf("vision copywrite provider returned status %d: %s", resp.StatusCode, snippet)
	}

	var result visionChatResp
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode vision copywrite response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("vision copywrite empty choices")
	}
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("vision copywrite empty content")
	}

	return content, nil
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+7:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	} else if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
		if end := strings.Index(s, "```"); end >= 0 {
			s = s[:end]
		}
	}
	// 提取第一个完整 JSON 对象
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	depth := 0
	for i, ch := range s[start:] {
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return s[start : start+i+1]
			}
		}
	}
	return s[start:]
}

func parseAndNormalize(raw string) (*CopywriteAnalysis, error) {
	raw = extractJSON(raw)
	var a CopywriteAnalysis
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, err
	}

	for i, p := range a.SellingPoints {
		a.SellingPoints[i].Icon = normalizeIcon(p.Icon)
		a.SellingPoints[i].Zh = truncateRunes(p.Zh, 6)
		a.SellingPoints[i].ZhDesc = truncateRunes(p.ZhDesc, 15)
		a.SellingPoints[i].En = truncateWords(p.En, 4)
		a.SellingPoints[i].EnDesc = truncateWords(p.EnDesc, 12)
	}

	if len(a.SellingPoints) == 0 {
		a.SellingPoints = []CopywriteSellingPoint{
			{Icon: "design", Zh: "核心卖点", ZhDesc: "突出商品质感"},
		}
	}

	if !validStyles[a.RecommendedStyle] {
		a.RecommendedStyle = "default_shoot"
	}

	// 场景双语 fallback：若新字段为空但旧 target_scenes 有值，回填中文场景
	if len(a.TargetScenesZh) == 0 && len(a.TargetScenes) > 0 {
		a.TargetScenesZh = a.TargetScenes
	}
	// 截断长度
	for i, s := range a.TargetScenesZh {
		a.TargetScenesZh[i] = truncateRunes(s, 6)
	}
	for i, s := range a.TargetScenesEn {
		a.TargetScenesEn[i] = truncateWords(s, 4)
	}
	if len(a.TargetScenesZh) > 3 {
		a.TargetScenesZh = a.TargetScenesZh[:3]
	}
	if len(a.TargetScenesEn) > 3 {
		a.TargetScenesEn = a.TargetScenesEn[:3]
	}

	return &a, nil
}

func formatCopywriteContent(a *CopywriteAnalysis) string {
	lines := []string{
		"【商品品类】" + a.ProductType,
		"",
		"【核心卖点】",
	}
	for _, p := range a.SellingPoints {
		lines = append(lines, p.Zh+"："+p.ZhDesc)
	}
	supplement := "【补充描述】" + a.TargetAudience
	if len(a.TargetScenes) > 0 {
		supplement += "，适合" + strings.Join(a.TargetScenes, "、")
	}
	lines = append(lines, "", supplement)
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func normalizeIcon(icon string) string {
	switch strings.TrimSpace(icon) {
	case "fabric", "fit", "design", "quality", "comfort", "function", "scene":
		return icon
	default:
		return "design"
	}
}

func truncateRunes(s string, max int) string {
	count := 0
	for i := range s {
		if count >= max {
			return s[:i]
		}
		count++
	}
	return s
}

func truncateWords(s string, max int) string {
	words := strings.Fields(s)
	if len(words) <= max {
		return s
	}
	return strings.Join(words[:max], " ")
}

func normalizeVisionImageURLs(imageURLs []string) []string {
	result := make([]string, 0, len(imageURLs))
	for _, imageURL := range imageURLs {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			continue
		}
		result = append(result, imageURL)
	}
	return result
}
