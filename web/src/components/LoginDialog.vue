<template>
  <div class="login-dialog w-full">
    <!-- 登录卡片 -->
    <div class="login-box" v-if="viewMode === 'login'">
      <custom-tabs v-model="loginActiveName" @tab-click="handleTabClick">
        <!-- 账号密码登录 -->
        <custom-tab-pane name="account" width="48">
          <template #label>
            <div class="flex items-center justify-center px-3">
              <i class="iconfont icon-user-fill mr-2"></i>
              <span>账号登录</span>
            </div>
          </template>
          <el-form :model="data" class="form space-y-5">
            <div class="block">
              <el-input placeholder="账号" size="large" v-model="data.username" autocomplete="off">
                <template #prefix>
                  <el-icon>
                    <Iphone />
                  </el-icon>
                </template>
              </el-input>
            </div>

            <div class="block">
              <el-input
                placeholder="请输入密码(8-16位)"
                maxlength="16"
                size="large"
                v-model="data.password"
                show-password
                autocomplete="off"
              >
                <template #prefix>
                  <el-icon>
                    <Lock />
                  </el-icon>
                </template>
              </el-input>
            </div>

            <el-row class="btn-row mt-8" :gutter="20">
              <el-col :span="24">
                <button
                  class="w-full h-12 rounded-xl text-base font-semibold text-white bg-gradient-to-r from-indigo-500 to-purple-600 hover:from-indigo-600 hover:to-purple-700 transition-all duration-300 hover:-translate-y-0.5 hover:shadow-lg active:translate-y-0 shadow-md flex items-center justify-center"
                  @click="submitLogin"
                  type="button"
                >
                  {{ loading ? '登录中...' : '立即登录' }}
                </button>
              </el-col>
            </el-row>

            <div class="w-full">
              <div
                class="text flex justify-center items-center pt-3 text-sm"
                style="color: var(--login-text-color)"
              >
                还没有账号？
                <el-button
                  size="small"
                  class="ml-2 rounded-md px-2 py-1 transition-colors duration-200"
                  style="color: var(--login-link-color)"
                  @click="viewMode = 'register'"
                  @mouseenter="$event.target.style.background = 'var(--login-link-hover-bg)'"
                  @mouseleave="$event.target.style.background = 'transparent'"
                  >注册</el-button
                >

                <el-button
                  type="info"
                  class="forget ml-4"
                  size="small"
                  @click="viewMode = 'reset'"
                  >忘记密码？</el-button
                >
              </div>
            </div>
          </el-form>
        </custom-tab-pane>

        <!-- 微信登录 -->
        <custom-tab-pane name="wechat" width="48">
          <template #label>
            <div class="flex items-center justify-center px-3">
              <i class="iconfont icon-wechat mr-2"></i>
              <span>微信登录</span>
            </div>
          </template>
          <div class="wechat-login pt-3">
            <div class="qr-code-container">
              <div class="qr-code-wrapper w-[200px] h-[200px] mx-auto" v-loading="qrcodeLoading">
                <img :src="wechatLoginQRCode" class="qr-frame" v-if="wechatLoginQRCode" />
                <div
                  v-else
                  class="w-[200px] h-[200px] flex justify-center items-center text-green-600"
                >
                  <i class="iconfont icon-wechat !text-3xl"></i>
                </div>
                <!-- 二维码过期蒙版 -->
                <div v-if="qrcodeExpired" class="qr-expired-mask">
                  <div class="expired-content">
                    <i class="iconfont icon-refresh-ccw expired-icon"></i>
                    <p class="expired-text">二维码已过期</p>
                    <button
                      @click="getWxLoginURL"
                      class="bg-gray-200 text-gray-600 px-2.5 py-1 rounded-md hover:bg-gray-300"
                    >
                      <i class="iconfont icon-refresh text-lg"></i>
                    </button>
                  </div>
                </div>
              </div>
              <p class="text-center mt-4 text-gray-600 dark:text-gray-400">
                请使用微信扫描二维码登录
              </p>
            </div>
          </div>
        </custom-tab-pane>
      </custom-tabs>
    </div>

    <!-- 注册卡片 -->
    <div class="register-box w-full" v-else-if="viewMode === 'register'">
      <el-form :model="data" class="form space-y-5" v-if="enableRegister">
        <el-tabs v-model="activeName" class="demo-tabs dark:text-white">
          <el-tab-pane label="手机注册" name="mobile" v-if="enableMobile">
            <div class="block">
              <el-input
                placeholder="手机号码"
                size="large"
                v-model="data.mobile"
                maxlength="11"
                autocomplete="off"
              >
                <template #prefix>
                  <el-icon>
                    <Iphone />
                  </el-icon>
                </template>
              </el-input>
            </div>
            <div class="block mt-4">
              <el-row :gutter="10">
                <el-col :span="12">
                  <el-input
                    placeholder="验证码"
                    size="large"
                    maxlength="30"
                    v-model="data.code"
                    autocomplete="off"
                  >
                    <template #prefix>
                      <el-icon>
                        <Checked />
                      </el-icon>
                    </template>
                  </el-input>
                </el-col>
                <el-col :span="12">
                  <send-msg size="large" :receiver="data.mobile" type="mobile" scene="register" />
                </el-col>
              </el-row>
            </div>
          </el-tab-pane>
          <el-tab-pane label="邮箱注册" name="email" v-if="enableEmail">
            <div class="block">
              <el-input placeholder="邮箱地址" size="large" v-model="data.email" autocomplete="off">
                <template #prefix>
                  <el-icon>
                    <Message />
                  </el-icon>
                </template>
              </el-input>
            </div>
            <div class="block mt-4">
              <el-row :gutter="10">
                <el-col :span="12">
                  <el-input
                    placeholder="验证码"
                    size="large"
                    maxlength="30"
                    v-model="data.code"
                    autocomplete="off"
                  >
                    <template #prefix>
                      <el-icon>
                        <Checked />
                      </el-icon>
                    </template>
                  </el-input>
                </el-col>
                <el-col :span="12">
                  <send-msg size="large" :receiver="data.email" type="email" scene="register" />
                </el-col>
              </el-row>
            </div>
          </el-tab-pane>
          <el-tab-pane label="用户名注册" name="username" v-if="enableUser">
            <div class="block">
              <el-input
                placeholder="用户名"
                size="large"
                v-model="data.username"
                autocomplete="off"
              >
                <template #prefix>
                  <el-icon>
                    <Iphone />
                  </el-icon>
                </template>
              </el-input>
            </div>
          </el-tab-pane>
        </el-tabs>

        <div class="block">
          <el-input
            placeholder="请输入密码(8-16位)"
            maxlength="16"
            size="large"
            v-model="data.password"
            show-password
            autocomplete="off"
          >
            <template #prefix>
              <el-icon>
                <Lock />
              </el-icon>
            </template>
          </el-input>
        </div>

        <div class="block">
          <el-input
            placeholder="重复密码(8-16位)"
            size="large"
            maxlength="16"
            v-model="data.repass"
            show-password
            autocomplete="off"
          >
            <template #prefix>
              <el-icon>
                <Lock />
              </el-icon>
            </template>
          </el-input>
        </div>

        <div class="block">
          <el-input
            placeholder="邀请码(可选)"
            size="large"
            v-model="data.invite_code"
            autocomplete="off"
          >
            <template #prefix>
              <el-icon>
                <Message />
              </el-icon>
            </template>
          </el-input>
        </div>

        <div class="block text-sm">
          <el-checkbox v-model="agreeChecked">
            我已阅读并同意
            <a href="javascript:void(0)" class="text-blue-500" @click="openAgreement"
              >《用户协议》</a
            >
            和
            <a href="javascript:void(0)" class="text-blue-500" @click="openPrivacy">《隐私政策》</a>
          </el-checkbox>
        </div>

        <div class="w-full">
          <button
            class="w-full h-12 rounded-xl text-base font-semibold text-white bg-gradient-to-r from-indigo-500 to-purple-600 hover:from-indigo-600 hover:to-purple-700 transition-all duration-300 hover:-translate-y-0.5 hover:shadow-lg active:translate-y-0 shadow-md flex items-center justify-center"
            @click="submitRegister"
            type="button"
          >
            {{ loading ? '注册中...' : '立即注册' }}
          </button>
        </div>

        <div
          class="text text-sm flex justify-center items-center w-full pt-3"
          style="color: var(--login-text-color)"
        >
          已有账号？
          <el-button
            size="small"
            class="ml-2 rounded-md px-2 py-1 transition-colors duration-200"
            style="color: var(--login-link-color)"
            @click="viewMode = 'login'"
            @mouseenter="$event.target.style.background = 'var(--login-link-hover-bg)'"
            @mouseleave="$event.target.style.background = 'transparent'"
            >登录</el-button
          >
        </div>
      </el-form>

      <div class="tip-result" v-else>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-result icon="error" title="注册功能已关闭">
              <template #sub-title>
                <p>抱歉，系统已关闭注册功能，请联系管理员添加账号！</p>
              </template>
            </el-result>
          </el-col>

          <el-col :span="12">
            <div class="wechat-card">
              <el-image :src="wxImg" />
            </div>
          </el-col>
        </el-row>
      </div>
    </div>

    <!-- 忘记密码卡片 -->
    <div class="reset-box w-full" v-else-if="viewMode === 'reset'">
      <el-form :model="resetForm" class="form space-y-5">
        <el-tabs v-model="resetForm.type" class="demo-tabs dark:text-white">
          <el-tab-pane label="手机号验证" name="mobile">
            <div class="block">
              <el-input
                v-model="resetForm.mobile"
                size="large"
                placeholder="手机号码"
                maxlength="11"
                autocomplete="off"
              >
                <template #prefix>
                  <el-icon><Iphone /></el-icon>
                </template>
              </el-input>
            </div>
            <div class="block mt-4">
              <el-row :gutter="10">
                <el-col :span="12">
                  <el-input
                    v-model="resetForm.code"
                    maxlength="6"
                    size="large"
                    placeholder="验证码"
                    autocomplete="off"
                  >
                    <template #prefix>
                      <el-icon><Checked /></el-icon>
                    </template>
                  </el-input>
                </el-col>
                <el-col :span="12">
                  <send-msg size="large" :receiver="resetForm.mobile" type="mobile" scene="reset_pass" />
                </el-col>
              </el-row>
            </div>
          </el-tab-pane>

          <el-tab-pane label="邮箱验证" name="email">
            <div class="block">
              <el-input
                v-model="resetForm.email"
                size="large"
                placeholder="邮箱地址"
                autocomplete="off"
              >
                <template #prefix>
                  <el-icon><Message /></el-icon>
                </template>
              </el-input>
            </div>
            <div class="block mt-4">
              <el-row :gutter="10">
                <el-col :span="12">
                  <el-input
                    v-model="resetForm.code"
                    maxlength="6"
                    size="large"
                    placeholder="验证码"
                    autocomplete="off"
                  >
                    <template #prefix>
                      <el-icon><Checked /></el-icon>
                    </template>
                  </el-input>
                </el-col>
                <el-col :span="12">
                  <send-msg size="large" :receiver="resetForm.email" type="email" scene="reset_pass" />
                </el-col>
              </el-row>
            </div>
          </el-tab-pane>
        </el-tabs>

        <div class="block">
          <el-input
            v-model="resetForm.password"
            type="password"
            placeholder="请输入新密码(8-16位)"
            maxlength="16"
            size="large"
            show-password
            autocomplete="off"
          >
            <template #prefix>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
        </div>

        <div class="block">
          <el-input
            v-model="resetForm.repass"
            type="password"
            placeholder="重复新密码(8-16位)"
            maxlength="16"
            size="large"
            show-password
            autocomplete="off"
          >
            <template #prefix>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
        </div>

        <div class="w-full">
          <button
            class="w-full h-12 rounded-xl text-base font-semibold text-white bg-gradient-to-r from-indigo-500 to-purple-600 hover:from-indigo-600 hover:to-purple-700 transition-all duration-300 hover:-translate-y-0.5 hover:shadow-lg active:translate-y-0 shadow-md flex items-center justify-center"
            @click="submitResetPass"
            type="button"
          >
            {{ resetLoading ? '重置中...' : '重置密码' }}
          </button>
        </div>

        <div
          class="text text-sm flex justify-center items-center w-full pt-3"
          style="color: var(--login-text-color)"
        >
          想起密码了？
          <el-button
            size="small"
            class="ml-2 rounded-md px-2 py-1 transition-colors duration-200"
            style="color: var(--login-link-color)"
            @click="viewMode = 'login'"
            @mouseenter="$event.target.style.background = 'var(--login-link-hover-bg)'"
            @mouseleave="$event.target.style.background = 'transparent'"
          >返回登录</el-button>
        </div>
      </el-form>
    </div>

    <captcha v-if="enableCaptcha" @success="submit" ref="captchaRef" />

    <el-dialog v-model="showAgreement" title="用户协议" :append-to-body="true">
      <div class="prose" v-html="agreementHtml"></div>
    </el-dialog>

    <el-dialog v-model="showPrivacy" title="隐私政策" :append-to-body="true">
      <div class="prose" v-html="privacyHtml"></div>
    </el-dialog>
  </div>
