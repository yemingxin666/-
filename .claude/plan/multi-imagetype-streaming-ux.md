# 📋 实施计划：多选图片类型流式展示 UX

## 任务类型
- [x] 后端 (Codex)
- [x] 前端 (Gemini)
- [x] 全栈

## 背景
当前 `GetTask` 返回平铺 `outputs: []string`，多选图片类型（如选 5 个）用户需等待全部完成（~50s）才能看到任何结果。后端 worker 已在 `metadata_json.image_type` 写入类型标签，具备逐步返回中间结果的基础。

worker 已修复为：单个类型失败时 `continue` 跳过（记录 error asset），继续执行后续类型；全部失败才返回错误。error asset 的 `metadata_json.error` 字段不为空，`OssKey` 为空。

## 技术方案
**Option A（最小侵入）**：在 `GetTask` 服务层构建结构化 `items`，保留 `outputs` 向后兼容。前端提交时立即初始化占位 items，轮询时合并更新——用户看到图片"一张一张出现"。

---

## 实施步骤

### 后端（3 个文件）

#### Step 1 — 新增 DTO + 工具函数（`api/service/aicommerce/image_service.go`）

在 `GetTask` 函数上方添加：

```go
// ImageTaskItem 单个图片类型的生成状态
type ImageTaskItem struct {
    ImageType string  `json:"image_type"`
    Label     string  `json:"label"`
    Status    string  `json:"status"` // "pending" | "running" | "succeeded" | "failed"
    URL       *string `json:"url"`
}

// ImageTaskResult GetTask 完整响应
type ImageTaskResult struct {
    Task     *model.AiImageTask
    Outputs  []string
    Items    []ImageTaskItem
    Progress int
}
```

新增工具函数（同文件，`GetTask` 之后）：

```go
// imageTypeLabel 返回图片类型的中文标签
func imageTypeLabel(imageType string) string {
    labels := map[string]string{
        // main_image
        "traffic_cover":       "引流封面",
        "core_selling_point":  "核心卖点",
        "scene_immersion":     "场景代入",
        "value_breakdown":     "价值拆解",
        "competitor_comparison": "竞品对比",
        "detail_display":      "细节展示",
        "effect_proof":        "效果证明",
        "trust_building":      "信任消疑",
        "final_push":          "临门一脚",
        // detail_page
        "hero_visual":         "首屏主视觉",
        "core_selling":        "核心卖点图",
        "usage_scene":         "使用场景图",
        "multi_angle":         "多视角图",
        "atmosphere":          "场景氛围图",
        "product_detail":      "商品细节图",
        "brand_story":         "品牌故事图",
        "size_capacity":       "尺寸容量尺码图",
        "effect_comparison":   "效果对比图",
        "spec_reference":      "详细规格参考图",
        "craft_process":       "工艺制作图",
        "accessory_gift":      "配件赠品图",
        "series_showcase":     "系列展示图",
        "ingredient":          "商品成分图",
        "after_sales":         "售后保障图",
        "usage_guide":         "使用建议图",
    }
    if l, ok := labels[imageType]; ok {
        return l
    }
    return imageType
}

// buildTaskItems 从 task + assets 推导每个类型的状态
func buildTaskItems(task *model.AiImageTask, assets []model.AiImageAsset) ([]ImageTaskItem, []string, int) {
    // outputs 保持向后兼容
    outputs := make([]string, 0, len(assets))
    for _, a := range assets {
        outputs = append(outputs, a.OssKey)
    }

    // assetByType: 从 metadata_json["image_type"] 建立映射
    // OssKey 非空 = succeeded；OssKey 为空但有 error 字段 = failed
    assetByType := make(map[string]*model.AiImageAsset, len(assets))
    for i := range assets {
        if t, ok := assets[i].MetadataJSON["image_type"].(string); ok && t != "" {
            assetByType[t] = &assets[i]
        }
    }

    requestedTypes := splitImageTypes(task.ImageType)
    total := len(requestedTypes)
    if total == 0 {
        return nil, outputs, task.Progress
    }

    items := make([]ImageTaskItem, 0, total)
    succeededCount := 0
    hasRunning := false

    for _, t := range requestedTypes {
        item := ImageTaskItem{
            ImageType: t,
            Label:     imageTypeLabel(t),
        }
        if a, ok := assetByType[t]; ok {
            if a.OssKey != "" {
                url := a.OssKey
                item.Status = "succeeded"
                item.URL = &url
                succeededCount++
            } else {
                // error asset：OssKey 为空，metadata_json.error 有值
                item.Status = "failed"
            }
        } else if task.Status == string(model.TaskStatusRunning) && !hasRunning {
            item.Status = "running"
            hasRunning = true
        } else {
            item.Status = "pending"
        }
        items = append(items, item)
    }

    progress := succeededCount * 100 / total
    return items, outputs, progress
}
```

