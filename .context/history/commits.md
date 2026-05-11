# Commit Decision History

> 此文件是 `commits.jsonl` 的人类可读视图，可由工具重生成。
> Canonical store: `commits.jsonl` (JSONL, append-only)

| Date | Context-Id | Commit | Summary | Decisions | Bugs | Risk |
|------|-----------|--------|---------|-----------|------|------|
| 2026-05-09 | 17d68088 | d2eb0ae3 | chore(context): 初始化 .context 决策追踪目录与 CLAUDE.md 引用 | 启用 .context 决策追踪体系 | - | low |
| 2026-05-10 | bbed9840 | f075ed9c | feat(aicommerce): 新增详情页参考图尺码表识别与注入链路 | Vision Prompt 注册表路由; size_chart JSON 跳过转义; 独立生命周期; Normalize 降级压缩 | - | medium |
| 2026-05-10 | 7955b1f0 | 01d81b1c | feat(ecom): 主图/详情/克隆页生图卡支持编辑功能并替换重新生成 | ImageTaskItem 新增 asset_no; clone 合成 items; useEcomEdit 抽离; 留页 + 通知跳转 | - | medium |
| 2026-05-10 | 64c8c521 | 7278918a | fix(ecom): 历史图库单图删除误删整任务的多张图 | 新增 asset 软删接口; 空任务级联清理; 兼容旧数据回退 | 单图删除→整任务消失 | medium |
| 2026-05-11 | 1354e0c6 | fafc2a87 | feat(ecom): 优化图片预览交互并隐藏历史图库提示词 | hide-on-click-modal; blur 移除焦点; PromptJSON nil 屏蔽 | - | low |
