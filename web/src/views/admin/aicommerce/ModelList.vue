<template>
  <div class="container">
    <div class="handle-box">
      <el-button type="primary" @click="add">新增模型</el-button>
    </div>

    <el-table :data="items" stripe border style="width:100%;margin-top:12px">
      <el-table-column prop="sort_order" label="排序" width="70" />
      <el-table-column prop="name" label="模型标识" width="160" />
      <el-table-column prop="display_name" label="显示名称" width="160" />
      <el-table-column prop="provider" label="提供商" width="120" />
      <el-table-column prop="model_type" label="类型" width="90">
        <template #default="scope">
          <el-tag type="info">{{ modelTypeLabel(scope.row.model_type) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="api_endpoint" label="API 地址" show-overflow-tooltip />
      <el-table-column prop="description" label="描述" show-overflow-tooltip />
      <el-table-column prop="capabilities" label="能力" width="140">
        <template #default="scope">
          <el-tag v-if="scope.row.capabilities?.includes('text2img')" size="small" type="success" style="margin-right:4px">文生图</el-tag>
          <el-tag v-if="scope.row.capabilities?.includes('img2img')" size="small" type="warning">图生图</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="90">
        <template #default="scope">
          <el-tag :type="scope.row.status === 'active' ? 'success' : 'info'">
            {{ scope.row.status === 'active' ? '启用' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="scope">
          <el-button size="small" @click="edit(scope.row)">编辑</el-button>
          <el-popconfirm title="确定删除该模型？" @confirm="remove(scope.row)">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showDialog" :title="dialogTitle" width="520px">
      <el-form :model="item" label-width="100px" ref="formRef" :rules="rules">
        <el-form-item label="模型标识" prop="name">
          <el-input v-model="item.name" placeholder="如 NanoBanana、GPT-Image-2" :disabled="item.id > 0" />
        </el-form-item>
        <el-form-item label="显示名称" prop="display_name">
          <el-input v-model="item.display_name" placeholder="前端展示用名称" />
        </el-form-item>
        <el-form-item label="提供商" prop="provider">
          <el-select v-model="item.provider" style="width:100%" allow-create filterable>
            <el-option value="openai" label="OpenAI" />
            <el-option value="aliyun" label="阿里云" />
            <el-option value="relay" label="中转代理 (Relay)" />
            <el-option value="custom" label="自定义" />
          </el-select>
        </el-form-item>
        <el-form-item label="模型类型">
          <el-select v-model="item.model_type" style="width:100%">
            <el-option value="image" label="图像生成" />
            <el-option value="text" label="文本生成" />
            <el-option value="video" label="视频生成" />
          </el-select>
        </el-form-item>
        <el-form-item label="API 地址">
          <el-input v-model="item.api_endpoint" placeholder="可选，留空使用默认端点" />
        </el-form-item>
        <el-form-item label="API 密钥">
          <el-input v-model="item.api_key" type="password" show-password placeholder="可选" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="item.description" type="textarea" :rows="2" placeholder="可选备注" />
        </el-form-item>
        <el-form-item label="模型能力">
          <el-select v-model="item.capabilities_array" multiple style="width:100%" placeholder="请选择模型能力">
            <el-option value="text2img" label="文生图 (text2img)" />
            <el-option value="img2img" label="图生图 (img2img)" />
          </el-select>
        </el-form-item>
        <el-form-item label="排序权重">
          <el-input-number v-model="item.sort_order" :min="0" :max="9999" />
          <span style="margin-left:8px;color:#909399;font-size:12px">越小越靠前</span>
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="item.status" active-value="active" inactive-value="disabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { httpGet, httpPost } from '@/utils/http'

const items = ref([])
const item = ref({})
const showDialog = ref(false)
const dialogTitle = ref('')
const formRef = ref(null)

const rules = {
  name: [{ required: true, message: '请填写模型标识', trigger: 'blur' }],
  display_name: [{ required: true, message: '请填写显示名称', trigger: 'blur' }],
  provider: [{ required: true, message: '请选择提供商', trigger: 'change' }],
}

const modelTypeLabel = (type) => {
  const map = { image: '图像', text: '文本', video: '视频' }
  return map[type] || type
}

const fetchData = () => {
  httpGet('/api/admin/ai-commerce/models')
    .then((res) => { items.value = res.data })
    .catch((err) => ElMessage.error(err.message))
}

const add = () => {
  item.value = { model_type: 'image', provider: 'custom', capabilities_array: [], sort_order: 0, status: 'active' }
  dialogTitle.value = '新增 AI 模型'
  showDialog.value = true
}

const edit = (row) => {
  item.value = { ...row }
  item.value.capabilities_array = (row.capabilities || '').split(',').filter(Boolean)
  dialogTitle.value = '编辑 AI 模型'
  showDialog.value = true
}

const save = () => {
  formRef.value.validate((valid) => {
    if (!valid) return
    item.value.capabilities = (item.value.capabilities_array || []).join(',')
    httpPost('/api/admin/ai-commerce/models/save', item.value)
      .then(() => {
        ElMessage.success('保存成功')
        showDialog.value = false
        fetchData()
      })
      .catch((err) => ElMessage.error(err.message))
  })
}

const remove = (row) => {
  httpGet('/api/admin/ai-commerce/models/remove?id=' + row.id)
    .then(() => {
      ElMessage.success('删除成功')
      items.value = items.value.filter((r) => r.id !== row.id)
    })
    .catch((err) => ElMessage.error(err.message))
}

onMounted(fetchData)
</script>

<style scoped>
.container { padding: 16px; }
.handle-box { display: flex; align-items: center; }
</style>