#### Step 2 — 修改 `GetTask` 签名（`api/service/aicommerce/image_service.go`）

```go
// 旧签名
func (s *ImageService) GetTask(ctx context.Context, userID uint, taskNo string) (*model.AiImageTask, []model.AiImageAsset, error)

// 新签名
func (s *ImageService) GetTask(ctx context.Context, userID uint, taskNo string) (*ImageTaskResult, error) {
    var task model.AiImageTask
    if err := s.db.Where("task_no = ? AND user_id = ? AND deleted_at IS NULL", taskNo, userID).First(&task).Error; err != nil {
        return nil, err
    }
    var assets []model.AiImageAsset
    if err := s.db.Where("task_id = ? AND kind = ? AND deleted_at IS NULL", task.Id, model.AssetKindGenerated).
        Order("created_at ASC, id ASC").Find(&assets).Error; err != nil {
        return nil, err
    }
    items, outputs, progress := buildTaskItems(&task, assets)
    return &ImageTaskResult{Task: &task, Outputs: outputs, Items: items, Progress: progress}, nil
}
```

#### Step 3 — 更新 Handler（`api/handler/aicommerce/image_handler.go`）

`GetTask` handler（约 L127）：
```go
// 旧：task, assets, err := h.service.GetTask(...)
// 新：
result, err := h.service.GetTask(c.Request.Context(), userID, taskNo)
if err != nil { resp.ERROR(c, "任务不存在"); return }

resp.SUCCESS(c, gin.H{
    "task_no":  result.Task.TaskNo,
    "module":   result.Task.Module,
    "status":   result.Task.Status,
    "progress": result.Progress,
    "outputs":  result.Outputs,
    "items":    result.Items,
    "error":    result.Task.ErrorMessage,
})
```

`TaskEvents` handler 中 ownership 校验（约 L154）：
```go
// 旧：if _, _, err := h.service.GetTask(...); err != nil
// 新：if _, err := h.service.GetTask(...); err != nil
```

---

### 前端（5 个文件）

#### Step 4 — Store 增加 `items`（`web/src/store/ecom.js`）