</template>

<script setup>
import Captcha from '@/components/Captcha.vue'
import SendMsg from '@/components/SendMsg.vue'
import CustomTabPane from '@/components/ui/CustomTabPane.vue'
import CustomTabs from '@/components/ui/CustomTabs.vue'
import { getSystemInfo } from '@/store/cache'
import { setUserToken } from '@/store/session'
import { useSharedStore } from '@/store/sharedata'
import { httpGet, httpPost } from '@/utils/http'
import { arrayContains } from '@/utils/libs'
import { validateEmail, validateMobile } from '@/utils/validate'
import { Checked, Iphone, Lock, Message } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { marked } from 'marked'
import QRCode from 'qrcode'
import { onMounted, onUnmounted, ref, watch } from 'vue'

// eslint-disable-next-line no-undef
const props = defineProps({
  show: Boolean,
  active: {
    type: String,
    default: 'login',
  },
  inviteCode: {
    type: String,
    default: '',
  },
})
const showDialog = ref(false)
watch(
  () => props.show,
  (newValue) => {
    showDialog.value = newValue
  }
)

const viewMode = ref(props.active === 'register' ? 'register' : 'login')
const loginActiveName = ref('account')
const data = ref({
  username: import.meta.env.VITE_USER,
  password: import.meta.env.VITE_PASS,
  mobile: '',
  email: '',
  repass: '',
  code: '',
  invite_code: props.inviteCode,
})

