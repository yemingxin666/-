<template>
<div class="workspace">
  <aside class="form-panel">
    <div class="panel-scroll">
      <el-form :model="form" label-position="top">

        <el-form-item>
          <template #label>
            <span class="field-label">产品名称 <em>(Product Name)</em></span>
          </template>
          <el-input v-model="form.product_name" placeholder="产品名称" />
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">商品卖点 <em>(可以手动填写)</em></span>
          </template>
          <div class="selling-points-box">
            <el-input
              v-model="form.selling_points"
              type="textarea"
              :rows="6"
              placeholder=""
            />
          </div>
          <el-tooltip
            :content="form.reference_assets.length ? '' : '请先上传参考图'"
            :disabled="form.reference_assets.length > 0"
            placement="top"
          >
            <button
              class="copywrite-btn"
              type="button"
              @click="copywrite"
              :disabled="copywriting || form.reference_assets.length === 0"
            >
              <template v-if="copywriting">
                <el-icon class="is-loading"><Loading /></el-icon>
                <span>AI 分析生成中...</span>
              </template>
              <template v-else>
                <span>AI 识别图片并代写卖点</span>
              </template>
            </button>
          </el-tooltip>
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">图片类型 <em>(要几张选几个)</em></span>
          </template>
          <EcomTypeChips :types="configStore.detailPageTypes" v-model="selectedTypes" />
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">生成比例 <em>(Generation Ratio)</em></span>
          </template>
          <EcomRatioPicker v-model="form.ratio" />
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">输出平台 <em>(Output Platform)</em></span>
          </template>
          <EcomPlatformSelect v-model="form.platform" />
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">输出语言 <em>(Output Language)</em></span>
          </template>
          <el-radio-group v-model="form.language">
            <el-radio value="zh-CN">简体中文</el-radio>
            <el-radio value="en-US">English</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">参考图片 <em>(最多3张产品白底图)</em></span>
          </template>
          <EcomImageUploader v-model:assetNos="form.reference_assets" :multiple="true" :limit="3" />
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">风格特点描述 <em>(Style Description)</em></span>
          </template>
          <el-input
            v-model="form.style_description"
            type="textarea"
            :rows="3"
            placeholder="请输入风格特点描述......"
          />
        </el-form-item>

      </el-form>
    </div>

    <div class="panel-footer">
      <EcomCreditBadge :estimated-cost="estimatedCost" class="footer-credit" />
      <button
        class="submit-btn"
        type="button"
        @click="submit"
        :disabled="taskStore.isRunning"
      >
        {{ taskStore.isRunning ? '生成中...' : '立即生成' }}
      </button>
    </div>
  </aside>

  <section class="result-panel">
    <EcomProgressBar v-if="taskStore.currentTask" />

    <div v-if="taskStore.outputs.length" class="result-grid">
      <EcomResultCard
        v-for="(url, i) in taskStore.outputs"
        :key="i"
        :url="url"
        @regenerate="submit"
        @delete="taskStore.reset()"
      />
    </div>

    <div v-else-if="!taskStore.currentTask" class="result-empty">
      <svg class="empty-svg" viewBox="0 0 80 80" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="8" y="14" width="64" height="52" rx="4" stroke="#d9d9d9" stroke-width="2.5"/>
        <path d="M8 46l18-14 14 12 10-8 22 16" stroke="#d9d9d9" stroke-width="2.5" stroke-linejoin="round"/>
        <circle cx="26" cy="30" r="5" stroke="#d9d9d9" stroke-width="2.2"/>
        <circle cx="56" cy="52" r="9" fill="#fff" stroke="#d9d9d9" stroke-width="2"/>
        <path d="M56 48v5M56 55v1" stroke="#faad14" stroke-width="2" stroke-linecap="round"/>
      </svg>
      <p class="empty-title">暂无生成的图片</p>
      <p class="empty-tip">在左侧配置参数后点击"立即生成"</p>
    </div>
  </section>
</div>
</template>