```js
// 新增 ref
const items = ref([])  // [{ image_type, label, status, url }]

// submitTask 中：提交成功后立即根据请求的 image_type 初始化占位
const submitTask = async (endpoint, data) => {
  const res = await httpPost(endpoint, data)
  const creditCost = res.data.credit_cost || 0
  currentTask.value = { task_no: res.data.task_no, status: res.data.status, progress: 0, credit_cost: creditCost }
  outputs.value = []
  // 初始化占位 items（仅 main_image / detail_page 模块有 image_type）
  if (data.image_type) {
    const typeLabels = [...configStore.mainImageTypes, ...configStore.detailPageTypes]
    items.value = data.image_type.split(',').filter(Boolean).map(t => {
      const meta = typeLabels.find(x => x.value === t)
      return { image_type: t, label: meta?.label || t, status: 'pending', url: null }
    })
  } else {
    items.value = []
  }
  useEcomConfigStore().deductPower(creditCost)
  startPolling()
  return res.data
}

// submitTask 成功后持久化 task_no 到 localStorage
// 放在 startPolling() 之前
localStorage.setItem('ecom_pending_task', JSON.stringify({
  task_no: res.data.task_no,
  module: data.module,
  image_type: data.image_type || ''
}))

// startPolling 结束后（succeeded/failed）清除 localStorage
// 在 stopPolling() 调用处同步清除：
localStorage.removeItem('ecom_pending_task')

// 新增 resumeIfPending：页面初始化时调用，检查 localStorage 是否有未完成任务
const resumeIfPending = async () => {
  const raw = localStorage.getItem('ecom_pending_task')
  if (!raw) return
  try {
    const { task_no, module, image_type } = JSON.parse(raw)
    // 先查一次任务状态，已完成则清除不恢复
    const res = await httpGet(`/api/ai-commerce/tasks/${task_no}`)
    if (res.code !== 200) { localStorage.removeItem('ecom_pending_task'); return }
    const taskData = res.data
    if (taskData.status === 'succeeded' || taskData.status === 'failed') {
      localStorage.removeItem('ecom_pending_task')
      return
    }
    // 恢复 store 状态
    const configStore = useEcomConfigStore()
    const typeLabels = [...configStore.mainImageTypes, ...configStore.detailPageTypes]
    currentTask.value = { task_no, status: taskData.status, progress: taskData.progress || 0 }
    outputs.value = taskData.outputs || []
    if (taskData.items?.length) {
      items.value = taskData.items
    } else if (image_type) {
      items.value = image_type.split(',').filter(Boolean).map(t => {
        const meta = typeLabels.find(x => x.value === t)
        return { image_type: t, label: meta?.label || t, status: 'pending', url: null }
      })
    }
    startPolling()
  } catch {
    localStorage.removeItem('ecom_pending_task')
  }
}

// startPolling 中：优先用 items 更新，兼容旧 outputs
const startPolling = () => {
  pollTimer = setInterval(async () => {
    const res = await httpGet(`/api/ai-commerce/tasks/${currentTask.value.task_no}`)
    if (res.code === 200) {
      Object.assign(currentTask.value, res.data)
      if (res.data.items?.length) {
        items.value = res.data.items
        outputs.value = res.data.outputs || []
      } else {
        outputs.value = res.data.outputs || []
      }
      if (res.data.status === 'succeeded' || res.data.status === 'failed') stopPolling()
    }
  }, 3000)
}

// reset 中：清空 items + 清除 localStorage
const reset = () => {
  stopPolling()
  currentTask.value = null
  outputs.value = []
  items.value = []
  localStorage.removeItem('ecom_pending_task')
}

// return 暴露 items 和 resumeIfPending
return { ..., items, resumeIfPending, ... }
```

#### Step 5 — 进度条加 chips（`web/src/components/ecom/EcomProgressBar.vue`）

在现有进度条下方增加 chips 行：
```vue
<template>
  <!-- 现有进度条内容保持不变 -->
  ...
  <!-- 新增：类型状态 chips -->
  <div v-if="items.length > 0" class="type-chips">
    <el-tag
      v-for="item in items"
      :key="item.image_type"
      :type="item.status === 'succeeded' ? 'success' : item.status === 'running' ? '' : 'info'"
      size="small"
      class="type-chip"
    >
      <el-icon v-if="item.status === 'running'" class="is-loading"><Loading /></el-icon>
      <el-icon v-else-if="item.status === 'succeeded'"><Check /></el-icon>
      {{ item.label }}
    </el-tag>
  </div>
</template>

<script setup>
defineProps({
  // 现有 props 保持不变
  items: { type: Array, default: () => [] }
})
</script>

<style scoped>
.type-chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
.type-chip { cursor: default; }
</style>
```

#### Step 6 — 结果卡片加 label + skeleton（`web/src/components/ecom/EcomResultCard.vue`）

