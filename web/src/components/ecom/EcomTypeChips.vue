<template>
  <div class="type-selector" :class="{ 'is-error': showError && touched }">
    <!-- 标题栏：计数 + 全选 -->
    <div class="selector-header">
      <span class="count-badge">已选 <em>{{ modelValue.length }}</em>/{{ types.length }}</span>
      <button type="button" class="toggle-all-btn" @click="toggleAll">
        {{ isAllSelected ? '全不选' : '全选' }}
      </button>
    </div>

    <!-- 2列 grid 格子 -->
    <div class="type-grid">
      <div
        v-for="t in types"
        :key="t.value"
        class="type-cell"
        :class="{ active: modelValue.includes(t.value) }"
        @click="toggle(t.value)"
        :title="t.label"
      >
        <span class="cell-label">{{ t.label }}</span>
        <!-- 选中右上角 ✓ 标记 -->
        <span v-if="modelValue.includes(t.value)" class="check-mark" aria-hidden="true">
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <circle cx="6" cy="6" r="6" fill="#1677ff"/>
            <path d="M3 6l2 2 4-4" stroke="#fff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </span>
      </div>
    </div>

    <!-- 校验提示 -->
    <div v-if="showError && touched" class="error-tip">
      <svg width="12" height="12" viewBox="0 0 12 12" fill="none"><circle cx="6" cy="6" r="5.5" stroke="#f56c6c"/><path d="M6 3.5v3M6 8.5v.5" stroke="#f56c6c" stroke-linecap="round"/></svg>
      请至少选择 1 个类型
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'

const props = defineProps({
  types: { type: Array, required: true },
  modelValue: { type: Array, default: () => [] },
  max: { type: Number, default: 9 },
  validate: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const touched = ref(false)
const showError = computed(() => props.modelValue.length === 0)
const isAllSelected = computed(() => props.modelValue.length === props.types.length)

watch(() => props.validate, (v) => { if (v) touched.value = true })

const toggle = (val) => {
  touched.value = true
  const cur = [...props.modelValue]
  const idx = cur.indexOf(val)
  if (idx === -1) {
    if (cur.length >= props.max) return
    cur.push(val)
  } else {
    cur.splice(idx, 1)
  }
  emit('update:modelValue', cur)
}

const toggleAll = () => {
  touched.value = true
  emit('update:modelValue', isAllSelected.value ? [] : props.types.map((t) => t.value))
}
</script>

<style scoped>
.type-selector { width: 100%; }

.selector-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.count-badge { font-size: 12px; color: var(--text-secondary); }
.count-badge em { font-style: normal; font-weight: 700; color: var(--el-color-primary); }
.toggle-all-btn {
  font-size: 12px;
  color: var(--el-color-primary);
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
  font-weight: 600;
  transition: opacity 0.15s;
}
.toggle-all-btn:hover { opacity: 0.8; }

/* 2 列格子 */
.type-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.type-cell {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 40px;
  border: 1px solid var(--theme-border-primary);
  border-radius: 10px;
  background: var(--theme-bg);
  color: var(--text-color);
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  padding: 0 8px;

  &:hover {
    border-color: var(--el-color-primary-light-3);
    background: var(--gray-btn-bg);
    color: var(--el-color-primary);
    transform: translateY(-1px);
  }

  &.active {
    border-color: var(--el-color-primary);
    background: var(--theme-btn-fill-tertiary);
    color: var(--el-color-primary);
    font-weight: 600;
  }
}

.cell-label {
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-align: center;
  line-height: 1.3;
}

.check-mark {
  position: absolute;
  top: 4px;
  right: 4px;
  display: flex;
  line-height: 1;
  svg circle {
    fill: var(--el-color-primary);
  }
}

/* 校验错误 */
.type-selector.is-error .type-grid {
  border: 1px solid #f56c6c;
  border-radius: 12px;
  padding: 6px;
  background: rgba(245, 108, 108, 0.05);
}

.error-tip {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
  font-size: 12px;
  color: #f56c6c;
}
</style>
