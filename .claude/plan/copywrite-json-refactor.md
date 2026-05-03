# 实施计划：AI 代写卖点 JSON 化改造（v2）

## 任务类型
- [x] 后端 (Go) + 前端 (Vue)

## 背景与问题
当前 Copywrite 接口返回纯文本字符串，AI Prompt 模板（text/template）中 `{{.SellingPoints}}` 需要自然语言文本。  
目标：后端强制 AI 返回 JSON（15字段 + 1新增风格字段） → 解析校验 → 转为自然语言文本填入 `content`（兼容模板链路）+ 额外返回 `analysis` JSON 对象（供前端结构化展示）。

## JSON 字段规范（16 字段）

与参考项目 ecommerce-image-suite 完全一致的 15 个字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `product_name` | string | 商品详细名称（含品类/材质/款型） |
| `product_description_for_prompt` | string | 英文描述，用于图像生成 Prompt，50词以内 |
| `product_type` | string | 服装 \| 3C数码 \| 家居 \| 美妆 \| 食品 \| 其他 |
| `garment_position` | string | top \| bottom \| full-body \| non-apparel |
| `visual_features` | []string | 视觉特征列表 |
| `selling_points` | []object | 3-5条卖点（含 icon/zh/en/zh_desc/en_desc/visual_keywords） |
| `target_audience` | string | 目标人群描述 |
| `target_scenes` | []string | 使用场景列表 |
| `product_style` | string | 商品风格（如：法式浪漫/运动休闲） |
| `color` | string | 精确英文色值描述 |
| `material` | string | 主要材质 |
| `style` | string | 版型描述（宽松/修身等） |
| `print_design` | string | 印花/设计描述（无则 none） |
| `print_design_lock` | string | 精确约束短语（用于图像生成一致性） |
| `product_name_zh` | string | 中文商品名简短版（用于文案叠加） |

新增第 16 个字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `recommended_style` | string | AI 推荐的生图风格（见风格规则） |

## 风格推荐规则（AI 按此决策）

6种风格选项：
- `default_shoot`：① 默认商拍 — 标准电商商拍，干净明亮，重点突出商品
- `lifestyle_mag`：② 生活杂志 — 自然光，有氛围感和生活质感
- `minimal_cold`：③ 极简高冷 — 极简留白，高反差，奢侈品质感
- `energetic_hit`：④ 活力爆款 — 高饱和度，大字冲击，活力感强
- `dark_quality`：⑤ 暗调质感 — 深色系，电影质感，戏剧性打光
- `asymmetric_layout`：⑥ 非对称布局 — 左侧大图（60%）+ 右侧细节图（40%），突出主次层次

AI 决策规则：
- 运动 / 街头 / 活力 + 年轻客群 → `energetic_hit`
- 商务 / 高端 / 极简 + 成熟人群 → `minimal_cold`
- 数码 / 运动 / 夜场 + 男性为主 → `dark_quality`
- 无明显风格特征或新手 → `default_shoot`
- 生活类 / 家居 / 食品 / 有场景氛围感 → `lifestyle_mag`
- 多SKU / 需要突出细节对比 → `asymmetric_layout`

## 技术方案

**后端**：
- 新增 Go 结构体 `CopywriteSellingPoint` + `CopywriteAnalysis`（16字段含 `RecommendedStyle`）
- 更新 system prompt（完全参照参考项目格式，强制 JSON only，三步分析，含风格推荐规则说明）
- 请求加 `response_format: json_object`（400 时降级重试）
- 解析 JSON → 枚举/字数规范化 → 转自然语言文本
- `RecommendedStyle` 枚举校验，无效值 fallback 到 `default_shoot`
- 返回 `{content, analysis}`

**前端**：
- 新建 `src/utils/ecomFormat.js`（icon→emoji 映射、formatAnalysisToText、风格 value→label 映射）
- `generateCopywriting()` 返回 `{content, analysis}`
- 三个页面联动：
  - `product_name`（为空时自动填充）
  - `selling_points` textarea（格式化文本）
  - `style_desc`（为空时自动填充 AI 推荐风格的中文描述）

## 实施步骤

