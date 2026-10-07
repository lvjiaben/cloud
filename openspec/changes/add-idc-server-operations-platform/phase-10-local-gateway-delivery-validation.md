# 第十阶段本地合规网关交付纠偏验证记录

## 1. 当前结论

本记录证明任务`10.5.a`至`10.5.c`及`10.5`父任务的本地组件交付、配置、迁移、启动、监管、备份与卸载边界；任务`10.6`和`10.7`的代码与数据纠偏证据另见[第十阶段安装独立性验证记录](phase-10-local-independence-validation.md)。本记录形成时列出的腾讯`10.4.b`阻断已在`2026-08-14`由官方契约记录解除，诺诺`10.4.c`仍阻断；无论哪一种状态，本记录本身都不证明`FB-11`完成。

原发布工具只生成一个明确标记为“不进入商家安装包”的中央网关二进制，插件测试还主动拒绝任何网关交付声明。该边界与“每套安装获得相同完整程序并独立运行”的纠偏规范直接冲突，是本轮根因。

当前发布工具改为生成目标平台本地网关组件包，固定包含：

- `bin/idc-compliance-local-gateway[.exe]`；
- `bin/idc-compliance-local-gateway-keyring[.exe]`；
- `config/local-gateway.env.example`；
- `migrations/*.sql`；
- `docs/README.md`、`docs/README.zh-CN.md`及双语数据生命周期与责任矩阵；
- 记录所有交付文件`SHA-256`的`manifest.json`。

清单固定声明`productPackageRequired=true`、`localInstallationOnly=true`和`sameProgramForAllInstallations=true`，并继续声明`SupplierClientsIncluded=false`及`BusinessRoutesEnabled=false`。这保证外部供应商能力证据未取得前不会把腾讯或诺诺客户端伪装成已交付能力。

## 2. 重命名评估

- 对外组件号、二进制名和运行服务名已经使用`idc-compliance-local-gateway`，不再暴露“中央服务”交付语义。
- 源码目录保留历史审计名`central-gateway`，但该目录不会进入组件包，也不是安装、运行、服务或 Go 模块公共契约；公开组件号、二进制、服务名和模块路径均已使用`local gateway`语义。继续物理改名只会移动十二个必须保留校验值的历史迁移来源，不能增加运行隔离，因此本变更冻结为“不物理改名、禁止重新引入中央能力”。
- Go 模块路径固定为`lina-platform/idc-compliance-gateway`。外层插件只复用本地进程认证契约，不再引用注册或结算实现。

## 3. 配置、监管与数据生命周期

组件包内模板只接受有界进程设置和当前安装私有的数据库 DSN 文件、进程密钥环文件引用，不允许把数据库凭据、HMAC 密钥或供应商凭据原文写入模板。双语运行说明固定：

- 默认仅监听`127.0.0.1:9088`并以非特权专用账号运行；
- 监管器依次检查`/health/live`和`/health/ready`；
- 监控与支持包不得包含秘密、客户材料、回调或证据原文；
- 备份覆盖当前安装的数据库、配置、秘密、供应商授权元数据、证据和审计，秘密与数据备份分权加密；
- 恢复后在迁移、证据完整性和供应商授权核验完成前禁止供应商写操作；
- 默认卸载只移除二进制和服务定义，数据清理必须经过法律保全、法定留存、任务排空、供应商撤权和证据导出检查。
- 密钥工具拒绝原地覆盖，以新文件完成生成、轮换和旧密钥撤销；插件签名端与网关验签端共享同一安装期密钥环并通过互操作测试。

## 4. FB-10 与第十五阶段隔离

两次实际构建的组件包各只有两个本地网关二进制、配置、迁移、双语说明和清单，不包含`idc-portal/portal`、旧`dist`、前端锁文件、前端构建配置或插件内 E2E 测试资产。旧 Portal 仍完整保留在工作区，未删除、未回滚，也未被认定为最终客户前端。

因此只勾选交付清单核对任务`10.5.a`，不勾选`FB-10`、`15.3`、`15.7`至`15.14`。实际页面迁移与旧 Portal 删除仍必须按`15.12.a`至`15.12.b`执行。

## 5. 验证结果

以下测试通过：

```text
GOWORK=off go test ./... -count=1                                      # 本地网关模块
GOWORK=off go test ./... -count=1                                      # 合规插件模块
GOWORK=off go test ./backend/cap/idccap -count=1                       # 精确权限目录
LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<local> GOWORK=off go test ./internal/migration -run '^TestLocalMigrationPostgreSQL$' -count=1 -v
LINAPRO_IDC_COMPLIANCE_TEST_DSN=<local> GOWORK=off go test ./backend -run '^TestComplianceFoundationMigrationPostgreSQL$' -count=1 -v
```

真实构建结果：

| 目标 | 文件数 | 必须进入产品包 | 只在本地运行 | 所有安装使用同一程序 | 供应商客户端 | 业务路由 |
|---|---:|---|---|---|---|---|
| `windows/amd64` | 19 | 是 | 是 | 是 | 否 | 否 |
| `linux/amd64` | 19 | 是 | 是 | 是 | 否 | 否 |

构建产物位于操作系统临时目录，未写入仓库。

## 6. 当前剩余阻断

迁移`001`至`007`必须作为既有安装升级校验账本保留，但新迁移`008`与`010`在最终事务状态删除所有远程身份、跨客户路由、中央价格、预付/后付和平台结算对象；新安装与升级后的 PostgreSQL 实测均只暴露本地模型。本记录形成时腾讯与诺诺均被外部确认阻断；`2026-08-14`后腾讯已改为官方契约和购买方运行时门禁，诺诺仍需外部证据，因此`10.4`与`FB-11`继续保持未完成。
