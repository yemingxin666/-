<template>
<div class="workspace">
  <aside class="form-panel">
    <div class="panel-scroll">
      <!-- 表单区 -->
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
              placeholder="【商品品类】&#10;&#10;【核心卖点】&#10;&#10;【补充描述】"
            />
          </div>
          <el-tooltip
            :content="form.reference_assets.length ? '' : '请先上传参考图'"
            :disabled="form.reference_assets.length > 0"
            placement="top"
          >
            <div class="copywrite-container">
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

              <transition name="fade">
                <el-progress
                  v-if="showProgress"
                  :percentage="percentage"
                  :stroke-width="4"
                  :show-text="false"
                  color="#ed8936"
                  class="btn-progress"
                />
              </transition>
            </div>
          </el-tooltip>
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">图片类型 <em>(要几张选几个)</em></span>
          </template>
          <EcomTypeChips :types="sortedTypes" v-model="selectedTypes" />
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">生成比例 <em>(Generation Ratio)</em></span>
          </template>
          <EcomRatioPicker v-model="form.ratio" :recommended="recommendedRatio" />
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
            <el-radio value="en">English</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">风格特点描述 <em>(Style Description)</em></span>
          </template>
          <el-input
            v-model="form.style_desc"
            type="textarea"
            :rows="3"
            placeholder="描述图片风格，如：简约白底、科技感蓝色调、温暖家居风..."
          />
        </el-form-item>

        <el-form-item>
          <template #label>
            <span class="field-label">参考图片 <em>(最多3张产品白底图)</em></span>
          </template>
          <EcomImageUploader v-model:assetNos="form.reference_assets" :multiple="true" :limit="3" />
        </el-form-item>


      </el-form>
    </div>

    <!-- 底部固定区 -->
    <div class="panel-footer">
      <EcomCreditBadge :estimated-cost="estimatedCost" class="footer-credit" />
      <el-tooltip
        :content="!form.reference_assets.length ? '请先上传参考图' : ''"
        :disabled="form.reference_assets.length > 0"
        placement="top"
      >
        <button
          class="submit-btn"
          type="button"
          @click="submit"
          :disabled="taskStore.isRunning || !form.reference_assets.length"
        >
          {{ taskStore.isRunning ? '生成中...' : '立即生成' }}
        </button>
      </el-tooltip>
    </div>
  </aside>

  <!-- 右侧结果区 -->
  <section class="result-panel">
    <!-- 当前任务结果（每张图独立进度条，已内嵌 ResultCard 加载态） -->
    <div v-if="taskStore.items.length || taskStore.outputs.length" class="result-grid">
      <template v-if="taskStore.items.length">
        <EcomResultCard
          v-for="item in taskStore.items"
          :key="item.image_type"
          :url="item.url"
          :label="item.label"
          :status="item.status"
          :progress="item.progress"
          :phase="item.phase"
          :image-type="item.image_type"
          :ratio="taskStore.submittedRatio"
          :editable="item.status === 'succeeded' && !!item.asset_no"
          @edit="(p) => openEdit(taskStore.currentTask, item, p)"
          @delete="taskStore.reset()"
        />
      </template>
      <template v-else>
        <EcomResultCard
          v-for="(url, i) in taskStore.outputs"
          :key="i"
          :url="url"
          :ratio="taskStore.submittedRatio"
          @delete="taskStore.reset()"
        />
      </template>
    </div>

    <!-- 历史结果（会话级，未刷新前保留） -->
    <EcomHistoryGroup @edit="(task, item, p) => openEdit(task, item, p)" />

    <EcomEditDialog
      v-model="editVisible"
      :url="editPayload.url"
      :ratio="editPayload.ratio"
      :task-no="editPayload.taskNo"
      :asset-no="editPayload.assetNo"
      @submitted="onEditSubmitted"
    />

    <div v-if="!taskStore.currentTask && !taskStore.items.length && !taskStore.outputs.length && !taskStore.history.length" class="result-empty">
      <!-- 对照截图：破损图片 SVG + 文字 -->
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
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { useEcomConfigStore, useEcomTaskStore } from '@/store/ecom'
import { useEcomLinkage } from '@/composables/useEcomLinkage'
import EcomImageUploader from '@/components/ecom/EcomImageUploader.vue'
import EcomTypeChips from '@/components/ecom/EcomTypeChips.vue'
import EcomPlatformSelect from '@/components/ecom/EcomPlatformSelect.vue'
import EcomRatioPicker from '@/components/ecom/EcomRatioPicker.vue'
import EcomCreditBadge from '@/components/ecom/EcomCreditBadge.vue'
import EcomResultCard from '@/components/ecom/EcomResultCard.vue'
import EcomHistoryGroup from '@/components/ecom/EcomHistoryGroup.vue'
import EcomEditDialog from '@/components/ecom/EcomEditDialog.vue'
import { useEcomEdit } from '@/composables/useEcomEdit'
import { useCopywriteProgress } from '@/composables/useCopywriteProgress'
import { formatAnalysisToText, getStyleDesc } from '@/utils/ecomFormat'

