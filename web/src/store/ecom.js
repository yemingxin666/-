import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'
import { httpGet, httpPost } from '@/utils/http'
import { checkSession } from '@/store/cache'

const MODULE_CAPS = {
  clone: 'img2img',
  ratio_convert: 'img2img',
}

export const useEcomConfigStore = defineStore('ecomConfig', () => {
  const userPower = ref(0)

  const platforms = [
    { value: 'pinduoduo', label: '拼多多' },
    { value: 'taobao',    label: '淘宝/天猫' },
    { value: 'jd',        label: '京东' },
    { value: 'douyin',    label: '抖音电商' },
    { value: 'xiaohongshu', label: '小红书' },
    { value: 'amazon',    label: 'Amazon' },
    { value: 'shopee',    label: 'Shopee' },
    { value: 'shopify',   label: 'Shopify/独立站' },
    { value: 'generic',   label: '通用' },
  ]

  const ratios = [
    { value: '1:1',   label: '1:1',   w: 48, h: 48 },
    { value: '4:3',   label: '4:3',   w: 56, h: 42 },
    { value: '16:9',  label: '16:9',  w: 64, h: 36 },
    { value: '9:16',  label: '9:16',  w: 36, h: 64 },
    { value: '3:4',   label: '3:4',   w: 42, h: 56 },
    { value: '3:2',   label: '3:2',   w: 60, h: 40 },
    { value: '2:3',   label: '2:3',   w: 40, h: 60 },
    { value: '21:9',  label: '21:9',  w: 70, h: 30 },
  ]

  const mainImageTypes = [
    { value: 'traffic_cover', label: '引流封面' },
    { value: 'core_selling_point', label: '核心卖点' },
    { value: 'scene_immersion', label: '场景代入' },
    { value: 'value_breakdown', label: '价值拆解' },
    { value: 'competitor_comparison', label: '竞品对比' },
    { value: 'detail_display', label: '细节展示' },
    { value: 'effect_proof', label: '效果证明' },
    { value: 'trust_building', label: '信任消疑' },
    { value: 'final_push', label: '临门一脚' },
  ]

  const detailPageTypes = [
    { value: 'hero_visual',       label: '首屏主视觉' },
    { value: 'core_selling',      label: '核心卖点图' },
    { value: 'usage_scene',       label: '使用场景图' },
    { value: 'multi_angle',       label: '多视角图' },
    { value: 'atmosphere',        label: '场景氛围图' },
    { value: 'product_detail',    label: '商品细节图' },
    { value: 'brand_story',       label: '品牌故事图' },
    { value: 'size_capacity',     label: '尺寸容量尺码图' },
    { value: 'effect_comparison', label: '效果对比图' },
    { value: 'spec_reference',    label: '详细规格参考图' },
    { value: 'craft_process',     label: '工艺制作图' },
    { value: 'accessory_gift',    label: '配件赠品图' },
    { value: 'series_showcase',   label: '系列展示图' },
    { value: 'ingredient',        label: '商品成分图' },
    { value: 'after_sales',       label: '售后保障图' },
    { value: 'usage_guide',       label: '使用建议图' },
  ]

  const aiModels = ref([])
  const platformConfigs = ref(new Map())
  const STORAGE_KEY = 'ecom_selected_model'
  const activeModule = ref('main_image')
  const selectedModel = ref(localStorage.getItem(STORAGE_KEY) || '')

  const filteredModels = computed(() => {
    const required = MODULE_CAPS[activeModule.value]
    if (!required) return aiModels.value
    return aiModels.value.filter(m => {
      const caps = (m.capabilities || '').split(',').map(c => c.trim())
      return caps.includes(required) || caps.length === 0 || m.capabilities === ''
    })
  })

  const setSelectedModel = (name) => {
    selectedModel.value = name
    localStorage.setItem('ecom_model_' + activeModule.value, name)
  }

  const loadUserPower = async () => {
    try {
      const user = await checkSession()
      userPower.value = user.power
    } catch (_) {}
  }

  const loadPlatformConfigs = async () => {
    try {
      const res = await httpGet('/api/ai-commerce/platform-configs')
      if (res.code === 0) {
        const m = new Map()
        ;(res.data?.items || []).forEach(item => m.set(item.value, item))
        platformConfigs.value = m
      }
    } catch (e) {
      console.error('[ecom] 加载平台配置失败:', e)
    }
  }

  const getPlatformConfig = (value) => platformConfigs.value.get(value) || null

  const loadModels = async (moduleName) => {
    if (moduleName) activeModule.value = moduleName
    loadPlatformConfigs() // 并行加载
    try {
      const res = await httpGet('/api/ai-commerce/models')
      aiModels.value = res.data || []
      // 若本地存储的模型已不在启用列表中，则重置为第一个
      const saved = localStorage.getItem('ecom_model_' + activeModule.value)
        || localStorage.getItem(STORAGE_KEY)
      const valid = filteredModels.value.find((m) => m.name === saved)
      if (valid) {
        selectedModel.value = saved
      } else if (filteredModels.value.length) {
        setSelectedModel(filteredModels.value[0].name)
      }
    } catch (e) {
      console.error('[ecom] 加载模型列表失败:', e)
    }
  }

  const deductPower = (amount) => {
    userPower.value = Math.max(0, userPower.value - amount)
  }


  const generateCopywriting = async (productName, hint, assetNos) => {
    const res = await httpPost('/api/ai-commerce/copywrite', {
      product_name: productName,
      hint: hint,
      reference_assets: (assetNos || []).slice(0, 3)
    })
    if (res.code !== 0) throw new Error(res.message || '生成失败')
    return {
      content: res.data.content,
      analysis: res.data.analysis
    }
  }

  return { userPower, platforms, ratios, mainImageTypes, detailPageTypes, aiModels, platformConfigs, activeModule, filteredModels, selectedModel, setSelectedModel, loadUserPower, loadPlatformConfigs, getPlatformConfig, loadModels, deductPower, generateCopywriting }
})

