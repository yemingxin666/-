# 历史图库「编辑」功能实施计划

> 由 /ccg:workflow 多模型协作产出（Codex 后端 + Gemini 前端 + Claude 综合）

## 需求摘要

将历史图库的"重新生成"按钮改造为「编辑」功能：点击 → 弹出输入框 → 用户输入 prompt → 后端基于原图 + prompt 调用 ImageToImage 生成新图（1K 分辨率，比例沿用原图）→ 作为新条目出现在历史图库。

## 核心决策

| 维度 | 决策 |
|------|------|
| 生图语义 | **基于原图 + prompt 编辑**（ImageToImage / qwen-image-edit） |
| 任务存放 | **新建独立任务**（新 `AiImageTask`，新 gallery 条目） |
| 入口范围 | **仅历史图库**（通过 `editable: true` prop 显式开启；当前任务区保留 regenerate） |
| 分辨率 | 复用 `provider.RatioToSize(task.Ratio)` —— 1K 标准映射 |
| 模型选择 | **复用 ecom 全局选中模型** `useEcomConfigStore.selectedModel`（与主图/详情页等保持一致）；用户在 `EcomTopNav` 切换模型立即生效；前端必填、后端不兜底，为空报错"请先选择生图模型" |
| 关联 ID | 后端 ListGallery 改为返回 `[{url, asset_no}]`，前端传 `task_no + asset_no` |

---

## 后端实施（Go）

### 1. `api/store/model/ai_image_task.go`
新增模块常量：
```go
const ModuleEdit = "edit"
```

### 2. `api/service/aicommerce/image_service.go`

**(a)** 顶部 const 块加 `ModuleEdit = "edit"`。

**(b)** 新增请求结构 + 响应结构：
```go
type EditReq struct {
    TaskNo  string `json:"task_no" binding:"required"`
    AssetNo string `json:"asset_no" binding:"required"`
    Prompt  string `json:"prompt" binding:"required"`
    Model   string `json:"model" binding:"required"` // 必填，由前端传入用户选中的模型
}

type OutputItem struct {
    Url     string `json:"url"`
    AssetNo string `json:"asset_no"`
}
```

**(c)** `GalleryTask.Outputs` 类型 `[]string` → `[]OutputItem`，并修改 `ListGallery` 组装逻辑（line ~342-353）。

**(d)** 新增 `SubmitEditTask(ctx, userID, req EditReq)`：
1. 校验 source task：`WHERE task_no=? AND user_id=? AND deleted_at IS NULL`
2. 校验 source asset：`WHERE asset_no=? AND user_id=? AND kind='generated' AND deleted_at IS NULL`
3. 选模型：**`req.Model` 必填**，为空直接返回错误「请先选择生图模型」；不做兜底（避免使用不支持 img2img 的模型）；dispatcher 现有 `requiredImageCapability(ModuleEdit)→img2img` 会自动拦截不支持的模型
4. 计费：`promptRepo.GetPriceByModel(modelName)`
5. 扣费：`deductCredit`
6. 序列化 InputJSON：`{prompt, source_task_no, source_asset_no, origin_ratio}`
7. 创建 task：`Module=ModuleEdit, Ratio=originTask.Ratio, Model=modelName`
8. 入队 + 失败退款（与 SubmitTask 一致）

### 3. `api/handler/aicommerce/image_handler.go`

注册路由 + Handler：
```go
group.POST("/edit", h.EditImage)

func (h *ImageHandler) EditImage(c *gin.Context) {
    userID := h.getLoginUserID(c)
    if userID == 0 { resp.ERROR(c, "未登录"); return }
    var req aicommerce.EditReq
    if err := c.ShouldBindJSON(&req); err != nil { resp.ERROR(c, "参数错误"); return }
    task, err := h.service.SubmitEditTask(c.Request.Context(), userID, req)
    if err != nil { resp.ERROR(c, err.Error()); return }
    resp.SUCCESS(c, gin.H{
        "task_no": task.TaskNo, "status": task.Status, "credit_cost": task.CreditCost,
    })
}
```

