<template>
  <template v-if="taskStore.history.length">
    <!-- 顶部全局清空入口：仅在历史 ≥ 2 组时显示 -->
    <div v-if="taskStore.history.length >= 2" class="history-toolbar">
      <span class="history-summary">共 {{ taskStore.history.length }} 组历史结果</span>
      <button class="clear-all-btn" type="button" @click="clearAll">清空全部历史</button>
    </div>

    <div class="history-section" v-for="batch in taskStore.history" :key="batch.task_no">
      <div class="history-header">
        <span class="history-title">历史结果</span>
        <span class="history-time">{{ formatTime(batch.ts) }}</span>
        <button class="header-btn" type="button" @click="removeBatch(batch)">移除整组</button>
      </div>
      <transition-group name="card-leave" tag="div" class="result-grid-inner">
        <EcomResultCard
          v-for="entry in normalizedEntries(batch)"
          :key="entry.key"
          :url="entry.url"
          :label="entry.label"
          :status="entry.status"
          :image-type="entry.imageType"
          :ratio="batch.ratio"
          :deletable="false"
          :editable="entry.status === 'succeeded' && !!entry.assetNo"
          @edit="(p) => emit('edit', batch, { url: entry.url, asset_no: entry.assetNo, image_type: entry.imageType }, p)"
          @regenerate="(imageType) => emit('regenerate', imageType)"
        />
      </transition-group>
    </div>
  </template>
</template>

<script setup>
import { h } from 'vue'
import { ElMessage } from 'element-plus'
import { useEcomTaskStore } from '@/store/ecom'
import EcomResultCard from '@/components/ecom/EcomResultCard.vue'

const taskStore = useEcomTaskStore()
const emit = defineEmits(['edit', 'regenerate'])

// 把 batch.items / batch.outputs 两种数据形态归一化为统一的卡片列表，方便 TransitionGroup 渲染
const normalizedEntries = (batch) => {
  if (batch.items && batch.items.length) {
    return batch.items.map((item) => ({
      key: `${batch.task_no}-${item.image_type}`,
      url: item.url,
      label: item.label,
      status: item.status,
      imageType: item.image_type,
      assetNo: item.asset_no || '',
      payload: { imageType: item.image_type, isOutput: false },
    }))
  }
  // 旧 outputs 数组只有 url，无 asset_no，编辑按钮自动 disabled
  return (batch.outputs || []).map((url, i) => ({
    key: `${batch.task_no}-out-${i}`,
    url,
    label: '',
    status: 'succeeded',
    imageType: '',
    assetNo: '',
    payload: { outputIndex: i, isOutput: true },
  }))
}

const formatTime = (ts) => {
  if (!ts) return ''
  const d = new Date(ts)
  const pad = (n) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

// 通用：弹出带"撤销"按钮的提示，5 秒撤销窗口期
const showUndoTip = (text) => {
  const instance = ElMessage({
    duration: 5000,
    showClose: true,
    grouping: true,
    message: h('div', { style: 'display:flex;align-items:center;gap:12px' }, [
      h('span', text),
      h(
        'button',
        {
          style:
            'border:none;background:transparent;color:var(--el-color-primary);font-weight:600;cursor:pointer;padding:0;font-size:13px',
          onClick: () => {
            const ok = taskStore.undoLastDelete()
            if (ok) ElMessage.success('已撤销')
            instance.close()
          },
        },
        '撤销'
      ),
    ]),
  })
}

const deleteItem = (batch, payload) => {
  const key = payload.isOutput ? payload.outputIndex : payload.imageType
  taskStore.deleteHistoryItem(batch.task_no, key, payload.isOutput)
  showUndoTip('已删除该图片')
}

const removeBatch = (batch) => {
  taskStore.removeHistory(batch.task_no)
  showUndoTip('已移除整组')
}

const clearAll = () => {
  const count = taskStore.history.length
  taskStore.clearHistory()
  showUndoTip(`已清空 ${count} 组历史`)
}
</script>

<style scoped>
.history-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 24px;
  padding: 10px 14px;
  background: var(--theme-bg);
  border: 1px dashed var(--theme-border-primary);
  border-radius: 10px;
}
.history-summary {
  font-size: 13px;
  color: var(--text-secondary);
}
.clear-all-btn {
  border: none;
  background: transparent;
  color: var(--el-color-danger);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  padding: 4px 10px;
  border-radius: 6px;
  transition: background 0.2s;
}
.clear-all-btn:hover {
  background: var(--el-color-danger-light-9);
}

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
.header-btn {
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  padding: 4px 10px;
  border-radius: 6px;
  transition: color 0.2s, background 0.2s;
}
.header-btn:hover {
  color: var(--el-color-danger);
  background: var(--el-color-danger-light-9);
}

.result-grid-inner {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 20px;
}

/* 卡片移除/重排动画 */
.card-leave-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}
.card-leave-leave-to {
  opacity: 0;
  transform: scale(0.85);
}
.card-leave-move {
  transition: transform 0.3s ease;
}
</style>
