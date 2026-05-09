<template>
  <template v-if="taskStore.history.length">
    <div class="history-section" v-for="batch in taskStore.history" :key="batch.task_no">
      <div class="history-header">
        <span class="history-title">历史结果</span>
        <span class="history-time">{{ formatTime(batch.ts) }}</span>
        <el-popconfirm title="移除这组历史图片？" @confirm="taskStore.removeHistory(batch.task_no)">
          <template #reference>
            <el-button size="small" link type="danger">移除</el-button>
          </template>
        </el-popconfirm>
      </div>
      <div class="result-grid">
        <template v-if="batch.items && batch.items.length">
          <EcomResultCard
            v-for="item in batch.items"
            :key="batch.task_no + '-' + item.image_type"
            :url="item.url"
            :label="item.label"
            :status="item.status"
            :image-type="item.image_type"
            :ratio="batch.ratio"
            @delete="taskStore.removeHistory(batch.task_no)"
          />
        </template>
        <template v-else>
          <EcomResultCard
            v-for="(url, i) in batch.outputs"
            :key="batch.task_no + '-' + i"
            :url="url"
            :ratio="batch.ratio"
            @delete="taskStore.removeHistory(batch.task_no)"
          />
        </template>
      </div>
    </div>
  </template>
</template>

<script setup>
import { useEcomTaskStore } from '@/store/ecom'
import EcomResultCard from '@/components/ecom/EcomResultCard.vue'

const taskStore = useEcomTaskStore()

const formatTime = (ts) => {
  if (!ts) return ''
  const d = new Date(ts)
  const pad = (n) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<style scoped>
.history-section {
  margin-top: 28px;
  padding-top: 20px;
  border-top: 1px dashed var(--theme-border-primary);
}
.history-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.history-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-color);
}
.history-time {
  font-size: 12px;
  color: var(--text-secondary);
  flex: 1;
}
.result-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 20px;
}
</style>
