<template>
  <div class="size-chart-card" v-if="data">
    <div class="card-header">
      <span class="card-icon">📋</span>
      <span class="card-title">已识别尺码表数据</span>
      <button
        class="del-btn"
        type="button"
        @click="$emit('delete')"
        title="移除"
        aria-label="移除尺码表"
      >✕</button>
    </div>
    <div class="card-body">
      <div class="meta-line">
        <span class="meta-item">共 <b>{{ rowCount }}</b> 行 × <b>{{ colCount }}</b> 列</span>
        <span class="meta-item">单位：<b>{{ unit }}</b></span>
      </div>
      <div v-if="headersText" class="headers-line">
        表头：<span class="headers-text">{{ headersText }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  data: { type: Object, default: null }
})
defineEmits(['delete'])

const rowCount = computed(() => props.data?.rows?.length || 0)
const colCount = computed(() => props.data?.headers?.length || 0)
const unit = computed(() => props.data?.unit || 'cm')
const headersText = computed(() => (props.data?.headers || []).join(' / '))
</script>

<style scoped>
.size-chart-card {
  margin-top: 10px;
  border: 1px solid var(--el-color-warning-light-5, #f6c089);
  background: var(--el-color-warning-light-9, rgba(237, 137, 54, 0.06));
  border-radius: 10px;
  padding: 10px 12px;
  font-size: 12px;
  color: var(--text-color);
}
.card-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  font-size: 13px;
}
.card-icon { font-size: 14px; }
.card-title { flex: 1; color: var(--el-color-warning, #ed8936); }
.del-btn {
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 14px;
  padding: 0 4px;
  border-radius: 4px;
  transition: background 0.2s;
}
.del-btn:hover { background: var(--el-color-warning-light-7, rgba(237, 137, 54, 0.15)); }
.del-btn:focus-visible {
  outline: 2px solid var(--el-color-warning, #ed8936);
  outline-offset: 1px;
}
.card-body { margin-top: 6px; line-height: 1.6; color: var(--text-secondary); }
.meta-line {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
}
.meta-item b { color: var(--el-color-warning-dark-2, #c2410c); font-weight: 700; }
.headers-line { margin-top: 2px; }
.headers-text { color: var(--text-color); }
</style>
