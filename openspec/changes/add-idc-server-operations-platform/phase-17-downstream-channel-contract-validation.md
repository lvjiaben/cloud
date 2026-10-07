# 第十七阶段下游渠道安全契约测试记录

本轮补齐了渠道边界可在本地验证的契约测试，未伪造任何外部适配器成功能力。测试覆盖：跨渠道编号统一不可见、令牌撤销立即失效、来源 CIDR 校验、同请求号重复下单与改载荷拒绝、信用结算未接线时失败关闭且不触发上游、重复 Webhook 事件编号拒绝、渠道停用时保留历史查询并在恢复后开放新购、未验证适配器保持禁用而 LinaPro 通用接口继续可用，以及续费、变配、重装、控制台、换 IP 和终止六类生命周期请求在资源拥有方与财务契约未接线时统一失败关闭。

## 测试文件

- `apps/lina-plugins/idc-channel/backend/internal/service/channel/channel_test.go`
  - 非法来源、过多来源/作用域和过期令牌在数据库访问前失败关闭；Webhook 表按渠道和事件编号唯一；外部适配器与未接线生命周期保持禁用。
- `apps/lina-plugins/idc-channel/hack/tests/e2e/TC005-channel-contract-matrix.ts`
  - 以上九类场景的 HTTP 契约和安全状态投影断言。
- `apps/lina-plugins/idc-channel/backend/api/channel/v1/channel.go`
  - 版本化生命周期请求与结果 DTO，明确六种操作和幂等请求号。

## 验证

| 命令 | 结果 |
|---|---|
| `GOWORK=off go test ./backend/... -count=1`（`idc-channel`） | 通过 |
| `node hack/tests/scripts/validate-e2e.mjs` | 通过（245 个测试文件，17 个作用域） |
| `git diff --check -- apps/lina-plugins/idc-channel` | 通过 |
| `pnpm --dir hack/tests test:module -- plugin:idc-channel` | 通过（17/17，使用`15666`后台和`19120`后端；未占用`9120`） |

后续闭环补充见[第十七阶段渠道商务闭环验证记录](phase-17-channel-commerce-closure-validation.md)：信用与逐币种财务事实、渠道停用持久化、资源拥有方、实名、价格、业务操作、异步执行器、管理页面和客户令牌自助页均已接通；未验证的外部兼容适配器仍保持`disabled_unverified`，不把本地闭环写成外部供应商已联调。
