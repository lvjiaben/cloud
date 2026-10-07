# FB-52 Provider 商品草稿退休验证记录

## 结论

旧“资源或 Provider 快照进入通用商品域并生成商品草稿”方向已经停止，不保留兼容入口。服务器供应商容量继续由服务器插件内部观测，只供异步履约判断；商家继续在商品管理中手工建立分类、商品、版本、简单库存和自动化接口绑定。

## 删除边界

- 删除`catalogcap`跨插件写能力、Provider 草稿事务、审核、物化和专用发布门禁。
- 删除五张 Provider 商品草稿开发期表及其 DAO、DO、Entity、字典和生成配置。
- 后台商品来源只保留手工创建、商品复制和迁移，发布预览不再出现“来源审核证据”。
- 普通商品发布、配置、价格、税务、简单库存和商品履约接口绑定保持原路径。

## 旧开发数据库处置

新增`089-retire-provider-catalog-draft.sql`清理迁移。不存在`provider_import`商品版本时，迁移会在原开发库幂等删除旧表、字典和约束，不要求重置数据库；仍存在旧版本时则失败关闭，因为当前没有可证明能安全删除全部关联事实的正常清理服务。此时必须先导出确有必要保留的测试数据，再用当前迁移重建开发数据库，只恢复不依赖旧路径的数据。迁移不会把旧版本伪装成手工或迁移来源，也不会自行级联删除订单、账单和服务事实。

## 验证

| 门禁 | 结果 |
| --- | --- |
| `GOWORK=off go test ./backend/...` | 通过；包含正常商品发布、发布预览、商品管理和插件装配回归 |
| `GOWORK=off golangci-lint run ./backend/...` | 通过，0 个问题 |
| `pnpm exec vitest run pages` | 通过，8 个文件、42 个断言 |
| `pnpm build` | 通过；生产类型检查、Vite 构建和独立包校验全部通过 |
| `pnpm test` | 插件用例本身未失败，但前置`vue-tsc`被仓库当前宿主类型环境阻断：缺少 Vite 虚拟模块和`ImportMeta.env/glob/hot`声明，并含宿主既有 ES2023 类型错误；本切片未修改宿主文件 |
| `make.cmd i18n.check` | 通过，运行时、多语言消息和前端键均为 0 个违规 |
| `make.cmd plugins.check` | 通过，扫描 3152 个文件、0 个治理问题 |
| `make.cmd lint.go` | 被并行中的`idc-resource`引用尚未落盘的`renewalresourcecap`阻断；本插件`GOWORK=off golangci-lint run ./backend/...`独立通过，0 个问题 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| PostgreSQL | 当前机器未设置`LINAPRO_IDC_CORE_TEST_DSN`，未连接或重置未知数据库；补充静态迁移门禁验证失败关闭、幂等删除和最终来源约束 |
