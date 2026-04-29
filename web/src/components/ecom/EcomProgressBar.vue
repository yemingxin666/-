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
    <el-alert v-if="task.status === 'failed'" :title="task.error || '任务执行失败'" type="error" :closable="false" class="mt-2" />
  </div>
</template>

<script setup>
import { computed } from 'vue'
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
</script>

<style scoped>
.progress-wrap { margin: 12px 0; }
.progress-info { display: flex; justify-content: space-between; margin-top: 4px; font-size: 12px; }
.status-text { color: #606266; }
.task-no { color: #c0c4cc; font-family: monospace; }
.mt-2 { margin-top: 8px; }
</style>
