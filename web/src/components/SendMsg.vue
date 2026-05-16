<template>
  <el-container class="send-verify-code">
    <el-button
      type="success"
      :size="props.size"
      :disabled="!canSend"
      :loading="loading"
      @click="sendMsg"
    >
      {{ btnText }}
    </el-button>

    <captcha @success="doSendMsg" ref="captchaRef" />
  </el-container>
</template>

<script setup>
import Captcha from '@/components/Captcha.vue'
import { httpGet, httpPost } from '@/utils/http'
import { validateEmail, validateMobile } from '@/utils/validate'
import { ElMessage } from 'element-plus'
import { computed, onMounted, onUnmounted, ref } from 'vue'

const props = defineProps({
  receiver: String,
  scene: { type: String, required: true },
  size: String,
  type: {
    type: String,
    default: 'mobile',
  },
})

const btnText = ref('发送验证码')
const canSend = ref(true)
const loading = ref(false)
const captchaRef = ref(null)
const enableCaptcha = ref(false)
let timer = null

const storageKey = computed(() => `sms_cd_${props.scene}_${props.receiver}`)

httpGet('/api/captcha/config').then((res) => {
  enableCaptcha.value = res.data['enabled']
})

const tickCooldown = () => {
  const expiry = Number(localStorage.getItem(storageKey.value) || 0)
  const remaining = Math.ceil((expiry - Date.now()) / 1000)
  if (remaining <= 0) {
    localStorage.removeItem(storageKey.value)
    canSend.value = true
    btnText.value = '重新发送'
    if (timer) {
      clearInterval(timer)
      timer = null
    }
    return
  }
  canSend.value = false
  btnText.value = `${remaining}s`
}

const startCooldown = (seconds = 60) => {
  const expiry = Date.now() + seconds * 1000
  localStorage.setItem(storageKey.value, String(expiry))
  canSend.value = false
  btnText.value = `${seconds}s`
  if (timer) clearInterval(timer)
  timer = setInterval(tickCooldown, 1000)
}

onMounted(() => {
  tickCooldown()
  timer = setInterval(tickCooldown, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const sendMsg = () => {
  if (!validateMobile(props.receiver) && props.type === 'mobile') {
    return ElMessage.error('请输入合法的手机号')
  }
  if (!validateEmail(props.receiver) && props.type === 'email') {
    return ElMessage.error('请输入合法的邮箱地址')
  }

  if (enableCaptcha.value) {
    captchaRef.value.loadCaptcha()
  } else {
    doSendMsg({})
  }
}

const doSendMsg = (data) => {
  if (!canSend.value) {
    return
  }

  canSend.value = false
  loading.value = true
  httpPost('/api/sms/code', {
    receiver: props.receiver,
    scene: props.scene,
    key: data.key,
    x: data.x,
  })
    .then(() => {
      if (props.type === 'mobile') {
        ElMessage.success('验证码发送成功')
      } else if (props.type === 'email') {
        ElMessage.success('验证码已发送至邮箱，如果长时间未收到，请检查是否在垃圾邮件中！')
      }
      startCooldown(60)
    })
    .catch((e) => {
      if (e.response?.status === 429) {
        ElMessage.warning('发送过于频繁，请稍后再试')
        startCooldown(60)
      } else {
        canSend.value = true
        ElMessage.error('验证码发送失败，请稍后重试')
      }
    })
    .finally(() => {
      loading.value = false
    })
}
</script>

<style lang="scss" scoped>
.send-verify-code {
  .send-btn {
    width: 100%;
  }
}
</style>
