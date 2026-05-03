# 实施计划：16字段视觉分析扩展 prompt 模板体系

## 任务类型
- [x] 全栈（后端 Go + DB 模板）

## 背景

当前 `prompt.Vars` 只有 9 个字段（ProductName / SellingPoints / StyleDesc 等），模板无法利用视觉代写返回的 16 字段结构化信息（颜色/材质/版型/印花约束/目标场景等）。

参考项目 `ecommerce-image-suite-main/scripts/generate.py` 的 `build_prompt()` 大量使用：
- `desc`（= `product_description_for_prompt`，英文商品描述）
- `selling_points`（结构化卖点列表）
- `garment_position`（top/bottom/full-body/non-apparel，决定模特搭配逻辑）
- `print_design_lock`（印花一致性约束短语）
- `color`、`material`、`style`（版型）
- `target_scenes`（目标场景列表）
- `product_type`（服装/3C数码/家居等）
- `template_set`（= recommended_style 映射的风格编号 1-6）

目标：
1. 扩展 `prompt.Vars`，加入上述字段
2. Worker fallback 调视觉代写后，把 `CopywriteAnalysis` 字段填入 `Vars`
3. 更新 `main_image.go` 渲染时传入扩展字段
4. **DB 模板可在 `user_template` 中使用新变量**（`{{.ProductDescForPrompt}}`、`{{.GarmentPosition}}` 等），管理员可在后台根据参考项目逻辑编写高质量英文 Prompt 模板

## 判断逻辑（核心）

```
Worker 收到任务后：
  若 task.InputJSON 中存在 analysis 字段（用户点击过"AI 代写卖点"）
    → 直接用该 analysis 的 16 字段填充 Vars，不再调 AI
    → SellingPoints 用前端文本（不覆盖）
  若 task.InputJSON 中不存在 analysis 字段（用户手动输入卖点）
    → 后端检查 reference_assets，为空 → return error（任务失败，前端轮询到 failed 提示用户）
    → 有参考图 → 以用户卖点为 hint 调视觉代写，取 analysis 16 字段
    → SellingPoints 不为空时不覆盖（保留用户输入）
  StyleDesc：前端传入不为空 → 用前端的；否则用 analysis.RecommendedStyle 映射的中文描述
```

## 技术方案

### Step 0：前端存储并提交 analysis（`MainImagePage.vue` + `DetailPagePage.vue` + `ClonePage.vue`）

三个页面的 `form` 增加 `analysis` 字段，`copywrite()` 成功后将 analysis 存入 form，`submit()` 通过 `...form.value` 自动带上：

```js
const form = ref({
  // ...原有字段不变...
  analysis: null,  // 新增：AI 代写返回的结构化分析结果
})

const copywrite = async () => {
  // ...原有逻辑...
  const { content, analysis } = await configStore.generateCopywriting(...)
  if (analysis) {
    if (!form.value.product_name) form.value.product_name = analysis.product_name || ''
    if (!form.value.style_desc) form.value.style_desc = getStyleDesc(analysis.recommended_style)
    form.value.analysis = analysis  // 存入 form，submit 时随表单带上
  }
  form.value.selling_points = formatAnalysisToText(analysis, content)
}
```

**注意**：用户若手动修改卖点后再次提交，`analysis` 仍在 form 中。为保持语义正确（手动修改卖点 = 不再信任旧 analysis），可在用户编辑 `selling_points` 时清除 `analysis`：

```js
// watch selling_points 手动编辑时清除 analysis
watch(() => form.value.selling_points, () => {
  // 仅在非 copywrite 填充期间清除
  if (!copywriting.value) form.value.analysis = null
})
```

### Step 1：扩展 `prompt.Vars`（`prompt/engine.go`）

新增字段，与 CopywriteAnalysis 16字段对齐：

