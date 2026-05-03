<template>
  <div class="container">
    <div class="handle-box">
      <el-select v-model="query.module" placeholder="模块" clearable style="width:140px">
        <el-option v-for="m in moduleOptions" :key="m.value" :value="m.value" :label="m.label" />
      </el-select>
      <el-select v-model="query.status" placeholder="状态" clearable style="width:120px;margin-left:8px">
        <el-option value="draft" label="草稿" />
        <el-option value="active" label="已发布" />
        <el-option value="archived" label="已归档" />
      </el-select>
      <el-button @click="fetchData" style="margin-left:8px">搜索</el-button>
      <el-button type="primary" @click="add" style="margin-left:8px">新增模板</el-button>
    </div>

    <el-table :data="items" stripe border style="width:100%;margin-top:12px">
      <el-table-column prop="module" label="模块" width="120" />
      <el-table-column prop="image_type" label="图片类型" width="140" />
      <el-table-column prop="version" label="版本" width="60" />
      <el-table-column prop="status" label="状态" width="90">
        <template #default="scope">
          <el-tag :type="statusTagType(scope.row.status)">{{ statusLabel(scope.row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="scope">
          <el-button size="small" @click="edit(scope.row)">编辑</el-button>
          <el-button size="small" type="success" @click="setStatus(scope.row, 'active')"
            v-if="scope.row.status !== 'active'">发布</el-button>
          <el-button size="small" type="warning" @click="setStatus(scope.row, 'archived')"
            v-if="scope.row.status === 'active'">归档</el-button>
          <el-popconfirm title="确定删除？" @confirm="remove(scope.row)">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- 编辑对话框 -->
    <el-dialog v-model="showDialog" :title="dialogTitle" width="900px" destroy-on-close>
      <el-form :model="item" label-width="100px" ref="formRef" :rules="rules">
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="模块" prop="module">
              <el-select v-model="item.module" style="width:100%">
                <el-option v-for="m in moduleOptions" :key="m.value" :value="m.value" :label="m.label" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="图片类型" prop="image_type">
              <el-input v-model="item.image_type" placeholder="如 lifestyle_scene" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="正向模板" prop="user_template">
          <el-input v-model="item.user_template" type="textarea" :rows="6"
            placeholder="支持 Go template 变量：{{.ProductName}} {{.SellingPoints}} 等" />
        </el-form-item>
        <el-form-item label="负向模板">
          <el-input v-model="item.negative_template" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="系统提示词">
          <el-input v-model="item.system_prompt" type="textarea" :rows="2" />
        </el-form-item>

        <!-- 实时预览区域 (6.3) -->
        <el-divider>实时预览</el-divider>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="产品名称">
              <el-input v-model="preview.product_name" placeholder="填入预览变量" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="卖点">
              <el-input v-model="preview.selling_points" placeholder="卖点描述" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item>
          <el-button type="primary" @click="doPreview" :loading="previewing">渲染预览</el-button>
        </el-form-item>
        <el-form-item label="正向结果" v-if="previewResult.positive">
          <el-input v-model="previewResult.positive" type="textarea" :rows="4" readonly />
        </el-form-item>
        <el-form-item label="负向结果" v-if="previewResult.negative">
          <el-input v-model="previewResult.negative" type="textarea" :rows="2" readonly />
        </el-form-item>
        <el-alert v-if="previewError" :title="previewError" type="error" :closable="false" />
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { httpGet, httpPost } from '@/utils/http'

const items = ref([])
const query = reactive({ module: '', status: '' })
const item = ref({})
const showDialog = ref(false)
const dialogTitle = ref('')
const formRef = ref(null)
const previewing = ref(false)
const preview = reactive({ product_name: '', selling_points: '' })
const previewResult = reactive({ positive: '', negative: '' })
const previewError = ref('')

const moduleOptions = [
  { value: 'main_image', label: '主图设计' },
  { value: 'detail_page', label: '详情页设计' },
  { value: 'white_bg', label: '白底图' },
  { value: 'clone', label: '克隆设计' },
  { value: 'ratio_convert', label: '比例转换' },
  { value: 'translate', label: '图文翻译' },
]

const rules = {
  module: [{ required: true, message: '请选择模块', trigger: 'change' }],
  image_type: [{ required: true, message: '请填写图片类型', trigger: 'blur' }],
  user_template: [{ required: true, message: '请填写正向模板', trigger: 'blur' }],
}

const statusTagType = (s) => ({ draft: 'info', active: 'success', archived: 'warning' }[s] || '')
const statusLabel = (s) => ({ draft: '草稿', active: '已发布', archived: '已归档' }[s] || s)

const fetchData = () => {
  httpGet('/api/admin/ai-commerce/templates', query).then((res) => {
    items.value = res.data
  }).catch((err) => ElMessage.error(err.message))
}

const add = () => {
  item.value = { module: 'main_image', status: 'draft' }
  dialogTitle.value = '新增 Prompt 模板'
  previewResult.positive = ''
  previewResult.negative = ''
  previewError.value = ''
  showDialog.value = true
}

const edit = (row) => {
  item.value = { ...row }
  dialogTitle.value = '编辑 Prompt 模板'
  previewResult.positive = ''
  previewResult.negative = ''
  previewError.value = ''
  showDialog.value = true
}

const save = () => {
  formRef.value.validate((valid) => {
    if (!valid) return
    httpPost('/api/admin/ai-commerce/templates/save', item.value).then((res) => {
      ElMessage.success('保存成功')
      showDialog.value = false
      fetchData()
    }).catch((err) => ElMessage.error(err.message))
  })
}

const remove = (row) => {
  httpGet('/api/admin/ai-commerce/templates/remove?id=' + row.id).then(() => {
    ElMessage.success('删除成功')
    items.value = items.value.filter((r) => r.id !== row.id)
  }).catch((err) => ElMessage.error(err.message))
}

const setStatus = (row, status) => {
  httpPost('/api/admin/ai-commerce/templates/status', { id: row.id, status }).then(() => {
    row.status = status
    ElMessage.success('状态已更新')
  }).catch((err) => ElMessage.error(err.message))
}

const doPreview = () => {
  previewError.value = ''
  previewResult.positive = ''
  previewResult.negative = ''
  previewing.value = true
  httpPost('/api/admin/ai-commerce/templates/preview', {
    user_template: item.value.user_template,
    negative_template: item.value.negative_template,
    product_name: preview.product_name,
    selling_points: preview.selling_points,
    image_type: item.value.image_type,
  }).then((res) => {
    previewResult.positive = res.data.positive
    previewResult.negative = res.data.negative
  }).catch((err) => {
    previewError.value = err.message
  }).finally(() => {
    previewing.value = false
  })
}

onMounted(fetchData)
</script>

<style scoped>
.container { padding: 16px; }
.handle-box { display: flex; align-items: center; flex-wrap: wrap; gap: 4px; }
</style>
