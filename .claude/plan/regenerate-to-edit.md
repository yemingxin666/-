# 计划：重新生成 → 编辑功能改造

## 背景
将主图设计 / 详情页 / 克隆设计 三个页面的"重新生成"按钮完全替换为"编辑"功能，复用历史图库已有的 EcomEditDialog。

## 决策
- 完全替换（无双按钮）
- 提交后留在当前页 + ElNotification 带跳转链接
- 后端 B+ 最小入侵：保留 outputs []string，items 增加 asset_no
- 前端 Composable 模式：useEcomEdit + 各页集成

---

## 后端改动（Codex 方案）

### 1. `api/service/aicommerce/image_service.go`

**ImageTaskItem 结构（line 260-267）** 新增字段：
```go
AssetNo string `json:"asset_no,omitempty"`
```

**buildTaskItems（line 331-397）**：
- 已有 typed item 分支：当 OssKey != "" 时填充 AssetNo
- clone 模块（无 image_type）：调用新 helper `buildCloneSyntheticItems`

**新增 buildCloneSyntheticItems**：
- 为每张克隆资产生成合成 item，image_type=`clone_0`/`clone_1`，label=`克隆设计 1/2`，status=succeeded，asset_no 从 asset 取
- 失败/缺失资产 → status=failed/queued，asset_no=""

**signTaskResult**：保持不变（asset_no 不需签名）

### 2. 测试场景（buildTaskItems）
1. 主图：多 image_type，全部成功
2. 主图：部分 phase 占位、部分成功
3. 详情页：单 image_type
4. 克隆：2 张成功资产
5. 克隆：1 张失败 + 1 张成功
6. 任务 queued（无 outputs）

---

## 前端改动（Gemini 方案）

### 1. 新增 `web/src/composables/useEcomEdit.js`
- 状态：`editVisible`、`editPayload { url, ratio, taskNo, assetNo }`
- 方法：`openEdit(taskNo, item)`、`onEditSubmitted()`
- onEditSubmitted：刷新当前页历史 + ElNotification（VNode 渲染"前往历史图库查看进度"链接，点击调用注入的 setModule('gallery')）

### 2. `web/src/views/ecom/EcomPage.vue`
- 通过 `provide('setModule', setModule)` 暴露切换 tab 方法

### 3. 三个页面集成（MainImagePage / DetailPagePage / ClonePage）
- `import { useEcomEdit } from '@/composables/useEcomEdit'`
- `inject('setModule')`
- EcomResultCard：
  - `:editable="item.status === 'succeeded' && !!item.asset_no"`
  - `@edit="(payload) => openEdit(task.task_no, { ...item, ...payload })"`
  - 移除 `@regenerate` 相关 props/handler
- 模板末尾挂载一个 `<EcomEditDialog v-model="editVisible" :url="..." @submitted="onEditSubmitted" />`

### 4. `EcomHistoryGroup.vue`
- 透传 `@edit` 事件给父组件（同 ResultCard 接口）
- 移除 `@regenerate`

### 5. `EcomResultCard.vue`
- 移除 regenerate 按钮（或保留 prop 但默认关闭）
- 编辑按钮 disabled 条件由父组件 editable 控制

---

## 影响范围
- 后端：1 个文件改动 + 1 个新 helper + 单测
- 前端：1 个 composable 新增、1 个 EcomPage provide、3 个页面集成、2 个组件清理
- 删除：所有 regenerate 相关 emit/handler/网络请求

## 验收
- 三个页面任务卡的"重新生成"图标变为"编辑"
- 点击编辑 → 弹出 EcomEditDialog（带原图）
- 提交成功 → 当前页保留 + ElNotification 出现 + 点击链接跳转到历史图库
- 任务进行中 / clone 任务的编辑按钮可用（asset_no 存在时）