```go
type Vars struct {
    // 原有字段
    ProductName         string
    SellingPoints       string   // 自然语言文本（模板兼容）
    ImageTypeDesc       string
    Platform            string
    PlatformRules       string
    Language            string
    Ratio               string
    StyleDesc           string
    ReferenceImageCount int

    // 新增：来自 CopywriteAnalysis 的视觉分析字段
    ProductDescForPrompt string   // product_description_for_prompt（英文，≤50词）
    ProductType          string   // 服装 | 3C数码 | 家居 | 美妆 | 食品 | 其他
    GarmentPosition      string   // top | bottom | full-body | non-apparel
    Color                string   // 精确英文色值
    Material             string   // 主要材质
    ProductStyle         string   // 商品风格（如：法式浪漫）
    StyleVariant         string   // 版型（宽松/修身 等）
    PrintDesign          string   // 印花描述
    PrintDesignLock      string   // 精确约束短语
    TargetAudience       string   // 目标人群
    TargetScenes         string   // 目标场景（逗号分隔）
    RecommendedStyle     string   // AI推荐风格 value
    VisualFeatures       string   // 视觉特征（逗号分隔）
    // 卖点结构化（供模板按索引访问）
    SP0Zh    string  // selling_points[0].zh
    SP0Desc  string  // selling_points[0].zh_desc
    SP0En    string  // selling_points[0].en
    SP1Zh    string
    SP1Desc  string
    SP1En    string
    SP2Zh    string
    SP2Desc  string
    SP2En    string
    SP3Zh    string
    SP3Desc  string
    SP3En    string
    SP4Zh    string
    SP4Desc  string
    SP4En    string
}
```

同步更新 `renderOne()` 中的 `safeVars` 赋值（所有新字段加 `html.EscapeString`）。

### Step 2：新增 `VarsFromAnalysis()` 辅助函数（`prompt/engine.go`）

```go
// VarsFromAnalysis 将 CopywriteAnalysis 字段合并进基础 Vars
func VarsFromAnalysis(base Vars, a *provider.CopywriteAnalysis) Vars {
    if a == nil {
        return base
    }
    v := base
    v.ProductDescForPrompt = a.ProductDescriptionForPrompt
    v.ProductType          = a.ProductType
    v.GarmentPosition      = a.GarmentPosition
    v.Color                = a.Color
    v.Material             = a.Material
    v.ProductStyle         = a.ProductStyle
    v.StyleVariant         = a.Style
    v.PrintDesign          = a.PrintDesign
    v.PrintDesignLock      = a.PrintDesignLock
    v.TargetAudience       = a.TargetAudience
    v.TargetScenes         = strings.Join(a.TargetScenes, "、")
    v.RecommendedStyle     = a.RecommendedStyle
    v.VisualFeatures       = strings.Join(a.VisualFeatures, "、")
    // 卖点索引展开
    for i, sp := range a.SellingPoints {
        if i >= 5 { break }
        switch i {
        case 0: v.SP0Zh, v.SP0Desc, v.SP0En = sp.Zh, sp.ZhDesc, sp.En
        case 1: v.SP1Zh, v.SP1Desc, v.SP1En = sp.Zh, sp.ZhDesc, sp.En
        case 2: v.SP2Zh, v.SP2Desc, v.SP2En = sp.Zh, sp.ZhDesc, sp.En
        case 3: v.SP3Zh, v.SP3Desc, v.SP3En = sp.Zh, sp.ZhDesc, sp.En
        case 4: v.SP4Zh, v.SP4Desc, v.SP4En = sp.Zh, sp.ZhDesc, sp.En
        }
    }
    return v
}
```

**注意**：`prompt` 包不能直接 import `provider` 包（避免循环依赖）。  
解决：`VarsFromAnalysis` 接收已展开的具名参数，或把它放在 `chains` 包。  
**实际方案**：在 `chains/main_image.go` 中写内联展开逻辑，不新增跨包依赖。

### Step 3：修改 `main_image.go` — 后台静默视觉代写 + 填充扩展 Vars

