<template>
  <div class="uploader-wrap">
    <el-upload
      v-model:file-list="fileList"
      :http-request="doUpload"
      :before-upload="beforeUpload"
      :on-remove="onRemove"
      :on-preview="handlePreview"
      :multiple="multiple"
      :limit="limit"
      list-type="picture-card"
      accept="image/jpeg,image/png,image/webp"
      drag
    >
      <el-icon><Plus /></el-icon>
      <div class="el-upload__text">拖拽或点击上传</div>
    </el-upload>
    <div class="uploader-tip">支持 JPG/PNG/WEBP，单张不超过 {{ maxSizeMB }}MB</div>

    <!-- 全功能图片预览（支持缩放、翻页） -->
    <el-image-viewer
      v-if="previewVisible"
      :url-list="previewUrlList"
      :initial-index="previewIndex"
      teleported
      @close="previewVisible = false"
    />
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { httpPost } from '@/utils/http'

const props = defineProps({
  multiple: { type: Boolean, default: false },
  limit: { type: Number, default: 5 },
  maxSizeMB: { type: Number, default: 10 },
})

const emit = defineEmits(['update:assetNos'])

const fileList = ref([])
const assetNos = ref([])
const previewVisible = ref(false)
const previewIndex = ref(0)

// 所有已上传文件的可预览 URL
const previewUrlList = computed(() =>
  fileList.value.map((f) => f.url || (f.raw ? URL.createObjectURL(f.raw) : ''))
)

const handlePreview = (file) => {
  const idx = fileList.value.findIndex((f) => f.uid === file.uid)
  previewIndex.value = idx >= 0 ? idx : 0
  previewVisible.value = true
}

const beforeUpload = (file) => {
  const allowed = ['image/jpeg', 'image/png', 'image/webp']
  if (!allowed.includes(file.type)) {
    ElMessage.error('仅支持 JPG/PNG/WEBP 格式')
    return false
  }
  if (file.size > props.maxSizeMB * 1024 * 1024) {
    ElMessage.error(`图片大小不能超过 ${props.maxSizeMB}MB`)
    return false
  }
  return true
}

const doUpload = async ({ file, onSuccess, onError }) => {
  const formData = new FormData()
  formData.append('file', file)
  try {
    const res = await httpPost('/api/ai-commerce/assets', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    assetNos.value.push(res.data.asset_no)
    emit('update:assetNos', [...assetNos.value])
    onSuccess(res)
  } catch (e) {
    ElMessage.error('上传失败：' + e.message)
    onError(e)
  }
}

const onRemove = (_, list) => {
  assetNos.value = list.filter((f) => f.response?.data?.asset_no).map((f) => f.response.data.asset_no)
  emit('update:assetNos', [...assetNos.value])
}
</script>

<style scoped>
.uploader-wrap { display: flex; flex-direction: column; gap: 6px; }
.uploader-tip { font-size: 12px; color: #909399; }
</style>