// 忘记密码表单（独立数据，避免与登录/注册表单互相干扰）
const resetForm = ref({
  mobile: '',
  email: '',
  type: 'mobile',
  code: '',
  password: '',
  repass: '',
})
const resetLoading = ref(false)

// 微信登录相关变量
const wechatLoginQRCode = ref('')
const wechatLoginState = ref('')
const qrcodeLoading = ref(false)
const pollingTimer = ref(null)
const qrcodeExpired = ref(false)
const qrcodeTimer = ref(null)

const enableMobile = ref(false)
const enableEmail = ref(false)
const enableUser = ref(false)
const enableRegister = ref(true)

const activeName = ref('')
const wxImg = ref('/images/wx.png')
const captchaRef = ref(null)
// eslint-disable-next-line no-undef
const emits = defineEmits(['hide', 'success', 'changeActive'])
const action = ref('login')
const enableCaptcha = ref(false)
const store = useSharedStore()
const loading = ref(false)
const agreeChecked = ref(false)
const showAgreement = ref(false)
const showPrivacy = ref(false)
const agreementHtml = ref('')
const privacyHtml = ref('')

watch(
  () => viewMode.value,
  (newValue) => {
    emits('changeActive', newValue)
  }
)

onMounted(() => {
  getSystemInfo()
    .then((res) => {
      if (res.data) {
        const registerWays = res.data['register_ways']
        if (arrayContains(registerWays, 'username')) {
          enableUser.value = true
          activeName.value = 'username'
        }
        if (arrayContains(registerWays, 'email')) {
          enableEmail.value = true
          activeName.value = 'email'
        }
        if (arrayContains(registerWays, 'mobile')) {
          enableMobile.value = true
          activeName.value = 'mobile'
        }
        // 是否启用注册
        enableRegister.value = res.data['enabled_register']
        // 使用后台上传的客服微信二维码
        if (res.data['wechat_card_url'] !== '') {
          wxImg.value = res.data['wechat_card_url']
        }
      }
    })
    .catch((e) => {
      ElMessage.error('获取系统配置失败：' + e.message)
    })

  httpGet('/api/captcha/config').then((res) => {
    enableCaptcha.value = res.data['enabled']
  })
})

