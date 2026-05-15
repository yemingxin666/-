<template>
  <div class="gallery-page">
    <div class="gallery-header">
      <el-tabs v-model="galleryStore.moduleFilter" @tab-change="onModuleChange">
        <el-tab-pane label="全部" name="" />
        <el-tab-pane label="主图设计" name="main_image" />
        <el-tab-pane label="详情页" name="detail_page" />
        <el-tab-pane label="白底图" name="white_bg" />
        <el-tab-pane label="克隆设计" name="clone" />
        <el-tab-pane label="比例转换" name="ratio_convert" />
        <el-tab-pane label="图文翻译" name="translate" />
      </el-tabs>
    </div>

    <div v-loading="galleryStore.loading" class="gallery-body">
      <div v-if="!galleryStore.loading && !galleryStore.items.length" class="gallery-empty">
        <div class="empty-icon">🖼</div>
        <p class="empty-title">暂无历史记录</p>
        <p class="empty-tip">生成的图片将在这里展示</p>
      </div>

      <div class="gallery-grid" v-else>
        <div v-for="task in galleryStore.items" :key="task.task_no" class="gallery-item">
          <div class="task-meta">
            <el-tag size="small" type="primary" effect="light">{{ taskTitle(task) }}</el-tag>
            <el-tag v-if="isRunning(task)" size="small" type="warning" effect="light">
              {{ task.status === 'running' ? '生成中' : '排队中' }}
            </el-tag>
            <span class="task-date">{{ formatDate(task.created_at) }}</span>
          </div>
          <!-- 运行中任务：占位框 + 进度条，按原图比例渲染（aria-live 让屏幕阅读器播报状态变化） -->
          <div
            v-if="isRunning(task)"
            class="task-running"
            :style="{ aspectRatio: ratioToCss(task.ratio) }"
            role="status"
            aria-live="polite"
            :aria-label="`任务 ${task.status === 'running' ? '正在生成' : '排队中'}，进度 ${task.progress || 0}%`"
          >
            <el-icon class="running-spinner" :size="28" aria-hidden="true"><Loading /></el-icon>
            <div class="running-text">{{ task.status === 'running' ? '正在生成…' : '排队中…' }}</div>
            <el-progress
              :percentage="task.progress || 0"
              :show-text="true"
              :stroke-width="6"
              class="running-progress"
            />
          </div>
          <div v-else-if="task.outputs?.length" class="task-outputs">
            <EcomResultCard
              v-for="(out, i) in task.outputs"
              :key="out.asset_no || i"
              :url="out.url"
              :ratio="task.ratio || '1:1'"
              :editable="true"
              :confirm-delete-title="task.outputs.length > 1 ? '确认删除该图片？' : '确认删除该图片？删除后任务将被移除'"
              @edit="(payload) => openEditDialog(task, out, payload)"
              @delete="onDeleteAsset(task, out)"
            />
          </div>
          <div v-else class="task-empty">暂无输出图片</div>
        </div>
      </div>
    </div>

    <EcomEditDialog
      v-model="editVisible"
      :url="editPayload.url"
      :ratio="editPayload.ratio"
      :task-no="editPayload.taskNo"
      :asset-no="editPayload.assetNo"
      @submitted="onEditSubmitted"
    />

    <div class="gallery-footer">
      <el-pagination
        v-model:current-page="galleryStore.page"
        v-model:page-size="galleryStore.pageSize"
        :total="galleryStore.total"
        :page-sizes="[20, 40]"
        layout="total, sizes, prev, pager, next"
        @current-change="galleryStore.fetchGallery"
        @size-change="galleryStore.fetchGallery"
      />
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useEcomGalleryStore } from '@/store/ecom'
import EcomResultCard from '@/components/ecom/EcomResultCard.vue'
import EcomEditDialog from '@/components/ecom/EcomEditDialog.vue'

const galleryStore = useEcomGalleryStore()

// 编辑弹窗状态
const editVisible = ref(false)
const editPayload = ref({ url: '', ratio: '1:1', taskNo: '', assetNo: '' })

const openEditDialog = (task, out, payload) => {
  editPayload.value = {
    url: out.url,
    ratio: payload?.ratio || task.ratio || '1:1',
    taskNo: task.task_no,
    assetNo: out.asset_no,
  }
  editVisible.value = true
}

const onDeleteAsset = async (task, out) => {
  if (!out?.asset_no) {
    // 兜底：旧数据无 asset_no，回退为删任务
    await galleryStore.deleteTask(task.task_no)
    return
  }
  try {
    await galleryStore.deleteAsset(task.task_no, out.asset_no)
  } catch (e) {
    ElMessage.error('删除失败：' + (e?.message || ''))
  }
}

const onEditSubmitted = async () => {
  // 后端任务异步执行，刷新列表后新任务以 queued/running 态出现在首位
  // 轮询由 watch(hasRunning) 自动启动，无需手动调用
  galleryStore.page = 1
  await galleryStore.fetchGallery()
}