<script setup>
import { ref, computed, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { useEcomConfigStore, useEcomTaskStore } from '@/store/ecom'
import EcomImageUploader from '@/components/ecom/EcomImageUploader.vue'
import EcomTypeChips from '@/components/ecom/EcomTypeChips.vue'
import EcomPlatformSelect from '@/components/ecom/EcomPlatformSelect.vue'
import EcomRatioPicker from '@/components/ecom/EcomRatioPicker.vue'
import EcomCreditBadge from '@/components/ecom/EcomCreditBadge.vue'
import EcomProgressBar from '@/components/ecom/EcomProgressBar.vue'
import EcomResultCard from '@/components/ecom/EcomResultCard.vue'

const configStore = useEcomConfigStore()
const taskStore = useEcomTaskStore()

const form = ref({
  product_name: '',
  selling_points: '【商品品类】\n\n【核心卖点】\n\n【补充描述】',
  style_description: '',
  platform: 'generic',
  ratio: '3:4',
  language: 'zh-CN',
  reference_assets: [],
})
const selectedTypes = ref(['hero_visual'])
const copywriting = ref(false)

const estimatedCost = computed(() => selectedTypes.value.length * 10)

const copywrite = async () => {
  if (!form.value.reference_assets.length) { ElMessage.warning('请先上传参考图'); return }
  copywriting.value = true
  try {
    const content = await configStore.generateCopywriting(
      form.value.product_name,
      form.value.selling_points,
      form.value.reference_assets
    )
    form.value.selling_points = content
    ElMessage.success('已根据参考图生成卖点')
  } catch (e) {
    ElMessage.error('代写失败：' + e.message)
  } finally {
    copywriting.value = false
  }
}

const submit = async () => {
  if (!selectedTypes.value.length) { ElMessage.warning('请至少选择一种图片类型'); return }
  if (configStore.userPower < estimatedCost.value) {
    ElMessage.error('算力不足，请充值后重试'); return
  }

  try {
    await taskStore.submitTask('/api/ai-commerce/detail-pages', {
      ...form.value,
      module: 'detail_page',
      image_type: selectedTypes.value.join(','),
      model: configStore.selectedModel,
    })
    configStore.deductPower(estimatedCost.value)
  } catch (e) {
    ElMessage.error('提交失败：' + e.message)
  }
}

onUnmounted(() => taskStore.stopPolling())
</script>

<style scoped>
.workspace {
  display: flex;
  height: 100%;
  width: 100%;
  overflow: hidden;
  background: var(--theme-bg);
}

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

.field-label {
  font-size: 14px;
  color: var(--text-color);
  font-weight: 600;
}
.field-label em {
  font-style: normal;
  font-size: 12px;
  color: var(--text-secondary);
  margin-left: 4px;
  font-weight: 400;
}

:deep(.el-form-item__label) {
  line-height: 1.6;
  padding-bottom: 8px !important;
}
:deep(.el-form-item) { margin-bottom: 20px; }

:deep(.el-input__wrapper), :deep(.el-textarea__inner) {
  border-radius: 10px;
  background: var(--gray-btn-bg);
  box-shadow: none !important;
  border: 1px solid var(--theme-border-primary);
  transition: all 0.2s;

  &:hover, &.is-focus {
    border-color: var(--el-color-primary-light-3);
    background: var(--theme-bg);
  }
}

.selling-points-box { width: 100%; }
.selling-points-box :deep(.el-textarea__inner) {
  font-size: 13px;
  line-height: 1.6;
}

.copywrite-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  margin-top: 10px;
  padding: 10px 0;
  background: linear-gradient(135deg, #f6ad55, #ed8936);
  border: none;
  border-radius: 10px;
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  box-shadow: 0 2px 4px rgba(237, 137, 54, 0.2);

  &:hover:not(:disabled) {
    transform: translateY(-1px);
    box-shadow: 0 4px 8px rgba(237, 137, 54, 0.3);
    background: linear-gradient(135deg, #ed8936, #dd6b20);
  }
  &:disabled { opacity: 0.6; cursor: not-allowed; }
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

  &:hover:not(:disabled) {
    transform: translateY(-1px);
    box-shadow: 0 6px 16px rgba(99, 102, 241, 0.4);
  }
  &:disabled {
    background: #e2e8f0;
    color: #94a3b8;
    box-shadow: none;
    cursor: not-allowed;
  }
}

.result-panel {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
  background: var(--gray-btn-bg);
}

.result-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 20px;
}

.result-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  opacity: 0.6;
}
.empty-svg {
  width: 100px;
  height: 100px;
  margin-bottom: 16px;
  color: var(--text-secondary);
}
.empty-title {
  font-size: 16px;
  color: var(--text-color);
  margin: 0 0 8px;
  font-weight: 600;
}
.empty-tip {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0;
}
</style>