// 监听登录标签页切换
watch(loginActiveName, (newValue) => {
  if (newValue === 'wechat') {
    getWxLoginURL()
  } else {
    if (pollingTimer.value) {
      clearInterval(pollingTimer.value)
    }
    if (qrcodeTimer.value) {
      clearTimeout(qrcodeTimer.value)
    }
  }
})

const handleTabClick = (tab) => {
  if (tab.paneName === 'wechat') {
    getWxLoginURL()
  } else {
    if (pollingTimer.value) {
      clearInterval(pollingTimer.value)
    }
    if (qrcodeTimer.value) {
      clearTimeout(qrcodeTimer.value)
    }
  }
}

const submit = (verifyData) => {
  if (action.value === 'login') {
    doLogin(verifyData)
  } else if (action.value === 'register') {
    doRegister(verifyData)
  }
}

const getWxLoginURL = () => {
  wechatLoginQRCode.value = ''
  qrcodeLoading.value = true
  qrcodeExpired.value = false

  if (qrcodeTimer.value) {
    clearTimeout(qrcodeTimer.value)
  }

  httpGet('/api/user/login/qrcode')
    .then((res) => {
      QRCode.toDataURL(res.data.url, { width: 200, height: 200, margin: 2 }, (error, url) => {
        if (error) {
          console.error(error)
        } else {
          wechatLoginQRCode.value = url
        }
      })
      wechatLoginState.value = res.data.state
      startPolling()

      qrcodeTimer.value = setTimeout(() => {
        qrcodeExpired.value = true
        if (pollingTimer.value) {
          clearInterval(pollingTimer.value)
        }
      }, 60 * 1000)
    })
    .catch((e) => {
      ElMessage.error('获取微信登录 URL 失败，' + e.message)
    })
    .finally(() => {
      qrcodeLoading.value = false
    })
}

