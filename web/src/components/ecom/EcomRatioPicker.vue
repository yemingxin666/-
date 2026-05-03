<template>
  <div class="ratio-picker">
    <button
      v-for="r in ratios"
      :key="r.value"
      class="ratio-btn"
      :class="{ active: modelValue === r.value, recommended: recommended === r.value }"
      @click="emit('update:modelValue', r.value)"
      type="button"
    >
      <span class="ratio-label">{{ r.label }}</span>
      <span v-if="recommended === r.value" class="recommend-badge">推荐</span>
    </button>
  </div>
</template>

<script setup>
import { useEcomConfigStore } from '@/store/ecom'
const store = useEcomConfigStore()
const ratios = store.ratios

defineProps({ 
  modelValue: { type: String, default: '1:1' },
  recommended: { type: String, default: '' }
})
const emit = defineEmits(['update:modelValue'])
</script>

<style scoped>
/* 参考截图：4列横排，选中蓝底白字 */
.ratio-picker {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  width: 100%;
}
.ratio-btn {
  position: relative;
  padding: 8px 0;
  font-size: 13px;
  text-align: center;
  color: var(--text-color);
  background: var(--gray-btn-bg);
  border: 1px solid var(--theme-border-primary);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  line-height: 1.3;
  white-space: nowrap;
}
.ratio-btn:hover {
  border-color: var(--el-color-primary-light-3);
  color: var(--el-color-primary);
  background: var(--theme-bg);
}
.ratio-btn.active {
  background: var(--el-color-primary);
  border-color: var(--el-color-primary);
  color: #fff;
  font-weight: 600;
}

.recommend-badge {
  position: absolute;
  top: -6px;
  right: -4px;
  background: #ff4d4f;
  color: #fff;
  font-size: 10px;
  padding: 0 4px;
  border-radius: 4px;
  line-height: 1.4;
  transform: scale(0.85);
  font-weight: 500;
  pointer-events: none;
  z-index: 1;
}

.ratio-btn.recommended:not(.active) {
  border-color: #ff4d4f66;
}
</style>