```go
func RunMainImage(...) error {
    input := task.InputJSON
    productName, _ := input["product_name"].(string)
    sellingPoints, _ := input["selling_points"].(string)
    styleDesc, _     := input["style_desc"].(string)

    // 基础 Vars（非分析字段先填）
    vars := prompt.Vars{
        ProductName:   productName,
        SellingPoints: sellingPoints,
        ImageTypeDesc: prompt.ImageTypeDesc(task.ImageType),
        Platform:      task.Platform,
        PlatformRules: prompt.PlatformRules(db, task.Platform),
        Language:      task.Language,
        Ratio:         task.Ratio,
        StyleDesc:     styleDesc,
    }

    // 判断前端是否已带入 AI 代写的 analysis（用户点击过"AI 代写卖点"按钮）
    var analysisJSON map[string]interface{}
    if raw, ok := input["analysis"]; ok && raw != nil {
        analysisJSON, _ = raw.(map[string]interface{})
    }

    if analysisJSON != nil {
        // Case A：前端已有 analysis → 直接解析填充，不再重新调 AI
        analysis := parseAnalysisFromMap(analysisJSON)
        vars = fillAnalysisVars(vars, analysis)
        if styleDesc == "" {
            vars.StyleDesc = getStyleDesc(analysis.RecommendedStyle)
        }
    } else {
        // Case B：用户手动输入卖点，无 analysis → 后端校验参考图，再调视觉代写
        assetNos, _ := extractStringSlice(input, "reference_assets")
        if len(assetNos) == 0 {
            return fmt.Errorf("请上传参考图后再生成")  // 返回错误，任务失败，前端轮询到 failed 状态后提示用户
        }
        imageURLs := resolveAssetURLs(db, task.UserId, assetNos, cfg)
        if visionClient, err := buildVisionClient(db); err == nil {
            // hint = 用户卖点（为空时 AI 自由生成卖点）
            if content, analysis, err := visionClient.GenerateCopywrite(ctx, productName, sellingPoints, imageURLs); err == nil {
                if sellingPoints == "" {
                    vars.SellingPoints = content  // 卖点为空才覆盖
                }
                vars = fillAnalysisVars(vars, analysis)
                if styleDesc == "" && analysis != nil {
                    vars.StyleDesc = getStyleDesc(analysis.RecommendedStyle)
                }
            }
        }
    }

    // 渲染 Prompt（Vars 已包含所有可用字段）
    tmpl, err := repo.FindTemplate(...)
    rendered, err := prompt.Render(tmpl.UserTemplate, tmpl.NegativeTemplate, vars)
    ...
}

// parseAnalysisFromMap 将 task.InputJSON["analysis"] 的 map 反序列化为 CopywriteAnalysis
// （InputJSON 经 JSON 往返后嵌套对象变成 map[string]interface{}，需二次序列化解析）
func parseAnalysisFromMap(m map[string]interface{}) *provider.CopywriteAnalysis {
    b, err := json.Marshal(m)
    if err != nil { return nil }
    var a provider.CopywriteAnalysis
    if err := json.Unmarshal(b, &a); err != nil { return nil }
    return &a
}

// getStyleDesc 风格 value → 中文描述（与前端 ecomFormat.js 保持一致）
func getStyleDesc(style string) string {
    switch style {
    case "default_shoot":     return "标准电商商拍，干净明亮，重点突出商品"
    case "lifestyle_mag":     return "自然光，有氛围感和生活质感"
    case "minimal_cold":      return "极简留白，高反差，奢侈品质感"
    case "energetic_hit":     return "高饱和度，大字冲击，活力感强"
    case "dark_quality":      return "深色系，电影质感，戏剧性打光"
    case "asymmetric_layout": return "非对称布局，左侧大图突出主体，右侧细节图"
    default:                  return ""
    }
}

// fillAnalysisVars 内联展开 CopywriteAnalysis 到 prompt.Vars（在 chains 包，避免循环依赖）
func fillAnalysisVars(v prompt.Vars, a *provider.CopywriteAnalysis) prompt.Vars {
    if a == nil { return v }
    v.ProductDescForPrompt = a.ProductDescriptionForPrompt
    v.ProductType          = a.ProductType
    v.GarmentPosition      = a.GarmentPosition
    v.Color                = a.Color
    v.Material             = a.Material
    v.ProductStyle         = a.ProductStyle
    v.StyleVariant         = a.Style
    v.PrintDesign          = a.PrintDesign
    v.PrintDesignLock      = a.PrintDesignLock
    v.TargetAudience       = a.TargetAudience
    v.TargetScenes         = strings.Join(a.TargetScenes, "、")
    v.RecommendedStyle     = a.RecommendedStyle
    v.VisualFeatures       = strings.Join(a.VisualFeatures, "、")
    for i, sp := range a.SellingPoints {
        if i >= 5 { break }
        switch i {
        case 0: v.SP0Zh, v.SP0Desc, v.SP0En = sp.Zh, sp.ZhDesc, sp.En
        case 1: v.SP1Zh, v.SP1Desc, v.SP1En = sp.Zh, sp.ZhDesc, sp.En
        case 2: v.SP2Zh, v.SP2Desc, v.SP2En = sp.Zh, sp.ZhDesc, sp.En
        case 3: v.SP3Zh, v.SP3Desc, v.SP3En = sp.Zh, sp.ZhDesc, sp.En
        case 4: v.SP4Zh, v.SP4Desc, v.SP4En = sp.Zh, sp.ZhDesc, sp.En
        }
    }
    return v
}
```

