<template>
  <el-dialog
    :model-value="modelValue"
    @update:model-value="(v) => emit('update:modelValue', v)"
    title="编辑图片"
    :width="dialogWidth"
    :close-on-click-modal="false"
    @closed="onClosed"
    :before-close="handleBeforeClose"
  >
    <div class="edit-layout">
      <!-- 左：原图缩略图 + 比例标签 -->
      <div class="preview-side">
        <div class="preview-frame" :style="{ aspectRatio: cssRatio }">
          <el-image v-if="url" :src="url" fit="contain" class="preview-img" />
        </div>
        <div class="meta-row">
          <span class="meta-label">原图比例</span>
          <el-tag size="small" effect="plain">{{ ratio || '1:1' }}</el-tag>
        </div>
        <div class="meta-row">
          <span class="meta-label">使用模型</span>
          <el-tag size="small" type="info" effect="plain" v-if="modelName">{{ modelName }}</el-tag>
          <el-tag size="small" type="warning" effect="plain" v-else>未选择</el-tag>
        </div>
      </div>

      <!-- 右：prompt 输入区 -->
      <div class="input-side">
        <label class="input-label" for="ecom-edit-prompt">编辑 Prompt（描述你希望如何修改这张图）</label>
        <el-input
          id="ecom-edit-prompt"
          v-model="prompt"
          type="textarea"
          :rows="8"
          placeholder="例如：将背景改为海滩，保持产品主体不变；增加柔和的暖色光线..."
          maxlength="500"
          show-word-limit
          aria-label="编辑 Prompt 输入框"
        />
        <div class="hint">提交后将作为新任务出现在历史图库；分辨率默认 1K，比例与原图一致。</div>
      </div>
    </div>

    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-tooltip :content="disabledReason" :disabled="!disabledReason" placement="top">
        <el-button
          type="primary"
          :loading="loading"
          :disabled="!!disabledReason"
          @click="handleSubmit"
        >确认生成</el-button>
      </el-tooltip>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useEcomConfigStore, useEcomGalleryStore } from '@/store/ecom'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  url: { type: String, default: '' },
  ratio: { type: String, default: '1:1' },
  taskNo: { type: String, default: '' },
  assetNo: { type: String, default: '' },
})

const emit = defineEmits(['update:modelValue', 'submitted'])

const configStore = useEcomConfigStore()
const galleryStore = useEcomGalleryStore()

const prompt = ref('')
const loading = ref(false)
const windowWidth = ref(typeof window !== 'undefined' ? window.innerWidth : 1024)

const modelName = computed(() => configStore.selectedModel || '')
const cssRatio = computed(() => (props.ratio || '1:1').replace(':', '/'))
// 响应式弹窗宽度：移动端 95%，平板 80%，桌面 640px
const dialogWidth = computed(() => {
  if (windowWidth.value < 768) return '95%'
  if (windowWidth.value < 1024) return '80%'
  return '640px'
})

const onResize = () => { windowWidth.value = window.innerWidth }
onMounted(() => window.addEventListener('resize', onResize))
onBeforeUnmount(() => window.removeEventListener('resize', onResize))

// 校验：prompt 必填、模型必选、关联 ID 完整
const disabledReason = computed(() => {
  if (!modelName.value) return '请先在顶部选择生图模型'
  if (!prompt.value.trim()) return '请输入编辑 prompt'
  if (!props.taskNo || !props.assetNo) return '原图信息缺失'
  return ''
})

// 关闭前若 prompt 非空，二次确认避免误关丢失输入
const handleBeforeClose = async (done) => {
  if (loading.value) return
  if (prompt.value.trim()) {
    try {
      await ElMessageBox.confirm('已输入的 Prompt 将丢失，确认关闭？', '提示', {
        type: 'warning',
        confirmButtonText: '关闭',
        cancelButtonText: '继续编辑',
      })
      done && done()
      emit('update:modelValue', false)
    } catch (_) { /* 取消关闭 */ }
  } else {
    done && done()
    emit('update:modelValue', false)
  }
}

const handleSubmit = async () => {
  if (disabledReason.value) return
  loading.value = true
  try {
    const data = await galleryStore.editTask(props.taskNo, props.assetNo, prompt.value.trim(), modelName.value)
    ElMessage.success('编辑任务已提交，请稍候在历史图库查看')
    emit('submitted', data)
    emit('update:modelValue', false)
  } catch (e) {
    ElMessage.error(e?.message || '提交失败')
  } finally {
    loading.value = false
  }
}

const onClosed = () => {
  prompt.value = ''
  loading.value = false
}
</script>

<style scoped>
.edit-layout {
  display: grid;
  grid-template-columns: 240px 1fr;
  gap: 20px;
}

@media (max-width: 768px) {
  .edit-layout {
    grid-template-columns: 1fr;
    gap: 14px;
  }
  .preview-frame {
    max-width: 200px;
    margin: 0 auto;
  }
}

.preview-side {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.preview-frame {
  width: 100%;
  background: var(--gray-btn-bg);
  border: 1px solid var(--theme-border-primary);
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.preview-img {
  width: 100%;
  height: 100%;
}

.meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.meta-label {
  color: var(--text-secondary);
  min-width: 64px;
}

.input-side {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.input-label {
  font-size: 13px;
  color: var(--text-color);
  font-weight: 500;
}

.hint {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
}
</style>