### Step 1：后端 — 新增结构体（openai_vision_copywrite.go）

```go
type CopywriteSellingPoint struct {
    Icon          string   `json:"icon"`
    Zh            string   `json:"zh"`
    En            string   `json:"en"`
    ZhDesc        string   `json:"zh_desc"`
    EnDesc        string   `json:"en_desc"`
    VisualKeywords []string `json:"visual_keywords"`
}

type CopywriteAnalysis struct {
    ProductName                 string                  `json:"product_name"`
    ProductDescriptionForPrompt string                  `json:"product_description_for_prompt"`
    ProductType                 string                  `json:"product_type"`
    GarmentPosition             string                  `json:"garment_position"`
    VisualFeatures              []string                `json:"visual_features"`
    SellingPoints               []CopywriteSellingPoint `json:"selling_points"`
    TargetAudience              string                  `json:"target_audience"`
    TargetScenes                []string                `json:"target_scenes"`
    ProductStyle                string                  `json:"product_style"`
    Color                       string                  `json:"color"`
    Material                    string                  `json:"material"`
    Style                       string                  `json:"style"`
    PrintDesign                 string                  `json:"print_design"`
    PrintDesignLock             string                  `json:"print_design_lock"`
    ProductNameZh               string                  `json:"product_name_zh"`
    RecommendedStyle            string                  `json:"recommended_style"` // 新增
}
```

### Step 2：后端 — 替换 system prompt

完全参照参考项目格式，追加风格推荐规则：

```
你是一位拥有15年以上电商经验的顶级视觉分析师和爆款文案策划师。

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

recommended_style 选择规则：
- 运动/街头/活力 + 年轻客群 → energetic_hit
- 商务/高端/极简 + 成熟人群 → minimal_cold
- 数码/运动/夜场 + 男性为主 → dark_quality
- 生活类/家居/食品/有氛围感 → lifestyle_mag
- 多SKU/需突出细节对比 → asymmetric_layout
- 无明显风格特征或新手 → default_shoot
```

### Step 3：后端 — 请求结构加 response_format

```go
type visionChatReq struct {
    Model          string              `json:"model"`
    Messages       []visionChatMessage `json:"messages"`
    ResponseFormat *struct {
        Type string `json:"type"`
    } `json:"response_format,omitempty"`
}
// 设置 JSON mode
req.ResponseFormat = &struct{ Type string `json:"type"` }{Type: "json_object"}
```

降级策略：若 HTTP 返回 400，去掉 `response_format` 字段重试一次。

### Step 4：后端 — 解析 JSON + 规范化

```go
var validStyles = map[string]bool{
    "default_shoot": true, "lifestyle_mag": true, "minimal_cold": true,
    "energetic_hit": true, "dark_quality": true, "asymmetric_layout": true,
}

func parseAndNormalize(raw string) (*CopywriteAnalysis, error) {
    raw = extractJSON(raw) // 去除 ```json 包裹
    var a CopywriteAnalysis
    if err := json.Unmarshal([]byte(raw), &a); err != nil {
        return nil, err
    }
    // 规范化卖点
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
    // 规范化风格
    if !validStyles[a.RecommendedStyle] {
        a.RecommendedStyle = "default_shoot"
    }
    return &a, nil
}

func normalizeIcon(icon string) string {
    switch strings.TrimSpace(icon) {
    case "fabric", "fit", "design", "quality", "comfort", "function", "scene":
        return icon
    default:
        return "design"
    }
}
```

### Step 5：后端 — 结构体转自然语言文本

```go
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
```

### Step 6：后端 — 修改 GenerateCopywrite 返回签名

```go
// provider/openai_vision_copywrite.go
func (c *OpenAIVisionCopywriter) GenerateCopywrite(
    ctx context.Context, productName, hint string, imageURLs []string,
) (content string, analysis *CopywriteAnalysis, err error)