### Step 4：更新 `renderOne()` 中的 `safeVars` 赋值

把所有新字段加入 `html.EscapeString` 赋值（用户来源字段），系统字段直接复制。

### Step 5：DB 模板示例（说明用，不做迁移）

管理员可在 `user_template` 中使用新变量编写模板，例如主图卖点图模板：

```
E-commerce product photography, {{.ProductDescForPrompt}}.
{{if eq .GarmentPosition "non-apparel"}}Product placed on clean surface.{{else}}Model wearing the product.{{end}}
Key feature 1: {{.SP0En}} — {{.SP0Desc}}
Key feature 2: {{.SP1En}} — {{.SP1Desc}}
Color: {{.Color}}. Material: {{.Material}}.
{{if .PrintDesignLock}}{{.PrintDesignLock}}{{end}}
Platform: {{.PlatformRules}}. Language: {{.Language}}. Style: {{.StyleDesc}}.
```

## 关键文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `web/src/views/ecom/MainImagePage.vue` | 修改 | form 加 analysis 字段；copywrite() 存入；watch selling_points 清除 |
| `web/src/views/ecom/DetailPagePage.vue` | 修改 | 同上 |
| `web/src/views/ecom/ClonePage.vue` | 修改 | 同上 |
| `api/service/aicommerce/prompt/engine.go` | 修改 | Vars 新增 20+ 字段，renderOne safeVars 同步更新 |
| `api/service/aicommerce/worker/chains/main_image.go` | 修改 | Case A/B 判断 + fillAnalysisVars + parseAnalysisFromMap + resolveAssetURLs + buildVisionClient + getStyleDesc |
| `api/service/aicommerce/worker/chains/utils.go` | 不变 | extractStringSlice / signedURL 已有 |
| `api/handler/admin/aicommerce_handler.go` | **不改** | admin 端无预览/渲染功能，跳过 |

## 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| 循环依赖（prompt ↔ provider） | fillAnalysisVars / parseAnalysisFromMap 放 chains 包 |
| renderOne safeVars 漏字段 | 编译器在 struct 字面量中可检测，配合 go vet 检查 |
| 旧模板用不到新字段 | Go template 未引用的变量不报错，完全向后兼容 |
| Case B 无参考图绕过前端限制 | 后端显式检查 reference_assets，为空 return error，任务标记 failed |
| vision API 超时阻塞 worker | 45s 超时已设置；worker 异步执行，不影响前端响应 |
| 用户修改卖点后 analysis 语义失效 | watch selling_points，非代写期间编辑时清空 form.analysis |
| analysis 字段经 JSON 往返变 map | parseAnalysisFromMap 二次序列化解决 |

## SESSION_ID（供 /ccg:execute 使用）
- CODEX_SESSION: N/A（Claude 直接实施）
- GEMINI_SESSION: N/A