### 4. `api/service/aicommerce/worker/dispatcher.go`

**(a)** `execute` switch 增加 `case model.ModuleEdit`：
```go
case model.ModuleEdit:
    imgClient, err := d.resolveImageClient(ctx, &task)
    if err != nil { execErr = err; break }
    execErr = chains.RunEdit(ctx, d.db, imgClient, uploader, d.cfg, &task)
```

**(b)** `requiredImageCapability` 增加 `model.ModuleEdit` 到 `img2img` 分支。

### 5. `api/service/aicommerce/worker/chains/edit.go` (新增)

```go
func RunEdit(ctx, db, imgClient, uploader, cfg, task) error {
    input := task.InputJSON
    prompt, _ := input["prompt"].(string)
    srcAssetNo, _ := input["source_asset_no"].(string)
    if prompt == "" || srcAssetNo == "" { return errors.New("edit: missing prompt or source asset") }

    var srcAsset model.AiImageAsset
    if err := db.Where("asset_no = ? AND user_id = ? AND deleted_at IS NULL",
        srcAssetNo, task.UserId).First(&srcAsset).Error; err != nil {
        return fmt.Errorf("source asset not found: %w", err)
    }
    updateProgress(db, task, 20)

    result, err := imgClient.ImageToImage(ctx, provider.ImageToImageReq{
        Model:     task.Model,
        Prompt:    prompt,
        ImageURL:  signedURL(srcAsset.OssKey, cfg),
        ImageSize: provider.RatioToSize(task.Ratio),
        Strength:  0.7,
    })
    if err != nil { return fmt.Errorf("edit imageToImage: %w", err) }
    if len(result.Images) == 0 { return errors.New("edit: no result image") }
    updateProgress(db, task, 70)

    ossKey, err := ossUploadURL(uploader, result.Images[0].URL)
    if err != nil { return err }
    updateProgress(db, task, 90)

    taskID := task.Id
    return db.Create(&model.AiImageAsset{
        AssetNo:   fmt.Sprintf("ed_%d_%d", task.Id, time.Now().UnixNano()),
        TaskId:    &taskID,
        UserId:    task.UserId,
        Kind:      model.AssetKindGenerated,
        OssBucket: cfg.OSSBucket,
        OssKey:    ossKey,
        MimeType:  "image/jpeg",
        CreatedAt: time.Now(),
    }).Error
}
```

---

## 前端实施（Vue）

### 1. `web/src/components/ecom/EcomResultCard.vue` 改造

- 引入 `Edit` 图标
- 新增 prop `editable: { type: Boolean, default: false }`
- emits 增加 `'edit'`
- 工具栏按钮根据 `editable` 切换：
  ```vue
  <button v-if="editable" class="tool-btn" @click="emit('edit', { url: stickyUrl, ratio })" title="编辑">
    <el-icon><Edit /></el-icon>
  </button>
  <button v-else class="tool-btn" @click="emit('regenerate', props.imageType)" title="重新生成">
    <el-icon><Refresh /></el-icon>
  </button>
  ```
- 失败态"重试"按钮保留 `regenerate` 事件不变

### 2. `web/src/components/ecom/EcomEditDialog.vue` (新增)

- props: `modelValue (visible)`、`url`、`ratio`、`taskNo`、`assetNo`
- emits: `update:modelValue`、`submitted (newTaskNo)`
- 布局：左原图缩略 (220px、保持 aspect-ratio) + 右 prompt textarea (rows=6)
- 点击"确认生成"按钮：调 `galleryStore.editTask` → loading → 成功 ElMessage + emit submitted + 关闭 → 失败 ElMessage 错误
- prompt 非空 trim 校验

### 3. `web/src/store/ecom.js` 改造

