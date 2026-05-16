<template>
  <el-container class="captcha-box">
    <el-dialog
      v-model="show"
      :close-on-click-modal="true"
      :show-close="isMobileInternal"
      :append-to-body="true"
      style="width: 360px; --el-dialog-padding-primary: 5px 15px 15px 15px"
    >
      <template #header>
        <div class="text-center p-3" style="color: var(--el-text-color-primary)">
          <span>人机验证</span>
        </div>
      </template>
      <slide-captcha
        :bg-img="bgImg"
        :bk-img="bkImg"
        :result="result"
        @refresh="getSlideCaptcha"
        @confirm="handleSlideConfirm"
        @hide="show = false"
      />
    </el-dialog>
  </el-container>
</template>

<script setup>
import SlideCaptcha from '@/components/SlideCaptcha.vue'
import { showMessageError } from '@/utils/dialog'
import { httpGet, httpPost } from '@/utils/http'
import { isMobile } from '@/utils/libs'
import { ref } from 'vue'

const show = ref(false)
const captKey = ref('')
const isMobileInternal = isMobile()

const emits = defineEmits(['success'])

const loadCaptcha = () => {
  show.value = true
  getSlideCaptcha()
}

const bgImg = ref('')
const bkImg = ref('')
const result = ref(0)

const getSlideCaptcha = () => {
  result.value = 0
  httpGet('/api/captcha/slide/get')
    .then((res) => {
      bkImg.value = res.data.bkImg
      bgImg.value = res.data.bgImg
      captKey.value = res.data.key
    })
    .catch((e) => {
      showMessageError('获取人机验证数据失败：' + e.message)
    })
}

const handleSlideConfirm = (x) => {
  httpPost('/api/captcha/slide/check', {
    key: captKey.value,
    x: x,
  })
    .then(() => {
      result.value = 1
      show.value = false
      emits('success', { key: captKey.value, x: x })
    })
    .catch(() => {
      result.value = 2
    })
}

defineExpose({
  loadCaptcha,
})
</script>

<style lang="scss">
.captcha-box {
  .el-dialog {
    .el-dialog__header {
      padding: 0;
    }

    .el-dialog__body {
      padding-bottom: 5px;
      overflow: hidden;
    }
  }
}
</style>
