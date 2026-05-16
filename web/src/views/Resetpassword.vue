<template>
  <div class="login-container">
    <div class="background-mesh">
      <div class="mesh-blob blob-1"></div>
      <div class="mesh-blob blob-2"></div>
      <div class="mesh-blob blob-3"></div>
    </div>
    <div class="particles-layer">
      <div class="particle-field"></div>
    </div>

    <div class="content-wrapper">
      <div class="login-card-wrapper">
        <div class="glow-border">
          <div class="glow-rotate"></div>
        </div>
        <div class="login-card">
          <div class="card-header">
            <h1 class="gradient-title">重置密码</h1>
            <p class="subtitle">通过手机号或邮箱验证重置您的密码</p>
          </div>

          <el-form :model="form" class="form space-y-5">
            <el-tabs v-model="form.type" class="demo-tabs">
              <el-tab-pane label="手机号验证" name="mobile">
                <div class="block">
                  <el-input
                    v-model="form.mobile"
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
                        v-model="form.code"
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
                      <send-msg size="large" :receiver="form.mobile" type="mobile" scene="reset_pass" />
                    </el-col>
                  </el-row>
                </div>
              </el-tab-pane>

              <el-tab-pane label="邮箱验证" name="email">
                <div class="block">
                  <el-input
                    v-model="form.email"
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
                        v-model="form.code"
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
                      <send-msg size="large" :receiver="form.email" type="email" scene="reset_pass" />
                    </el-col>
                  </el-row>
                </div>
              </el-tab-pane>
            </el-tabs>

            <div class="block">
              <el-input
                v-model="form.password"
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
                v-model="form.repass"
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
                @click="save"
                type="button"
              >
                {{ loading ? '重置中...' : '重置密码' }}
              </button>
            </div>

            <div
              class="text text-sm flex justify-center items-center w-full pt-3"
              style="color: rgba(255, 255, 255, 0.5)"
            >
              想起密码了？
              <el-button
                size="small"
                class="ml-2 rounded-md px-2 py-1 transition-colors duration-200"
                style="color: rgba(255, 255, 255, 0.7)"
                @click="goLogin"
                @mouseenter="$event.target.style.background = 'rgba(255, 255, 255, 0.06)'"
                @mouseleave="$event.target.style.background = 'transparent'"
              >返回登录</el-button>
            </div>
          </el-form>
        </div>
      </div>
    </div>

    <footer-bar />
  </div>
</template>

<script setup>
import FooterBar from '@/components/FooterBar.vue'
import SendMsg from '@/components/SendMsg.vue'
import { Checked, Iphone, Lock, Message } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { httpPost } from '@/utils/http'

const router = useRouter()
const loading = ref(false)
const form = ref({
  mobile: '',
  email: '',
  type: 'mobile',
  code: '',
  password: '',
  repass: '',
})

const goLogin = () => {
  router.push('/login')
}

const save = () => {
  if (form.value.type === 'mobile' && !form.value.mobile) {
    return ElMessage.error('请输入手机号')
  }
  if (form.value.type === 'email' && !form.value.email) {
    return ElMessage.error('请输入邮箱地址')
  }
  if (form.value.code === '') {
    return ElMessage.error('请输入验证码')
  }
  if (form.value.password.length < 8) {
    return ElMessage.error('密码长度必须大于8位')
  }
  if (form.value.repass !== form.value.password) {
    return ElMessage.error('两次输入密码不一致')
  }

  loading.value = true
  httpPost('/api/user/resetPass', form.value)
    .then(() => {
      ElMessage.success({
        message: '重置密码成功，即将跳转登录页',
        duration: 1500,
        onClose: () => router.push('/login'),
      })
    })
    .catch((e) => {
      ElMessage.error('重置密码失败：' + e.message)
    })
    .finally(() => {
      loading.value = false
    })
}
</script>

