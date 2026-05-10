// useEcomEdit：统一管理"编辑生成图"弹窗状态 + 提交后跳转提示
// 适用页面：MainImagePage / DetailPagePage / ClonePage 等所有展示生成图的页面
//
// 用法：
//   const { editVisible, editPayload, openEdit, onEditSubmitted } = useEcomEdit()
//   <EcomResultCard :editable="!!item.asset_no" @edit="(p) => openEdit(task, item, p)" />
//   <EcomEditDialog v-model="editVisible" :url="editPayload.url" ... @submitted="onEditSubmitted" />
import { h, inject, ref } from 'vue'
import { ElNotification } from 'element-plus'

export function useEcomEdit() {
  const editVisible = ref(false)
  const editPayload = ref({ url: '', ratio: '1:1', taskNo: '', assetNo: '' })

  // 由 EcomLayout 提供；非 EcomLayout 内使用时提示通知不带跳转链接
  const setModule = inject('setEcomModule', null)

  // taskNoOrTask: 字符串 task_no 或包含 task_no/ratio 的对象
  // item: 必须包含 asset_no（无则不允许编辑）
  // payload: EcomResultCard 的 @edit 事件 payload，至少含 { url, ratio }
  const openEdit = (taskNoOrTask, item, payload = {}) => {
    if (!item || !item.asset_no) return
    const taskNo = typeof taskNoOrTask === 'string' ? taskNoOrTask : taskNoOrTask?.task_no
    const ratio = payload.ratio || (typeof taskNoOrTask === 'object' && taskNoOrTask?.ratio) || '1:1'
    editPayload.value = {
      url: payload.url || item.url || '',
      ratio,
      taskNo: taskNo || '',
      assetNo: item.asset_no,
    }
    editVisible.value = true
  }

  const onEditSubmitted = () => {
    ElNotification({
      title: '编辑任务已提交',
      duration: 5000,
      type: 'success',
      message: setModule
        ? h('div', [
            h('span', '后台正在处理，'),
            h(
              'a',
              {
                href: 'javascript:;',
                style: 'color: var(--el-color-primary); font-weight: 600;',
                onClick: () => setModule('gallery'),
              },
              '前往历史图库查看进度',
            ),
          ])
        : '后台正在处理，可在历史图库查看进度',
    })
  }

  return { editVisible, editPayload, openEdit, onEditSubmitted }
}
