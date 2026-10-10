---
name: openspec-archive-change
description: Archive a completed change in the experimental workflow. Use when the
  user wants to finalize and archive a change after implementation is complete.
license: MIT
metadata:
  author: openspec
  version: '1.0'
  generatedBy: 1.3.1
  compatibility: Requires openspec CLI.
---

# 归档完成变更

仅在用户请求或确认归档时执行，遵循`AGENTS.md`、`.agents/rules/openspec.md`和`.agents/rules/context-loading.md`。不得因任务全勾选就自动归档，也不得触发其他手动限定归档技能。

1. 确定用户授权归档的变更；未指明且授权对象不明确时，用`openspec list --json`列活跃候选并询问，不自行选择。
2. 运行`openspec status --change "<name>" --json`并核对任务完成情况；缺失产物或未完成任务按规则处理，不能默认视为已完成。
3. 调用`lina-review`覆盖整个变更，并运行`openspec validate <change> --strict`。强制门禁不通过则先处理阻断，不以归档警告代替验证。
4. 读取该变更每个完整增量需求及其`Scenario`，与受影响 baseline 的完整需求比较，汇总新增、修改、删除和重命名；无关 baseline 不读取。已读且未变化正文可复用，正文丢失或范围变化时重读。
5. 同步需求时保留完整需求块和验收语义，确认删除或重命名不会遗留旧条目。依照规则和用户已授权的同步选择执行；同步取舍尚未确定时展示具体差异并确认，不静默丢弃增量规范。
6. 使用用户时区当前日期构造`openspec/changes/archive/YYYY-MM-DD-<name>`；目标已存在则停止，禁止覆盖。移动整个变更目录，保留`.openspec.yaml`及其余记录。
7. 报告归档路径、同步结果、实际验证和未解决事项。文档语言按唯一规则处理，不为了归档额外翻译；不自动提交或推送。
