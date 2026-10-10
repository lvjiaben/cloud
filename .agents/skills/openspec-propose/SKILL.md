---
name: openspec-propose
description: Propose a new change with all artifacts generated in one step. Use when
  the user wants to quickly describe what they want to build and get a complete proposal
  with design, specs, and tasks ready for implementation.
license: MIT
metadata:
  author: openspec
  version: '1.0'
  generatedBy: 1.3.1
  compatibility: Requires openspec CLI.
---

# 创建变更提案

将清晰的需求生成可实施的最小变更记录。规则入口为`AGENTS.md`、`.agents/rules/openspec.md`和`.agents/rules/context-loading.md`；产物语言遵循文档规则。

1. 从请求提取目标、范围和验收；业务或交互存在真实歧义时先按顶层澄清门禁确认，不自行补业务规则。内部实现选择可按规则自主处理并记录依据。
2. 已有相关活跃变更时先检查其相关任务与完整需求，避免重复建变更；确需新建时取`kebab-case`名称，运行`openspec new change "<name>"`。同名已存在时先判断是否为用户要继续的变更，归属不明确才询问。
3. 运行`openspec status --change "<name>" --json`了解 schema 和`applyRequires`；按依赖顺序使用`openspec instructions <artifact-id> --change "<name>" --json`获取模板、输出路径和约束。
4. 依赖路径只是候选。先读相关完整章节、完整`Requirement`及其`Scenario`；同一任务未变化且正文仍可见的产物与规则可复用。依赖不明确、正文丢失或范围扩展时扩大读取，不批量读取所有规范。
5. 按 schema 生成必要产物，保留可验证需求、必要设计决策和具体任务。`context`与`rules`用于约束，不复制到文档；不重复偏好、领域规则或无关历史。
6. 更新状态并运行规则要求的严格校验，确认必要产物完成。输出变更名称、产物路径、范围和验证结果；只有用户已授权实施时继续实现，否则报告提案已就绪。
