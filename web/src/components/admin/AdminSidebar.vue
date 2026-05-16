<template>
  <div :class="'sidebar ' + theme">
    <a class="logo w-full flex items-center" href="/" target="_blank">
      <img :src="logo" />
      <span class="text" v-show="!sidebar.collapse">{{ title }}</span>
    </a>

    <el-menu
      class="sidebar-el-menu"
      :default-active="onRoutes"
      :collapse="sidebar.collapse"
      background-color="#324157"
      text-color="#bfcbd9"
      active-text-color="#20a0ff"
      unique-opened
      router
    >
      <template v-for="item in items">
        <template v-if="item.subs">
          <el-sub-menu :index="item.index" :key="item.index">
            <template #title>
              <i :class="'iconfont icon-' + item.icon"></i>
              <span>{{ item.title }}</span>
            </template>
            <template v-for="subItem in item.subs">
              <el-sub-menu v-if="subItem.subs" :index="subItem.index" :key="subItem.index">
                <template #title>{{ subItem.title }}</template>
                <el-menu-item
                  v-for="(threeItem, i) in subItem.subs"
                  :key="i"
                  :index="threeItem.index"
                >
                  {{ threeItem.title }}
                </el-menu-item>
              </el-sub-menu>
              <el-menu-item v-else :index="subItem.index" :key="subItem.index">
                <i v-if="subItem.icon" :class="'iconfont icon-' + subItem.icon"></i>
                {{ subItem.title }}
              </el-menu-item>
            </template>
          </el-sub-menu>
        </template>
        <template v-else>
          <el-menu-item :index="item.index" :key="item.index">
            <i :class="'iconfont icon-' + item.icon"></i>
            <template #title>{{ item.title }}</template>
          </el-menu-item>
        </template>
      </template>
    </el-menu>
  </div>
</template>

<script setup>
import { useSharedStore } from '@/store/sharedata'
import { setMenuItems, useSidebarStore } from '@/store/sidebar'
import { httpGet } from '@/utils/http'
import { ElMessage } from 'element-plus'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

const title = ref('')
const logo = ref('')

// 加载系统配置
httpGet('/api/admin/config/get?key=system')
  .then((res) => {
    title.value = res.data.admin_title
    logo.value = res.data.logo
  })
  .catch((e) => {
    ElMessage.error('加载系统配置失败: ' + e.message)
  })
const store = useSharedStore()
const theme = ref(store.theme)
watch(
  () => store.theme,
  (val) => {
    theme.value = val
  }
)
const items = [
  {
    icon: 'home',
    index: '/admin/dashboard',
    title: '仪表盘',
  },

  {
    icon: 'user-fill',
    index: '/admin/user',
    title: '用户管理',
  },
  {
    icon: 'recharge',
    index: '/admin/product',
    title: '充值产品',
  },
  {
    icon: 'order',
    index: '/admin/order',
    title: '充值订单',
  },
  {
    icon: 'reward',
    index: '/admin/redeem',
    title: '兑换码',
  },
  {
    icon: 'image',
    index: 'ecom-group',
    title: '电商生图',
    subs: [
      {
        icon: 'model',
        index: '/admin/aicommerce/models',
        title: 'AI 模型管理',
      },
      {
        icon: 'prompt',
        index: '/admin/aicommerce/templates',
        title: 'Prompt 模板',
      },
      {
        icon: 'recharge',
        index: '/admin/aicommerce/prices',
        title: '积分定价',
      },
      {
        icon: 'list',
        index: '/admin/aicommerce/tasks',
        title: '任务审计',
      },
      {
        icon: 'config',
        index: '/admin/aicommerce/platforms',
        title: '平台规范',
      },
    ],
  },
  {
    icon: 'role',
    index: '/admin/manger',
    title: '管理员',
  },

  {
    icon: 'config',
    index: 'config-center',
    title: '系统设置',
    subs: [
      {
        icon: 'config',
        index: '/admin/config/basic',
        title: '基础配置',
      },
      {
        icon: 'power',
        index: '/admin/config/power',
        title: '算力配置',
      },

      {
        icon: 'menu',
        index: '/admin/config/menu',
        title: '菜单配置',
      },
      {
        icon: 'recharge',
        index: '/admin/config/payment',
        title: '支付配置',
      },
      {
        icon: 'menu',
        index: '/admin/config/storage',
        title: '存储配置',
      },
      {
        icon: 'sms',
        index: '/admin/config/sms',
        title: '短信配置',
      },
      {
        icon: 'email',
        index: '/admin/config/smtp',
        title: '邮件配置',
      },
      {
        icon: 'plugin',
        index: '/admin/config/plugin',
        title: '插件配置',
      },
    ],
  },
  {
    icon: 'linggan',
    index: 'content-config',
    title: '文案配置',
    subs: [
      {
        icon: 'speaker',
        index: '/admin/config/notice',
        title: '公告配置',
      },
      {
        icon: 'info',
        index: '/admin/config/agreement',
        title: '用户协议',
      },
      {
        icon: 'info',
        index: '/admin/config/privacy',
        title: '隐私声明',
      },
    ],
  },
  {
    icon: 'log',
    index: '/admin/powerLog',
    title: '用户算力日志',
  },
  {
    icon: 'log',
    index: '/admin/loginLog',
    title: '用户登录日志',
  },
  // {
  //   icon: 'menu',
  //   index: '1',
  //   title: '常用模板页面',
  //   subs: [
  //     {
  //       index: '/admin/demo/form',
  //       title: '表单页面',
  //     },
  //     {
  //       index: '/admin/demo/table',
  //       title: '常用表格',
  //     },
  //     {
  //       index: '/admin/demo/import',
  //       title: '导入Excel',
  //     },
  //     {
  //       index: '/admin/demo/editor',
  //       title: '富文本编辑器',
  //     },
  //   ],
  // },
]

const route = useRoute()
const onRoutes = computed(() => {
  return route.path
})

const sidebar = useSidebarStore()
setMenuItems(items)
</script>

<style scoped lang="scss">
.sidebar {
  display: block;
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  overflow-y: scroll;
  background-color: #324157;

  .logo {
    display: flex;
    padding: 6px 15px;
    cursor: pointer;
    background-color: #324157;

    img {
      height: 36px;
      padding-top: 5px;
      border-radius: 100%;
      background: #fff;
      border: 2px solid #754ff6;
      padding: 2px;
    }

    .text {
      color: #ffffff;
      font-weight: bold;
      padding: 12px 0 12px 10px;
      transition: width 2s ease;
    }
  }

  ul {
    height: auto;
    min-height: 100%;

    .el-menu-item,
    .el-sub-menu {
      .iconfont {
        font-size: 16px;
        margin-right: 5px;
      }
    }

    .el-menu-item.is-active {
      background-color: rgb(40, 52, 70);
    }
  }

  .sidebar-el-menu:not(.el-menu--collapse) {
    width: 250px;
  }
}

.sidebar::-webkit-scrollbar {
  width: 0;
}

.sidebar.dark {
  border-right: 1px solid var(--el-border-color-dark);

  .logo {
    background: var(--el-bg-color);
    border-right: 1px solid var(--el-border-color);

    .text {
      color: var(--el-text-color-regular);
    }
  }

  ul {
    background: var(--el-bg-color);

    .el-menu-item.is-active {
      background-color: var(--el-menu-bg-color-dark);
    }

    .el-menu-item:hover {
      background-color: var(--el-menu-bg-color-darker);
    }
  }

  .sidebar-el-menu:not(.el-menu--collapse) {
    width: 250px;
  }

  .el-menu {
    border-color: var(--el-border-color);
  }
}
</style>
