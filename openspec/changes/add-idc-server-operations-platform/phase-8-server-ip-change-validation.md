# 第 8 阶段客户自助换 IP 验证记录

## 范围

本记录对应任务 `8.8`。换 IP 的业务拥有方仍是 `idc-core`，库存和真实资源投影由 `idc-resource` 提供；客户入口放在 `@lina/web-customer`，商家策略页放在核心插件自己的 `frontend/`。没有向 `web-antd` 写入 IDC 业务页面、路由、接口客户端、状态、多语言或依赖。

## 已实现能力

- 商家按币种发布是否开放、滚动免费次数、滚动天数、单日上限、超额价格和库存快照最大年龄；每次发布形成不可变版本，旧报价不会被新规则悄悄改写。
- 客户预览时重新检查本人服务归属、权益和运行状态、退款锁、服务级互斥、当天次数、滚动免费次数及资源库存新鲜度。
- 资源插件只返回安全的 IP 库存预留结果。业务插件保存预留编号和旧、新地址摘要，不保存 Provider 凭据或原始返回。
- 免费换 IP 不伪造账单；超出免费次数时只生成当前客户自己的待付款账单，只有 `financially_settled` 后才允许创建 Provider 写任务。
- 报价、结算和执行分别使用稳定请求摘要。重复请求返回同一结果，请求内容变化则失败关闭。
- 明确失败会释放库存预留；结果未知保持预留和服务锁，先查询权威资源状态，禁止直接重放写请求。
- 成功收敛后消费预留、追加旧地址和新地址摘要证据，并更新本地资源安全投影；旧地址原文不进入业务审计。
- 客户页同时展示资格、免费或付费结果、次数窗口、库存不可用和结果未知提示；插件后台页只维护规则，不直接替客户扣余额。

## 数据、性能与安全

- `064-server-ip-change.sql` 使用完整插件表名前缀、不可变规则版本、报价、结算引用、库存预留引用和只追加事件时间线。
- 资格读取使用固定次数集合查询；不会按历史操作、库存条目或客户数量逐行访问数据库。
- IP 库存必须带权威观察时间且在商家配置的新鲜度窗口内；缺少资源能力、库存过期或预留冲突都失败关闭。
- 管理员读取与发布策略使用两项独立权限。客户接口只允许当前客户自己的服务，不能用服务编号探测其他客户数据。

## 已通过验证

```text
GOWORK=off go test ./backend/... -count=1
PASS

LINAPRO_IDC_CORE_TEST_DSN=... go test ./backend -run TestServiceRenewalFoundationMigrationPostgreSQL -count=3
PASS（包含 064 迁移连续重放）

LINAPRO_IDC_CORE_TEST_DSN=... go test ./backend -run TestServerOperationCreateReplayAndConvergePostgreSQL -count=1
PASS（免费报价、免费结算、成功收敛、付费报价、付费待结算和结算前零执行）

GOWORK=off go test ./backend/... -count=1  # idc-resource
PASS

pnpm run typecheck && pnpm run build  # idc-core/frontend
PASS

pnpm run typecheck && pnpm run build  # @lina/web-customer
PASS

make.cmd i18n.check plugins=1
PASS

TC001-server-lifecycle.ts / TC-1d（真实 Chromium）
PASS（影响确认、免费报价确认、库存结算请求以及新旧安全地址投影）
```

## 结论

`2026-08-23` 已完成 `TC001-server-lifecycle.ts` 的 `TC-1d` 真实 Chromium 回归。任务 `8.8` 的资格与频率、免费/付费报价、库存预留、结算门禁、失败释放、结果未知保护、安全地址审计、双语页面和浏览器门禁已经闭环。
