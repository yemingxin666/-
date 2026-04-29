<template>
  <div class="result-card">
    <div class="card-image-wrap">
      <el-image :src="url" fit="cover" class="result-img" :preview-src-list="[url]" preview-teleported />
      <!-- hover overlay 操作层 -->
      <div class="card-overlay">
        <el-button circle @click="download" title="下载">
          <el-icon><Download /></el-icon>
        </el-button>
        <el-button circle @click="emit('regenerate')" title="重新生成">
          <el-icon><Refresh /></el-icon>
        </el-button>
        <el-popconfirm title="确定删除？" @confirm="emit('delete')">
          <template #reference>
            <el-button circle type="danger" title="删除">
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-popconfirm>
      </div>
    </div>
  </div>
</template>

<script setup>
import { Download, Refresh, Delete } from '@element-plus/icons-vue'

const props = defineProps({ url: { type: String, required: true } })
const emit = defineEmits(['regenerate', 'delete'])

const download = () => {
  const a = document.createElement('a')
  a.href = props.url
  a.download = 'ecom_' + Date.now() + '.png'
  a.target = '_blank'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}
</script>

<style scoped>
.result-card {
  border-radius: 16px;
  overflow: hidden;
  background: var(--theme-bg);
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--theme-border-primary);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.result-card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-4px);
  border-color: var(--el-color-primary-light-5);
}

.card-image-wrap {
  position: relative;
  overflow: hidden;
}

.result-img {
  width: 100%;
  aspect-ratio: 1;
  display: block;
  transition: transform 0.5s;
}
.result-card:hover .result-img {
  transform: scale(1.05);
}

.card-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  opacity: 0;
  transition: all 0.3s;
}
.result-card:hover .card-overlay,
.result-card:focus-within .card-overlay {
  opacity: 1;
}

:deep(.el-button.is-circle) {
  width: 40px;
  height: 40px;
  background: rgba(255, 255, 255, 0.9);
  border: none;
  color: #333;
  transition: all 0.2s;
  
  &:hover {
    transform: scale(1.1);
    background: #fff;
    color: var(--el-color-primary);
  }
}

.result-card { will-change: transform; }
</style>
