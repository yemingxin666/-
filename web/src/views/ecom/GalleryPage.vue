<template>
  <div class="gallery-page">
    <div class="gallery-header">
      <el-tabs v-model="galleryStore.moduleFilter" @tab-change="onModuleChange">
        <el-tab-pane label="全部" name="" />
        <el-tab-pane label="主图设计" name="main_image" />
        <el-tab-pane label="详情页" name="detail_page" />
        <el-tab-pane label="白底图" name="white_bg" />
        <el-tab-pane label="克隆设计" name="clone" />
        <el-tab-pane label="比例转换" name="ratio_convert" />
        <el-tab-pane label="图文翻译" name="translate" />
      </el-tabs>
    </div>

    <div v-loading="galleryStore.loading" class="gallery-body">
      <div v-if="!galleryStore.loading && !galleryStore.items.length" class="gallery-empty">
        <div class="empty-icon">🖼</div>
        <p class="empty-title">暂无历史记录</p>
        <p class="empty-tip">生成的图片将在这里展示</p>
      </div>

      <div class="gallery-grid" v-else>
        <div v-for="task in galleryStore.items" :key="task.task_no" class="gallery-item">
          <div class="task-meta">
            <el-tag size="small" type="primary" effect="light">{{ moduleLabel(task.module) }}</el-tag>
            <span class="task-date">{{ formatDate(task.created_at) }}</span>
          </div>
          <div class="task-outputs" v-if="task.outputs?.length">
            <EcomResultCard
              v-for="(url, i) in task.outputs"
              :key="i"
              :url="url"
              @regenerate="() => {}"
              @delete="galleryStore.deleteTask(task.task_no)"
            />
          </div>
          <div v-else class="task-empty">暂无输出图片</div>
        </div>
      </div>
    </div>

    <div class="gallery-footer">
      <el-pagination
        v-model:current-page="galleryStore.page"
        v-model:page-size="galleryStore.pageSize"
        :total="galleryStore.total"
        :page-sizes="[20, 40]"
        layout="total, sizes, prev, pager, next"
        @current-change="galleryStore.fetchGallery"
        @size-change="galleryStore.fetchGallery"
      />
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useEcomGalleryStore } from '@/store/ecom'
import EcomResultCard from '@/components/ecom/EcomResultCard.vue'

const galleryStore = useEcomGalleryStore()

const moduleMap = { main_image: '主图设计', detail_page: '详情页', white_bg: '白底图', clone: '克隆设计', ratio_convert: '比例转换', translate: '图文翻译' }
const moduleLabel = (m) => moduleMap[m] || m
const formatDate = (t) => t ? new Date(t).toLocaleDateString('zh-CN') : ''

const onModuleChange = () => {
  galleryStore.page = 1
  galleryStore.fetchGallery()
}

onMounted(() => galleryStore.fetchGallery())
</script>

<style scoped>
.gallery-page {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  background: var(--gray-btn-bg);
  overflow: hidden;
}

.gallery-header {
  flex-shrink: 0;
  padding: 0 20px;
  background: var(--theme-bg);
  border-bottom: 1px solid var(--theme-border-primary);
}

:deep(.el-tabs__nav-wrap::after) { height: 1px; background: var(--theme-border-primary); }
:deep(.el-tabs__item) { font-size: 13px; color: var(--text-secondary); }
:deep(.el-tabs__item.is-active) { color: var(--el-color-primary); font-weight: 600; }
:deep(.el-tabs__active-bar) { background: var(--el-color-primary); }
:deep(.el-tabs__header) { margin-bottom: 0; }

.gallery-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 0;
  background: var(--gray-btn-bg);
}

.gallery-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
  padding: 0 20px;
}

@media (min-width: 1200px) {
  .gallery-grid { grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); }
}
@media (min-width: 1600px) {
  .gallery-grid { grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 18px; }
}
@media (min-width: 1920px) {
  .gallery-grid { grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 20px; }
}

.gallery-item {
  background: var(--theme-bg);
  border: 1px solid var(--theme-border-primary);
  border-radius: 8px;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: box-shadow 0.2s, transform 0.2s;
}
.gallery-item:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
  transform: translateY(-2px);
}

.task-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.task-date { font-size: 12px; color: var(--text-secondary); }

/* 单图：充满卡片；多图：2 列网格 */
.task-outputs { display: grid; grid-template-columns: 1fr; gap: 6px; }
.task-outputs:has(> *:nth-child(2)) { grid-template-columns: 1fr 1fr; }

.task-empty { font-size: 13px; color: var(--text-secondary); text-align: center; padding: 12px 0; }

.gallery-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  opacity: 0.6;
}
.empty-icon { font-size: 56px; margin-bottom: 16px; }
.empty-title { font-size: 15px; color: var(--text-secondary); margin: 0 0 6px; font-weight: 500; }
.empty-tip { font-size: 13px; color: var(--text-secondary); margin: 0; }

.gallery-footer {
  flex-shrink: 0;
  padding: 12px 24px 16px;
  background: var(--theme-bg);
  border-top: 1px solid var(--theme-border-primary);
  display: flex;
  justify-content: flex-end;
}
</style>
