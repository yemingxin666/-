<template>
<div class="workspace">
  <aside class="form-panel">
    <div class="panel-scroll">
      <h3 class="panel-title">比例转换</h3>
      <el-form label-position="top">
        <el-form-item>
          <template #label>
            <span class="field-label">参考图片 <em>(最多3张)</em></span>
          </template>
          <EcomImageUploader v-model:assetNos="assetNos" :multiple="true" :limit="3" />
        </el-form-item>
        <el-form-item>
          <template #label>
            <span class="field-label">目标比例 <em>(Target Ratio)</em></span>
          </template>
          <EcomRatioPicker v-model="ratio" />
        </el-form-item>
        <el-form-item>
          <template #label>
            <span class="field-label">转换模式 <em>(Convert Mode)</em></span>
          </template>
          <div class="mode-cards">
            <button
              v-for="m in modeOptions"
              :key="m.value"
              class="mode-card"
              :class="{ active: mode === m.value }"
              type="button"
              @click="mode = m.value"
            >
              <span class="mode-icon">{{ m.icon }}</span>
              <span class="mode-name">{{ m.name }}</span>
              <span class="mode-desc">{{ m.desc }}</span>
              <span class="mode-cost">{{ m.cost }} 积分/张</span>
            </button>
          </div>
        </el-form-item>
      </el-form>
    </div>
    <div class="panel-footer">
      <EcomCreditBadge :estimated-cost="estimatedCost" class="footer-credit" />
      <el-tooltip
        :content="!assetNos.length ? '请先上传参考图片' : ''"
        :disabled="assetNos.length > 0"
        placement="top"
      >
        <button class="submit-btn" type="button" @click="submit" :disabled="taskStore.isRunning || !assetNos.length">
          {{ taskStore.isRunning ? '处理中...' : '开始转换' }}
        </button>
      </el-tooltip>
    </div>
  </aside>

  <section class="result-panel">
    <div v-if="taskStore.outputs.length || taskStore.isRunning" class="result-grid">
      <template v-if="taskStore.outputs.length">
        <EcomResultCard v-for="(url, i) in taskStore.outputs" :key="i" :url="url" @regenerate="submit" @delete="taskStore.reset()" />
      </template>
      <EcomResultCard
        v-else
        :url="null"
        :status="taskStore.currentTask?.status || 'pending'"
        :progress="taskStore.currentTask?.progress || 0"
        :ratio="taskStore.submittedRatio"
        @regenerate="submit"
        @delete="taskStore.reset()"
      />
    </div>
    <EcomHistoryGroup />
    <div v-if="!taskStore.currentTask && !taskStore.outputs.length && !taskStore.history.length" class="result-empty">
      <svg class="empty-svg" viewBox="0 0 80 80" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="8" y="14" width="64" height="52" rx="4" stroke="currentColor" stroke-width="2.5"/>
        <path d="M8 46l18-14 14 12 10-8 22 16" stroke="currentColor" stroke-width="2.5" stroke-linejoin="round"/>
        <circle cx="26" cy="30" r="5" stroke="currentColor" stroke-width="2.2"/>
      </svg>
      <p class="empty-title">暂无生成的图片</p>
      <p class="empty-tip">上传图片并选择目标比例后开始转换</p>
    </div>
  </section>
</div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useEcomConfigStore, useEcomTaskStore } from '@/store/ecom'
import EcomImageUploader from '@/components/ecom/EcomImageUploader.vue'
import EcomRatioPicker from '@/components/ecom/EcomRatioPicker.vue'
import EcomCreditBadge from '@/components/ecom/EcomCreditBadge.vue'
import EcomResultCard from '@/components/ecom/EcomResultCard.vue'
import EcomHistoryGroup from '@/components/ecom/EcomHistoryGroup.vue'

const configStore = useEcomConfigStore()
const taskStore = useEcomTaskStore()
const assetNos = ref([])
const ratio = ref('1:1')
const mode = ref('crop')

const modeOptions = [
  { value: 'crop',     icon: '✂️', name: '裁剪', desc: '保留图片中心区域', cost: 3 },
  { value: 'outpaint', icon: '✨', name: '扩图', desc: 'AI 智能填充边缘',  cost: 10 },
]

const estimatedCost = computed(() => {
  const unitCost = mode.value === 'outpaint' ? 10 : 3
  const count = Math.max(1, assetNos.value.length)
  return unitCost * count
})

