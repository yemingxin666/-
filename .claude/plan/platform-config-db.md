# 📋 实施计划：平台规范 DB 化 + Admin 管理面板

## 任务类型
- [x] 后端 (Go)
- [x] 前端 (Vue)
- [x] 全栈

## 技术方案

将平台规范从 `platform_rules.go` 硬编码迁移至 MySQL 表 `geekai_ai_platform_configs`，通过 Admin CRUD API 管理，用户端只读接口供前端动态加载；前端通过共享 composable 实现平台切换表单联动。

---

## 实施步骤

### Phase 1：后端数据模型

**步骤 1** — 新建 `store/model/ai_platform_config.go`

```go
type AiPlatformConfig struct {
    Id              uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
    Value           string    `gorm:"column:value;type:varchar(32);uniqueIndex;not null" json:"value"`
    Label           string    `gorm:"column:label;type:varchar(64);not null" json:"label"`
    DefaultLanguage string    `gorm:"column:default_language;type:varchar(16);not null;default:zh-CN" json:"default_language"`
    DefaultRatio    string    `gorm:"column:default_ratio;type:varchar(16);not null;default:1:1" json:"default_ratio"`
    PromptStyle     string    `gorm:"column:prompt_style;type:mediumtext;not null" json:"prompt_style"`
    PriorityImages  JSONMap   `gorm:"column:priority_images;type:json" json:"priority_images"`
    Constraints     JSONMap   `gorm:"column:constraints;type:json" json:"constraints"`
    Status          string    `gorm:"column:status;type:varchar(16);index;not null;default:active" json:"status"`
    SortOrder       int       `gorm:"column:sort_order;type:int;not null;default:0" json:"sort_order"`
    CreatedAt       time.Time `gorm:"column:created_at;not null" json:"created_at"`
    UpdatedAt       time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
}
func (m *AiPlatformConfig) TableName() string { return "geekai_ai_platform_configs" }
const (
    PlatformStatusActive   = "active"
    PlatformStatusDisabled = "disabled"
)
```

**步骤 2** — `service/migration_service.go` → `TableMigration()` 新增

```go
s.db.AutoMigrate(&model.AiPlatformConfig{})
s.SeedAiPlatformConfigs()  // 幂等种子数据（ON CONFLICT DO NOTHING）
```

种子数据覆盖：pinduoduo/taobao/tmall/jd/douyin/xiaohongshu/amazon/shopee/shopify/lazada/aliexpress/generic，含各自 PromptStyle、DefaultLanguage、DefaultRatio、Constraints。

**步骤 3** — 升级 `service/aicommerce/prompt/platform_rules.go`

```go
// 签名从 PlatformRules(platform string) 改为 PlatformRules(db *gorm.DB, platform string)
func PlatformRules(db *gorm.DB, platform string) string {
    cfg, err := FindPlatformConfig(db, platform, true)
    if err == nil && cfg.PromptStyle != "" { return cfg.PromptStyle }
    cfg, _ = FindPlatformConfig(db, "generic", true)
    if cfg != nil { return cfg.PromptStyle }
    return "通用电商风格：商品主体清晰，背景简洁，光线均衡"  // 兜底
}
```

同步更新所有调用点：
- `service/aicommerce/worker/chains/main_image.go`
- `service/aicommerce/worker/chains/clone.go`
- `handler/admin/aicommerce_handler.go` (PreviewTemplate)

---

### Phase 2：后端 API

**步骤 4** — `handler/admin/aicommerce_handler.go` 新增 4 个 Admin CRUD 方法

| 路由 | 方法 | 说明 |
|------|------|------|
| GET  `/api/admin/ai-commerce/platform-configs` | ListPlatformConfigs | 支持 status/keyword 过滤 |
| POST `/api/admin/ai-commerce/platform-configs/save` | SavePlatformConfig | 新建/更新 |
| GET  `/api/admin/ai-commerce/platform-configs/remove?id=` | RemovePlatformConfig | 删除 |
| POST `/api/admin/ai-commerce/platform-configs/status` | SetPlatformConfigStatus | 启用/禁用 |

**步骤 5** — `handler/aicommerce/image_handler.go` 新增用户只读接口

```
GET /api/ai-commerce/platform-configs
→ { items: [{ value, label, default_language, default_ratio, priority_images, constraints, sort_order }] }
仅返回 status=active 记录，按 sort_order ASC 排序
```

---

### Phase 3：前端 Store 扩展

**步骤 6** — `web/src/store/ecom.js` 扩展

```javascript
const platformConfigs = ref(new Map())  // value → config object

const loadPlatformConfigs = async () => {
  const res = await httpGet('/api/ai-commerce/platform-configs')
  const map = new Map()
  ;(res.data?.items || []).forEach(c => map.set(c.value, c))
  platformConfigs.value = map
}

const getPlatformConfig = (value) => platformConfigs.value.get(value) || null

// 导出新增字段
return { ..., platformConfigs, loadPlatformConfigs, getPlatformConfig }
```