const startPolling = () => {
  if (pollingTimer.value) {
    clearInterval(pollingTimer.value)
  }

  pollingTimer.value = setInterval(() => {
    checkLoginStatus()
  }, 1000)
}

const checkLoginStatus = () => {
  if (!wechatLoginState.value) return

  httpGet(`/api/user/login/status?state=${wechatLoginState.value}`)
    .then((res) => {
      const status = res.data.status

      switch (status) {
        case 'success':
          clearInterval(pollingTimer.value)
          clearTimeout(qrcodeTimer.value)
          setUserToken(res.data.token)
          store.setIsLogin(true)
          ElMessage.success('登录成功！')
          emits('hide')
          emits('success')
          break

        case 'expired':
          clearInterval(pollingTimer.value)
          clearTimeout(qrcodeTimer.value)
          qrcodeExpired.value = true
          break

        case 'pending':
          break

        default:
          clearInterval(pollingTimer.value)
          clearTimeout(qrcodeTimer.value)
          ElMessage.error('登录失败，请重试')
          break
      }
    })
    .catch((e) => {
      clearInterval(pollingTimer.value)
      clearTimeout(qrcodeTimer.value)
      qrcodeExpired.value = true
    })
}

// 登录操作
const submitLogin = () => {
  if (!data.value.username) {
    return ElMessage.error('请输入用户名')
  }
  if (!data.value.password) {
    return ElMessage.error('请输入密码')
  }
  if (enableCaptcha.value) {
    captchaRef.value.loadCaptcha()
    action.value = 'login'
  } else {
    doLogin({})
  }
}

