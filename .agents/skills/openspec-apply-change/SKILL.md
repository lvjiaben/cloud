---
name: openspec-apply-change
description: Implement tasks from an OpenSpec change. Use when the user wants to start
  implementing, continue implementation, or work through tasks.
license: MIT
metadata:
  author: openspec
  version: '1.0'
  generatedBy: 1.3.1
  compatibility: Requires openspec CLI.
---

# 执行变更任务

实现用户指定或上下文明确的活跃变更。遵循`AGENTS.md`、`.agents/rules/openspec.md`和`.agents/rules/context-loading.md`，不加载无关规范或归档。

1. 选择变更并告知名称；无法唯一确定时用`openspec list --json`列候选，只对真实歧义询问用户。
2. 运行`openspec status --change "<name>" --json`和`openspec instructions apply --change "<name>" --json`，确认 schema、任务与阻断。缺必要产物则补齐获授权的工作或说明阻断；任务全勾选仍须核对用户当前目标与新反馈，存在遗漏或反证则补修相关任务，不自动归档。
3. `contextFiles`是候选路径，不是全量读取命令。先读当前任务，再定位相关完整`Requirement`及其`Scenario`和相关设计章节；存在跨能力依赖或信息不足时逐步扩大读取。
4. 根据任务影响读取命中规则，正文复用按`context-loading.md`执行。跨会话、正文丢失、文件变化或范围变化时重读必要内容。
5. 按用户完整目标连续实施明确且已授权的任务及必要依赖，不在单个任务完成后提前结束。业务或交互未确定时按顶层门禁澄清；发现设计偏差则先更新受影响规范，不猜测补齐。普通实现障碍先自主排查，真正依赖用户时说明阻断。
6. 对本任务运行必要验证并调用`lina-review`。证据复用与补验统一按`context-loading.md`的“完整性与复用”判断；满足实现、验证和审查门禁后再将任务改为`- [x]`。
7. 报告本次完成任务、实际验证、总体进度和剩余阻断。保留变更文档，不自动提交、推送或归档。
