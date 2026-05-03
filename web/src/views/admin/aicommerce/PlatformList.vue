<template>
  <div class="container">
    <div class="handle-box">
      <el-button type="primary" @click="add">新增平台配置</el-button>
    </div>

    <el-table :data="items" stripe border style="width:100%;margin-top:12px">
      <el-table-column prop="sort_order" label="排序" width="70" />
      <el-table-column prop="value" label="平台标识" width="120" />
      <el-table-column prop="label" label="显示名称" width="120" />
      <el-table-column prop="default_language" label="默认语言" width="100" />
      <el-table-column prop="default_ratio" label="默认比例" width="100" />
      <el-table-column label="平台约束" width="180">
        <template #default="scope">
          <el-tag v-if="scope.row.constraints?.force_white_bg" size="small" type="warning" style="margin-right:4px">强制白底</el-tag>
          <el-tag v-if="scope.row.constraints?.no_text_overlay" size="small" type="danger">禁文字覆盖</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100">
        <template #default="scope">
          <el-switch
            v-model="scope.row.status"
            active-value="active"
            inactive-value="disabled"
            @change="toggleStatus(scope.row)"
          />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="scope">
          <el-button size="small" @click="edit(scope.row)">编辑</el-button>
          <el-popconfirm title="确定删除该配置？" @confirm="remove(scope.row)">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="showDialog" :title="dialogTitle" width="600px">
      <el-form :model="item" label-width="110px" ref="formRef" :rules="rules">
        <el-form-item label="平台标识" prop="value">
          <el-input v-model="item.value" placeholder="如 pinduoduo, amazon" :disabled="!!item.id" />
        </el-form-item>
        <el-form-item label="显示名称" prop="label">
          <el-input v-model="item.label" placeholder="平台中文名" />
        </el-form-item>
        <el-form-item label="默认语言">
          <el-select v-model="item.default_language" style="width:100%">
            <el-option value="zh-CN" label="简体中文" />
            <el-option value="en-US" label="English" />
          </el-select>
        </el-form-item>
        <el-form-item label="默认比例">
          <el-select v-model="item.default_ratio" style="width:100%">
            <el-option v-for="r in ratios" :key="r.value" :value="r.value" :label="r.label" />
          </el-select>
        </el-form-item>
        <el-form-item label="提示词风格">
          <el-input v-model="item.prompt_style" type="textarea" :rows="3" placeholder="该平台的 AI 绘图提示词增强风格" />
        </el-form-item>
        
        <el-divider content-position="left">图片优先级配置</el-divider>
        <el-form-item label="必选图片">
          <el-select v-model="item.priority_images.must_have" multiple filterable style="width:100%">
            <el-option v-for="opt in imageTypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="推荐图片">
          <el-select v-model="item.priority_images.recommended" multiple filterable style="width:100%">
            <el-option v-for="opt in imageTypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="可选图片">
          <el-select v-model="item.priority_images.optional" multiple filterable style="width:100%">
            <el-option v-for="opt in imageTypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </el-form-item>

        <el-divider content-position="left">平台约束</el-divider>
        <el-form-item label="强制白底">
          <el-switch v-model="item.constraints.force_white_bg" />
        </el-form-item>
        <el-form-item label="不要文字覆盖">
          <el-switch v-model="item.constraints.no_text_overlay" />
        </el-form-item>

        <el-divider />
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
import { useEcomConfigStore } from '@/store/ecom'

const configStore = useEcomConfigStore()
const ratios = configStore.ratios

const items = ref([])
const item = ref({
  priority_images: { must_have: [], recommended: [], optional: [] },
  constraints: { force_white_bg: false, no_text_overlay: false }
})
const showDialog = ref(false)
const dialogTitle = ref('')
const formRef = ref(null)

const rules = {
  value: [{ required: true, message: '请填写平台标识', trigger: 'blur' }],
  label: [{ required: true, message: '请填写显示名称', trigger: 'blur' }],
}

const imageTypeOptions = [
  ...configStore.mainImageTypes,
  ...configStore.detailPageTypes
]

const fetchData = () => {
  httpGet('/api/admin/ai-commerce/platform-configs')
    .then((res) => { items.value = res.data?.items || [] })
    .catch((err) => ElMessage.error(err.message))
}

const add = () => {
  item.value = { 
    default_language: 'zh-CN',
    default_ratio: '1:1',
    priority_images: { must_have: [], recommended: [], optional: [] },
    constraints: { force_white_bg: false, no_text_overlay: false },
    sort_order: 0, 
    status: 'active' 
  }
  dialogTitle.value = '新增平台配置'
  showDialog.value = true
}

const edit = (row) => {
  item.value = JSON.parse(JSON.stringify(row))
  const pi = item.value.priority_images || {}
  item.value.priority_images = {
    must_have:   Array.isArray(pi.must_have)   ? pi.must_have   : [],
    recommended: Array.isArray(pi.recommended) ? pi.recommended : [],
    optional:    Array.isArray(pi.optional)    ? pi.optional    : [],
  }
  const co = item.value.constraints || {}
  item.value.constraints = {
    force_white_bg:  !!co.force_white_bg,
    no_text_overlay: !!co.no_text_overlay,
  }
  dialogTitle.value = '编辑平台配置'
  showDialog.value = true
}

const save = () => {
  formRef.value.validate((valid) => {
    if (!valid) return
    httpPost('/api/admin/ai-commerce/platform-configs/save', item.value)
      .then(() => {
        ElMessage.success('保存成功')
        showDialog.value = false
        fetchData()
        configStore.loadPlatformConfigs() // 同步更新 store
      })
      .catch((err) => ElMessage.error(err.message))
  })
}

const toggleStatus = (row) => {
  httpPost('/api/admin/ai-commerce/platform-configs/save', row)
    .then(() => {
      ElMessage.success(`配置已${row.status === 'active' ? '启用' : '禁用'}`)
      configStore.loadPlatformConfigs()
    })
    .catch((err) => {
      ElMessage.error(err.message)
      row.status = row.status === 'active' ? 'disabled' : 'active' // 还原
    })
}

const remove = (row) => {
  httpGet('/api/admin/ai-commerce/platform-configs/remove?id=' + row.id)
    .then(() => {
      ElMessage.success('删除成功')
      items.value = items.value.filter((r) => r.id !== row.id)
      configStore.loadPlatformConfigs() // 同步更新 store
    })
    .catch((err) => ElMessage.error(err.message))
}

onMounted(fetchData)
</script>

<style scoped>
.container { padding: 16px; }
.handle-box { display: flex; align-items: center; }
</style>