const doLogin = (verifyData) => {
  data.value.key = verifyData.key
  data.value.x = verifyData.x
  loading.value = true
  httpPost('/api/user/login', data.value)
    .then((res) => {
      setUserToken(res.data.token)
      store.setIsLogin(true)
      ElMessage.success('登录成功！')
      emits('hide')
      emits('success')
    })
    .catch((e) => {
      ElMessage.error('登录失败，' + e.message)
    })
    .finally(() => {
      loading.value = false
    })
}

// 注册操作
const submitRegister = () => {
  if (activeName.value === 'username' && data.value.username === '') {
    return ElMessage.error('请输入用户名')
  }

  if (activeName.value === 'mobile' && !validateMobile(data.value.mobile)) {
    return ElMessage.error('请输入合法的手机号')
  }

  if (activeName.value === 'email' && !validateEmail(data.value.email)) {
    return ElMessage.error('请输入合法的邮箱地址')
  }

  if (data.value.password.length < 8) {
    return ElMessage.error('密码的长度为8-16个字符')
  }
  if (data.value.repass !== data.value.password) {
    return ElMessage.error('两次输入密码不一致')
  }

  if ((activeName.value === 'mobile' || activeName.value === 'email') && data.value.code === '') {
    return ElMessage.error('请输入验证码')
  }
  if (!agreeChecked.value) {
    return ElMessage.error('请先阅读并同意《用户协议》和《隐私政策》')
  }
  if (enableCaptcha.value) {
    captchaRef.value.loadCaptcha()
    action.value = 'register'
  } else {
    doRegister({})
  }
}

const doRegister = (verifyData) => {
  data.value.key = verifyData.key
  data.value.x = verifyData.x
  data.value.reg_way = activeName.value
  loading.value = true
  httpPost('/api/user/register', data.value)
    .then((res) => {
      setUserToken(res.data.token)
      ElMessage.success({
        message: '注册成功!',
        onClose: () => {
          emits('hide')
          emits('success')
        },
        duration: 1000,
      })
    })
    .catch((e) => {
      ElMessage.error('注册失败，' + e.message)
    })
    .finally(() => {
      loading.value = false
    })
}

// 忘记密码操作
const submitResetPass = () => {
  if (resetForm.value.type === 'mobile' && !resetForm.value.mobile) {
    return ElMessage.error('请输入手机号')
  }
  if (resetForm.value.type === 'email' && !resetForm.value.email) {
    return ElMessage.error('请输入邮箱地址')
  }
  if (resetForm.value.code === '') {
    return ElMessage.error('请输入验证码')
  }
  if (resetForm.value.password.length < 8) {
    return ElMessage.error('密码长度必须大于8位')
  }
  if (resetForm.value.repass !== resetForm.value.password) {
    return ElMessage.error('两次输入密码不一致')
  }

  resetLoading.value = true
  httpPost('/api/user/resetPass', resetForm.value)
    .then(() => {
      ElMessage.success({
        message: '重置密码成功，请登录',
        duration: 1500,
        onClose: () => {
          viewMode.value = 'login'
        },
      })
      resetForm.value = { mobile: '', email: '', type: 'mobile', code: '', password: '', repass: '' }
    })
    .catch((e) => {
      ElMessage.error('重置密码失败：' + e.message)
    })
    .finally(() => {
      resetLoading.value = false
    })
}

