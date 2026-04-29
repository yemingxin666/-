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
import { ref } from 'vue'
import EcomTopNav from './EcomTopNav.vue'

const activeModule = ref('main_image')
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
  background: #fff;
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
