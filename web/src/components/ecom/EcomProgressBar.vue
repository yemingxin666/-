<template>
  <div class="progress-wrap" v-if="task">
    <el-progress
      :percentage="task.progress || 0"
      :status="progressStatus"
      :stroke-width="10"
      striped
      :striped-flow="task.status === 'running'"
    />
    <div class="progress-info">
      <span class="status-text">{{ statusText }}</span>
      <span class="task-no" v-if="task.task_no">{{ task.task_no }}</span>
    </div>
    <div v-if="taskStore.items.length" class="type-chips">
      <el-tooltip v-for="item in taskStore.items" :key="item.image_type" :content="phaseTextMap[item.phase] || item.phase" placement="top">
        <el-tag :type="tagTypeMap[item.status] || 'info'" size="small" round class="type-chip">
          <el-icon v-if="item.status === 'running'" class="is-loading"><Loading /></el-icon>
          <el-icon v-else-if="item.status === 'succeeded'"><Check /></el-icon>
          <el-icon v-else-if="item.status === 'failed'"><Close /></el-icon>
          {{ item.label }}
        </el-tag>
      </el-tooltip>
    </div>
    <el-alert v-if="task.status === 'failed'" :title="task.error || '任务执行失败'" type="error" :closable="false" class="mt-2" />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Check, Close, Loading } from '@element-plus/icons-vue'
import { useEcomTaskStore } from '@/store/ecom'

const taskStore = useEcomTaskStore()
const task = computed(() => taskStore.currentTask)

const progressStatus = computed(() => {
  if (!task.value) return ''
  const s = task.value.status
  if (s === 'succeeded') return 'success'
  if (s === 'failed') return 'exception'
  return ''
})

const statusText = computed(() => {
  if (!task.value) return ''
  const map = { pending: '等待中', queued: '排队中', running: '生成中...', succeeded: '生成完成', failed: '生成失败', cancelled: '已取消' }
  return map[task.value.status] || task.value.status
})

const tagTypeMap = { pending: 'info', running: '', succeeded: 'success', failed: 'danger' }
const phaseTextMap = { rendering: '渲染中', generating: '生图中', uploading: '上传中', succeeded: '已完成', failed: '已失败', pending: '等待中' }
</script>

<style scoped>
.progress-wrap { margin: 12px 0; }
.progress-info { display: flex; justify-content: space-between; margin-top: 4px; font-size: 12px; }
.status-text { color: #606266; }
.task-no { color: #c0c4cc; font-family: monospace; }
.mt-2 { margin-top: 8px; }
.type-chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
:deep(.type-chip .el-tag) { display: inline-flex; align-items: center; gap: 3px; cursor: default; }
</style>
