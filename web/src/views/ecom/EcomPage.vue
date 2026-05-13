<template>
  <EcomLayout>
    <template #default="{ activeModule }">
      <MainImagePage v-if="activeModule === 'main_image'" />
      <DetailPagePage v-else-if="activeModule === 'detail_page'" />
      <WhiteBgPage v-else-if="activeModule === 'white_bg'" />
      <ClonePage v-else-if="activeModule === 'clone'" />
      <RatioConvertPage v-else-if="activeModule === 'ratio_convert'" />
      <TranslatePage v-else-if="activeModule === 'translate'" />
      <GalleryPage v-else-if="activeModule === 'gallery'" />
    </template>
  </EcomLayout>

  <!-- 积分不足引导弹窗 (7.18) -->
  <el-dialog v-model="showRechargeDialog" title="算力不足" width="360px" :close-on-click-modal="false">
    <div style="text-align:center;padding:16px 0">
      <p style="margin-bottom:16px;color:var(--text-secondary)">当前算力余额不足，请充值后继续使用</p>
      <el-button type="primary" @click="goRecharge">立即充值</el-button>
      <el-button @click="showRechargeDialog = false" style="margin-left:8px">稍后再说</el-button>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, provide, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useEcomConfigStore } from '@/store/ecom'
import EcomLayout from './EcomLayout.vue'
import MainImagePage from './MainImagePage.vue'
import DetailPagePage from './DetailPagePage.vue'
import WhiteBgPage from './WhiteBgPage.vue'
import ClonePage from './ClonePage.vue'
import RatioConvertPage from './RatioConvertPage.vue'
import TranslatePage from './TranslatePage.vue'
import GalleryPage from './GalleryPage.vue'

const configStore = useEcomConfigStore()
const router = useRouter()
const showRechargeDialog = ref(false)

provide('showRechargeDialog', showRechargeDialog)

const goRecharge = () => {
  showRechargeDialog.value = false
  router.push('/member')
}

onMounted(async () => {
  await configStore.loadUserPower()
})
</script>

