# 阶段 14：AI 原生运营副驾驶验证记录

阶段 14 已完成。`idc-ai`保持为独立源码插件，只依赖`idc-core`与`idc-resource`公开只读契约。

| 范围 | 实现与证据 |
|---|---|
| 商家配置 | `plugin_idc_ai_config`保存地址、模型、预算、频率和留存；密钥使用 AES-GCM 加密，仅管理员 API 可读写。|
| 只读工具 | `ReadOnlyTools`仅包含运营摘要、账单概览、资源状态、知识检索和当前客户服务；无执行方法，危险请求在模型调用前拒绝。|
| 运营与客户助手 | `/admin/ai/chat`返回证据、数据时间、不确定性；`/customer/ai/*`只返回启用状态和普通客户页面导航，不泄露配置。|
| 安全与降级 | HTTPS 地址校验、敏感材料过滤、提示注入拒绝、预算/频率限制、超时和普通页面 fallback 均已实现。|
| 审计与反馈 | `plugin_idc_ai_conversation`、`plugin_idc_ai_tool_audit`、`plugin_idc_ai_feedback`追加记录，不保存密钥或原始合规材料。|
| E2E | `TC001-readonly-copilot.ts`覆盖洞察、时效、草稿不发送和危险拒绝；`TC002-customer-assistant.ts`覆盖本人数据、配置不可见和异常导航。|

验证命令：

- `go test ./backend/... -count=1`（通过）
- `node hack/tests/scripts/validate-e2e.mjs`（通过，238 个文件）
- `openspec validate add-idc-server-operations-platform --strict`（通过）

未接入腾讯电子签或诺诺电子发票，遵循当前暂停决定。
