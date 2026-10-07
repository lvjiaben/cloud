# 第四阶段公开商品配置器验证记录

> **`FB-52`失效声明：**旧配置器把服务器组件和兼容规则当作所有商品的通用配置图，相关公开路由已经删除。本记录只用于服务器插件迁移和旧页面清理对账，不能作为当前插件声明配置器完成证据。

## 交付结论

`4.7.b`完成公开商品配置器纵向切片。匿名访客可以读取当前正式商品的双语配置图、选择规格并取得服务端权威兼容校验；本切片不计算价格，也不把“配置有效”冒充可支付报价，购买周期与权威价格构成继续由`4.7.c`交付。

## API、安全与数据权限

- `GET /public/catalog/products/{publicSlug}/configuration`只返回当前`active`商品、启用分类和`published`版本的安全配置图；`POST /public/catalog/products/{publicSlug}/configuration-validations`要求携带同一公开 slug 和 43 字符不透明修订。
- 修订使用部署密钥派生的独立`HMAC-SHA-256`用途密钥绑定公开 slug 与内部版本。响应不含内部版本号、数据库主键、原始规范值、价格、成本、资源池或 Provider 字段。
- 配置图最多 5,000 个组、5,000 个启用规格、8,000 条启用兼容规则且编码内容连同预留信封不超过 1 MiB。查询固定为目标、组、规格、规则和最终当前版本复核，不随返回规格逐项访问数据库，也不调用 Provider。
- 选择输入拒绝未知、重复、零数量和规格范围外数量；组选择数量以及`requires/excludes/allows`方向规则由服务端统一执行。可修正组合问题只返回稳定问题码与公开组/规格业务号，最多 256 项并明确截断。
- 两个端点是有界匿名销售投影，不接收客户主体或后台身份，也不暴露对象存在性以外的内部资料；后台接口的认证、商家上下文、精确权限和数据范围没有变化。

## SQL 与缓存一致性

- `018-catalog-public-configuration-indexes.sql`为启用规格的版本/组稳定顺序和启用兼容规则的版本顺序补充部分索引；迁移使用`CREATE INDEX IF NOT EXISTS`，可重复执行，不含种子数据、删除或软删除变化。
- PostgreSQL 是唯一权威源。正式版本配置图不可变，进程内只缓存最多 256 个按不透明修订区分的安全图；每次校验仍先查询共享数据库中的当前商品、启用分类和正式版本，因此商品/分类停用、版本切换或部署密钥轮换的最大陈旧窗口为零。
- 多实例不广播图内容，各节点按相同数据库目标和版本化修订判断；未命中、驱逐、重启或损坏条目只退化为固定查询次数的数据库重建。缓存提供命中、未命中、写入、驱逐和损坏原子计数快照，指标采集不参与业务判断。

## Portal 与真实浏览器验证

- 商品详情的“选择配置”打开真实配置器，支持`single/multiple/quantity`、默认数量、固定套餐只读、220 ms 防抖校验、并发响应序列保护、加载/失败/重载/有效/冲突状态和中英文解释。
- 前端不计算或展示金额；每次校验提交完整的“公开规格业务号 + 正整数数量 + 不透明修订”。数量输入与选择按钮是同级交互控件，具有可读标签和`aria-pressed`，不存在输入框嵌套按钮。
- `TC005-public-catalog.ts`与`TC006-public-configurator.ts`在真实 Chromium 中 7/7 通过，覆盖安全投影、服务端筛选、配置后报价提示、默认配置权威校验、兼容冲突与修正、精确请求载荷和 390 px 无横向溢出。
- 人工检查截图：`temp/20260810/033525-idc-public-configurator-en.png`与`temp/20260810/033525-idc-public-configurator-mobile.png`。桌面双列配置、成功状态和移动端商品/配置衔接均正常。

## 自动化与构建证据

- `GOWORK=off go test ./backend/... -count=20 -shuffle=on`：核心与 Portal 后端通过；配置服务和公开控制器修复后再次连续 20 轮通过。
- `LINAPRO_IDC_CORE_TEST_DSN=<local> GOWORK=off go test ./backend -run '^TestCatalogFoundationMigrationPostgreSQL$' -count=20 -shuffle=on`：通过；每轮隔离 schema，覆盖`018`迁移重复执行、索引、配置图、服务端校验、旧修订拒绝和分类停用隐藏。
- `make.cmd lint.go dir=apps/lina-plugins/idc-core`与 Portal 对应命令：`golangci-lint`和`staticcheck U1000`均为 0 问题；`make.cmd plugins.check`扫描 711 个文件、689 个 Go 文件，0 项治理发现。
- Portal `vue-tsc`、E2E TypeScript 检查与 Vite 生产构建通过；配置目录块约 165.29 kB、主入口约 421.67 kB，无 500 kB 以上新分块警告。
- 宿主`Test(SourcePlugin|BuiltinBootstrap)`通过，验证源码插件安装、内建启动升级和来源升级；`openspec validate add-idc-server-operations-platform --strict`通过。
- 官方`make.cmd build plugins=1 platforms=windows/amd64,linux/amd64 verbose=1`通过，生成 Windows 与 Linux 宿主产物并嵌入 Portal、SQL、双语资源和 Go 后端。
- `make.cmd i18n.check plugins=1`确认本次运行时扫描和宿主/插件消息覆盖为 0 问题；仓库总门禁仍只被禁止在本变更修改的宿主`notice-preview-modal.vue`既有 6 个缺失键阻断，本切片没有新增缺失键。

## Lina 审查结论

任务级审查范围由根与插件子仓状态、全部未跟踪文件清单和`4.7.b`任务文件共同确定；两个目标插件根目录均不存在附加`AGENTS.md`。实际读取并遵守`openspec.md`、`documentation.md`、`architecture.md`、`data-permission.md`、`plugin.md`、`api-contract.md`、`backend-go.md`、`database.md`、`testing.md`、`frontend-ui.md`、`i18n.md`和`cache-consistency.md`。

审查中发现并修复三项问题：LRU 的未检查链表类型断言、数量输入嵌套选择按钮、缓存缺少可观测计数。另复核并保留此前已修正的启用分类门禁、单选默认项可替换、启用规格最大数量不得为零和响应信封预留。最终严重问题 0 项、警告 0 项，未发现阻塞问题。