const configStore = useEcomConfigStore()
const taskStore = useEcomTaskStore()
const { percentage, showProgress, start: startProgress, finish: finishProgress } = useCopywriteProgress()
const { editVisible, editPayload, openEdit, onEditSubmitted } = useEcomEdit()

const form = ref({
  product_name: '',
  selling_points: '',
  platform: 'pinduoduo',
  ratio: '1:1',
  language: 'zh-CN',
  style_desc: '',
  reference_assets: [],
  analysis: null,
})

const { recommendedRatio } = useEcomLinkage(form)

const selectedTypes = ref(['traffic_cover'])
const copywriting = ref(false)

watch(() => form.value.selling_points, () => {
  if (!copywriting.value) form.value.analysis = null
})

const sortedTypes = computed(() => {
  const cfg = configStore.getPlatformConfig(form.value.platform)
  if (!cfg || !cfg.priority_images) return configStore.mainImageTypes

  const priority = cfg.priority_images
  const mustHave = priority.must_have || []
  const recommended = priority.recommended || []
  const optional = priority.optional || []

  const getScore = (val) => {
    if (mustHave.includes(val)) return 3
    if (recommended.includes(val)) return 2
    if (optional.includes(val)) return 1
    return 0
  }

  return [...configStore.mainImageTypes].sort((a, b) => getScore(b.value) - getScore(a.value))
})

const estimatedCost = computed(() => selectedTypes.value.length * 10)

const copywrite = async () => {
  if (!form.value.reference_assets.length) { ElMessage.warning('请先上传参考图'); return }
  copywriting.value = true
  startProgress()
  try {
    const { content, analysis } = await configStore.generateCopywriting(
      form.value.product_name,
      form.value.selling_points,
      form.value.reference_assets
    )
    if (analysis) {
      if (!form.value.product_name) form.value.product_name = analysis.product_name || ''
      if (!form.value.style_desc) form.value.style_desc = getStyleDesc(analysis.recommended_style)
      form.value.analysis = analysis
    }
    form.value.selling_points = formatAnalysisToText(analysis, content)
    ElMessage.success('已根据参考图生成卖点')
  } catch (e) {
    ElMessage.error('代写失败：' + e.message)
  } finally {
    copywriting.value = false
    finishProgress()
  }
}

const submit = async () => {
  if (!selectedTypes.value.length) { ElMessage.warning('请至少选择一种图片类型'); return }
  if (configStore.userPower < estimatedCost.value) {
    ElMessage.error('算力不足，请充值后重试'); return
  }

  try {
    await taskStore.submitTask('/api/ai-commerce/main-images', {
      ...form.value,
      module: 'main_image',
      image_type: selectedTypes.value.join(','),
      model: configStore.selectedModel,
    })
  } catch (e) {
    ElMessage.error('提交失败：' + e.message)
  }
}

onMounted(() => taskStore.resumeIfPending())
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

/* 左侧面板 */
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

/* 中英双语标签 */
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

/* 卖点输入框 */
.selling-points-box { width: 100%; }
.selling-points-box :deep(.el-textarea__inner) {
  font-size: 13px;
  line-height: 1.6;
}

/* AI 代写按钮 */
.copywrite-container {
  position: relative;
  width: 100%;
  margin-top: 10px;
}
.copywrite-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
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

.btn-progress {
  display: block;
  width: 100%;
  margin-top: 6px;
}

/* 淡出动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* 底部固定 */
.panel-footer {
  flex-shrink: 0;
  padding: 16px 20px 24px;
  border-top: 1px solid var(--theme-border-primary);
  background: var(--theme-bg);
}
.footer-credit { margin-bottom: 12px; width: 100%; }

/* 立即生成按钮 */
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

/* 右侧结果区 */
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


/* 空态 */
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
