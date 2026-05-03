# 📋 实施计划：Vision 图片分析代写商品卖点

## 任务类型
- [x] 全栈（后端 + 前端）

## 背景
三个表单（主图设计/详情页/克隆设计）已有"AI代写商品卖点"按钮，调用纯文本 Tongyi 接口。
现在扩展：表单已上传参考图时，调用中转站 Vision Chat API 分析图片生成更精准卖点。
API Key 从 `geekai_ai_models` 表读取（`name='gpt-4o' AND model_type='chat' AND status='active'`）。

## 技术方案
- **必须上传参考图**才能使用 AI 代写，无图时前端禁用按钮 + 后端返回 400 错误
- 前端传 `asset_no` 数组（不传 URL），后端验证 `user_id` 归属后拼 OSS URL
- 有图：查 DB 取 `gpt-4o` 的 `api_key` + `api_endpoint`，调 Vision Chat
- 图片上限 3 张（前后端双重限制）
- **算力扣减**：调用前扣减，失败时退还（与生图任务保持一致）
- 移除 Tongyi 纯文本 fallback，删除现有 Tongyi.GenerateCopywrite 的调用路径

## 全局编码约束（强制）
- 所有文件写入必须使用 **UTF-8 编码（无 BOM）**
- Go 文件中的中文字符串（注释、错误消息、日志）直接写 UTF-8，不使用转义序列
- 写入文件前确认编辑器/工具的输出编码为 UTF-8
- 禁止将中文写成 `\u` Unicode 转义或乱码字节序列

---

## 实施步骤

### Step 1：新增 Vision Copywrite Provider
**文件**：`api/service/aicommerce/provider/openai_vision_copywrite.go`（新建）

```go
package provider

type visionTextPart  struct { Type string `json:"type"`; Text string `json:"text"` }
type visionImagePart struct { Type string `json:"type"`; ImageURL struct{ URL string `json:"url"` } `json:"image_url"` }
type visionChatMessage struct { Role string `json:"role"`; Content interface{} `json:"content"` }

type OpenAIVisionCopywriter struct { baseURL, apiKey, model string; client *http.Client }

func NewOpenAIVisionCopywriter(baseURL, apiKey, model string) *OpenAIVisionCopywriter

func (c *OpenAIVisionCopywriter) GenerateCopywrite(ctx, productName, hint string, imageURLs []string) (string, error):
  // 校验：baseURL/apiKey/model 非空，imageURLs 1-3 张
  // 构造 messages：system(string) + user([]interface{}{textPart, imagePart...})
  // POST {baseURL}/chat/completions，Header: Authorization Bearer, Content-Type json
  // io.LimitReader(resp.Body, 2MB) 防内存溢出
  // 非 2xx 返回 "vision chat status N: error.message"（不暴露 apiKey）
  // 空 choices/content 返回错误
  // System Prompt 内容：电商卖点专家，识别图片商品特征，禁止虚构参数
```

### Step 2：扩展 ImageService.Copywrite
**文件**：`api/service/aicommerce/image_service.go`（修改）

新增结构体：
```go
type CopywriteReq struct {
    ProductName string
    Hint        string
    AssetNos    []string
}
const maxCopywriteImageCount = 3
```

修改签名：`Copywrite(ctx, userID uint, req CopywriteReq) (string, error)`

逻辑：
```
assetNos = 去重去空(req.AssetNos)

// 1. 强制要求参考图（无图直接拒绝）
if len(assetNos) == 0:
    return error "请先上传参考图，AI代写需要分析商品图片"

if len(assetNos) > 3:
    return error "参考图最多支持3张"

// 2. 查算力定价
creditCost = s.promptRepo.GetPriceByModel("vision-copywrite")
  // 找不到定价时用默认值 8

// 3. 扣减算力（先扣后用）
if err = s.deductCredit(ctx, userID, creditCost); err != nil:
    return error "积分不足: ..."

// 4. 解析资产 URL
imageURLs, err = s.resolveCopywriteImageURLs(ctx, userID, assetNos)
  // DB: SELECT * FROM geekai_ai_image_assets WHERE user_id=? AND asset_no IN ? AND deleted_at IS NULL
  // 任一 asset_no 不存在或不属于当前用户 → error
  // 拼 URL: https://{OssBucket}.oss-cn-hangzhou.aliyuncs.com/{OssKey}
  // OssBucket 为空时 fallback s.cfg.OSSBucket
if err != nil:
    _ = s.refundCredit(ctx, userID, creditCost)
    return error

// 5. 查 Vision 模型配置
visionModel, err = s.resolveCopywriteVisionModel(ctx)
  // DB: SELECT * FROM geekai_ai_models WHERE name='gpt-4o' AND model_type='chat' AND status='active' LIMIT 1
  // 找不到 → error
  // ApiKey 或 ApiEndpoint 为空 → error
if err != nil:
    _ = s.refundCredit(ctx, userID, creditCost)
    return error

// 6. 调用 Vision Chat
client = NewOpenAIVisionCopywriter(visionModel.ApiEndpoint, visionModel.ApiKey, visionModel.Name)
content, err = client.GenerateCopywrite(ctx, req.ProductName, req.Hint, imageURLs)
if err != nil:
    _ = s.refundCredit(ctx, userID, creditCost)
    return error

return content, nil
```

### Step 3：修改 Handler
**文件**：`api/handler/aicommerce/image_handler.go`（修改，L178-200）

