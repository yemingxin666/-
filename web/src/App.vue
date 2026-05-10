<template>
  <el-config-provider>
    <router-view />
  </el-config-provider>
</template>

<script setup>
import { checkSession, getSystemInfo } from '@/store/cache'
import { useSharedStore } from '@/store/sharedata'
import { ElConfigProvider } from 'element-plus'
import { onMounted } from 'vue'

const debounce = (fn, delay) => {
  let timer
  return (...args) => {
    if (timer) {
      clearTimeout(timer)
    }
    timer = setTimeout(() => {
      fn(...args)
    }, delay)
  }
}

const _ResizeObserver = window.ResizeObserver
window.ResizeObserver = class ResizeObserver extends _ResizeObserver {
  constructor(callback) {
    callback = debounce(callback, 200)
    super(callback)
  }
}

const store = useSharedStore()
onMounted(() => {
  // 获取系统参数
  getSystemInfo().then((res) => {
    const link = document.createElement('link')
    link.rel = 'shortcut icon'
    link.href = res.data.logo
    document.head.appendChild(link)
  })
  checkSession()
    .then(() => {
      store.setIsLogin(true)
    })
    .catch(() => {})

  // 设置主题
  document.documentElement.setAttribute('data-theme', store.theme)
})
</script>

<style lang="scss">
html,
body {
  margin: 0;
  padding: 0;
}

#app {
  margin: 0 !important;
  padding: 0 !important;
  font-family: Helvetica Neue, Helvetica, PingFang SC, Hiragino Sans GB, Microsoft YaHei, Arial,
    sans-serif;
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;

  // --primary-color: #21aa93

  h1 {
    font-size: 2em;
  } /* 通常是 2em */
  h2 {
    font-size: 1.5em;
  } /* 通常是 1.5em */
  h3 {
    font-size: 1.17em;
  } /* 通常是 1.17em */
  h4 {
    font-size: 1em;
  } /* 通常是 1em */
  h5 {
    font-size: 0.83em;
  } /* 通常是 0.83em */
  h6 {
    font-size: 0.67em;
  } /* 通常是 0.67em */
}

.el-overlay-dialog {
  display: flex;
  justify-content: center;
  align-items: center;

  .el-dialog {
    margin: 0;

    .el-dialog__body {
      overflow-y: auto;
      max-height: calc(100vh - 100px);
    }
  }
}

.el-popper.is-customized {
  /* 设置内边距以保证高度为32px */
  padding: 6px 12px;
  background: linear-gradient(180deg, #e1bee7, #7e57c2);
  color: #fff;
}

.el-popper.is-customized .el-popper__arrow::before {
  background: linear-gradient(180deg, #b39ddb, #7e57c2);
  right: 0;
}

/* 省略显示 */
.ellipsis {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.van-toast--fail {
  background: #fef0f0;
  color: #f56c6c;
}

.van-toast--success {
  background: #d6fbcc;
  color: #07c160;
}
</style>
