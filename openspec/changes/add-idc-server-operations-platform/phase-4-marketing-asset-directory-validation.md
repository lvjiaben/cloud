# 第四阶段营销资产管理目录验证记录

## 范围

本记录覆盖任务`4.10.b.2.1`：`idc-core`后台优惠券/代金券资产、积分账户与流水、充值赠送/返现权益的有界分页列表、类型化详情、六项独立精确权限和 LinaPro 中英文管理界面。本切片严格只读，不包含发券、撤销、积分调整、赠送权益调整或资金账本写入；这些受审计动作仍由`4.10.b.2.2`承接，因此`4.10.b.2`、`4.10.b`和`4.10`保持未完成。

## 已实现边界

- 新增`idc:admin:marketing:asset:list/query`、`idc:admin:marketing:points:list/query`和`idc:admin:marketing:grant:list/query`六项互不替代的精确权限。授权在筛选校验和目标存在性读取前执行；继续要求默认商家、直接活动员工和可表达的数据范围。
- 新增六个本地只读接口：`GET /admin/marketing/assets`、`GET /admin/marketing/assets/{assetNo}`、`GET /admin/marketing/points/accounts`、`GET /admin/marketing/points/accounts/{customerNo}`、`GET /admin/marketing/grants`和`GET /admin/marketing/grants/{grantNo}`。
- 三个列表都以`updated_at + id`加密 keyset 游标分页，游标绑定资源类型、客户、活动、类型、状态、币种和页大小；单页最多 100 项，编码响应不超过 1 MiB。客户和活动筛选使用精确业务编号，不返回数据库主键。
- 资产详情在只读可重复读事务内固定 2 次查询并限制 100 条预占；积分详情固定 3 次查询并限制 100 个批次、200 条流水；赠送权益详情固定 2 次查询并限制 200 条流水。查询次数不随返回行数增长。
- 读取投影再次校验券种与权益规则匹配、领取和耗尽生命周期、积分批次与赠送权益金额桶守恒、来源和流水类型枚举、冲回引用以及财务入账引用。畸形数据失败关闭，不把数据库异常状态透传成可信运营事实。
- 金额只使用整数最小货币单位并携带显式币种；积分保持非货币数量，不作为余额、现金或佣金。管理页面不提供跨币种换算、合计或兜底。
- LinaPro 页面新增三个可搜索、前后翻页的响应式目录和三个类型化详情抽屉。精确客户筛选只在回车或清空时请求，避免按键逐次查询；卡片按可用内容宽度自动换行，窄内容区不再硬挤三列。

## 验证结果

| 门禁 | 结果 |
|---|---|
| 营销资产目录授权、游标、投影和控制器测试 | 通过；覆盖授权先行、资源/筛选游标绑定、类型化`nil`依赖、金额与积分边界、畸形生命周期/来源/流水拒绝及六个接口 DTO 映射 |
| `GOWORK=off go test ./... -count=20 -shuffle=on` | 通过；插件后端全量 20 轮随机顺序回归 |
| 真实 PostgreSQL 营销集成测试 20 轮 | 通过；在独立随机 schema 中覆盖既有营销生命周期并读取真实资产、积分和赠送权益目录 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend` | 通过；`golangci-lint`为 0 个问题，`staticcheck U1000`通过 |
| LinaPro 管理端`pnpm run check:type` | 通过 |
| `TC003-marketing-operations.ts`真实 Chromium | 通过，5/5；中文、英文、刷新、分群活动详情及资产/积分/权益详情均通过 |
| 页面截图人工检查 | 通过；资产和积分表格在当前 LinaPro 内容宽度下完整显示关键列，无原始翻译键；金额显示`CNY`，积分保持独立数量 |
| API 文档与运行时`i18n` | 新增 136 个中文 API 文档键并与英文源元数据逐项核对，运行时中英文资源对等；仓库级`i18n.check`仍仅报告禁止在本插件任务中修改的宿主通知预览页既有 6 个缺失键 |
| `make.cmd plugins.check`与`openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `make.cmd build plugins=1 platforms=windows/amd64,linux/amd64 verbose=1` | 通过；管理端、Portal、11 个插件及 Windows/Linux 后端正式产物完成，当前核心插件版本`v0.1.30` |

## 审查结论

- 未发现本切片内的`P0`或`P1`问题。六项读取权限均独立，列表权限不能替代详情权限，营销总览及分群/活动权限也不能替代。
- 数据权限：只允许默认商家中的直接活动员工和支持的数据范围；授权发生在输入验证与存在性查询前，避免泄露资源是否存在。
- API：新增六个只读接口和类型化 DTO；所有集合、游标和编码响应都有硬上限，时间统一为 Unix 毫秒。
- SQL：仅为资产、积分账户和赠送权益工作台增加匹配筛选与 keyset 顺序的幂等索引；没有 Seed、Mock、软删除、自增主键或写入语义变化。
- 缓存：无影响。目录直接读取本地 PostgreSQL 一致快照，没有新增缓存、外部调用或 Provider 依赖。
- `i18n`：新增中英文运行时文案和中文 API 文档翻译；英文 API 文档继续直接使用 DTO 源文本，`en-US/apidoc`保持空对象。
- 本切片没有发券、撤销、积分或权益调整等危险操作。下一子任务必须另设精确写权限、幂等请求、服务端权威重检、必要审批和不可变员工业务审计，不能扩张现有只读权限。

## Git 状态

本轮没有执行`git add`、提交、推送或创建 PR；工作区原有修改均保持不动。