**步骤 7** — 新建 `web/src/composables/useEcomLinkage.js`

```javascript
export function useEcomLinkage(form) {
  const store = useEcomConfigStore()
  watch(() => form.value.platform, (newVal) => {
    const cfg = store.getPlatformConfig(newVal)
    if (!cfg) return
    // 1. 自动预填语言（用户可覆盖）
    form.value.language = cfg.default_language
    // 2. 合规提示（国际平台）
    if (cfg.constraints?.no_text_overlay || cfg.constraints?.force_white_bg) {
      ElNotification({ title: '平台合规提示', message: '该平台要求严格...', type: 'warning' })
    }
  })
  // 推荐 ratio 的 computed（供模板高亮）
  const recommendedRatio = computed(() =>
    store.getPlatformConfig(form.value.platform)?.default_ratio || null
  )
  return { recommendedRatio }
}
```

**步骤 8** — 三个表单页集成 composable

在 `MainImagePage.vue` / `DetailPagePage.vue` / `ClonePage.vue` 中：
```javascript
import { useEcomLinkage } from '@/composables/useEcomLinkage'
const { recommendedRatio } = useEcomLinkage(form)
```
Ratio 选择器对推荐比例加"推荐"角标（非强制）。

**步骤 9** — 图片类型排序（在各页面 computed 中）

```javascript
const sortedTypes = computed(() => {
  const cfg = configStore.getPlatformConfig(form.value.platform)
  const priority = cfg?.priority_images || {}
  const rank = (val) => {
    if (priority.must_have?.includes(val)) return 0
    if (priority.recommended?.includes(val)) return 1
    return 2
  }
  return [...allTypes].sort((a, b) => rank(a.value) - rank(b.value))
})
```
`must_have` 类型加"必做"标签（el-tag type="danger"）。

**步骤 10** — 在应用入口或 ecom layout 调用 `loadPlatformConfigs()`（一次性加载，避免每页重复请求）

---

### Phase 4：Admin 管理页面

**步骤 11** — 新建 `web/src/views/admin/aicommerce/PlatformList.vue`

组件结构：
- `el-table`：列出 value / label / default_language / default_ratio / sort_order / status / 操作
- `el-dialog` 编辑表单字段：
  - 平台标识（value，新增时可编辑）
  - 显示名称（label）
  - 默认语言（el-select：zh-CN / en-US）
  - 默认比例（el-select：1:1 / 3:4 / 16:9 等）
  - Prompt 风格描述（el-input textarea）
  - 套图优先级（三列 multiselect：must_have / recommended / optional）
  - 合规约束（el-switch：force_white_bg / no_text_overlay）
  - 排序权重 / 状态

**步骤 12** — 在 Admin 路由中注册 PlatformList.vue（`/admin/aicommerce/platforms`）

---

## 关键文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/store/model/ai_platform_config.go` | 新建 | DB 模型定义 |
| `api/service/migration_service.go` | 修改 | AutoMigrate + SeedAiPlatformConfigs |
| `api/service/aicommerce/prompt/platform_rules.go` | 修改 | 签名加 db 参数，改为 DB 查询 |
| `api/service/aicommerce/worker/chains/main_image.go` | 修改 | 更新 PlatformRules 调用 |
| `api/service/aicommerce/worker/chains/clone.go` | 修改 | 更新 PlatformRules 调用 |
| `api/handler/admin/aicommerce_handler.go` | 修改 | 新增 4 个平台配置 CRUD 方法 + 路由 |
| `api/handler/aicommerce/image_handler.go` | 修改 | 新增用户只读 ListPlatformConfigs |
| `web/src/store/ecom.js` | 修改 | 新增 platformConfigs、loadPlatformConfigs、getPlatformConfig |
| `web/src/composables/useEcomLinkage.js` | 新建 | 平台切换联动 composable |
| `web/src/views/ecom/MainImagePage.vue` | 修改 | 集成 composable，sortedTypes computed |
| `web/src/views/ecom/DetailPagePage.vue` | 修改 | 同上 |
| `web/src/views/ecom/ClonePage.vue` | 修改 | 同上 |
| `web/src/views/admin/aicommerce/PlatformList.vue` | 新建 | Admin 管理页面 |

---

## 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| DB 查询导致 PlatformRules() 性能下降（每次生图调用） | 加简单内存缓存（sync.Map + TTL），5 分钟过期 |
| SeedAiPlatformConfigs 覆盖运营已改数据 | 用 ON CONFLICT DO NOTHING，只插入不更新 |
| 前端 platformConfigs 加载失败时联动中断 | getPlatformConfig 返回 null 时 watch 跳过，不影响生图提交 |
| PlatformRules 签名变更影响所有调用点 | 统一替换（3 处），编译期即发现遗漏 |

---

## SESSION_ID（供 /ccg:execute 使用）
- CODEX_SESSION: 019ddd79-f2f3-78c3-b976-aeeb0c4e96c6
- GEMINI_SESSION: 113633c7-465d-4fef-a1e1-0e909eb64cf2