// service/aicommerce/image_service.go
func (s *ImageService) Copywrite(
    ctx context.Context, userID uint, req CopywriteReq,
) (content string, analysis *provider.CopywriteAnalysis, err error)
```

### Step 7：后端 — Handler 修改响应

```go
content, analysis, err := h.service.Copywrite(...)
resp.SUCCESS(c, gin.H{"content": content, "analysis": analysis})
```

### Step 8：前端 — 新建 src/utils/ecomFormat.js

```js
const ICON_MAP = {
  fabric: '🧶', fit: '👗', design: '✨',
  quality: '💎', comfort: '☁️', function: '⚙️', scene: '🎭'
}

// 风格 value → 中文描述（用于 style_desc 自动填充）
const STYLE_DESC_MAP = {
  default_shoot:      '标准电商商拍，干净明亮，重点突出商品',
  lifestyle_mag:      '自然光，有氛围感和生活质感',
  minimal_cold:       '极简留白，高反差，奢侈品质感',
  energetic_hit:      '高饱和度，大字冲击，活力感强',
  dark_quality:       '深色系，电影质感，戏剧性打光',
  asymmetric_layout:  '非对称布局，左侧大图突出主体，右侧细节图'
}

export function formatAnalysisToText(analysis, fallbackContent = '') {
  if (!analysis?.selling_points?.length) return fallbackContent
  return analysis.selling_points.map(item => {
    const emoji = ICON_MAP[item.icon] || '📍'
    return `${emoji} ${item.zh}${item.zh_desc ? '\n   ' + item.zh_desc : ''}`
  }).join('\n')
}

export function getStyleDesc(recommendedStyle) {
  return STYLE_DESC_MAP[recommendedStyle] || ''
}
```

### Step 9：前端 — 修改 src/store/ecom.js

```js
const generateCopywriting = async (productName, hint, assetNos) => {
  const res = await httpPost('/api/ai-commerce/copywrite', {
    product_name: productName,
    hint: hint,
    reference_assets: (assetNos || []).slice(0, 3)
  })
  if (res.code !== 200) throw new Error(res.message || '生成失败')
  return { content: res.data.content, analysis: res.data.analysis }
}
```

### Step 10：前端 — 修改三个页面的 copywrite 函数

```js
// 导入
import { formatAnalysisToText, getStyleDesc } from '@/utils/ecomFormat'

// copywrite 函数内
const { content, analysis } = await configStore.generateCopywriting(...)

// 联动填充（仅在字段为空时填入）
if (analysis) {
  if (!form.value.product_name) {
    form.value.product_name = analysis.product_name || ''
  }
  if (!form.value.style_desc) {
    form.value.style_desc = getStyleDesc(analysis.recommended_style)
  }
}
form.value.selling_points = formatAnalysisToText(analysis, content)
```

## 关键文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/service/aicommerce/provider/openai_vision_copywrite.go` | 修改 | 16字段结构体、新 prompt、response_format、JSON 解析规范化、格式化文本 |
| `api/service/aicommerce/image_service.go` | 修改 | Copywrite() 返回签名改为三值 |
| `api/handler/aicommerce/image_handler.go` | 修改 | 响应增加 analysis 字段 |
| `web/src/utils/ecomFormat.js` | 新建 | formatAnalysisToText + getStyleDesc 工具函数 |
| `web/src/store/ecom.js` | 修改 | generateCopywriting() 返回 {content, analysis} |
| `web/src/views/ecom/MainImagePage.vue` | 修改 | 联动填充 product_name + style_desc + selling_points |
| `web/src/views/ecom/DetailPagePage.vue` | 修改 | 同上 |
| `web/src/views/ecom/ClonePage.vue` | 修改 | 同上 |

## 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| 兼容 API 不支持 response_format | 400 时去掉该字段重试一次 |
| 模型返回 JSON 但字段缺失 | 规范化兜底（默认卖点 / default_shoot 风格） |
| recommended_style 值不合法 | 枚举校验 fallback 到 default_shoot |
| 前端 analysis 为 null | formatAnalysisToText + getStyleDesc 均做 null 检查降级 |
| 下游生图链路格式变化 | content 保持自然语言文本，{{.SellingPoints}} 链路不受影响 |

## SESSION_ID（供 /ccg:execute 使用）
- CODEX_SESSION: 019de18d-c9c9-7132-b040-53c741428744
- GEMINI_SESSION: e33bd56e-c974-43ad-8965-b1a830c284c8
