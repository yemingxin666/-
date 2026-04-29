<template>
  <div class="ecom-topnav">
    <!-- 宸︿晶锛氬搧鐗?logo + 妯″潡鍥炬爣 tab -->
    <div class="nav-left">
      <div class="brand">
        <svg class="brand-svg" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="32" height="32" rx="8" fill="#1677ff"/>
          <path d="M8 22L12 10l4 8 4-6 4 8" stroke="#fff" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span class="brand-name">鐢靛晢鐢熷浘</span>
      </div>
      <div class="divider-v"></div>

      <nav class="module-tabs" role="tablist" aria-label="鐢靛晢鐢熷浘鍔熻兘妯″潡">
        <div
          v-for="m in modules"
          :key="m.value"
          class="tab-item"
          :class="{ active: activeModule === m.value }"
          @click="emit('update:activeModule', m.value)"
          role="tab"
          :aria-selected="activeModule === m.value"
          :tabindex="activeModule === m.value ? 0 : -1"
          :title="m.label"
        >
          <!-- SVG 鍥炬爣 -->
          <span class="tab-icon" aria-hidden="true" v-html="m.svg"></span>
          <span class="tab-label">{{ m.label }}</span>
          <span v-if="activeModule === m.value" class="tab-indicator" aria-hidden="true"></span>
        </div>
      </nav>
    </div>

    <!-- 鍙充晶锛氭ā鍨嬮€夋嫨瑙﹀彂鎸夐挳 + 绠楀姏鏄剧ず -->
    <div class="nav-right">
      <button v-if="showModelSelector" class="model-trigger" @click="showModelDialog = true">
        <svg width="15" height="15" viewBox="0 0 15 15" fill="none">
          <rect x="1" y="3" width="13" height="9" rx="2" stroke="currentColor" stroke-width="1.3"/>
          <path d="M5 7.5h5M7.5 5v5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
        </svg>
        <span class="model-trigger-label">AI妯″瀷</span>
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" class="chevron">
          <path d="M3 4.5l3 3 3-3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>

      <div class="credit-display">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><circle cx="7" cy="7" r="6" fill="#f90" stroke="#f90"/><text x="7" y="10.5" text-anchor="middle" font-size="8" fill="#fff" font-weight="bold">楼</text></svg>
        <span class="credit-num">{{ store.userPower }}</span>
        <span class="credit-unit">绠楀姏</span>
      </div>
    </div>
  </div>

  <!-- 妯″瀷閫夋嫨寮圭獥 -->
  <el-dialog
    v-model="showModelDialog"
    title="閫夋嫨 AI 妯″瀷"
    width="480px"
    :close-on-click-modal="true"
    class="model-dialog"
  >
    <div v-if="modelsLoading" class="model-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>鍔犺浇涓?..</span>
    </div>
    <div v-else-if="!store.filteredModels.length" class="model-empty">
      鏆傛棤鍙敤妯″瀷锛岃鑱旂郴绠＄悊鍛橀厤缃?    </div>
    <div v-else class="model-grid">
      <div
        v-for="m in store.filteredModels"
        :key="m.name"
        class="model-card"
        :class="{ selected: store.selectedModel === m.name }"
        @click="selectModel(m.name)"
      >
        <div class="model-card-icon">
          <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
            <rect width="28" height="28" rx="8" fill="currentColor" opacity=".1"/>
            <path d="M9 14h10M14 9v10" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
          </svg>
        </div>
        <div class="model-card-info">
          <div class="model-card-name">{{ m.display_name }}</div>
          <div class="model-card-provider">{{ m.provider }}</div>
          <div v-if="m.description" class="model-card-desc">{{ m.description }}</div>
          <div class="model-badges" v-if="m.capabilities">
            <el-tag v-if="m.capabilities.includes('text2img')" size="small" type="success" style="margin-right:4px">文生图</el-tag>
            <el-tag v-if="m.capabilities.includes('img2img')" size="small" type="warning">图生图</el-tag>
          </div>
        </div>
        <div class="model-card-check" v-if="store.selectedModel === m.name">
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
            <circle cx="9" cy="9" r="8" fill="var(--el-color-primary)"/>
            <path d="M5.5 9l2.5 2.5 5-5" stroke="#fff" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
      </div>
    </div>
    <template #footer>
      <el-button @click="showModelDialog = false">鍏抽棴</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { useEcomConfigStore } from '@/store/ecom'

