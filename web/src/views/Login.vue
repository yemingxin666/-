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
            <h1 class="gradient-title">{{ title }}</h1>
            <p class="subtitle">{{ subtitle }}</p>
          </div>
          <div class="login-dialog-wrapper">
            <login-dialog
              :show="true"
              :active="active"
              :inviteCode="inviteCode"
              @success="handleRegisterSuccess"
              @changeActive="handleChangeActive"
              ref="loginDialogRef"
            />
          </div>
        </div>
      </div>
    </div>

    <footer-bar />
  </div>
</template>

<script setup>
import FooterBar from '@/components/FooterBar.vue'
import LoginDialog from '@/components/LoginDialog.vue'
import { isMobile } from '@/utils/libs'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { setUserToken } from '@/store/session'

const router = useRouter()
const loginDialogRef = ref(null)
const inviteCode = ref(router.currentRoute.value.query.invite_code || '')
const token = ref(router.currentRoute.value.query.token || '')
const isRegister = ref(router.currentRoute.value.path === '/register')
const active = ref(isRegister.value ? 'register' : 'login')
const title = computed(() => (isRegister.value ? '用户注册' : '用户登录'))
const subtitle = computed(() =>
  isRegister.value ? '创建您的账户以开始使用服务' : '登录您的账户以继续使用服务'
)

const handleRegisterSuccess = () => {
  if (isMobile()) {
    router.push('/mobile')
  } else {
    router.push('/ecom')
  }
}

const handleChangeActive = (newValue) => {
  isRegister.value = !newValue
}

onMounted(() => {
  if (loginDialogRef.value) {
    loginDialogRef.value.login = !isRegister
  }
  if (token.value) {
    setUserToken(token.value)
    handleRegisterSuccess()
  }
})
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

// ---- 背景 Mesh Blob ----
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

// ---- 星尘粒子 ----
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

// ---- 内容包装 ----
.content-wrapper {
  position: relative;
  z-index: 10;
  width: 100%;
  max-width: 480px;
  padding: 20px;
}

// ---- 流光边框容器 ----
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

// ---- 登录卡片 ----
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

// ---- 标题渐变流动 ----
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

// ---- Keyframes ----
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

// ---- 响应式 ----
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

  .el-checkbox__label { color: rgba(255, 255, 255, 0.6) !important; }
  .el-checkbox__inner {
    background: rgba(0, 0, 0, 0.3) !important;
    border-color: rgba(255, 255, 255, 0.2) !important;
  }
  .el-checkbox__input.is-checked .el-checkbox__inner {
    background: #6366f1 !important;
    border-color: #6366f1 !important;
  }

  .el-button--info {
    color: rgba(255, 255, 255, 0.45) !important;
    background: transparent !important;
    border: none !important;
    &:hover { color: #fff !important; background: rgba(255, 255, 255, 0.06) !important; }
  }

  .custom-tabs-header {
    background: rgba(0, 0, 0, 0.2) !important;
  }

  .custom-tab-item {
    color: rgba(255, 255, 255, 0.55) !important;
    &:hover { background: rgba(255, 255, 255, 0.06) !important; }
  }

  .custom-tab-active {
    color: #fff !important;
    background: #6366f1 !important;
    &:hover { background: #6366f1 !important; }
  }

  .foot-container .footer {
    a, span { color: rgba(255, 255, 255, 0.35) !important; }
    a:hover { color: rgba(255, 255, 255, 0.6) !important; }
  }
}
</style>
