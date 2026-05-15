<template>
<div class="workspace">
  <aside class="form-panel">
    <div class="panel-scroll">
      <h3 class="panel-title">图文翻译</h3>
      <el-form label-position="top">
        <el-form-item>
          <template #label>
            <span class="field-label">参考图片 <em>(最多3张产品白底图)</em></span>
          </template>
          <EcomImageUploader v-model:assetNos="assetNos" :multiple="true" :limit="3" />
        </el-form-item>
        <el-form-item>
          <template #label>
            <span class="field-label">目标语言 <em>(Target Language)</em></span>
          </template>
          <el-select v-model="targetLang" style="width:100%">
            <el-option value="en" label="英文" />
            <el-option value="zh" label="中文" />
            <el-option value="ja" label="日文" />
            <el-option value="ko" label="韩文" />
            <el-option value="de" label="德文" />
            <el-option value="fr" label="法文" />
          </el-select>
        </el-form-item>
      </el-form>
    </div>
    <div class="panel-footer">
      <EcomCreditBadge :estimated-cost="8" class="footer-credit" />
      <el-tooltip
        :content="!assetNos.length ? '请先上传参考图片' : ''"
        :disabled="assetNos.length > 0"
        placement="top"
      >
        <button class="submit-btn" type="button" @click="submit" :disabled="taskStore.isRunning || !assetNos.length">
          {{ taskStore.isRunning ? '翻译中...' : '开始翻译' }}
        </button>
      </el-tooltip>
    </div>
  </aside>

  <section class="result-panel">
    <div v-if="taskStore.items.length || taskStore.outputs.length || taskStore.isRunning" class="result-grid">
      <template v-if="taskStore.items.length">
        <EcomResultCard
          v-for="item in taskStore.items"
          :key="item.image_type || item.asset_no"
          :url="item.url"
          :status="item.status"
          :progress="item.progress"
          :phase="item.phase"
          :ratio="taskStore.submittedRatio"
          :deletable="false"
          @regenerate="submit"
        />
      </template>
      <template v-else-if="taskStore.outputs.length">
        <EcomResultCard v-for="(url, i) in taskStore.outputs" :key="i" :url="url" :deletable="false" @regenerate="submit" />
      </template>
      <EcomResultCard
        v-else
        :url="null"
        :status="taskStore.currentTask?.status || 'pending'"
        :progress="taskStore.currentTask?.progress || 0"
        :ratio="taskStore.submittedRatio"
        :deletable="false"
        @regenerate="submit"
      />
    </div>
    <EcomHistoryGroup />
    <div v-if="!taskStore.currentTask && !taskStore.outputs.length && !taskStore.items.length && !taskStore.history.length" class="result-empty">
      <svg class="empty-svg" viewBox="0 0 80 80" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="8" y="14" width="64" height="52" rx="4" stroke="currentColor" stroke-width="2.5"/>
        <path d="M8 46l18-14 14 12 10-8 22 16" stroke="currentColor" stroke-width="2.5" stroke-linejoin="round"/>
        <circle cx="26" cy="30" r="5" stroke="currentColor" stroke-width="2.2"/>
      </svg>
      <p class="empty-title">暂无生成的图片</p>
      <p class="empty-tip">上传含文字的图片，AI 将识别并翻译文字</p>
    </div>
  </section>
</div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useEcomConfigStore, useEcomTaskStore } from '@/store/ecom'
import EcomImageUploader from '@/components/ecom/EcomImageUploader.vue'
import EcomCreditBadge from '@/components/ecom/EcomCreditBadge.vue'
import EcomResultCard from '@/components/ecom/EcomResultCard.vue'
import EcomHistoryGroup from '@/components/ecom/EcomHistoryGroup.vue'

const configStore = useEcomConfigStore()
const taskStore = useEcomTaskStore()
const assetNos = ref([])
const targetLang = ref('en')

const submit = async () => {
  if (!assetNos.value.length) { ElMessage.warning('请先上传图片'); return }
  if (configStore.userPower < 8) { ElMessage.error('算力不足，请充值'); return }
  try {
    await taskStore.submitTask('/api/ai-commerce/image-text-translations', {
      module: 'translate', language: targetLang.value, reference_assets: assetNos.value, model: configStore.selectedModel,
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

:deep(.el-select .el-input__wrapper) {
  border-radius: 10px;
  background: var(--gray-btn-bg);
  box-shadow: none !important;
  border: 1px solid var(--theme-border-primary);
}
:deep(.el-select .el-input__wrapper:hover),
:deep(.el-select .el-input.is-focus .el-input__wrapper) {
  border-color: var(--el-color-primary-light-3);
  background: var(--theme-bg);
}

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
.submit-btn:disabled { background: var(--disabled-btn-bg); color: var(--disabled-btn-color); box-shadow: none; cursor: not-allowed; }

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