const props = defineProps({ activeModule: { type: String, default: 'main_image' } })
const emit = defineEmits(['update:activeModule'])
const store = useEcomConfigStore()

const showModelDialog = ref(false)
const modelsLoading = ref(false)
const showModelSelector = computed(() => props.activeModule !== 'white_bg')

const currentModelLabel = computed(() => {
  if (modelsLoading.value) return '鍔犺浇涓?..'
  const m = store.filteredModels.find((m) => m.name === store.selectedModel)
  return m ? m.display_name : '閫夋嫨妯″瀷'
})

const selectModel = (name) => {
  store.setSelectedModel(name)
  showModelDialog.value = false
}

watch(() => props.activeModule, (val) => {
  store.activeModule = val
  const valid = store.filteredModels.find(m => m.name === store.selectedModel)
  if (!valid && store.filteredModels.length) {
    store.setSelectedModel(store.filteredModels[0].name)
  }
}, { immediate: true })

onMounted(async () => {
  modelsLoading.value = true
  await store.loadModels(props.activeModule)
  modelsLoading.value = false
})

const modules = [
  {
    value: 'main_image', label: '主图设计',
    svg: `<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="1.5" y="1.5" width="15" height="15" rx="2" stroke="currentColor" stroke-width="1.5"/><circle cx="6" cy="6.5" r="1.5" stroke="currentColor" stroke-width="1.3"/><path d="M1.5 12l4-3.5 3 3 2.5-2.5 5 5" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/></svg>`
  },
  {
    value: 'detail_page', label: '详情页设计',
    svg: `<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="2" y="1.5" width="14" height="15" rx="1.5" stroke="currentColor" stroke-width="1.5"/><path d="M5 5.5h8M5 8.5h8M5 11.5h5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>`
  },
  {
    value: 'white_bg', label: '白底图设计',
    svg: `<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="1.5" y="1.5" width="15" height="15" rx="2" stroke="currentColor" stroke-width="1.5"/><rect x="5" y="5" width="8" height="8" rx="1" fill="currentColor" opacity=".15" stroke="currentColor" stroke-width="1.2"/></svg>`
  },
  {
    value: 'clone', label: '克隆设计',
    svg: `<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="1.5" y="5.5" width="10" height="10" rx="1.5" stroke="currentColor" stroke-width="1.5"/><path d="M7 5.5V4A1.5 1.5 0 018.5 2.5h6A1.5 1.5 0 0116 4v6a1.5 1.5 0 01-1.5 1.5H13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>`
  },
  {
    value: 'ratio_convert', label: '比例转换',
    svg: `<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="1.5" y="4" width="8" height="10" rx="1.5" stroke="currentColor" stroke-width="1.5"/><rect x="11" y="6" width="5.5" height="6" rx="1" stroke="currentColor" stroke-width="1.3"/><path d="M10 9h1" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>`
  },
  {
    value: 'translate', label: '图文翻译',
    svg: `<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M2 4h7M5.5 2v2M3 7c.5 1.5 2 3 2.5 3M8 7c-.5 1.5-2 3-2.5 3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/><path d="M9.5 10l2-5 2 5M10.3 8.5h2.4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/><path d="M10 14h6" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/></svg>`
  },
  {
    value: 'gallery', label: '历史图库',
    svg: `<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="1.5" y="1.5" width="6.5" height="6.5" rx="1" stroke="currentColor" stroke-width="1.4"/><rect x="10" y="1.5" width="6.5" height="6.5" rx="1" stroke="currentColor" stroke-width="1.4"/><rect x="1.5" y="10" width="6.5" height="6.5" rx="1" stroke="currentColor" stroke-width="1.4"/><rect x="10" y="10" width="6.5" height="6.5" rx="1" stroke="currentColor" stroke-width="1.4"/></svg>`
  },
]


