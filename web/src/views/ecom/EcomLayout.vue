<template>
  <div class="ecom-layout">
    <header class="ecom-header">
      <EcomTopNav v-model:activeModule="activeModule" />
    </header>
    <main class="ecom-body">
      <slot :activeModule="activeModule" />
    </main>
  </div>
</template>

<script setup>
import { provide, ref, watch } from 'vue'
import EcomTopNav from './EcomTopNav.vue'

const STORAGE_KEY = 'ecom_active_module'
const validModules = ['main_image', 'detail_page', 'white_bg', 'clone', 'ratio_convert', 'translate', 'gallery']

// 从 localStorage 恢复上次选中的 tab，无效值则回退到默认
const saved = localStorage.getItem(STORAGE_KEY)
const initial = validModules.includes(saved) ? saved : 'main_image'
const activeModule = ref(initial)

// tab 切换时自动持久化
watch(activeModule, (val) => {
  localStorage.setItem(STORAGE_KEY, val)
})

// 提供给子页面用于跨 tab 跳转（如编辑提交后跳到历史图库）
const setModule = (mod) => { activeModule.value = mod }
provide('setEcomModule', setModule)
</script>

<style scoped>
.ecom-layout {
  /* 直接锚定视口，脱离父级 .content{overflow:scroll} 的高度截断 */
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 65px; /* 避开左侧全局 sidebar（.menu-list width:65px） */
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--theme-bg);
  z-index: 20;
}
.ecom-header {
  height: 64px;
  flex-shrink: 0;
  background: var(--theme-bg);
  border-bottom: 1px solid var(--theme-border-primary);
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.03);
  z-index: 21;
}
.ecom-body {
  flex: 1;
  display: flex;
  overflow: hidden;
  background: var(--gray-btn-bg);
}
</style>
