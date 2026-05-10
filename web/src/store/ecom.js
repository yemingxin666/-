import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { httpGet, httpPost, httpDelete } from '@/utils/http'
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


  const generateCopywriting = async (productName, hint, assetNos, imageType) => {
    const res = await httpPost('/api/ai-commerce/copywrite', {
      product_name: productName,
      hint: hint,
      reference_assets: (assetNos || []).slice(0, 3),
      image_type: imageType || ''
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
  const submittedRatio = ref('1:1')
  // 会话级历史：未刷新浏览器时保留已完成任务的图片
  const history = ref([])
  let pollTimer = null

  // 模块中文名映射（与后端 image_service.go moduleLabel 保持一致）
  const MODULE_LABELS = {
    main_image: '主图设计', detail_page: '详情页', white_bg: '白底图',
    clone: '克隆设计', ratio_convert: '比例转换', translate: '图文翻译', edit: '图片编辑',
  }
  const moduleLabel = (m) => MODULE_LABELS[m] || m || '任务'

  // 合并轮询返回的 items：以 image_type 为键 in-place 更新已有项，
  // 保护已 succeeded 项不被回退（防止后端临时丢字段导致前端闪烁）
  const mergeItems = (oldItems, newItems) => {
    if (!oldItems?.length) return newItems
    const map = new Map(oldItems.map(i => [i.image_type, i]))
    return newItems.map(n => {
      const old = map.get(n.image_type)
      if (!old) return n
      // 已 succeeded 且有 url 的项：保留旧 url/status，仅同步无关字段
      if (old.status === 'succeeded' && old.url) {
        return { ...old, ...n, url: old.url, status: 'succeeded' }
      }
      // 其他情况：in-place 合并到旧对象，保持引用稳定
      Object.assign(old, n)
      return old
    })
  }

  // 把当前任务的结果快照存入历史（去重 by task_no）
  const archiveCurrent = () => {
    const taskNo = currentTask.value?.task_no
    if (!taskNo) return
    const hasImage = (items.value || []).some(i => i.url) || (outputs.value || []).length > 0
    if (!hasImage) return
    if (history.value.some(h => h.task_no === taskNo)) return
    history.value.unshift({
      task_no: taskNo,
      items: JSON.parse(JSON.stringify(items.value || [])),
      outputs: [...(outputs.value || [])],
      ratio: submittedRatio.value,
      ts: Date.now(),
    })
  }

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
    // 提交新任务前，把当前未归档的成果先存入历史
    archiveCurrent()
    const creditCost = res.data.credit_cost || 0
    const taskNo = res.data.task_no
    currentTask.value = { task_no: taskNo, status: res.data.status, progress: 0, credit_cost: creditCost }
    outputs.value = []
    submittedRatio.value = data.ratio || '1:1'

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
    } else if (Array.isArray(data.reference_assets) && data.reference_assets.length) {
      // white_bg / clone / ratio_convert 等无 image_type 的模块：
      // 按参考图张数预填占位，避免提交后 3 秒轮询窗口内只显示 1 张兜底 loading，
      // 然后第一次拿到后端 items 时突然从 1 变成 N。
      items.value = data.reference_assets.map((_, i) => ({
        image_type: `${data.module || 'task'}_${i}`,
        label: `${moduleLabel(data.module)} ${i + 1}`,
        status: 'pending',
        phase: 'pending',
        progress: 0,
        url: null,
        asset_no: '',
      }))
    } else {
      items.value = []
    }

    localStorage.setItem('ecom_pending_task', JSON.stringify({
      task_no: taskNo,
      module: data.module || '',
      image_type: data.image_type || '',
      ratio: data.ratio || '1:1',
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
          // 合并而非整体替换 items：以 image_type 为键，保护已 succeeded 项不被回退
          // 同时保留对象引用稳定性，避免每次轮询都让 ResultCard 闪烁
          items.value = mergeItems(items.value, res.data.items || [])
          outputs.value = res.data.outputs || []
          if (res.data.status === 'succeeded') {
            archiveCurrent()
            currentTask.value = null
            outputs.value = []
            items.value = []
            localStorage.removeItem('ecom_pending_task')
            _stopPolling()
          } else if (res.data.status === 'failed') {
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

  // 历史操作（仅作用于会话内存中的历史结果，不动当前任务）
  // 最近一次删除快照，用于撤销恢复（仅保留最后一次，新删除会覆盖旧快照）
  const lastDeleteAction = ref(null)

  // 从指定 batch 删除单张图。key 为 image_type 字符串（items 模式）或数字 index（outputs 模式）。
  // 删除前快照位置信息；若 batch 删空，自动整组移除并记录 batch 索引以便撤销时整体恢复。
  const deleteHistoryItem = (taskNo, key, isOutput = false) => {
    const batchIdx = history.value.findIndex(h => h.task_no === taskNo)
    if (batchIdx < 0) return
    const batch = history.value[batchIdx]
    let removedItem = null
    let removedIndex = -1
    if (isOutput) {
      removedIndex = typeof key === 'number' ? key : Number(key)
      if (removedIndex < 0 || removedIndex >= (batch.outputs || []).length) return
      removedItem = batch.outputs[removedIndex]
      batch.outputs.splice(removedIndex, 1)
    } else {
      removedIndex = (batch.items || []).findIndex(i => i.image_type === key)
      if (removedIndex < 0) return
      removedItem = batch.items[removedIndex]
      batch.items.splice(removedIndex, 1)
    }
    // batch 删空：整体移除并记入快照（用于撤销恢复）
    let removedBatch = null
    let removedBatchIndex = -1
    const remaining = (batch.items?.length || 0) + (batch.outputs?.length || 0)
    if (remaining === 0) {
      removedBatch = batch
      removedBatchIndex = batchIdx
      history.value.splice(batchIdx, 1)
    }
    lastDeleteAction.value = {
      type: 'item',
      ts: Date.now(),
      data: { taskNo, isOutput, removedItem, removedIndex, removedBatch, removedBatchIndex },
    }
  }

  // 整组移除：先快照 batch 与原索引
  const removeHistory = (taskNo) => {
    const idx = history.value.findIndex(h => h.task_no === taskNo)
    if (idx < 0) return
    const batch = history.value[idx]
    history.value.splice(idx, 1)
    lastDeleteAction.value = {
      type: 'batch',
      ts: Date.now(),
      data: { batch, index: idx },
    }
  }

  // 清空全部：先快照
  const clearHistory = () => {
    if (!history.value.length) return
    const snapshot = history.value.slice()
    history.value = []
    lastDeleteAction.value = {
      type: 'all',
      ts: Date.now(),
      data: { batches: snapshot },
    }
  }

  // 撤销最近一次删除；超时（>10s）的快照视为已失效
  const undoLastDelete = () => {
    const action = lastDeleteAction.value
    if (!action) return false
    if (Date.now() - action.ts > 10000) { lastDeleteAction.value = null; return false }
    if (action.type === 'item') {
      const { taskNo, isOutput, removedItem, removedIndex, removedBatch, removedBatchIndex } = action.data
      // 若 batch 已被整体移除，先恢复 batch 容器
      if (removedBatch && removedBatchIndex >= 0) {
        history.value.splice(removedBatchIndex, 0, removedBatch)
      }
      const batch = history.value.find(h => h.task_no === taskNo)
      if (batch) {
        if (isOutput) {
          batch.outputs.splice(removedIndex, 0, removedItem)
        } else {
          batch.items.splice(removedIndex, 0, removedItem)
        }
      }
    } else if (action.type === 'batch') {
      const { batch, index } = action.data
      history.value.splice(Math.min(index, history.value.length), 0, batch)
    } else if (action.type === 'all') {
      history.value = action.data.batches.slice()
    }
    lastDeleteAction.value = null
    return true
  }

  const resumeIfPending = async () => {
    const raw = localStorage.getItem('ecom_pending_task')
    if (!raw) return
    try {
      const { task_no, image_type, ratio } = JSON.parse(raw)
      const res = await httpGet(`/api/ai-commerce/tasks/${task_no}`)
      if (res.code !== 0) { localStorage.removeItem('ecom_pending_task'); return }
      const taskData = res.data
      if (taskData.status === 'succeeded' || taskData.status === 'failed') {
        // 页面关闭期间任务已完成：恢复当前结果（成功的会在下次提交时归档进 history）
        currentTask.value = { task_no, status: taskData.status, progress: taskData.progress || 100 }
        submittedRatio.value = ratio || '1:1'
        outputs.value = taskData.outputs || []
        items.value = taskData.items || []
        if (taskData.status === 'succeeded') {
          archiveCurrent()
          currentTask.value = null
          outputs.value = []
          items.value = []
        } else {
          currentTask.value = null
        }
        localStorage.removeItem('ecom_pending_task')
        return
      }
      currentTask.value = { task_no, status: taskData.status, progress: taskData.progress || 0 }
      submittedRatio.value = ratio || '1:1'
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

  return { currentTask, outputs, items, history, isRunning, isDone, submittedRatio, submitTask, startPolling, stopPolling, reset, resumeIfPending, removeHistory, clearHistory, deleteHistoryItem, undoLastDelete, lastDeleteAction }
})

export const useEcomGalleryStore = defineStore('ecomGallery', () => {
  const items = ref([])
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(20)
  const moduleFilter = ref('')
  const loading = ref(false)

  // silent: 轮询时使用，不切换 loading 态，避免列表闪烁
  const fetchGallery = async (opts = {}) => {
    const silent = opts && opts.silent
    if (!silent) loading.value = true
    try {
      const params = { page: page.value, page_size: pageSize.value }
      if (moduleFilter.value) params.module = moduleFilter.value
      const res = await httpGet('/api/ai-commerce/gallery', params)
      items.value = res.data.items
      total.value = res.data.total
    } finally {
      if (!silent) loading.value = false
    }
  }

  const deleteTask = async (taskNo) => {
    try {
      await httpDelete(`/api/ai-commerce/tasks/${taskNo}`)
      items.value = items.value.filter((t) => t.task_no !== taskNo)
      total.value = Math.max(0, total.value - 1)
    } catch (e) {
      console.error('[ecom] 删除任务失败:', e)
      throw e
    }
  }

  // deleteAsset 删除单张图；若任务下已无图，后端会级联删任务，此时本地也移除任务
  const deleteAsset = async (taskNo, assetNo) => {
    try {
      await httpDelete(`/api/ai-commerce/assets/${assetNo}`)
      const task = items.value.find((t) => t.task_no === taskNo)
      if (task && task.outputs) {
        task.outputs = task.outputs.filter((o) => o.asset_no !== assetNo)
        if (task.outputs.length === 0) {
          items.value = items.value.filter((t) => t.task_no !== taskNo)
          total.value = Math.max(0, total.value - 1)
        }
      }
    } catch (e) {
      console.error('[ecom] 删除图片失败:', e)
      throw e
    }
  }

  // editTask 基于历史图库某张图 + prompt 编辑生成新图
  // 模型必须由前端传入（用户在 EcomTopNav 选中的模型，保持与生图模块一致）
  const editTask = async (taskNo, assetNo, prompt, modelName) => {
    const res = await httpPost('/api/ai-commerce/edit', {
      task_no: taskNo,
      asset_no: assetNo,
      prompt,
      model: modelName,
    })
    if (res.code !== 0) throw new Error(res.message || '编辑失败')
    // 联动扣减算力，与 useEcomTaskStore.submitTask 保持一致
    useEcomConfigStore().deductPower(res.data?.credit_cost || 0)
    return res.data
  }

  return { items, total, page, pageSize, moduleFilter, loading, fetchGallery, deleteTask, deleteAsset, editTask }
})
