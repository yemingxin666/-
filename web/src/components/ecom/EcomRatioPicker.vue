<template>
  <div class="ratio-picker">
    <button
      v-for="r in ratios"
      :key="r.value"
      class="ratio-btn"
      :class="{ active: modelValue === r.value }"
      @click="emit('update:modelValue', r.value)"
      type="button"
    >
      {{ r.label }}
    </button>
  </div>
</template>

<script setup>
import { useEcomConfigStore } from '@/store/ecom'
const store = useEcomConfigStore()
const ratios = store.ratios

defineProps({ modelValue: { type: String, default: '1:1' } })
const emit = defineEmits(['update:modelValue'])
</script>

<style scoped>
/* 参考截图：4列横排，选中蓝底白字 */
.ratio-picker {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 5px;
  width: 100%;
}
.ratio-btn {
  padding: 6px 0;
  font-size: 12px;
  text-align: center;
  color: #595959;
  background: #f5f5f5;
  border: 1px solid #e8e8e8;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.18s;
  line-height: 1.3;
  white-space: nowrap;
}
.ratio-btn:hover {
  border-color: #1677ff;
  color: #1677ff;
  background: #f0f5ff;
}
.ratio-btn.active {
  background: #1677ff;
  border-color: #1677ff;
  color: #fff;
  font-weight: 600;
}
</style>
