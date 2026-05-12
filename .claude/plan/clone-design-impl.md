# 克隆设计完整实施计划

> 任务：修复并完整实现「克隆设计」功能，将单输出修复为 N 风格图→N 结果，采用风格图为 source 策略，参考主图链路的 phase 占位模式。

## 决策摘要

| 决策项 | 选择 |
|--------|------|
| 生成策略 | 风格图（clone_assets[i]）作为 ImageToImage source，Prompt 描述产品身份 |
| 输出张数 | `len(clone_assets)`，每张风格图生成 1 张克隆结果 |
| 算力计费 | `credit_cost = len(clone_assets) × 12`，预扣 |
| 失败处理 | fail-soft：部分失败按张退还，全失败由 dispatcher 全退 |
| 占位模式 | 模仿主图链路：每张 clone_i 创建 phaseAsset，updatePhaseAsset 推进阶段 |
| 进度条 | 5%→10%+(i+1)/total×85% 平滑推进；每个占位卡片显示 phase |
| URL 解析 | 全部走 `resolveAssetURLs`，替换废弃的 `signedURL` |

---

## 后端实施

### 文件 1：`api/service/aicommerce/image_service.go`

**改动 1：常量定义**
```go
const CloneCreditPerImage = 12
```

**改动 2：`GenerateReq` 添加字段**
```go
type GenerateReq struct {
    // ... 现有字段
    ReferenceAssets []string `json:"reference_assets"`
    CloneAssets     []string `json:"clone_assets"`   // 新增：克隆风格图
    Model           string   `json:"model"`
}
```

**改动 3：`SubmitTask` 增加 clone 分支（switch 结构）**
```go
switch req.Module {
case ModuleWhiteBg:
    rembgPrice, _ := s.promptRepo.GetPriceByModel("rembg")
    if rembgPrice <= 0 { rembgPrice = 5 }
    n := len(req.ReferenceAssets)
    if n == 0 { return nil, fmt.Errorf("请上传至少 1 张参考图") }
    creditCost = rembgPrice * n
case ModuleClone:
    n := len(req.CloneAssets)
    if n == 0 { return nil, fmt.Errorf("请上传至少 1 张克隆参考图") }
    if len(req.ReferenceAssets) == 0 {
        return nil, fmt.Errorf("请上传至少 1 张商品参考图")
    }
    creditCost = CloneCreditPerImage * n
}
// 用 deductCredit 原子扣减，天然避免 TOCTOU
```

---

### 文件 2：`api/service/aicommerce/worker/chains/clone.go`（完全重构）

**核心结构**：
1. 读取 `reference_assets` + `clone_assets`，前者校验，后者迭代
2. 用 `resolveAssetURLs` 解析产品图 URL（用于校验存在性）
3. 为每张 clone_assets 创建 placeholder（`createPhaseAsset(db, task, "clone_i", PhaseRendering)`）
4. 循环 `processOneClone`，每次：
   - `updatePhaseAsset` 推进阶段：Rendering → Generating → Uploading
   - 风格图 URL 作为 `ImageToImageReq.ImageURL`
   - `buildClonePrompt`：保留产品身份，迁移风格
   - 失败 → `saveTypeError(db, task, "clone_i", err, placeholderID)`，继续
   - 成功 → `finalizePhaseAsset` 升级为正式 asset
5. 收尾：
   - 全成功 → return nil
   - 部分成功 → `refundFailedCloneCredits` 内部退款，更新 `task.CreditCost`，return nil
   - 全失败 → return firstErr（dispatcher 全退）

**关键函数**：
- `processOneClone(...)` — 处理单张风格图全流程
- `buildClonePrompt(...)` — 结构化 Prompt（保留产品 + 迁移风格 + 禁止 logo 抄袭）
- `cloneImageType(i int) string` — 返回 `"clone_{i}"`
- `refundFailedCloneCredits(db, task, total, succeeded, unit)` — 事务退款

**Prompt 模板**（核心句式）：
```
E-commerce product image generation.
Use the provided source image only as visual style, layout, lighting, composition, color palette, background mood, and photography reference.
Do not copy the source image product identity, logo, watermark, text, brand, packaging, or protected design.
Replace the source image product with the target product described below.
Preserve target product identity, category, material, color, shape, function, and key selling points.
Target product name: {productName}.
Target selling points: {sellingPoints}.
Platform requirements: {platformRules}.
Output language: {language}. Aspect ratio: {ratio}.
High quality, professional commercial photography, clean product presentation, realistic details.
```