// 打开并加载协议
const openAgreement = () => {
  if (!agreementHtml.value) {
    httpGet('/api/config/get?key=agreement')
      .then((res) => {
        agreementHtml.value = marked.parse(res.data?.content || '')
        showAgreement.value = true
      })
      .catch((e) => ElMessage.error('加载用户协议失败：' + e.message))
  } else {
    showAgreement.value = true
  }
}

// 打开并加载隐私政策
const openPrivacy = () => {
  if (!privacyHtml.value) {
    httpGet('/api/config/get?key=privacy')
      .then((res) => {
        privacyHtml.value = marked.parse(res.data?.content || '')
        showPrivacy.value = true
      })
      .catch((e) => ElMessage.error('加载隐私政策失败：' + e.message))
  } else {
    showPrivacy.value = true
  }
}

// 组件卸载时清除定时器
onUnmounted(() => {
  if (pollingTimer.value) {
    clearInterval(pollingTimer.value)
  }
  if (qrcodeTimer.value) {
    clearTimeout(qrcodeTimer.value)
  }
})
</script>

<style lang="scss">
.login-dialog {
  border-radius: 10px;

  // 微信登录样式
  .wechat-login {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 240px;

    .qr-code-container {
      text-align: center;

      .qr-code-wrapper {
        display: inline-block;
        border: 1px solid var(--el-border-color);
        border-radius: 8px;
        overflow: hidden;
        position: relative;

        .qr-frame {
          display: block;
          width: 100%;
          height: 100%;
        }

        .qr-expired-mask {
          position: absolute;
          top: 0;
          left: 0;
          right: 0;
          bottom: 0;
          background: rgba(0, 0, 0, 0.7);
          display: flex;
          align-items: center;
          justify-content: center;
          border-radius: 8px;

          .expired-content {
            text-align: center;
            color: white;

            .expired-icon {
              font-size: 48px;
              color: #f56565;
              margin-bottom: 12px;
              display: block;
            }

            .expired-text {
              font-size: 16px;
              margin: 0 0 16px 0;
              font-weight: 500;
            }
          }
        }
      }
    }
  }

  // CustomTabs 组件样式优化
  :deep(.custom-tabs-header) {
    background: var(--el-fill-color-light);
    border-radius: 8px;
    margin-bottom: 20px;
  }

  :deep(.custom-tab-item) {
    font-weight: 500;
    transition: all 0.3s ease;

    &:hover {
      background: var(--el-fill-color);
    }
  }

  :deep(.custom-tab-active) {
    background: var(--el-color-primary);
    color: white !important;

    &:hover {
      background: var(--el-color-primary);
    }
  }

  :deep(.el-input) {
    .el-input__wrapper {
      background: var(--el-fill-color-blank);
      border-color: var(--el-border-color);

      &.is-focus {
        border-color: var(--el-color-primary);
      }
    }

    .el-input__inner {
      color: var(--el-text-color-primary);

      &::placeholder {
        color: var(--el-text-color-placeholder);
      }
    }

    .el-input__prefix,
    .el-input__suffix {
      color: var(--el-text-color-regular);
    }
  }

  :deep(.el-button) {
    &.el-button--info {
      color: var(--el-text-color-regular);
      background: transparent;
      border: none;

      &:hover {
        background: var(--el-fill-color-light);
      }
    }
  }
}

// 响应式设计
@media (max-width: 576px) {
  .login-dialog {
    .wechat-login {
      .qr-code-wrapper {
        width: 240px !important;
        height: 240px !important;

        .qr-expired-mask {
          .expired-content {
            .expired-icon {
              font-size: 36px;
              margin-bottom: 8px;
            }

            .expired-text {
              font-size: 14px;
              margin: 0 0 12px 0;
            }
          }
        }
      }
    }
  }
}
</style>