```go
var body struct {
    ProductName     string   `json:"product_name"`
    Hint            string   `json:"hint"`
    AssetNos        []string `json:"asset_nos"`
    ReferenceAssets []string `json:"reference_assets"` // 兼容表单字段名
}
// 优先用 asset_nos，为空则取 reference_assets
assetNos := body.AssetNos
if len(assetNos) == 0 { assetNos = body.ReferenceAssets }

result, err := h.service.Copywrite(c.Request.Context(), userID, aicommerce.CopywriteReq{
    ProductName: body.ProductName,
    Hint:        body.Hint,
    AssetNos:    assetNos,
})
```

### Step 4：数据库补录（运维/配置步骤，非迁移 SQL）

**4a. 插入算力定价**（追加到 `database/aicommerce_migration.sql` 的 INSERT IGNORE 块）：
```sql
INSERT IGNORE INTO geekai_ai_model_price_config (model, module, credit_per_image, description)
VALUES ('vision-copywrite', 'copywrite', 8, 'AI代写商品卖点（图片分析，必须上传参考图）');
```

**4b. 插入 chat 模型配置**（手动执行，含敏感 api_key，不提交代码）：
```sql
INSERT INTO geekai_ai_models (name, display_name, provider, model_type, api_endpoint, api_key, capabilities, status, sort_order)
VALUES ('gpt-4o', 'GPT-4o Vision', 'relay', 'chat', 'https://gpt-best.apifox.cn/v1', '<YOUR_KEY>', 'vision', 'active', 100);
```

### Step 5：扩展前端 Store
**文件**：`web/src/store/ecom.js`（修改）

在 `useEcomConfigStore` 中新增并 return `generateCopywriting`：
```javascript
const generateCopywriting = async (productName, hint, assetNos = []) => {
  const res = await httpPost('/api/ai-commerce/copywrite', {
    product_name: productName,
    hint: hint,
    reference_assets: (assetNos || []).slice(0, 3)
  })
  if (res.code !== 200) throw new Error(res.message || '生成失败')
  return res.data.content
}
```

### Step 6：修改三个表单组件
**文件**：`web/src/views/ecom/MainImagePage.vue`、`DetailPagePage.vue`、`ClonePage.vue`

三个文件改动一致（ClonePage 确认只传 `reference_assets`，不传 clone_assets）：

**模板层**（按钮）：
```html
<!-- 无图时禁用按钮，Tooltip 提示原因 -->
<el-tooltip
  :content="form.reference_assets.length ? '' : '请先上传参考图'"
  :disabled="form.reference_assets.length > 0"
  placement="top"
>
  <button
    class="copywrite-btn"
    type="button"
    @click="copywrite"
    :disabled="copywriting || form.reference_assets.length === 0"
  >
    <template v-if="copywriting">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>AI 分析生成中...</span>
    </template>
    <template v-else>
      <span>AI 识别图片并代写卖点</span>
    </template>
  </button>
</el-tooltip>
```

**Script 层**（替换 copywrite 函数）：
```javascript
import { useEcomConfigStore } from '@/store/ecom'
const configStore = useEcomConfigStore()

const copywrite = async () => {
  if (!form.value.product_name) { ElMessage.warning('请先输入产品名称'); return }
  if (!form.value.reference_assets.length) { ElMessage.warning('请先上传参考图'); return }
  copywriting.value = true
  try {
    const content = await configStore.generateCopywriting(
      form.value.product_name,
      form.value.selling_points,
      form.value.reference_assets
    )
    form.value.selling_points = content
    ElMessage.success('已根据参考图生成卖点')
  } catch (e) {
    ElMessage.error('代写失败：' + e.message)
  } finally {
    copywriting.value = false
  }
}
```

---

## 关键文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/service/aicommerce/provider/openai_vision_copywrite.go` | 新建 | Vision Chat provider |
| `api/service/aicommerce/image_service.go` | 修改 | 扩展 Copywrite 签名，增加算力扣减、资产查询和模型路由 |
| `api/handler/aicommerce/image_handler.go` | 修改 L178-200 | 接受 asset_nos/reference_assets，传 userID 给 service |
| `database/aicommerce_migration.sql` | 修改 | 追加 copywrite / vision-copywrite 定价 INSERT |
| `web/src/store/ecom.js` | 修改 | 新增 generateCopywriting action |
| `web/src/views/ecom/MainImagePage.vue` | 修改 L27-30, L160-167 | 按钮 UX + 调用 store action |
| `web/src/views/ecom/DetailPagePage.vue` | 修改 L26-29, L156-163 | 同上 |
| `web/src/views/ecom/ClonePage.vue` | 修改 L26-29, L146-153 | 同上（仅传 reference_assets） |

## 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| 算力未扣就调用 API | 先 `deductCredit`，任何错误路径都 `refundCredit` |
| 定价记录缺失 | `GetPriceByModel` 找不到时用硬编码默认值 8，不阻断流程 |
| 用户绕过前端直接调接口（无图） | 后端 `len(assetNos)==0` 直接返回 400，不调用任何 AI |
| gpt-4o 未配置 | 返回明确错误"需在 geekai_ai_models 配置 gpt-4o chat 模型" |
| 图片 OSS URL 鉴权过期 | 当前用公开 URL 拼接（与现有 signedURL 风格一致）；后续可加签名 |
| Vision API 超时（>45s） | client timeout 45s，前端 loading 全程覆盖 |
| 越权访问他人资产 | DB 查询强制 `user_id = ?`，不匹配直接报错不 fallback |
| ApiKey 泄漏到错误响应 | error 仅含 status code 和 error.message，不打印 apiKey |

## SESSION_ID（供 /ccg:execute 使用）
- CODEX_SESSION: 019dd9ba-4769-71e1-a315-5b724f151ebd
- GEMINI_SESSION: db197c8c-1e1b-4a82-943e-b3b110ba7520