const submit = async () => {
  if (!assetNos.value.length) { ElMessage.warning('请先上传图片'); return }
  const cost = estimatedCost.value
  if (configStore.userPower < cost) { ElMessage.error('算力不足，请充值'); return }
  try {
    await taskStore.submitTask('/api/ai-commerce/ratio-conversions', {
      module: 'ratio_convert', ratio: ratio.value, reference_assets: assetNos.value, style_desc: mode.value, model: configStore.selectedModel,
    })
  } catch (e) { ElMessage.error('提交失败：' + e.message) }
}

onMounted(() => taskStore.resumeIfPending())
onUnmounted(() => taskStore.stopPolling())
</script>

<style scoped>
.workspace { display: flex; height: 100%; width: 100%; overflow: hidden; background: var(--theme-bg); }

.form-panel {
  width: 320px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--theme-border-primary);
  background: var(--theme-bg);
  height: 100%;
  box-shadow: 4px 0 10px rgba(0, 0, 0, 0.02);
  z-index: 5;
}

.panel-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  scrollbar-gutter: stable;
}

.panel-title { margin: 0 0 16px; font-size: 15px; font-weight: 700; color: var(--text-color); }

.field-label { font-size: 14px; color: var(--text-color); font-weight: 600; }
.field-label em { font-style: normal; font-size: 12px; color: var(--text-secondary); margin-left: 4px; font-weight: 400; }

:deep(.el-form-item__label) { line-height: 1.6; padding-bottom: 8px !important; }
:deep(.el-form-item) { margin-bottom: 20px; }

:deep(.el-input__wrapper), :deep(.el-textarea__inner) {
  border-radius: 10px;
  background: var(--gray-btn-bg);
  box-shadow: none !important;
  border: 1px solid var(--theme-border-primary);
  transition: all 0.2s;
}
:deep(.el-input__wrapper:hover), :deep(.el-textarea__inner:hover),
:deep(.el-input__wrapper.is-focus), :deep(.el-textarea__inner:focus) {
  border-color: var(--el-color-primary-light-3);
  background: var(--theme-bg);
}

.mode-cards { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; width: 100%; }
.mode-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 12px 8px;
  background: var(--gray-btn-bg);
  border: 1.5px solid var(--theme-border-primary);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  text-align: center;
}
.mode-card:hover { border-color: var(--el-color-primary-light-3); background: var(--theme-bg); }
.mode-card.active { border-color: var(--el-color-primary); background: var(--el-color-primary-light-9, #f0f4ff); }
.mode-icon { font-size: 20px; line-height: 1; }
.mode-name { font-size: 13px; font-weight: 700; color: var(--text-color); }
.mode-desc { font-size: 11px; color: var(--text-secondary); line-height: 1.3; }
.mode-cost { font-size: 11px; font-weight: 600; color: var(--el-color-primary); margin-top: 2px; }

.panel-footer {
  flex-shrink: 0;
  padding: 16px 20px 24px;
  border-top: 1px solid var(--theme-border-primary);
  background: var(--theme-bg);
}
.footer-credit { margin-bottom: 12px; width: 100%; }

.submit-btn {
  display: block;
  width: 100%;
  padding: 12px 0;
  background: linear-gradient(135deg, var(--el-color-primary), var(--el-color-primary-dark-2));
  border: none;
  border-radius: 12px;
  color: #fff;
  font-size: 15px;
  font-weight: 700;
  cursor: pointer;
  letter-spacing: 1px;
  transition: all 0.2s;
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
}
.submit-btn:hover:not(:disabled) { transform: translateY(-1px); box-shadow: 0 6px 16px rgba(99, 102, 241, 0.4); }
.submit-btn:disabled { background: #e2e8f0; color: #94a3b8; box-shadow: none; cursor: not-allowed; }

.result-panel { flex: 1; padding: 24px; overflow-y: auto; background: var(--gray-btn-bg); }
.result-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 20px; }

.result-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  opacity: 0.6;
}
.empty-svg { width: 100px; height: 100px; margin-bottom: 16px; color: var(--text-secondary); }
.empty-title { font-size: 15px; color: var(--text-secondary); margin: 0 0 6px; font-weight: 500; }
.empty-tip { font-size: 13px; color: var(--text-secondary); margin: 0; }
</style>
