import { ref, onUnmounted } from 'vue'

export function useCopywriteProgress() {
  const percentage = ref(0)
  const showProgress = ref(false)
  let timer = null

  const start = () => {
    reset()
    showProgress.value = true
    timer = setInterval(() => {
      if (percentage.value < 30) {
        percentage.value += 5
      } else if (percentage.value < 90) {
        // 30->90% 减速衰减
        const increment = (90 - percentage.value) * 0.1
        percentage.value = Math.min(89.9, percentage.value + increment)
      }
    }, 200)
  }

  const finish = () => {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
    percentage.value = 100
    // 300ms 后开始淡出
    setTimeout(() => {
      showProgress.value = false
      // 等待淡出动画结束后重置进度
      setTimeout(() => {
        percentage.value = 0
      }, 300)
    }, 300)
  }

  const reset = () => {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
    percentage.value = 0
    showProgress.value = false
  }

  onUnmounted(() => {
    reset()
  })

  return {
    percentage,
    showProgress,
    start,
    finish,
    reset
  }
}
