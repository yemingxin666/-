<template>
  <div class="container">
    <div class="handle-box">
      <el-button type="primary" @click="add">新增定价</el-button>
    </div>

    <el-table :data="items" stripe border style="width:100%;margin-top:12px">
      <el-table-column prop="model" label="模型名称" width="180" />
      <el-table-column prop="module" label="适用模块" width="130" />
      <el-table-column prop="credit_per_image" label="每张积分" width="100" />
      <el-table-column prop="description" label="描述" />
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
          <el-popconfirm title="确定删除？" @confirm="remove(scope.row)">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showDialog" :title="dialogTitle" width="500px">
      <el-form :model="item" label-width="100px" ref="formRef" :rules="rules">
        <el-form-item label="模型名称" prop="model">
          <el-input v-model="item.model" placeholder="如 kolors、flux、rembg" />
        </el-form-item>
        <el-form-item label="适用模块">
          <el-select v-model="item.module" style="width:100%">
            <el-option value="all" label="所有模块" />
            <el-option value="main_image" label="主图设计" />
            <el-option value="detail_page" label="详情页设计" />
            <el-option value="white_bg" label="白底图" />
            <el-option value="clone" label="克隆设计" />
            <el-option value="ratio_convert" label="比例转换" />
            <el-option value="translate" label="图文翻译" />
          </el-select>
        </el-form-item>
        <el-form-item label="每张积分" prop="credit_per_image">
          <el-input-number v-model="item.credit_per_image" :min="1" :max="9999" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="item.description" placeholder="可选备注" />
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
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { httpGet, httpPost } from '@/utils/http'

const items = ref([])
const item = ref({})
const showDialog = ref(false)
const dialogTitle = ref('')
const formRef = ref(null)

const rules = {
  model: [{ required: true, message: '请填写模型名称', trigger: 'blur' }],
  credit_per_image: [{ required: true, message: '请填写积分', trigger: 'change' }],
}

const fetchData = () => {
  httpGet('/api/admin/ai-commerce/prices').then((res) => {
    items.value = res.data
  }).catch((err) => ElMessage.error(err.message))
}

const add = () => {
  item.value = { module: 'all', credit_per_image: 10, status: 'active' }
  dialogTitle.value = '新增定价'
  showDialog.value = true
}

const edit = (row) => {
  item.value = { ...row }
  dialogTitle.value = '编辑定价'
  showDialog.value = true
}

const save = () => {
  formRef.value.validate((valid) => {
    if (!valid) return
    httpPost('/api/admin/ai-commerce/prices/save', item.value).then(() => {
      ElMessage.success('保存成功')
      showDialog.value = false
      fetchData()
    }).catch((err) => ElMessage.error(err.message))
  })
}

const remove = (row) => {
  httpGet('/api/admin/ai-commerce/prices/remove?id=' + row.id).then(() => {
    ElMessage.success('删除成功')
    items.value = items.value.filter((r) => r.id !== row.id)
  }).catch((err) => ElMessage.error(err.message))
}

onMounted(fetchData)
</script>

<style scoped>
.container { padding: 16px; }
.handle-box { display: flex; align-items: center; }
</style>
