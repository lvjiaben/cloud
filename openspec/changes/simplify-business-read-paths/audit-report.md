# 业务简化检查与实施记录

本轮覆盖`apps/lina-plugins`全部31个插件，生产Go语法解析2452个文件（排除测试及生成DAO/DO/Entity）、前端请求循环候选、插件发现调用，以及官方lint和U1000检查。检查聚焦用户所指重复解析、重复数据库往返、串行等待和无用代码，不把静态扫描等同于全部业务场景验收。

## 已确认并修复

| 路径 | 原问题 | 当前行为及反证 |
| --- | --- | --- |
| idc-core/catalog、productautomation | 解释性选配和核价重复发现插件及解析包 | 从已发布数据库数据生成预览；资源候选读取拥有方快照；真正购买冻结仍拒绝停用供应商。catalog/productautomation真实PG通过 |
| idc-resource/interfacecandidate | 候选快照读重复读取可执行插件证据 | 存储的契约身份定位快照，凭据/连接修订隔离；后台刷新及实际调用保留准入。真实PG覆盖无GetExact、并发刷新、失败保留与修订变化 |
| website-qycn | 改配等待与缺少分类商品入口 | 120ms合并、评估与报价并行、同值去重；侧栏独立读取真实分类/商品并支持分页、重试、中英文及手机布局；最终40项E2E与14项单测 |
| idc-core/contenthub | 最多500个可见客户逐条写 | 固定3类范围最多6次写，授权、事务和其他内容范围保持；重复键失败回滚及空集合PG通过 |
| org-core/post | 批量删除按每个岗位查询使用情况 | 租户过滤后的单次集合查询；500岗位、整批冲突拒绝、跨租户隔离PG通过 |
| idc-resource/accessorydirectory | 最多10000条附件逐条写 | 每500条一批，共用快照事务；1001项3批、对象编号稳定、第二批故障整体回滚及空目录PG通过 |
| idc-payment/reconciliation | 最多1000来源及差异逐条写，来源客户摘要重复扫描本地支付 | 每500条批写，客户摘要集合索引；1000来源2批+1000差异2批，金额500500分及已匹配客户归属/999差异、幂等重放、第二批差异故障整体回滚及渠道健康不变PG通过 |

## 全量候选甄别

初次扫描85处循环候选；最终81处、72个函数，仍有21个调用插件发现的文件。数量只用于定位，不能把所有候选认定为缺陷。完整机器定位记录在`temp/20261023/qycn-under1/business-audit-before.json`及`business-audit.json`，逐函数上下文位于`loop-contexts.txt`。

- PostgreSQL `rows.Next/Scan`和clamd流扫描：已取结果的遍历，不产生逐行数据库往返。
- 已分批的退款、续费、资源快照和对账：固定批大小，保留。内容范围三个固定表、注销脱敏固定表、范围校验固定种类也不随返回行增长。
- 账本账户按顺序加锁、资金成熟/退款/补偿/工单任务、附件消费、支付尝试关闭和外部回调：逐条执行有独立状态、锁、版本、结果或外部动作，不能合并成失去围栏的无条件写入。
- 配置管理中的工单自定义选项上限100、通知偏好每命令上限30、辅助报价最多8项：有明确上限，属于低频命令并含版本或输入顺序语义，本轮未引入复杂批量更新框架。
- 模板配置的发现按每页100、最多5页；供应商选择和真正执行中的发现用于当前准入，保留。公开候选的重复解析已从热路径移除。
- 前端12个循环候选逐项检查：目录客户端拼查询参数后只请求一次；语言加载为有界重试；附件需先预留再上传，逐文件串行限制内存与并发；DOM渲染和配置参数循环不属于网络循环。
- 官方U1000未发现当前生产入口可证明无用的Go函数；不依据名字或单个文本搜索删除仍由注册、路由、插件能力或语言资源使用的代码。

## 源码库存

| 插件 | 生产Go文件 |
| --- | ---: |
| `content-notice` | 21 |
| `customer-default` | 4 |
| `customer-tdesign` | 3 |
| `idc-ai` | 15 |
| `idc-baota` | 5 |
| `idc-channel` | 84 |
| `idc-compliance` | 247 |
| `idc-core` | 1342 |
| `idc-dcim` | 8 |
| `idc-notify` | 138 |
| `idc-payment` | 61 |
| `idc-payment-alipay` | 2 |
| `idc-payment-epay` | 2 |
| `idc-payment-unionpay` | 2 |
| `idc-payment-wechatpay` | 2 |
| `idc-peer` | 10 |
| `idc-portal` | 46 |
| `idc-resource` | 236 |
| `idc-v10` | 9 |
| `idc-zjmf-finance` | 10 |
| `idc-zjmfcloud` | 31 |
| `idc-zjmfmigrate` | 13 |
| `org-core` | 49 |
| `shared-services` | 23 |
| `tenant-core` | 66 |
| `vben-adapter` | 2 |
| `website-default` | 4 |
| `website-html` | 4 |
| `website-minimal` | 4 |
| `website-qycn` | 4 |
| `yunsu-ssl` | 5 |

## 验证与版本

27个其他插件官方lint与U1000结果全部为0（`temp/20261023/qycn-under1/lint/results.json`）；本轮修改的core、resource、org、payment另有最新官方门禁日志。notify和customer-tdesign的账号/改绑范围由“重做用户中心”负责，主协调已收到结果并安排同一产物验证。

版本：website-qycn最终v0.1.8；idc-resource v0.1.70；org-core v0.1.1；idc-payment v0.1.13；core v0.1.162、notify v0.1.8由账户任务统一维护。resource/org/payment为builtin，使用正式启动同步；官网为动态包，使用上传和升级。没有数据库结构变更、新DI或宿主修改，没有交易价格缓存。

当前隔离浏览器核价证据：商品11，CPU4为CNY31、CPU2为CNY21、套餐默认CNY11；桌面224–244ms，手机219–302ms，非法步长CPU3禁止加购。最终联合构建和共享运行验收见本变更`quality-review.md`，当前官网v0.1.8。