const moduleMap = { main_image: '主图设计', detail_page: '详情页', white_bg: '白底图', clone: '克隆设计', ratio_convert: '比例转换', translate: '图文翻译', edit: '图片编辑' }
const moduleLabel = (m) => moduleMap[m] || m
const taskTitle = (task) => {
  const label = moduleLabel(task.module)
  const name = task.input_json?.product_name
  return name ? `${name} · ${label}` : label
}
const formatDate = (t) => t ? new Date(t).toLocaleDateString('zh-CN') : ''
const isRunning = (t) => ['queued', 'running', 'pending'].includes(t.status)
const ratioToCss = (r) => (r || '1:1').replace(':', '/')

const onModuleChange = () => {
  galleryStore.page = 1
  galleryStore.fetchGallery()
}

// 轮询：watch hasRunning 自动启停，递归 setTimeout 防请求堆积，连续失败 5 次自动停
const POLL_INTERVAL = 4000
const MAX_POLL_FAIL = 5
let pollTimer = null
let polling = false           // 防止 fetch 未返回时再次发起
let pollFailCount = 0

const hasRunning = computed(() => galleryStore.items.some(isRunning))

const stopPolling = () => {
  if (pollTimer) { clearTimeout(pollTimer); pollTimer = null }
  polling = false
}

const scheduleNextPoll = () => {
  if (!hasRunning.value) { stopPolling(); return }
  pollTimer = setTimeout(runPoll, POLL_INTERVAL)
}

const runPoll = async () => {
  if (polling) return
  polling = true
  try {
    await galleryStore.fetchGallery({ silent: true })
    pollFailCount = 0
  } catch (e) {
    pollFailCount += 1
    if (pollFailCount >= MAX_POLL_FAIL) {
      stopPolling()
      ElMessage.error('任务进度刷新失败，请检查网络后重试')
      return
    }
  } finally {
    polling = false
  }
  scheduleNextPoll()
}

const startPolling = () => {
  if (pollTimer || polling) return
  pollFailCount = 0
  scheduleNextPoll()
}

// 跟随 hasRunning 自动启停：覆盖 tab 切换、分页、提交后等场景
watch(hasRunning, (running) => {
  if (running) startPolling()
  else stopPolling()
})

onMounted(() => galleryStore.fetchGallery())
onBeforeUnmount(stopPolling)
</script>

<style scoped>
.gallery-page {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  background: var(--gray-btn-bg);
  overflow: hidden;
}

.gallery-header {
  flex-shrink: 0;
  padding: 0 20px;
  background: var(--theme-bg);
  border-bottom: 1px solid var(--theme-border-primary);
}

:deep(.el-tabs__nav-wrap::after) { height: 1px; background: var(--theme-border-primary); }
:deep(.el-tabs__item) { font-size: 13px; color: var(--text-secondary); }
:deep(.el-tabs__item.is-active) { color: var(--el-color-primary); font-weight: 600; }
:deep(.el-tabs__active-bar) { background: var(--el-color-primary); }
:deep(.el-tabs__header) { margin-bottom: 0; }

.gallery-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 0;
  background: var(--gray-btn-bg);
}

.gallery-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
  padding: 0 20px;
}

@media (min-width: 1200px) {
  .gallery-grid { grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); }
}
@media (min-width: 1600px) {
  .gallery-grid { grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 18px; }
}
@media (min-width: 1920px) {
  .gallery-grid { grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 20px; }
}

.gallery-item {
  background: var(--theme-bg);
  border: 1px solid var(--theme-border-primary);
  border-radius: 8px;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: box-shadow 0.2s, transform 0.2s;
}
.gallery-item:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
  transform: translateY(-2px);
}

.task-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.task-date { font-size: 12px; color: var(--text-secondary); }

/* 单图：充满卡片；多图：2 列网格 */
.task-outputs { display: grid; grid-template-columns: 1fr; gap: 6px; }
.task-outputs:has(> *:nth-child(2)) { grid-template-columns: 1fr 1fr; }

.task-empty { font-size: 13px; color: var(--text-secondary); text-align: center; padding: 12px 0; }

.task-running {
  width: 100%;
  background: var(--gray-btn-bg);
  border: 1px dashed var(--theme-border-primary);
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px;
  color: var(--text-secondary);
}
.running-spinner { animation: spin 1s linear infinite; color: var(--el-color-primary); }
.running-text { font-size: 12px; }
.running-progress { width: 80%; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

.gallery-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  opacity: 0.6;
}
.empty-icon { font-size: 56px; margin-bottom: 16px; }
.empty-title { font-size: 15px; color: var(--text-secondary); margin: 0 0 6px; font-weight: 500; }
.empty-tip { font-size: 13px; color: var(--text-secondary); margin: 0; }

.gallery-footer {
  flex-shrink: 0;
  padding: 12px 24px 16px;
  background: var(--theme-bg);
  border-top: 1px solid var(--theme-border-primary);
  display: flex;
  justify-content: flex-end;
}
</style>
