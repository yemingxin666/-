<template>
  <div class="result-card" :class="{ 'is-failed': status === 'failed' }" :style="{ '--card-ratio': cssRatio }">
    <div class="card-image-wrap">

      <!-- 成功：显示图片 + 标签徽章 + 操作覆盖层 -->
      <template v-if="url && status !== 'failed'">
        <el-image :src="url" fit="cover" class="result-img" :preview-src-list="[url]" preview-teleported />
        <div v-if="label" class="label-badge">{{ label }}</div>
        <div class="card-overlay">
          <el-button circle @click="download" title="下载">
            <el-icon><Download /></el-icon>
          </el-button>
          <el-button circle @click="emit('regenerate', props.imageType)" title="重新生成">
            <el-icon><Refresh /></el-icon>
          </el-button>
          <el-popconfirm title="删除此任务的全部图片？" @confirm="emit('delete')">
            <template #reference>
              <el-button circle type="danger" title="删除">
                <el-icon><Delete /></el-icon>
              </el-button>
            </template>
          </el-popconfirm>
        </div>
      </template>

      <!-- 失败态 -->
      <div v-else-if="status === 'failed'" class="error-card">
        <el-icon class="error-icon"><CircleCloseFilled /></el-icon>
        <div class="error-label">{{ label || '生成失败' }}</div>
        <el-button size="small" @click="emit('regenerate', props.imageType)" class="retry-btn">重试</el-button>
      </div>

      <!-- 加载态：pending / running -->
      <div v-else class="skeleton-card">
        <el-skeleton :loading="true" animated>
          <template #template>
            <el-skeleton-item variant="image" class="skeleton-img" />
          </template>
        </el-skeleton>
        <div class="skeleton-info">
          <div class="skeleton-label">{{ label || '生成中...' }}</div>
          <el-progress
            v-if="status === 'running' && progress > 0"
            :percentage="progress"
            :stroke-width="3"
            :show-text="false"
            class="skeleton-progress"
          />
          <div v-else class="skeleton-phase">{{ status === 'running' ? '生图中...' : '排队中...' }}</div>
        </div>
      </div>

    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Download, Refresh, Delete, CircleCloseFilled } from '@element-plus/icons-vue'

const props = defineProps({
  url: { type: String, default: null },
  label: { type: String, default: '' },
  status: { type: String, default: 'succeeded' },
  progress: { type: Number, default: 0 },
  imageType: { type: String, default: '' },
  ratio: { type: String, default: '1:1' },
})

// "16:9" → "16/9"，CSS aspect-ratio 语法
const cssRatio = computed(() => props.ratio.replace(':', '/'))
const emit = defineEmits(['regenerate', 'delete'])

const download = async () => {
  try {
    const res = await fetch(props.url)
    const blob = await res.blob()
    const objectUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = 'ecom_' + Date.now() + '.png'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(objectUrl)
  } catch {
    // 跨域 fetch 失败时降级为新标签打开
    window.open(props.url, '_blank')
  }
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
.result-card.is-failed {
  border-color: var(--el-color-danger-light-5);
}

.card-image-wrap {
  position: relative;
  overflow: hidden;
}

.result-img {
  width: 100%;
  aspect-ratio: var(--card-ratio, 1);
  display: block;
  transition: transform 0.5s;
}
.result-card:hover .result-img {
  transform: scale(1.05);
}

.label-badge {
  position: absolute;
  top: 8px;
  left: 8px;
  background: rgba(0, 0, 0, 0.5);
  color: #fff;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  z-index: 2;
  pointer-events: none;
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

/* 加载态 */
.skeleton-card {
  position: relative;
  width: 100%;
  aspect-ratio: var(--card-ratio, 1);
  overflow: hidden;
}
.skeleton-img {
  width: 100%;
  height: 100%;
  display: block;
}
.skeleton-info {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 10px 12px 12px;
  background: linear-gradient(transparent, rgba(0, 0, 0, 0.5));
}
.skeleton-label {
  font-size: 13px;
  color: #fff;
  margin-bottom: 6px;
  font-weight: 500;
}
.skeleton-progress {
  width: 100%;
}
:deep(.skeleton-progress .el-progress-bar__outer) {
  background: rgba(255, 255, 255, 0.3);
}
.skeleton-phase {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.7);
}

/* 失败态 */
.error-card {
  width: 100%;
  aspect-ratio: var(--card-ratio, 1);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: var(--gray-btn-bg);
  padding: 16px;
}
.error-icon {
  font-size: 36px;
  color: var(--el-color-danger);
}
.error-label {
  font-size: 13px;
  color: var(--text-secondary);
  text-align: center;
}
.retry-btn {
  margin-top: 4px;
}

.result-card { will-change: transform; }
</style>
