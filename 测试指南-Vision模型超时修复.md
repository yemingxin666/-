# Vision 模型超时修复 - 测试指南

## 修复内容

### 问题
1. Vision API（图片识别）调用超时或失败时，任务状态一直显示"运行中"
2. 代码硬编码查询 `gpt-4o`，无法使用其他 chat 模型

### 解决方案
1. ✅ 为 Vision API 调用添加 **30 秒独立超时**
2. ✅ Vision API 失败时记录日志，但**不阻断生图任务**（这是可选增强功能）
3. ✅ 支持使用任意 `model_type='chat'` 的模型（优先 gpt-4o，不存在则按 sort_order 查找）

### 修改文件
- `api/service/aicommerce/worker/chains/main_image.go`
- `api/service/aicommerce/image_service.go`

---

## 测试步骤

### 1. 检查数据库配置

确认你的新 Vision 模型已正确配置：

```sql
-- 查看当前配置的 chat 模型
SELECT id, name, display_name, model_type, provider, api_endpoint, status, sort_order
FROM geekai_ai_models
WHERE model_type = 'chat'
ORDER BY sort_order ASC, id ASC;
```

**预期结果**：
- 至少有一条 `status='active'` 的记录
- `api_key` 和 `api_endpoint` 不为空
- 如果有多条，`sort_order` 最小的会被优先使用

### 2. 启动后端服务

```bash
cd D:\电商\geekai\api
.\geekai.exe -c config.toml
```

**观察日志**：服务启动后应该没有报错

### 3. 测试场景 A：正常流程

**操作**：
1. 打开前端：主图设计 / 详情页设计 / 克隆设计
2. 上传 1-3 张参考图
3. 填写产品名称（可选）
4. 点击"立即生成"

**预期结果**：
- 任务提交成功
- 后端日志中可以看到 Vision API 调用（如果模型配置正确）
- 任务正常完成，状态变为"成功"

### 4. 测试场景 B：Vision API 超时

**模拟方法**：
1. 在数据库中临时修改 chat 模型的 `api_endpoint` 为一个不存在的地址：
   ```sql
   UPDATE geekai_ai_models
   SET api_endpoint = 'https://invalid-endpoint-test.example.com/v1'
   WHERE model_type = 'chat' AND status = 'active';
   ```

2. 重启后端服务

3. 执行测试场景 A 的操作

**预期结果**：
- 任务提交成功
- 后端日志中会出现类似警告（30 秒后）：
  ```
  [WARN] task 123 vision copywrite failed (non-blocking): context deadline exceeded
  ```
- **关键**：任务不会卡住，会继续执行生图流程
- 任务最终状态变为"成功"（即使 Vision API 失败）

**恢复配置**：
```sql
UPDATE geekai_ai_models
SET api_endpoint = '你的正确endpoint'
WHERE model_type = 'chat' AND status = 'active';
```

### 5. 测试场景 C：Vision 模型未配置

**模拟方法**：
1. 临时禁用所有 chat 模型：
   ```sql
   UPDATE geekai_ai_models
   SET status = 'inactive'
   WHERE model_type = 'chat';
   ```

2. 重启后端服务

3. 执行测试场景 A 的操作

**预期结果**：
- 任务提交成功
- 后端日志中会出现警告：
  ```
  [WARN] task 123 build vision client failed (non-blocking): 未找到可用的视觉识别模型
  ```
- 任务继续执行生图（不使用 Vision 增强）
- 任务最终状态变为"成功"

**恢复配置**：
```sql
UPDATE geekai_ai_models
SET status = 'active'
WHERE model_type = 'chat';
```

### 6. 测试场景 D：多个 chat 模型

**模拟方法**：
1. 在数据库中添加多个 chat 模型，设置不同的 `sort_order`
2. 重启后端服务
3. 执行测试场景 A 的操作

**预期结果**：
- 系统优先使用 `name='gpt-4o'` 的模型
- 如果 gpt-4o 不存在，使用 `sort_order` 最小的模型
- 后端日志中可以看到使用的模型名称

---

## 验证要点

### ✅ 任务不会卡住
- 即使 Vision API 超时/失败，任务也会在 30 秒后继续
- 任务最终状态会正确更新（不会永久显示"运行中"）

### ✅ 日志可追溯
- Vision API 失败时有明确的警告日志
- 日志包含任务 ID 和错误原因

### ✅ 模型灵活配置
- 不再硬编码 gpt-4o
- 支持使用任意 chat 模型
- 按 sort_order 自动选择

### ✅ 不影响生图
- Vision API 是可选增强功能
- 失败时不阻断生图任务
- 用户仍然能得到生成的图片

---

## 常见问题

### Q1: 如何查看后端日志？
**A**: 在启动后端的终端窗口中实时查看，或者查看日志文件（如果配置了日志输出）

### Q2: Vision API 失败后，生成的图片质量会下降吗？
**A**: 可能会。Vision API 用于识别图片特征并生成更精准的卖点描述。失败后会使用用户手动输入的卖点，或者使用默认 Prompt。

### Q3: 如何强制使用特定的 chat 模型？
**A**: 
1. 将目标模型的 `sort_order` 设置为最小值（如 1）
2. 或者将其他模型的 `status` 设置为 `inactive`

### Q4: 30 秒超时是否可以调整？
**A**: 可以。修改 `main_image.go` 中的超时时间：
```go
visionCtx, visionCancel := context.WithTimeout(ctx, 30*time.Second)
// 改为 60 秒：
visionCtx, visionCancel := context.WithTimeout(ctx, 60*time.Second)
```

---

## 回滚方案

如果修复后出现问题，可以回滚：

```bash
cd D:\电商\geekai
git checkout api/service/aicommerce/worker/chains/main_image.go
git checkout api/service/aicommerce/image_service.go
cd api
go build -o geekai.exe main.go
```

---

## 联系支持

如有问题，请提供：
1. 后端日志（包含 WARN 和 ERROR 级别）
2. 任务 ID（task_no）
3. 数据库中 chat 模型的配置（隐藏 api_key）
