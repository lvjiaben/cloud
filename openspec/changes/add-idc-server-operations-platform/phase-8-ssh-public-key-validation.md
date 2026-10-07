# 第 8 阶段 SSH 公钥验证记录

## 范围

本记录对应任务 `8.13`。客户 SSH 公钥属于 `idc-core` 和 `@lina/web-customer`，不向 `web-antd` 写入任何 IDC 业务内容。

## 已实现能力

- 客户可以保存多把具名 SSH 公钥，后台只展示算法、`SHA256` 指纹、状态和时间，不回显完整公钥正文。
- 服务端按 OpenSSH 公钥格式解析并重新生成规范正文；算法、长度或格式不合法时失败关闭。
- OpenSSH、PEM 和 PuTTY 私钥特征在服务端强制拒绝；客户页面也会提前拦截明显私钥，但浏览器检查不代替服务端检查。
- 新购开通和重装只保存一次性选择快照。只有商品、资源池和 Provider 能力都明确支持时才注入；不支持时保持原流程并显示明确降级，不把公钥误当密码。
- 公钥撤销后不会用于新的开通或重装；已经完成的历史任务保留当时指纹证据，不反向改写 Provider。
- 后台手工导入或绑定已有服务器默认不发送任何 Provider 写请求，只有明确选择并确认的后续操作才会产生副作用。

## 已通过验证

```text
GOWORK=off go test ./backend/internal/service/sshkey -count=1
PASS（规范公钥、指纹、私钥拒绝和安全投影）

GOWORK=off go test ./backend/... -count=1
PASS

LINAPRO_IDC_CORE_TEST_DSN=... go test ./backend -run TestServiceRenewalFoundationMigrationPostgreSQL -count=3
PASS（包含 063 迁移连续重放）

pnpm run typecheck && pnpm run i18n:check  # @lina/web-customer
PASS

node hack/tests/scripts/validate-e2e.mjs
PASS（230 个 E2E 文件）

pnpm exec playwright test tests/e2e/TC017-ssh-public-keys.ts --project=chromium
PASS（真实本地运行时；`TC017a` 4.4 秒、`TC017b` 4.0 秒、`TC017c` 3.6 秒）
```

新增 `TC017-ssh-public-keys.ts`，并于 `2026-08-23` 在真实 Chromium 中完成三项回归：私钥在页面端拒绝且业务接口零提交；创建后只显示指纹、不显示完整公钥正文，撤销后状态从“可用”变为“已撤销”；英文 `390px` 手机页面不泄漏翻译键且没有横向溢出。

## 结论

任务 `8.13` 的服务端格式与私钥拒收、一次性注入快照、无副作用边界、客户页面、双语、生产构建和真实 Chromium 门禁已经闭环。`web-antd` 没有新增 IDC 业务源码、依赖或路由。
