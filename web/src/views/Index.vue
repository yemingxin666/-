<template>
  <div class="index-page">
    <!-- 主题切换 -->
    <ThemeChange />
    <div class="menu-box">
      <el-menu mode="horizontal" :ellipsis="false">
        <div class="menu-item">
          <img :src="logo" class="logo" alt="韩絮服饰" />
        </div>
        <div class="menu-item">
          <span v-if="!isLogin">
            <el-button
              @click="router.push('/login')"
              class="btn-go animate__animated animate__pulse animate__infinite"
              round
              >登录/注册</el-button
            >
          </span>
          <span v-if="isLogin">
            <el-button
              @click="logout"
              class="btn-go animate__animated animate__pulse animate__infinite"
              round
            >
              退出登录
            </el-button>
          </span>
        </div>
      </el-menu>
    </div>
    <div class="content">
      <h1 class="animate__animated animate__fadeInDown">
        {{ title }}
      </h1>
      <p class="subtitle animate__animated animate__fadeInUp animate__delay-1s" v-if="title">
        开启您的 AI 智能创作之旅
      </p>

      <div class="navs animate__animated animate__fadeInUp animate__delay-1s">
        <el-space wrap :size="24">
          <div
            v-for="item in navs"
            :key="item.url"
            class="nav-item-box"
            @click="router.push(item.url)"
          >
            <i :class="'iconfont mb-2 ' + item.icon" v-if="item.icon.startsWith('icon')"></i>
            <el-image :src="item.icon" class="rounded-lg w-10 h-10 mb-2" alt="韩絮服饰" v-else />
            <div>{{ item.name }}</div>
          </div>
        </el-space>
      </div>
    </div>

    <footer-bar />

    <!-- 网站公告对话框 -->
    <el-dialog v-model="showNotice" :show-close="true" class="notice-dialog" title="网站公告">
      <div class="notice">
        <div v-html="notice"></div>
      </div>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="notShow" type="primary">我知道了，不再显示</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import FooterBar from '@/components/FooterBar.vue'
import ThemeChange from '@/components/ThemeChange.vue'
import { checkSession, getSystemInfo } from '@/store/cache'
import { removeUserToken } from '@/store/session'
import { httpGet } from '@/utils/http'
import { isMobile } from '@/utils/libs'
import { ElMessage } from 'element-plus'
import MarkdownIt from 'markdown-it'
import emoji from 'markdown-it-emoji'
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

const title = ref('')
const logo = ref('')
const isLogin = ref(false)
const docsURL = ref(import.meta.env.VITE_DOCS_URL)
const githubURL = ref(import.meta.env.VITE_GITHUB_URL)
const giteeURL = ref(import.meta.env.VITE_GITEE_URL)
const navs = ref([])

// 公告相关变量
const showNotice = ref(false)
const notice = ref('')
const noticeKey = ref('SYSTEM_NOTICE')

// Markdown 解析器
const md = new MarkdownIt({
  breaks: true,
  html: true,
  linkify: true,
  typographer: true,
}).use(emoji)

onMounted(() => {
  if (isMobile()) {
    router.push('/mobile/index')
    return
  }
  getSystemInfo()
    .then((res) => {
      title.value = res.data.title
      logo.value = res.data.logo
    })
    .catch((e) => {
      ElMessage.error('获取系统配置失败：' + e.message)
    })

  httpGet('/api/menu/list?index=1')
    .then((res) => {
      navs.value = res.data
    })
    .catch((e) => {
      ElMessage.error('获取导航菜单失败：' + e.message)
    })

  checkSession()
    .then(() => {
      isLogin.value = true
    })
    .catch(() => {})

  // 获取系统公告
  httpGet('/api/config/get?key=notice')
    .then((res) => {
      try {
        notice.value = md.render(res.data['content'])
        const oldNotice = localStorage.getItem(noticeKey.value)
        // 如果公告有更新，则显示公告
        if (oldNotice !== notice.value && notice.value.length > 10) {
          showNotice.value = true
        }
      } catch (e) {
        console.warn(e)
      }
    })
    .catch((e) => {
      ElMessage.error('获取系统配置失败：' + e.message)
    })
})

const logout = () => {
  removeUserToken()
  router.push('/login')
}

// 不再显示公告
const notShow = () => {
  localStorage.setItem(noticeKey.value, notice.value)
  showNotice.value = false
}
</script>

<style lang="scss" scoped>
@use '../assets/css/index.scss' as *;
</style>

<style lang="scss">
.notice-dialog {
  .el-dialog__header {
    padding-bottom: 0;
  }

  .el-dialog__body {
    padding: 0 20px;

    h2 {
      margin: 20px 0 15px 0;
    }

    ol,
    ul {
      padding-left: 10px;
    }

    ol {
      list-style: decimal-leading-zero;
      padding-left: 20px;
    }

    ul {
      list-style: inside;
    }
  }
}
</style>
