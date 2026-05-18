<template>
  <div class="result-card" :class="{ 'is-failed': status === 'failed', 'is-timeout': status === 'timeout' }" :style="{ '--card-ratio': cssRatio }">
    <div class="card-image-wrap">
      <!-- 成功：显示图片 + 标签徽章 + 操作覆盖层（粘性显示，使用 stickyUrl 避免闪烁） -->
      <template v-if="stickyUrl && status !== 'failed'">
        <el-image
          :src="stickyUrl"
          fit="cover"
          class="result-img"
          :preview-src-list="[stickyUrl]"
          preview-teleported
          hide-on-click-modal
        />
        <div v-if="label" class="label-badge">{{ label }}</div>
        <div class="preview-hint">点击图片可放大预览</div>
      </template>

      <!-- 失败态 -->
      <div v-else-if="status === 'failed'" class="error-card">
        <el-icon class="error-icon"><CircleCloseFilled /></el-icon>
        <div class="error-label">{{ label || '生成失败' }}</div>
        <el-button size="small" @click="emit('regenerate', props.imageType)" class="retry-btn">重试</el-button>
      </div>

      <!-- 超时态 -->
      <div v-else-if="status === 'timeout'" class="error-card timeout-card" role="alert">
        <el-icon class="error-icon timeout-icon"><Clock /></el-icon>
        <div class="error-label">任务超时</div>
        <div class="timeout-tip">请查看历史记录或重试</div>
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
            class="skeleton-progress"
            :percentage="displayProgress"
            :stroke-width="6"
            :show-text="false"
            :indeterminate="status !== 'running'"
            :duration="2"
            :color="progressColor"
          />
          <div class="skeleton-phase-row">
            <span class="skeleton-phase">{{ phaseText }}</span>
            <span v-if="status === 'running'" class="skeleton-percent">{{ displayProgress }}%</span>
          </div>
        </div>
      </div>

    </div>

    <!-- 独立操作工具栏：图片下方常驻条带，永不与图片争空间，永不被裁切 -->
    <div v-if="stickyUrl && status !== 'failed'" class="card-toolbar">
      <button class="tool-btn" @click="download" title="下载" aria-label="下载图片">
        <el-icon><Download /></el-icon>
      </button>
      <button v-if="editable" class="tool-btn" @click="emit('edit', { url: stickyUrl, ratio: props.ratio })" title="编辑" aria-label="编辑该图片">
        <el-icon><Edit /></el-icon>
      </button>
      <button v-else class="tool-btn" @click="emit('regenerate', props.imageType)" title="重新生成" aria-label="重新生成该图片">
        <el-icon><Refresh /></el-icon>
      </button>
      <template v-if="deletable">
        <el-popconfirm
          v-if="confirmDelete"
          :title="confirmDeleteTitle"
          confirm-button-text="删除"
          cancel-button-text="取消"
          @confirm="emit('delete')"
        >
          <template #reference>
            <button class="tool-btn tool-btn-danger" title="删除" aria-label="删除该图片">
              <el-icon><Delete /></el-icon>
            </button>
          </template>
        </el-popconfirm>
        <button v-else class="tool-btn tool-btn-danger" title="删除" aria-label="删除该图片" @click="emit('delete')">
          <el-icon><Delete /></el-icon>
        </button>
      </template>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { Download, Refresh, Edit, Delete, CircleCloseFilled, Clock } from '@element-plus/icons-vue'

const props = defineProps({
  url: { type: String, default: null },
  label: { type: String, default: '' },
  status: { type: String, default: 'succeeded' },
  progress: { type: Number, default: 0 },
  phase: { type: String, default: '' },
  imageType: { type: String, default: '' },
  ratio: { type: String, default: '1:1' },
  // 历史图库走撤销机制（无需 popconfirm）；当前任务区域走二次确认避免误删
  confirmDelete: { type: Boolean, default: true },
  confirmDeleteTitle: { type: String, default: '确认删除该图片？' },
  // 是否启用"编辑"模式（仅历史图库需要；其他场景保持"重新生成"按钮）
  editable: { type: Boolean, default: false },
  // 是否显示删除按钮（当前任务区域不需要删除，历史图库需要）
  deletable: { type: Boolean, default: true },
})

// "16:9" → "16/9"，CSS aspect-ratio 语法
const cssRatio = computed(() => props.ratio.replace(':', '/'))
const emit = defineEmits(['regenerate', 'delete', 'edit'])

// 粘性 URL：一旦获得 url，即使后续轮询暂时返回 null/undefined 也保留显示
// 防止已完成的图片在新一轮轮询时闪回加载态
const stickyUrl = ref(props.url || null)

// 比较 URL 的 path 部分（忽略签名 query 参数）：
// 后端每次轮询都对 OSS 重新签名，会导致 query 不同但指向同一图片，
// 直接赋值会让浏览器重新请求图片造成闪屏
const urlPath = (u) => {
  if (!u) return ''
  const i = u.indexOf('?')
  return i >= 0 ? u.slice(0, i) : u
}