**finalizePhaseAsset MetadataJSON**：
```go
{
    "image_type":            "clone_{i}",
    "source_style_asset_no": cloneAssetNo,
    "product_reference_nos": referenceAssetNos,
    "ratio":                 task.Ratio,
    "generation_strategy":   "style_image_as_source",
    "output_credit_charged": 12,
}
```

---

## 前端实施

### 文件 3：`web/src/views/ecom/ClonePage.vue`

**改动点**（按 diff）：

| 行 | 旧 | 新 |
|----|----|----|
| 99 | `参考图片 (最多3张产品白底图)` | `产品图 (最多3张产品白底图)` |
| 106 | `克隆参考图 (最多12张)` | `风格参考图 (最多12张)` |
| 116 | `:estimated-cost="12"` | `:estimated-cost="form.clone_assets.length * 12"` |
| 118 | tooltip 提到"参考图片"和"克隆参考图" | 改为"产品图"和"风格参考图" |
| 182 | `上传克隆参考图后点击...` | `上传产品图与风格参考图后点击...` |
| 256 | `请上传克隆参考图` | `请上传风格参考图` |
| 257 | `userPower < 12` | `userPower < form.clone_assets.length * 12` |

---

### 文件 4：`web/src/store/ecom.js`

**改动 1：`submitTask` 增加 clone 分支**（在 `data.image_type` 分支之后、`reference_assets` 通用分支之前）：

```js
} else if (data.module === 'clone' && Array.isArray(data.clone_assets) && data.clone_assets.length) {
  // 克隆模块：每张风格参考图对应 1 张输出，与后端 clone_{i} placeholder 对齐
  items.value = data.clone_assets.map((_, i) => ({
    image_type: `clone_${i}`,
    label: `风格克隆 ${i + 1}`,
    status: 'pending',
    phase: 'pending',
    progress: 0,
    url: null,
    asset_no: '',
  }))
} else if (Array.isArray(data.reference_assets) && data.reference_assets.length) {
  // white_bg / ratio_convert 等模块沿用旧逻辑
  ...
}
```

**改动 2：`resumeIfPending` 同步增加 clone 分支**（从 `taskData.input_json.clone_assets` 恢复占位）

---

## 视觉流程

```
[1] 用户上传：1 产品图 + 3 风格图
[2] 实时反馈：算力徽章 36 (3×12)
[3] 点击提交 → submitTask 立即预填 3 个 placeholder cards
[4] 后端按序生成：
    Card 1: rendering → generating → uploading → succeeded ✅
    Card 2: rendering → generating → failed ❌（退还 12 算力）
    Card 3: rendering → generating → uploading → succeeded ✅
[5] 任务 succeeded，task.credit_cost 更新为 24（2×12）
[6] 历史图库归档为 1 个任务下的 3 个 items
```

---

## 验收清单

- [ ] 上传 N 风格图 → 输出 N 张克隆结果
- [ ] 算力按 N×12 动态展示和扣费
- [ ] 单张失败不阻塞其他张
- [ ] 部分失败正确退还失败部分
- [ ] 全失败由 dispatcher 全退
- [ ] 占位卡片立即可见（提交后无 3 秒空窗）
- [ ] 每个 placeholder 显示 phase 进度
- [ ] OSS URL 全部走 `resolveAssetURLs`
- [ ] 历史图库归档为 1 任务 N 图
- [ ] 刷新页面可恢复进行中任务的 N 占位

---

## 风险与缓解

| 风险 | 缓解 |
|------|------|
| 用户算力不足导致 SubmitTask 失败 | `deductCredit` 原子扣减，WHERE power >= cost |
| ImageToImage 单图限制无法保留产品身份 | 通过 Prompt 详细描述产品（name + selling_points），禁止抄袭风格图的 logo |
| 串行 12 张耗时长 | YAGNI：当前阶段不引入并发，progress 每张递增让用户感知进展 |
| 部分退款事务失败 | 不返回 dispatcher，写 error_message 供运维补偿 |
| 历史克隆任务（旧格式）兼容 | EcomHistoryGroup 已基于 items 渲染，旧任务 items 为空时自然 fallback outputs |

---

## SESSION_ID（供后续阶段复用）
- CODEX_SESSION: `019e149e-032b-7521-9828-7c917f535466`
- GEMINI_SESSION: `27fdfb94-28ad-4c48-9580-f5f080351be0`
