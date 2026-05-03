import { computed, watch } from 'vue'
import { ElNotification } from 'element-plus'
import { useEcomConfigStore } from '@/store/ecom'

export function useEcomLinkage(form) {
  const configStore = useEcomConfigStore()

  const applyPlatformConfig = (platform, showNotification = true) => {
    const cfg = configStore.getPlatformConfig(platform)
    if (!cfg) return
    if (cfg.default_language) form.value.language = cfg.default_language
    if (cfg.default_ratio) form.value.ratio = cfg.default_ratio
    if (!showNotification) return
    const constraints = cfg.constraints || {}
    if (constraints.no_text_overlay) {
      ElNotification({
        title: '平台限制提示',
        message: `${cfg.label} 平台建议不要有文字覆盖，已为您优化提示词。`,
        type: 'warning',
        duration: 5000,
      })
    }
    if (constraints.force_white_bg) {
      ElNotification({
        title: '平台限制提示',
        message: `${cfg.label} 平台强制要求白底图，已为您自动开启。`,
        type: 'warning',
        duration: 5000,
      })
    }
  }

  // 用户主动切换平台
  watch(() => form.value.platform, (newVal) => applyPlatformConfig(newVal, true))

  // 配置首次加载完成后，补填当前选中平台的默认值（静默，不弹通知）
  watch(() => configStore.platformConfigs.size, (size) => {
    if (size > 0) applyPlatformConfig(form.value.platform, false)
  }, { once: true })

  const recommendedRatio = computed(() => {
    const cfg = configStore.getPlatformConfig(form.value.platform)
    return cfg?.default_ratio || ''
  })

  return { recommendedRatio }
}