```vue
<!-- props 新增 label、status -->
defineProps({
  url: String,
  label: { type: String, default: '' },
  status: { type: String, default: 'succeeded' }  // pending | running | succeeded | failed
})

<!-- template：url 为空时显示 skeleton，有 label 时显示左上角标签 -->
<el-skeleton :loading="!url" animated>
  <template #template>
    <el-skeleton-item variant="image" style="width:100%;aspect-ratio:1" />
    <div class="skeleton-label">{{ label || '生成中...' }}</div>
  </template>
  <template #default>
    <div class="card-wrap">
      <img :src="url" ... />
      <div v-if="label" class="label-badge">{{ label }}</div>
      <!-- 现有操作按钮保持不变 -->
    </div>
  </template>
</el-skeleton>
```

#### Step 7 — MainImagePage.vue + DetailPagePage.vue 使用 items

```vue
<!-- 结果区域：优先用 items（支持占位），降级用 outputs -->
<div v-if="taskStore.items.length || taskStore.outputs.length" class="result-grid">
  <template v-if="taskStore.items.length">
    <EcomResultCard
      v-for="item in taskStore.items"
      :key="item.image_type"
      :url="item.url"
      :label="item.label"
      :status="item.status"
      @regenerate="submit"
      @delete="taskStore.reset()"
    />
  </template>
  <template v-else>
    <!-- 降级：其他模块的旧逻辑 -->
    <EcomResultCard v-for="(url, i) in taskStore.outputs" :key="i" :url="url" @regenerate="submit" @delete="taskStore.reset()" />
  </template>
</div>

<!-- 进度条传入 items -->
<EcomProgressBar v-if="taskStore.currentTask" :items="taskStore.items" />

// script setup 中：
onMounted(() => taskStore.resumeIfPending())
```

---

## 关键文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/service/aicommerce/image_service.go` | 修改 | 新增 DTO、`buildTaskItems`、`imageTypeLabel`；修改 `GetTask` 签名 |
| `api/handler/aicommerce/image_handler.go` | 修改 | `GetTask` handler 使用新签名；更新 SSE ownership 校验 |
| `web/src/store/ecom.js` | 修改 | 新增 `items` ref；`submitTask` 初始化占位 + 写 localStorage；`startPolling` 合并 items + 完成时清 localStorage；新增 `resumeIfPending` |
| `web/src/components/ecom/EcomProgressBar.vue` | 修改 | 新增 `items` prop；增加 chips 行 |
| `web/src/components/ecom/EcomResultCard.vue` | 修改 | 新增 `label`/`status` props；skeleton 占位 |
| `web/src/views/ecom/MainImagePage.vue` | 修改 | 结果区域使用 items；进度条传入 items；`onMounted` 调用 `resumeIfPending` |
| `web/src/views/ecom/DetailPagePage.vue` | 修改 | 同 MainImagePage |

## 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| 旧 assets 无 `metadata_json.image_type`（非 main_image 模块生成的） | `buildTaskItems` 回退：单类型任务 totalTypes=1，items 仅含 1 项；`outputs` 始终正常返回 |
| `WhiteBg`/`Clone` 等页面未使用 items | store 保持 `outputs` 兼容；这些页面 `items` 为空，继续走旧逻辑 |
| `GetTask` 签名变更影响 SSE handler | 明确在 Step 3 同步修改 `TaskEvents` 里的调用点 |
| model.TaskStatusRunning 常量名 | 执行前 grep 确认实际常量值 |
| localStorage key 冲突（多用户同一浏览器） | key 加 userID 后缀：`ecom_pending_task_${userID}`；userID 从 configStore 取 |
| 刷新后恢复的 items 全为 pending（后端还在跑） | `resumeIfPending` 先查一次 GetTask，用返回的 items 初始化，避免全 pending 闪烁 |
| 任务已完成但 localStorage 未清（异常退出） | `resumeIfPending` 检查 status 为 succeeded/failed 时直接清除，不恢复轮询 |

## SESSION_ID（供 /ccg:execute 使用）
- CODEX_SESSION: 019deb90-3097-7a13-ae4d-b18bf921d7c7
- GEMINI_SESSION: c46f1f8e-4c28-47a4-bf31-6b5faf66c9e9
