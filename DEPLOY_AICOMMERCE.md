# 电商 AI 生图模块 — 部署指南

## 1. 前置要求

- GeeKAI 主体已正常部署（MySQL / Redis / OSS 已配置）
- Go 1.21+（后端构建）
- Node.js 18+（前端构建）

---

## 2. 环境变量清单

在 `docker/docker-compose.yaml` 或服务器 `.env` 文件中配置以下变量：

| 变量名 | 说明 | 必填 |
|--------|------|------|
| `SILICON_FLOW_API_KEY` | 硅基流动 API Key（[申请地址](https://siliconflow.cn)） | ✅ |
| `SILICON_FLOW_MODEL` | 生图模型，默认 `Kwai-Kolors/Kolors` | |
| `TONGYI_API_KEY` | 阿里云百炼 API Key（通义千问） | ✅ |
| `TONGYI_MODEL` | 代写模型，默认 `qwen-turbo` | |
| `BAIDU_OCR_APP_ID` | 百度云 OCR 应用 ID | ✅（翻译模块） |
| `BAIDU_OCR_API_KEY` | 百度云 OCR API Key | ✅（翻译模块） |
| `BAIDU_OCR_SECRET_KEY` | 百度云 OCR Secret Key | ✅（翻译模块） |
| `BAIDU_TRANSLATE_APP_ID` | 百度翻译 APP ID | ✅（翻译模块） |
| `BAIDU_TRANSLATE_SECRET` | 百度翻译 Secret | ✅（翻译模块） |
| `ALIYUN_VISION_ACCESS_KEY_ID` | 阿里云 AccessKey ID（背景移除） | ✅（白底图模块） |
| `ALIYUN_VISION_ACCESS_KEY_SECRET` | 阿里云 AccessKey Secret | ✅（白底图模块） |
| `OSS_BUCKET` | 阿里云 OSS Bucket 名称（复用 GeeKAI 已有配置） | ✅ |
| `AI_COMMERCE_QUEUE_NAME` | Redis 任务队列名，默认 `ai_commerce_tasks` | |
| `AI_COMMERCE_WORKER_CONCURRENCY` | Worker 并发数，默认 `3` | |
| `AI_COMMERCE_ASSET_URL_TTL` | 资产 URL 有效期（秒），默认 `3600` | |

---

## 3. 数据库迁移

在 MySQL 中执行以下脚本（需连接到 GeeKAI 数据库）：

```bash
mysql -u root -p geekai_db < database/aicommerce_migration.sql
```

脚本创建 4 张表：
- `geekai_ai_image_tasks` — 生图任务
- `geekai_ai_image_assets` — 生成资产（OSS 引用）
- `geekai_ai_prompt_templates` — Prompt 模板
- `geekai_ai_model_price_config` — 模型积分定价

并插入默认定价数据（kolors=10, flux=15, hunyuan=12, rembg=5, translate=8）。

---

## 4. 首批 Prompt 模板导入

进入管理后台 → 电商生图 → Prompt 模板，点击「新增模板」手动添加，或使用以下 SQL 批量导入。

**模板字段说明：**
- `module`：`main_image` / `detail_page` / `white_bg` / `clone` / `ratio_convert` / `translate`
- `image_type`：图片类型（如 `lifestyle_scene`）
- `platform`：`generic` / `taobao` / `jingdong` / `amazon` / `douyin`
- `language`：`zh-CN` / `en-US`
- `ratio`：`any` / `1:1` / `4:3` / `3:4` / `16:9` / `9:16`
- `user_template`：支持 Go template 变量：
  - `{{.ProductName}}` — 产品名称
  - `{{.SellingPoints}}` — 卖点描述
  - `{{.ImageTypeDesc}}` — 图片类型描述
  - `{{.PlatformRules}}` — 平台风格规则
  - `{{.Language}}` — 输出语言
  - `{{.Ratio}}` — 图片比例
  - `{{.StyleDesc}}` — 风格描述

**示例模板（主图/生活场景/通用/中文）：**

```sql
INSERT INTO geekai_ai_prompt_templates
  (template_key, module, image_type, platform, language, ratio, model, user_template, negative_template, status, version, created_at, updated_at)
VALUES
  ('main_image:lifestyle_scene:generic:zh-CN:any', 'main_image', 'lifestyle_scene', 'generic', 'zh-CN', 'any', 'kolors',
   '商业摄影，{{.ProductName}}，{{.ImageTypeDesc}}，{{.SellingPoints}}，高端质感，自然光线，清晰细节，专业布景，4K超清',
   'blurry, low quality, watermark, text, ugly, deformed, cartoon',
   'active', 1, NOW(), NOW());
```

---

## 5. 后端构建与启动

```bash
cd api
go build -o geekai-server .
./geekai-server
```

Worker 协程随 API 进程自动启动（无需单独部署）。

---

## 6. 前端构建

```bash
cd web
npm install
npm run build
```

构建产物在 `web/dist/`，通过 Nginx 静态服务即可。

---

## 7. Docker Compose 部署

```bash
cd docker

# 1. 复制并编辑环境变量文件
cp .env.example .env
vim .env   # 填写各服务 API Key

# 2. 执行数据库迁移
docker exec -i geekai-mysql mysql -u root -pmhSCk0NheGhmtsha geekai_db < ../database/aicommerce_migration.sql

# 3. 重启服务生效新环境变量
docker compose up -d --force-recreate geekai-api
```

---

## 8. 管理后台配置

登录 `/admin` → 电商生图菜单：

1. **积分定价**：核对各模型价格，可按需调整
2. **Prompt 模板**：添加/发布首批模板（至少保证 `generic` + `zh-CN` + `any` 的兜底模板存在）
3. **任务审计**：可实时查看任务状态和错误日志

---

## 9. 验证清单

- [ ] 数据库 4 张表已创建，定价数据已插入
- [ ] 管理后台可正常访问「电商生图」菜单
- [ ] 发布至少 1 条 Prompt 模板（status=active）
- [ ] 前端 `/ecom` 页面可正常访问
- [ ] 提交一张白底图任务，验证 Worker 正常处理
- [ ] 确认 OSS 图片 URL 可公网访问