watch(
  () => props.url,
  (val) => {
    if (val) {
      // 仅当指向不同对象时才更新，避免新签名导致闪屏
      if (urlPath(val) !== urlPath(stickyUrl.value)) {
        stickyUrl.value = val
      }
    } else if (props.status === 'failed') {
      stickyUrl.value = null
    }
  }
)
// status 变为 failed 时清空粘性 URL，让失败态正常显示
watch(
  () => props.status,
  (s) => {
    if (s === 'failed' || s === 'timeout') stickyUrl.value = null
  }
)

// 加载态显示的进度：running 用真实值，否则用 0（条带动画依赖 indeterminate）
const displayProgress = computed(() => {
  const p = Number(props.progress) || 0
  return Math.max(0, Math.min(99, Math.round(p)))
})

// phase 文案：优先取后端 phase，否则按 status 回退
const phaseText = computed(() => {
  const phaseMap = {
    pending: '等待中',
    rendering: '渲染中...',
    generating: '生图中...',
    uploading: '上传中...',
  }
  if (props.phase && phaseMap[props.phase]) return phaseMap[props.phase]
  if (props.status === 'running') return '生图中...'
  return '排队中...'
})

// phase 渐变色：pending 灰 → generating/rendering 蓝 → uploading 绿
const progressColor = computed(() => {
  if (props.phase === 'uploading') return '#67c23a'
  if (props.phase === 'pending' || props.status !== 'running') return '#909399'
  return '#409eff'
})

// MIME → 扩展名映射，按 blob.type 推断最可靠
const MIME_EXT = {
  'image/png': 'png',
  'image/jpeg': 'jpg',
  'image/jpg': 'jpg',
  'image/webp': 'webp',
  'image/gif': 'gif',
  'image/bmp': 'bmp',
}

// 从 URL 路径提取扩展名（剥离 query/hash），失败返回空串
const guessExtFromUrl = (url) => {
  try {
    const path = new URL(url).pathname
    const m = path.match(/\.([a-zA-Z0-9]{2,5})$/)
    return m ? m[1].toLowerCase() : ''
  } catch {
    return ''
  }
}

const buildFilename = (ext) => `ecom_${Date.now()}.${ext || 'png'}`

const download = async () => {
  try {
    const res = await fetch(props.url)
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const blob = await res.blob()
    const ext = MIME_EXT[blob.type] || guessExtFromUrl(props.url) || 'png'
    const objectUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = buildFilename(ext)
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(objectUrl)
  } catch (err) {
    // 跨域 fetch 失败时降级：尽量用 <a download> 触发下载，跨域时浏览器可能忽略 download 属性
    console.warn('[ecom download] fetch fallback:', err)
    const ext = guessExtFromUrl(props.url) || 'png'
    const a = document.createElement('a')
    a.href = props.url
    a.download = buildFilename(ext)
    a.target = '_blank'
    a.rel = 'noopener'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
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
.result-card.is-failed,
.result-card.is-timeout {
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

/* 独立操作工具栏：位于图片下方，常驻可见，hover 时强化背景。永不与图片争空间。 */
.card-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  height: 36px;
  padding: 0 10px;
  background: var(--theme-bg);
  border-top: 1px solid var(--theme-border-primary);
}

.tool-btn {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: none;
  border-radius: 8px;
  background: var(--gray-btn-bg);
  color: var(--text-color);
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease, transform 0.18s ease;
}
.tool-btn .el-icon {
  font-size: 14px;
}
.tool-btn:hover {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  transform: translateY(-1px);
}
.tool-btn-danger:hover {
  background: var(--el-color-danger-light-9);
  color: var(--el-color-danger);
}

/* 图片层指针变手型，提示用户可点击放大 */
.result-img { cursor: zoom-in; }

/* hover 时顶部浮现提示文字，不拦截点击 */
.preview-hint {
  position: absolute;
  top: 8px;
  right: 8px;
  padding: 3px 10px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 12px;
  border-radius: 12px;
  opacity: 0;
  transform: translateY(-4px);
  transition: opacity 0.25s, transform 0.25s;
  pointer-events: none;
  z-index: 3;
  white-space: nowrap;
}
.result-card:hover .preview-hint {
  opacity: 1;
  transform: translateY(0);
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
  margin-bottom: 4px;
}
:deep(.skeleton-progress .el-progress-bar__outer) {
  background: rgba(255, 255, 255, 0.25);
}
.skeleton-phase-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 11px;
  color: rgba(255, 255, 255, 0.85);
}
.skeleton-phase { font-size: 11px; color: rgba(255, 255, 255, 0.85); }
.skeleton-percent {
  font-size: 11px;
  color: #fff;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
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
.timeout-icon {
  color: var(--el-color-warning);
}
.timeout-tip {
  font-size: 11px;
  color: var(--text-secondary);
}

.result-card { will-change: transform; }
</style>
