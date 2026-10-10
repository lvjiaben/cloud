---
name: openspec-explore
description: Enter explore mode - a thinking partner for exploring ideas, investigating
  problems, and clarifying requirements. Use when the user wants to think through
  something before or during a change.
license: MIT
metadata:
  author: openspec
  version: '1.0'
  generatedBy: 1.3.1
  compatibility: Requires openspec CLI.
---

# 探索需求与问题

用于用户希望先讨论、调查或厘清需求的场景。只读调查与对话；探索不实施产品代码。用户要求落地时进入相应实施流程，不把明确授权变成重复确认。

- 以当前问题为入口，先搜索路径、符号和规则标题，再读直接相关源码与完整需求章节。规则入口为`AGENTS.md`、`.agents/rules/openspec.md`和`.agents/rules/context-loading.md`。
- 需要了解变更时用`openspec list --json`列候选；仅读取用户指定或高度相关变更的目标任务、完整`Requirement`及其`Scenario`和必要设计章节，不展开所有活跃变更或归档。
- 区分事实、假设和待确认业务结论。需要改变业务或交互且现有约束不能唯一确定答案时，遵守顶层澄清门禁。
- 同一任务正文仍可见且文件未变化时复用；跨会话、正文丢失、文件变化或范围扩展时重新读取必要内容。
- 只在用户要求保存探索成果或创建提案时写文档；将确认结论保存到对应偏好/规范，不为普通讨论自动创建变更。
- 输出与问题有关的发现、可选方案及取舍、未决条件和可执行下一步，避免通用教学、无关例子和固定长报告。
