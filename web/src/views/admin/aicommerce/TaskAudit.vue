<template>
  <div class="container">
    <div class="handle-box">
      <el-input v-model="query.user_id" placeholder="用户ID" clearable style="width:110px" />
      <el-select v-model="query.module" placeholder="模块" clearable style="width:130px;margin-left:8px">
        <el-option value="main_image" label="主图设计" />
        <el-option value="detail_page" label="详情页设计" />
        <el-option value="white_bg" label="白底图" />
        <el-option value="clone" label="克隆设计" />
        <el-option value="ratio_convert" label="比例转换" />
        <el-option value="translate" label="图文翻译" />
      </el-select>
      <el-select v-model="query.status" placeholder="状态" clearable style="width:110px;margin-left:8px">
        <el-option value="pending" label="待处理" />
        <el-option value="queued" label="队列中" />
        <el-option value="running" label="执行中" />
        <el-option value="succeeded" label="成功" />
        <el-option value="failed" label="失败" />
        <el-option value="cancelled" label="已取消" />
      </el-select>
      <el-date-picker v-model="query.start_date" type="date" placeholder="开始日期"
        value-format="YYYY-MM-DD" style="width:140px;margin-left:8px" />
      <el-date-picker v-model="query.end_date" type="date" placeholder="结束日期"
        value-format="YYYY-MM-DD" style="width:140px;margin-left:8px" />
      <el-button @click="fetchData" style="margin-left:8px">搜索</el-button>
    </div>

    <el-table :data="items" stripe border style="width:100%;margin-top:12px">
      <el-table-column prop="task_no" label="任务编号" width="200" />
      <el-table-column prop="user_id" label="用户ID" width="80" />
      <el-table-column prop="module" label="模块" width="110" />
      <el-table-column prop="image_type" label="图片类型" width="120" />
      <el-table-column prop="model" label="模型" width="100" />
      <el-table-column prop="credit_cost" label="积分消耗" width="90" />
      <el-table-column prop="status" label="状态" width="90">
        <template #default="scope">
          <el-tag :type="statusTagType(scope.row.status)">{{ statusLabel(scope.row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="error_message" label="错误信息" show-overflow-tooltip />
      <el-table-column label="创建时间" width="160">
        <template #default="scope">{{ formatTime(scope.row.created_at) }}</template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="query.page"
      v-model:page-size="query.page_size"
      :total="total"
      :page-sizes="[20, 50, 100]"
      layout="total, sizes, prev, pager, next"
      style="margin-top:12px"
      @current-change="fetchData"
      @size-change="fetchData"
    />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { httpGet } from '@/utils/http'

const items = ref([])
const total = ref(0)
const query = reactive({
  user_id: '',
  module: '',
  status: '',
  start_date: '',
  end_date: '',
  page: 1,
  page_size: 20,
})

const statusTagType = (s) => ({
  pending: 'info', queued: '', running: 'warning',
  succeeded: 'success', failed: 'danger', cancelled: 'info',
}[s] || '')

const statusLabel = (s) => ({
  pending: '待处理', queued: '队列中', running: '执行中',
  succeeded: '成功', failed: '失败', cancelled: '已取消',
}[s] || s)

const formatTime = (t) => {
  if (!t) return '-'
  return new Date(t).toLocaleString('zh-CN', { hour12: false })
}

const fetchData = () => {
  const params = {}
  Object.entries(query).forEach(([k, v]) => { if (v !== '' && v !== null) params[k] = v })
  httpGet('/api/admin/ai-commerce/tasks', params).then((res) => {
    items.value = res.data.items
    total.value = res.data.total
  }).catch((err) => ElMessage.error(err.message))
}

onMounted(fetchData)
</script>

<style scoped>
.container { padding: 16px; }
.handle-box { display: flex; align-items: center; flex-wrap: wrap; gap: 4px; }
</style>