<style lang="scss" scoped>
.login-container {
  position: relative;
  width: 100%;
  min-height: 100vh;
  background-color: #0d1435;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.background-mesh {
  position: absolute;
  inset: 0;
  z-index: 0;
  filter: blur(80px);
  opacity: 0.6;

  .mesh-blob {
    position: absolute;
    border-radius: 50%;
    animation: meshMove 20s infinite alternate ease-in-out;
  }

  .blob-1 {
    width: 600px;
    height: 600px;
    background: radial-gradient(circle, #6366f1 0%, transparent 70%);
    top: -10%;
    left: -10%;
  }

  .blob-2 {
    width: 500px;
    height: 500px;
    background: radial-gradient(circle, #a855f7 0%, transparent 70%);
    bottom: -10%;
    right: -5%;
    animation-delay: -5s;
    animation-duration: 25s;
  }

  .blob-3 {
    width: 400px;
    height: 400px;
    background: radial-gradient(circle, #1e293b 0%, transparent 70%);
    top: 40%;
    left: 50%;
    transform: translate(-50%, -50%);
    animation-duration: 30s;
  }
}

.particles-layer {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;

  .particle-field {
    width: 100%;
    height: 200%;
    background-image:
      radial-gradient(1.5px 1.5px at 10% 20%, rgba(255, 255, 255, 0.9), transparent),
      radial-gradient(1px 1px at 25% 45%, rgba(255, 255, 255, 0.6), transparent),
      radial-gradient(2px 2px at 50% 15%, rgba(167, 139, 250, 0.8), transparent),
      radial-gradient(1px 1px at 70% 60%, rgba(255, 255, 255, 0.5), transparent),
      radial-gradient(1.5px 1.5px at 85% 30%, rgba(255, 255, 255, 0.7), transparent),
      radial-gradient(1px 1px at 40% 75%, rgba(129, 140, 248, 0.6), transparent),
      radial-gradient(1.5px 1.5px at 60% 85%, rgba(255, 255, 255, 0.8), transparent),
      radial-gradient(1px 1px at 15% 65%, rgba(255, 255, 255, 0.4), transparent),
      radial-gradient(2px 2px at 90% 50%, rgba(168, 85, 247, 0.5), transparent),
      radial-gradient(1px 1px at 35% 10%, rgba(255, 255, 255, 0.6), transparent);
    background-size: 250px 250px;
    opacity: 0.35;
    animation: particlesFloat 80s linear infinite;
  }
}

.content-wrapper {
  position: relative;
  z-index: 10;
  width: 100%;
  max-width: 480px;
  padding: 20px;
}

.login-card-wrapper {
  position: relative;
  border-radius: 16px;
  animation: slideUpFade 0.8s cubic-bezier(0.16, 1, 0.3, 1) both;
  transition: transform 0.6s cubic-bezier(0.16, 1, 0.3, 1),
              box-shadow 0.6s cubic-bezier(0.16, 1, 0.3, 1);

  &:hover {
    transform: translateY(-6px);
    box-shadow: 0 0 40px rgba(99, 102, 241, 0.25);

    .glow-rotate {
      opacity: 1;
    }
  }
}

.glow-border {
  position: absolute;
  inset: -2px;
  border-radius: 18px;
  overflow: hidden;
  z-index: 0;
  pointer-events: none;

  .glow-rotate {
    position: absolute;
    inset: -50%;
    background: conic-gradient(
      from 0deg,
      transparent 0deg,
      #6366f1 60deg,
      #a855f7 120deg,
      #ec4899 180deg,
      #a855f7 240deg,
      #6366f1 300deg,
      transparent 360deg
    );
    animation: borderSpin 4s linear infinite;
    opacity: 0.7;
  }
}

.login-card {
  position: relative;
  z-index: 1;
  width: 100%;
  background: rgba(17, 23, 56, 0.85);
  backdrop-filter: blur(25px);
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
}

.card-header {
  text-align: center;
  margin-bottom: 32px;

  .gradient-title {
    font-size: 30px;
    font-weight: 800;
    margin: 0 0 8px;
    background: linear-gradient(
      90deg,
      #6366f1 0%,
      #a855f7 25%,
      #ec4899 50%,
      #a855f7 75%,
      #6366f1 100%
    );
    background-size: 200% auto;
    background-clip: text;
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    animation: titleShine 4s ease-in-out infinite;
    letter-spacing: -0.5px;
  }

  .subtitle {
    color: rgba(255, 255, 255, 0.45);
    font-size: 14px;
    margin: 0;
    font-weight: 300;
    line-height: 1.5;
  }
}

@keyframes borderSpin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes titleShine {
  0%, 100% { background-position: 0% center; }
  50% { background-position: 200% center; }
}

@keyframes meshMove {
  from { transform: translate(0, 0) scale(1); }
  to { transform: translate(10%, 15%) scale(1.1); }
}

@keyframes particlesFloat {
  from { transform: translateY(0); }
  to { transform: translateY(-50%); }
}

@keyframes slideUpFade {
  from { opacity: 0; transform: translateY(40px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (max-width: 768px) {
  .login-card { padding: 32px 24px; }
  .background-mesh { filter: blur(50px); }
  .blob-1 { width: 400px; height: 400px; }
  .blob-2 { width: 350px; height: 350px; }
  .blob-3 { width: 250px; height: 250px; }
  .card-header .gradient-title { font-size: 26px; }
}

@media (max-width: 480px) {
  .content-wrapper { padding: 16px; }
  .login-card { padding: 24px 20px; }
  .card-header .gradient-title { font-size: 24px; }
  .card-header .subtitle { font-size: 13px; }
}

@media (prefers-reduced-motion: reduce) {
  .login-card-wrapper,
  .mesh-blob,
  .particle-field,
  .gradient-title,
  .glow-rotate {
    animation: none !important;
    transition: none !important;
  }
  .glow-rotate { opacity: 0.3; }
  .background-mesh { opacity: 0.4; }
}
</style>

<style lang="scss">
.login-container {
  .el-input__wrapper {
    background: rgba(0, 0, 0, 0.2) !important;
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.1) inset !important;
    border: none !important;
    transition: box-shadow 0.3s ease;

    &:hover { box-shadow: 0 0 0 1px rgba(99, 102, 241, 0.3) inset !important; }
    &.is-focus { box-shadow: 0 0 0 1px #6366f1 inset !important; }
  }

  .el-input__inner {
    color: #fff !important;
    &::placeholder { color: rgba(255, 255, 255, 0.4) !important; }
  }

  .el-input__prefix .el-icon,
  .el-input__suffix .el-icon {
    color: rgba(255, 255, 255, 0.45) !important;
  }

  .el-tabs__item {
    color: rgba(255, 255, 255, 0.5) !important;
    &.is-active { color: #fff !important; }
    &:hover { color: rgba(255, 255, 255, 0.7) !important; }
  }

  .el-tabs__active-bar { background: #6366f1 !important; }
  .el-tabs__nav-wrap::after { background: rgba(255, 255, 255, 0.06) !important; }

  .foot-container .footer {
    a, span { color: rgba(255, 255, 255, 0.35) !important; }
    a:hover { color: rgba(255, 255, 255, 0.6) !important; }
  }
}
</style>
