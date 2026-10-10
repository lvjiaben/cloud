# 快速定位导航

这是按需导航，不是业务规范。先根据错误、页面、插件或业务词定位源码，再检索相关偏好、OpenSpec能力和任务；读取命中条款全文，不读无关历史。

## 常用入口

```bash
# 规范与偏好，只输出少量命中位置。
go run ./.agents/tools/context.go locate 退款
go run ./.agents/tools/context.go locate security-keyring
# 确认路径后读完整章节或条目。
go run ./.agents/tools/context.go read .agents/memory/preferences/36-refunds.md
# 上一条locate给出的行号可用于读完整条目。
go run ./.agents/tools/context.go read .agents/memory/preferences/14-integrations.md @10
# 代码定位仍使用rg，不把整个源码库载入上下文。
rg -n '具体错误或函数名' apps/lina-plugins/idc-payment
```

单次默认最多12条定位结果。未找到时改用功能名、英文接口名或插件ID；仍不明确再看[偏好主题索引](memory/preferences/index.md)的标题。不要在任务开始时默认读完整索引或全部文件。

## 领域到代码与规范

代码路径相对仓库；插件统一在`apps/lina-plugins/<id>/`。规范名称在`openspec/specs/<能力>/spec.md`或相关活跃变更的`specs/<能力>/spec.md`中检索，不把下表的名称当作固定唯一变更。

| 问题领域 | 优先代码目录 | 偏好检索词 | 能力检索词 |
| --- | --- | --- | --- |
| 客户、账号、权限、组织 | `idc-core`、`staff-access`、`org-core`、`tenant-core` | 客户、账号、员工、数据范围 | `idc-customer-account`、`user-auth` |
| 商品、定价、库存、营销 | `idc-core`及资源拥有插件 | 商品、报价、优惠、库存 | `idc-catalog-pricing`、`idc-marketing-engagement` |
| 用户中心服务菜单、购买销售分类 | `idc-core/backend/internal/service/productdirectory`、`idc-core/frontend/pages/product-directories.vue`、`apps/lina-vben/apps/web-customer/src/views/account/product-directories` | 商家菜单、商品关联、自动选中分类 | `customer-product-directory` |
| 购物车、订单、结账 | `idc-core` | 订单、结账、实名、履约 | `idc-checkout-order`、`idc-product-fulfillment` |
| 账单、余额、授信、续费 | `idc-core` | 账本、授信、续费、结清 | `idc-billing-ledger`、`idc-credit-account` |
| 支付和在线退款 | `idc-payment`及`idc-payment-*` | 支付、退款、渠道 | `idc-payment-refund`及账本能力 |
| 服务器、Provider、任务 | `idc-resource`、`idc-zjmfcloud`、`idc-dcim`等拥有者 | Provider、服务器、未知结果、人工接管 | `idc-server-lifecycle`、`idc-provider-integration` |
| 实名、合同、发票、合规网关 | `idc-compliance`、`idc-compliance/central-gateway` | 个人、企业、合同、发票、网关 | `idc-compliance-center` |
| 工单、知识库、内容 | `idc-core`、`content-notice` | 工单、SLA、内容、下载 | `idc-support-notification` |
| 短信、邮件、通知 | `idc-notify` | 通知、短信、退订 | `idc-support-notification` |
| AI、官网、客户中心 | `idc-ai`、`idc-portal`、`apps/lina-websites`、`apps/lina-vben/apps/web-customer` | AI、官网、用户中心、视觉 | `idc-ai-copilot`、`idc-merchant-portal` |
| 渠道、代理、迁移 | `idc-channel`、`idc-zjmfmigrate`等适配器 | 渠道、代理、迁移 | `idc-sales-channel`、`idc-migration-reconciliation` |
| 宿主通用能力 | `apps/lina-core`、`apps/lina-vben` | 具体接口或模块名 | 在`openspec/specs/`按能力名查 |
| AI规则、技能、上下文与流程 | `AGENTS.md`、`.agents/rules`、`.agents/skills`、`.agents/prompts` | 上下文、流程、规则 | `agent-context-navigation` |

## 大变更的读取方式

```bash
# 先看目录及相关文档标题，不拼接整个变更。
openspec list --json
rg -n '^#{1,4} ' openspec/changes/<change>/design.md
rg -n '目标任务ID|目标业务词' openspec/changes/<change>/tasks.md
rg -n '^### Requirement:|^#### Scenario:' openspec/changes/<change>/specs/<能力>/spec.md
```

只读当前任务、目标Requirement及相关设计节；相关依赖不明确时补读。活跃任务仍以原始`tasks.md`为准，导航不会勾选、删除或归档任务。`archive/`与`evidence/`仅在历史依据确实影响当前问题时显式查阅。

## 维护

新增插件、能力或偏好主题时更新对应入口；代码与业务正文优先于导航提示。主题原文迁移记录在`memory/preferences/migration-manifest.json`，不默认读取。当前读取和复用约束见`rules/context-loading.md`，流程门槛见`rules/openspec.md`。