`useEcomGalleryStore` 内新增：
```js
const editTask = async (taskNo, assetNo, prompt) => {
  const res = await httpPost('/api/ai-commerce/edit', {
    task_no: taskNo, asset_no: assetNo, prompt
  })
  if (res.code !== 0) throw new Error(res.message || '编辑失败')
  useEcomConfigStore().deductPower(res.data.credit_cost || 0)
  return res.data
}
```

返回字段 `{editTask, ...}`。

### 4. `web/src/views/ecom/GalleryPage.vue` 接入

**(a)** 模板修改（line ~30）：
```vue
<EcomResultCard
  v-for="(out, i) in task.outputs"
  :key="i"
  :url="out.url"
  :ratio="task.ratio || '1:1'"
  :editable="true"
  @edit="(payload) => openEditDialog(task, out, payload)"
  @delete="galleryStore.deleteTask(task.task_no)"
/>
<EcomEditDialog
  v-model="editVisible"
  :url="editPayload.url"
  :ratio="editPayload.ratio"
  :task-no="editPayload.taskNo"
  :asset-no="editPayload.assetNo"
  @submitted="onEditSubmitted"
/>
```

**(b)** script 新增：
```js
import EcomEditDialog from '@/components/ecom/EcomEditDialog.vue'
import { ElMessage } from 'element-plus'
const editVisible = ref(false)
const editPayload = ref({ url: '', ratio: '1:1', taskNo: '', assetNo: '' })
const openEditDialog = (task, out, payload) => {
  editPayload.value = { url: out.url, ratio: payload.ratio || task.ratio, taskNo: task.task_no, assetNo: out.asset_no }
  editVisible.value = true
}
const onEditSubmitted = () => {
  ElMessage.success('编辑任务已提交，请稍候在历史中查看')
  galleryStore.fetchGallery()
}
```

---

## 文件清单

### 后端（5 个文件）
- ✏️ `api/store/model/ai_image_task.go` —— 加 `ModuleEdit` 常量
- ✏️ `api/service/aicommerce/image_service.go` —— 加 EditReq、OutputItem、SubmitEditTask、改 GalleryTask
- ✏️ `api/handler/aicommerce/image_handler.go` —— 加 `/edit` 路由 + EditImage handler
- ✏️ `api/service/aicommerce/worker/dispatcher.go` —— switch 新增 case + capability 表
- ➕ `api/service/aicommerce/worker/chains/edit.go` —— 新增 RunEdit

### 前端（4 个文件）
- ✏️ `web/src/components/ecom/EcomResultCard.vue` —— 加 editable prop、Edit 图标、edit 事件
- ➕ `web/src/components/ecom/EcomEditDialog.vue` —— 新增弹窗组件
- ✏️ `web/src/store/ecom.js` —— useEcomGalleryStore 加 editTask
- ✏️ `web/src/views/ecom/GalleryPage.vue` —— 适配 outputs 新结构 + 接入弹窗

---

## 风险与回退

| 风险 | 缓解 |
|------|------|
| qwen-image-edit 模型未在 ai_models 启用 | 安装前需 DBA 确认；否则返回明确错误 |
| ListGallery 结构变更影响其他调用方 | grep 检查无其他调用方（已确认仅 GalleryPage） |
| 跨用户引用 source asset | SubmitEditTask 强制校验 user_id |
| 编辑失败需退款 | 复用 dispatcher.execute 失败分支退款逻辑 |

## 验收标准

- ✅ 历史图库 ResultCard 显示"编辑"图标（铅笔），其他页面仍是"重新生成"图标
- ✅ 点击编辑 → 弹窗显示原图缩略图 + prompt 输入框
- ✅ 提交后 ElMessage 提示，新任务出现在 gallery 首位（fetchGallery 刷新后）
- ✅ 新任务比例与原图一致，分辨率为 1K 标准（如 1024x1024）
- ✅ 跨用户编辑被拒绝（403/错误）
- ✅ 余额不足时返回明确错误