</script>

<style scoped>
.ecom-topnav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 100%;
  padding: 0 24px;
}

.nav-left {
  display: flex;
  align-items: center;
  height: 100%;
  gap: 12px;
}

/* 鍝佺墝鍖?*/
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-right: 16px;
  flex-shrink: 0;
}
.brand-svg { 
  width: 30px; 
  height: 30px; 
  filter: drop-shadow(0 2px 4px rgba(22, 119, 255, 0.2));
}
.brand-name {
  font-size: 16px;
  font-weight: 800;
  background: linear-gradient(135deg, #1677ff, #6366f1);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  white-space: nowrap;
  letter-spacing: -0.01em;
}

.divider-v {
  width: 1px;
  height: 24px;
  background: var(--theme-border-primary);
  margin: 0 8px;
  flex-shrink: 0;
}

/* 妯″潡鏍囩 */
.module-tabs {
  display: flex;
  align-items: center;
  height: 100%;
  gap: 4px;
}

.tab-item {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 0 16px;
  height: 52px;
  border-radius: 12px;
  cursor: pointer;
  color: var(--text-secondary);
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  min-width: 72px;

  &:hover {
    color: var(--el-color-primary);
    background: var(--gray-btn-bg);
  }

  &.active {
    color: var(--el-color-primary);
    background: var(--theme-btn-fill-tertiary);
    font-weight: 600;
    
    .tab-indicator {
      transform: translateX(-50%) scaleX(1);
      opacity: 1;
    }
  }
}

.tab-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  svg {
    width: 20px;
    height: 20px;
  }
}
.tab-label {
  font-size: 12px;
  line-height: 1;
  white-space: nowrap;
}

.tab-indicator {
  position: absolute;
  bottom: 6px;
  left: 50%;
  transform: translateX(-50%) scaleX(0.5);
  width: 20px;
  height: 2.5px;
  background: var(--el-color-primary);
  border-radius: 4px;
  opacity: 0;
  transition: all 0.2s ease;
}

/* 鍙充晶鎿嶄綔鍖?*/
.nav-right {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-shrink: 0;
}

.model-trigger {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 6px 12px;
  background: var(--gray-btn-bg);
  border: 1px solid var(--theme-border-primary);
  border-radius: 10px;
  color: var(--text-color);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
  &:hover {
    border-color: var(--el-color-primary-light-3);
    color: var(--el-color-primary);
    background: var(--theme-btn-fill-tertiary);
  }
}
.model-trigger-label {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.chevron { opacity: 0.5; flex-shrink: 0; }

/* 寮圭獥鍐呭 */
.model-loading, .model-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px 0;
  color: var(--text-secondary);
  font-size: 14px;
}
.model-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 4px 0;
}
.model-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  border: 1.5px solid var(--theme-border-primary);
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.18s;
  position: relative;
  &:hover {
    border-color: var(--el-color-primary-light-3);
    background: var(--theme-btn-fill-tertiary);
  }
  &.selected {
    border-color: var(--el-color-primary);
    background: var(--theme-btn-fill-tertiary);
  }
}
.model-card-icon {
  flex-shrink: 0;
  color: var(--el-color-primary);
}
.model-card-info {
  flex: 1;
  min-width: 0;
}
.model-card-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-color);
}
.model-card-provider {
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 2px;
}
.model-card-desc {
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.model-card-check {
  flex-shrink: 0;
}

.credit-display {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--gray-btn-bg);
  padding: 4px 4px 4px 12px;
  border-radius: 12px;
  border: 1px solid var(--theme-border-primary);
}
.credit-num {
  font-weight: 800;
  color: #f90;
  font-size: 14px;
}
.credit-unit { color: var(--text-secondary); font-size: 12px; }
</style>