export const useEcomTaskStore = defineStore('ecomTask', () => {
  const currentTask = ref(null)
  const outputs = ref([])
  const items = ref([])
  const submitting = ref(false)
  let pollTimer = null

  const isRunning = computed(() =>
    submitting.value ||
    currentTask.value?.status === 'queued' ||
    currentTask.value?.status === 'running'
  )
  const isDone = computed(() =>
    currentTask.value?.status === 'succeeded' || currentTask.value?.status === 'failed'
  )

  const submitTask = async (endpoint, data) => {
    if (submitting.value) return
    submitting.value = true
    let res
    try {
      res = await httpPost(endpoint, data)
    } finally {
      submitting.value = false
    }
    const creditCost = res.data.credit_cost || 0
    const taskNo = res.data.task_no
    currentTask.value = { task_no: taskNo, status: res.data.status, progress: 0, credit_cost: creditCost }
    outputs.value = []

    if (data.image_type) {
      const configStore = useEcomConfigStore()
      const allTypes = [...configStore.mainImageTypes, ...configStore.detailPageTypes]
      items.value = data.image_type.split(',').filter(Boolean).map(t => ({
        image_type: t,
        label: allTypes.find(x => x.value === t)?.label || t,
        status: 'pending',
        phase: 'pending',
        progress: 0,
        url: null,
      }))
    } else {
      items.value = []
    }

    localStorage.setItem('ecom_pending_task', JSON.stringify({
      task_no: taskNo,
      module: data.module || '',
      image_type: data.image_type || '',
    }))

    useEcomConfigStore().deductPower(creditCost)
    startPolling()
    return res.data
  }

  const startPolling = () => {
    if (pollTimer) clearInterval(pollTimer)
    pollTimer = setInterval(async () => {
      const taskNo = currentTask.value?.task_no
      if (!taskNo) { _stopPolling(); return }
      try {
        const res = await httpGet(`/api/ai-commerce/tasks/${taskNo}`)
        if (currentTask.value?.task_no !== taskNo) return // stale response guard
        if (res.code === 0) {
          Object.assign(currentTask.value, res.data)
          items.value = res.data.items || []
          outputs.value = res.data.outputs || []
          if (res.data.status === 'succeeded' || res.data.status === 'failed') {
            localStorage.removeItem('ecom_pending_task')
            _stopPolling()
          }
        }
      } catch (_) { _stopPolling() }
    }, 3000)
  }

  // 内部停止（不清 localStorage）
  const _stopPolling = () => {
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  }

  // 对外暴露的 stopPolling（页面 unmount 时调用，不清 localStorage，任务可能仍在进行）
  const stopPolling = () => _stopPolling()

  const reset = () => {
    _stopPolling()
    submitting.value = false
    currentTask.value = null
    outputs.value = []
    items.value = []
    localStorage.removeItem('ecom_pending_task')
  }

  const resumeIfPending = async () => {
    const raw = localStorage.getItem('ecom_pending_task')
    if (!raw) return
    try {
      const { task_no, image_type } = JSON.parse(raw)
      const res = await httpGet(`/api/ai-commerce/tasks/${task_no}`)
      if (res.code !== 0) { localStorage.removeItem('ecom_pending_task'); return }
      const taskData = res.data
      if (taskData.status === 'succeeded' || taskData.status === 'failed') {
        // task completed while page was closed — show result then clear storage
        currentTask.value = { task_no, status: taskData.status, progress: taskData.progress || 0 }
        outputs.value = taskData.outputs || []
        items.value = taskData.items || []
        localStorage.removeItem('ecom_pending_task')
        return
      }
      currentTask.value = { task_no, status: taskData.status, progress: taskData.progress || 0 }
      outputs.value = taskData.outputs || []
      if (taskData.items?.length) {
        items.value = taskData.items
      } else if (image_type) {
        const configStore = useEcomConfigStore()
        const allTypes = [...configStore.mainImageTypes, ...configStore.detailPageTypes]
        items.value = image_type.split(',').filter(Boolean).map(t => ({
          image_type: t,
          label: allTypes.find(x => x.value === t)?.label || t,
          status: 'pending',
          phase: 'pending',
          progress: 0,
          url: null,
        }))
      }
      startPolling()
    } catch (_) {
      localStorage.removeItem('ecom_pending_task')
    }
  }

  return { currentTask, outputs, items, isRunning, isDone, submitTask, startPolling, stopPolling, reset, resumeIfPending }
})

export const useEcomGalleryStore = defineStore('ecomGallery', () => {
  const items = ref([])
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(20)
  const moduleFilter = ref('')
  const loading = ref(false)

  const fetchGallery = async () => {
    loading.value = true
    try {
      const params = { page: page.value, page_size: pageSize.value }
      if (moduleFilter.value) params.module = moduleFilter.value
      const res = await httpGet('/api/ai-commerce/gallery', params)
      items.value = res.data.items
      total.value = res.data.total
    } finally {
      loading.value = false
    }
  }

  const deleteTask = async (taskNo) => {
    await axios.delete(`/api/ai-commerce/tasks/${taskNo}`)
    items.value = items.value.filter((t) => t.task_no !== taskNo)
    total.value = Math.max(0, total.value - 1)
  }

  return { items, total, page, pageSize, moduleFilter, loading, fetchGallery, deleteTask }
})
