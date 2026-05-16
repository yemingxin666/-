// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
// * Copyright 2023 The Geek-AI Authors. All rights reserved.
// * Use of this source code is governed by a Apache-2.0 license
// * that can be found in the LICENSE file.
// * @Author yangjian102621@163.com
// * +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    name: 'Index',
    path: '/',
    redirect: '/home',
  },
  {
    name: 'home',
    path: '/home',
    redirect: '/ecom',
    component: () => import('@/views/Home.vue'),
    children: [
      {
        name: 'member',
        path: '/member',
        meta: { title: '会员充值中心' },
        component: () => import('@/views/Member.vue'),
      },
      {
        name: 'user-invitation',
        path: '/invite',
        meta: { title: '推广计划' },
        component: () => import('@/views/Invitation.vue'),
      },
      {
        name: 'powerLog',
        path: '/powerLog',
        meta: { title: '消费日志' },
        component: () => import('@/views/PowerLog.vue'),
      },
      {
        name: 'ExternalLink',
        path: '/external',
        component: () => import('@/views/ExternalPage.vue'),
      },
      {
        name: 'ecom',
        path: '/ecom',
        meta: { title: '电商生图' },
        component: () => import('@/views/ecom/EcomPage.vue'),
      },
    ],
  },
  {
    name: 'login',
    path: '/login',
    meta: { title: '用户登录' },
    component: () => import('@/views/Login.vue'),
  },
  {
    name: 'register',
    path: '/register',
    meta: { title: '用户注册' },
    component: () => import('@/views/Login.vue'),
  },
  {
    name: 'resetpassword',
    path: '/resetpassword',
    meta: { title: '重置密码' },
    component: () => import('@/views/Resetpassword.vue'),
  },
  {
    path: '/admin/login',
    name: 'admin-login',
    meta: { title: '控制台登录' },
    component: () => import('@/views/admin/Login.vue'),
  },

  {
    name: 'admin',
    path: '/admin',
    redirect: '/admin/dashboard',
    component: () => import('@/views/admin/Home.vue'),
    meta: { title: '韩絮服饰 控制台' },
    children: [
      {
        path: '/admin/dashboard',
        name: 'admin-dashboard',
        meta: { title: '仪表盘' },
        component: () => import('@/views/admin/Dashboard.vue'),
      },
      {
        path: '/admin/config/basic',
        name: 'admin-config-basic',
        meta: { title: '基础配置' },
        component: () => import('@/views/admin/settings/BasicConfig.vue'),
      },
      {
        path: '/admin/config/power',
        name: 'admin-config-power',
        meta: { title: '算力配置' },
        component: () => import('@/views/admin/settings/PowerConfig.vue'),
      },
      {
        path: '/admin/config/payment',
        name: 'admin-config-payment',
        meta: { title: '支付配置' },
        component: () => import('@/views/admin/settings/PaymentConfig.vue'),
      },
      {
        path: '/admin/config/storage',
        name: 'admin-config-storage',
        meta: { title: '存储配置' },
        component: () => import('@/views/admin/settings/StorageConfig.vue'),
      },
      {
        path: '/admin/config/sms',
        name: 'admin-config-sms',
        meta: { title: '短信配置' },
        component: () => import('@/views/admin/settings/SmsConfig.vue'),
      },
      {
        path: '/admin/config/smtp',
        name: 'admin-config-smtp',
        meta: { title: '邮件配置' },
        component: () => import('@/views/admin/settings/SmtpConfig.vue'),
      },
      {
        path: '/admin/config/plugin',
        name: 'admin-config-plugin',
        meta: { title: '插件配置' },
        component: () => import('@/views/admin/settings/PluginConfig.vue'),
      },
      {
        path: '/admin/config/notice',
        name: 'admin-config-notice',
        meta: { title: '公告配置' },
        component: () => import('@/views/admin/settings/NoticeConfig.vue'),
      },
      {
        path: '/admin/config/agreement',
        name: 'admin-config-agreement',
        meta: { title: '用户协议' },
        component: () => import('@/views/admin/settings/AgreementConfig.vue'),
      },
      {
        path: '/admin/config/privacy',
        name: 'admin-config-privacy',
        meta: { title: '隐私声明' },
        component: () => import('@/views/admin/settings/PrivacyConfig.vue'),
      },
      {
        path: '/admin/config/menu',
        name: 'admin-config-menu',
        meta: { title: '菜单配置' },
        component: () => import('@/views/admin/settings/MenuConfig.vue'),
      },
      {
        path: '/admin/user',
        name: 'admin-user',
        meta: { title: '用户管理' },
        component: () => import('@/views/admin/Users.vue'),
      },
      {
        path: '/admin/product',
        name: 'admin-product',
        meta: { title: '充值产品' },
        component: () => import('@/views/admin/Product.vue'),
      },
      {
        path: '/admin/order',
        name: 'admin-order',
        meta: { title: '充值订单' },
        component: () => import('@/views/admin/Order.vue'),
      },
      {
        path: '/admin/redeem',
        name: 'admin-redeem',
        meta: { title: '兑换码管理' },
        component: () => import('@/views/admin/Redeem.vue'),
      },
      {
        path: '/admin/loginLog',
        name: 'admin-loginLog',
        meta: { title: '登录日志' },
        component: () => import('@/views/admin/LoginLog.vue'),
      },
      {
        path: '/admin/powerLog',
        name: 'admin-power-log',
        meta: { title: '算力日志' },
        component: () => import('@/views/admin/PowerLog.vue'),
      },
      {
        path: '/admin/manger',
        name: 'admin-manger',
        meta: { title: '管理员' },
        component: () => import('@/views/admin/Manager.vue'),
      },
      {
        path: '/admin/aicommerce/templates',
        name: 'admin-aicommerce-templates',
        meta: { title: 'Prompt 模板管理' },
        component: () => import('@/views/admin/aicommerce/TemplateList.vue'),
      },
      {
        path: '/admin/aicommerce/prices',
        name: 'admin-aicommerce-prices',
        meta: { title: '积分定价配置' },
        component: () => import('@/views/admin/aicommerce/PriceConfig.vue'),
      },
      {
        path: '/admin/aicommerce/models',
        name: 'admin-aicommerce-models',
        meta: { title: 'AI 模型管理' },
        component: () => import('@/views/admin/aicommerce/ModelList.vue'),
      },
      {
        path: '/admin/aicommerce/tasks',
        name: 'admin-aicommerce-tasks',
        meta: { title: '任务审计' },
        component: () => import('@/views/admin/aicommerce/TaskAudit.vue'),
      },
      {
        path: '/admin/aicommerce/platforms',
        name: 'admin-aicommerce-platforms',
        meta: { title: '平台规范管理' },
        component: () => import('@/views/admin/aicommerce/PlatformList.vue'),
      },
    ],
  },

  {
    name: 'NotFound',
    path: '/:all(.*)',
    meta: { title: '页面没有找到' },
    component: () => import('@/views/404.vue'),
  },
]

// console.log(MY_VARIABLE)
const router = createRouter({
  history: createWebHistory(),
  routes: routes,
})

let prevRoute = null
// dynamic change the title when router change
router.beforeEach((to, from, next) => {
  document.title = to.meta.title
  prevRoute = from
  next()
})

export { prevRoute, router }
