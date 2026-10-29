# 官网分支：官网之外的逐文件冲突清单

来源：`codex/官网交付`，提交`240aec4`。比较基线为双方共同的`origin/main`，本地比较包含未提交修改、未跟踪文件和已删除文件。

**用户已授权代理按官网必要性及不影响当前功能的原则决定全部50项。** 以下保留原差异说明，并记录最终取舍。翻译只补缺失键，同键本地优先；文档与业务规则本地优先。

本次只迁入`website-main`完整源码并适配当前契约，不整体合并分支。所有编号已处理，无待确认项。

## C01 `.agents/memory/preferences/72-customer-website-visual.md`

- 分支改动：加入该分支的旧业务确认，包括数值选项升序和该官网独立版本规则。
- 本地冲突：本地含最新界面规范和开发服务规则，商家选项顺序不能被旧确认覆盖。
- 最终决定：以本地为主，补充2026-10-29本轮最新选择性迁入决定。

## C02 `.agents/memory/preferences/75-product-commerce.md`

- 分支改动：规定凭据只显示自动生成说明、不接受自填，并加入旧字段顺序、国旗与购买提示规则。
- 本地冲突：本地已确认允许自填、留空生成、逐台配置，且首个可售选项决定默认值。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C03 `apps/lina-plugins/customer-default/hack/tests/tasks.md`

- 分支改动：把另一台环境的客户中心 v0.2.3 发布和验收记录写为已完成。
- 本地冲突：本地另有真实修复与验收记录，不能把另一环境的结果当成本地证据。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C04 `apps/lina-plugins/customer-default/plugin.yaml`

- 分支改动：将客户中心版本改为 v0.2.3。
- 本地冲突：本地已有更新版本，不能按远端旧版本回退。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C05 `apps/lina-plugins/idc-compliance/frontend/vendor/manifest.json`

- 分支改动：更新旧 vendor 资源摘要。
- 本地冲突：本地已删除此文件，不能把退役产物重新带回。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C06 `apps/lina-plugins/idc-core/backend/api/account/account.go`

- 分支改动：注册旧购买配置交接领取接口。
- 本地冲突：本地已新增服务器登录信息等 API；生成接口文件不可直接拼接覆盖。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C07 `apps/lina-plugins/idc-core/backend/api/admin/admin.go`

- 分支改动：注册商家手动创建商品新版本接口。
- 本地冲突：本地已使用保存即生效的编辑 API；生成接口文件不可直接拼接覆盖。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C08 `apps/lina-plugins/idc-core/backend/api/admin/v1/catalog_product_create.go`

- 分支改动：增加商品 NEW 标记开关。
- 本地冲突：本地新增商品契约已经扩展，直接覆盖会丢当前字段。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C09 `apps/lina-plugins/idc-core/backend/api/admin/v1/catalog_product_description.go`

- 分支改动：修改商品说明接口，并增加 NEW 标记更新。
- 本地冲突：本地说明保存已整合基本资料、自动规则等当前字段。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C10 `apps/lina-plugins/idc-core/backend/api/admin/v1/catalog_product_revision.go`

- 分支改动：新增旧版商品草稿创建 API，与本地同名 API 定义不同。
- 本地冲突：本地已有当前的修订和上下架 API，与旧同名定义不能并存。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C11 `apps/lina-plugins/idc-core/backend/api/public/v1/commerce_cart.go`

- 分支改动：放开无配置商品传空配置数组。
- 本地冲突：本地已修改当前购物车契约，需保留登录信息及当前字段验证。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C12 `apps/lina-plugins/idc-core/backend/internal/controller/account/account_new.go`

- 分支改动：注入旧购买交接服务并改变控制器构造。
- 本地冲突：本地构造已含登录信息等共享服务，旧构造会丢依赖。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C13 `apps/lina-plugins/idc-core/backend/internal/controller/admin/admin_v1_catalog_product_create.go`

- 分支改动：创建商品时保存 NEW 标记。
- 本地冲突：本地创建流程已经变化，旧参数不能整体覆盖。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C14 `apps/lina-plugins/idc-core/backend/internal/controller/admin/admin_v1_catalog_product_description_update.go`

- 分支改动：说明更新仅走旧字段与 NEW 标记。
- 本地冲突：本地统一保存已包含更多字段与版本校验。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C15 `apps/lina-plugins/idc-core/backend/internal/controller/public/controller.go`

- 分支改动：给匿名控制器注入旧购买交接服务。
- 本地冲突：本地公开控制器构造已变化，不能恢复旧服务图。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C16 `apps/lina-plugins/idc-core/backend/internal/service/catalog/catalog_import_whole_price.go`

- 分支改动：抽取并调整转售整机价格读取。
- 本地冲突：本地当前报价读取已重写，需逐段核对，不能整文件替换。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C17 `apps/lina-plugins/idc-core/backend/internal/service/catalog/catalog_management.go`

- 分支改动：在商家领域结构增加分类提示、NEW 标记等字段。
- 本地冲突：本地当前商品领域结构已有重构，旧结构不能整体覆盖。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C18 `apps/lina-plugins/idc-core/backend/internal/service/catalog/catalog_management_detail_persistence.go`

- 分支改动：在商品详情读取中加入 NEW 标记及 SQL 列。
- 本地冲突：本地当前详情读取已变化，需保留现有查询及字段。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C19 `apps/lina-plugins/idc-core/backend/internal/service/catalog/catalog_management_persistence.go`

- 分支改动：在持久化结构和校验中加入分类提示、NEW 标记等字段。
- 本地冲突：本地当前保存校验与持久化已变化，不能覆盖。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C20 `apps/lina-plugins/idc-core/backend/internal/service/catalog/catalog_product_description.go`

- 分支改动：在说明更新事务加入 NEW 标记写入与审计。
- 本地冲突：本地基本资料统一保存及当前规则已经重写。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C21 `apps/lina-plugins/idc-core/backend/internal/service/catalog/catalog_product_description_postgres_test.go`

- 分支改动：为旧商品说明事务及 NEW 标记增加数据库测试。
- 本地冲突：本地测试覆盖当前统一保存流程，旧测试表结构不同。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C22 `apps/lina-plugins/idc-core/backend/internal/service/catalog/catalog_product_revision.go`

- 分支改动：新增手动修订业务：保留旧发布、库存从零开始。
- 本地冲突：本地已有内部修订实现，商家不再操作草稿/发布步骤。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C23 `apps/lina-plugins/idc-core/backend/internal/service/catalogdraft/catalogdraft_copy_postgres_test.go`

- 分支改动：增加旧修订授权及合同策略数据库测试。
- 本地冲突：本地测试覆盖当前复制、保存及合同字段语义。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C24 `apps/lina-plugins/idc-core/backend/internal/service/contenthub/contenthub_audience.go`

- 分支改动：修正可见范围候选搜索的 SQL LIKE 转义。
- 本地冲突：本地只保留客户组查询，已移除客户/服务可见候选；不能整文件带回旧逻辑，现存客户组转义修复可单独评估。
- 最终决定：只合入现存客户组查询的SQL转义修复；不恢复公告范围或已删除候选。

## C25 `apps/lina-plugins/idc-core/backend/internal/service/contenthub/contenthub_postgres_integration_test.go`

- 分支改动：增加旧内容可见范围、搜索及上传的数据库测试。
- 本地冲突：本地内容测试已按无可见范围及当前功能修改。
- 最终决定：保留本地测试，新增当前客户组和上传缺陷回归测试；不引入旧范围测试。

## C26 `apps/lina-plugins/idc-core/backend/internal/service/contenthub/contenthub_read.go`

- 分支改动：修正旧内容搜索 SQL LIKE 转义。
- 本地冲突：本地内容公开读取已重写，需单独判断当前是否还有该转义问题。
- 最终决定：保留本地；同类搜索转义已经修好，不重复替换。

## C27 `apps/lina-plugins/idc-core/backend/internal/service/productautomation/productautomation_configuration_pricing.go`

- 分支改动：将 GetScan 单值接收改为结构体并增加修订校验函数。
- 本地冲突：本地已用 GetValue 修正同一问题，并包含复用现有计价维度的处理。
- 最终决定：保留本地；当前GetValue已经解决同一读取问题。

## C28 `apps/lina-plugins/idc-core/backend/plugin.go`

- 分支改动：装配旧购买交接服务及清理定时任务。
- 本地冲突：本地插件装配已有新服务与任务，不能整体覆盖或重建旧服务图。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C29 `apps/lina-plugins/idc-core/frontend/pages/catalog-client.ts`

- 分支改动：在旧商品前端 DTO 加入 NEW 标记、分类提示及修订接口。
- 本地冲突：本地前端已重构当前五页签和统一保存契约。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C30 `apps/lina-plugins/idc-core/frontend/pages/components/catalog-product-create-modal.vue`

- 分支改动：在旧新增商品表单增加 NEW 标记开关。
- 本地冲突：本地新增商品表单已变化；NEW 标记是否需要由你决定。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C31 `apps/lina-plugins/idc-core/frontend/pages/components/catalog-product-detail-drawer.vue`

- 分支改动：在旧商品详情抽屉增加手动编辑新版本及 NEW 标记。
- 本地冲突：本地已删除此旧抽屉，当前使用新的五页签编辑器。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C32 `apps/lina-plugins/idc-core/hack/tests/pages/CatalogManagementPage.ts`

- 分支改动：为旧说明编辑器、NEW 标记增加页面对象操作。
- 本地冲突：本地页面对象服务当前表单，旧定位器会失效。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C33 `apps/lina-plugins/idc-core/hack/tests/pages/CatalogWorkspacePage.ts`

- 分支改动：为旧修订流程增加截图和页面对象操作。
- 本地冲突：本地页面对象服务当前五页签，旧修订测试不适用。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C34 `apps/lina-plugins/idc-core/hack/tests/tasks.md`

- 分支改动：写入分支自己的旧分类、修订、NEW 标记测试编号及完成记录。
- 本地冲突：本地已分配较新测试编号，不能混用同号的不同测试。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C35 `apps/lina-plugins/idc-core/manifest/i18n/en-US/catalog-workspace.json`

- 分支改动：增加手动草稿、发布、分类提示及 NEW 标记英文文案。
- 本地冲突：本地文案对应当前保存即生效流程，不能恢复旧发布提示。
- 最终决定：仅补缺失翻译键，全部本地同键值保留；不导入旧字段或流程。

## C36 `apps/lina-plugins/idc-core/manifest/i18n/zh-CN/apidoc/index.json`

- 分支改动：增加旧修订 API、分类提示、NEW 标记和月价等文档翻译。
- 本地冲突：本地已有当前 API 翻译，旧字段键与当前契约不一致。
- 最终决定：仅补缺失翻译键，全部本地同键值保留；不导入旧字段或流程。

## C37 `apps/lina-plugins/idc-core/manifest/i18n/zh-CN/catalog-workspace.json`

- 分支改动：增加手动草稿、发布、分类提示及 NEW 标记中文文案。
- 本地冲突：本地文案对应当前保存即生效流程，不能恢复旧发布提示。
- 最终决定：仅补缺失翻译键，全部本地同键值保留；不导入旧字段或流程。

## C38 `apps/lina-plugins/idc-core/plugin.yaml`

- 分支改动：将核心版本改为 v0.1.105。
- 本地冲突：本地版本已更新，不能回退。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C39 `apps/lina-plugins/idc-portal/backend/api/admin/v1/site_configuration.go`

- 分支改动：以头部/底部 Logo 替换单一 Logo，并加入版权、QQ、二维码、备案及电信许可证字段。
- 本地冲突：本地沿用 logoUrl、policeNumber/policeUrl，字段替换会影响既有模板和配置。
- 最终决定：保留本地公共资料、保存方式和后台；在官网插件内适配当前字段。

## C40 `apps/lina-plugins/idc-portal/backend/internal/controller/public/brand.go`

- 分支改动：按分支自己的 Logo、备案、版权等字段输出公共品牌。
- 本地冲突：本地公共品牌已服务当前模板，字段语义不同。
- 最终决定：保留本地公共资料、保存方式和后台；在官网插件内适配当前字段。

## C41 `apps/lina-plugins/idc-portal/backend/internal/service/brand/brand.go`

- 分支改动：更换公共品牌结构中的 Logo、备案等字段命名。
- 本地冲突：本地公共品牌仍使用当前 Logo 与公安备案字段。
- 最终决定：保留本地公共资料、保存方式和后台；在官网插件内适配当前字段。

## C42 `apps/lina-plugins/idc-portal/backend/internal/service/siteconfiguration/service.go`

- 分支改动：替换公共资料结构，并增加保留历史配置字段的 legacyCommon。
- 本地冲突：本地共用资料保留当前字段及存储方式，不能未经确认迁移。
- 最终决定：保留本地公共资料、保存方式和后台；在官网插件内适配当前字段。

## C43 `apps/lina-plugins/idc-portal/backend/internal/service/siteconfiguration/siteconfiguration_store.go`

- 分支改动：按分支声明筛选模板字段，改变公共配置保存与回读。
- 本地冲突：本地模板选择、配置和导航逻辑已变化，需保留当前语义。
- 最终决定：保留本地公共资料、保存方式和后台；在官网插件内适配当前字段。

## C44 `apps/lina-plugins/idc-portal/frontend/pages/portal-brand.vue`

- 分支改动：以分支的公共配置字段重做官网后台表单并增加翻译兜底。
- 本地冲突：本地官网配置表单使用当前字段和交互，不可直接替换。
- 最终决定：保留本地公共资料、保存方式和后台；在官网插件内适配当前字段。

## C45 `apps/lina-plugins/idc-portal/frontend/pages/website-configuration-client.ts`

- 分支改动：更换公共配置前端 DTO 的 Logo、备案等字段。
- 本地冲突：本地 DTO 配合当前公共资料契约，不能单独换字段。
- 最终决定：保留本地公共资料、保存方式和后台；在官网插件内适配当前字段。

## C46 `apps/lina-plugins/idc-portal/manifest/i18n/en-US/plugin.json`

- 分支改动：更换 Logo、备案及许可证等配置的英文翻译键。
- 本地冲突：本地运行界面与键对应当前公共资料。
- 最终决定：仅补缺失翻译键，全部本地同键值保留；不导入旧字段或流程。

## C47 `apps/lina-plugins/idc-portal/manifest/i18n/zh-CN/apidoc/index.json`

- 分支改动：更换官网公共配置接口字段文档翻译。
- 本地冲突：本地接口文档翻译对应当前契约。
- 最终决定：仅补缺失翻译键，全部本地同键值保留；不导入旧字段或流程。

## C48 `apps/lina-plugins/idc-portal/manifest/i18n/zh-CN/plugin.json`

- 分支改动：更换 Logo、备案及许可证等配置的中文翻译键。
- 本地冲突：本地运行界面与键对应当前公共资料。
- 最终决定：仅补缺失翻译键，全部本地同键值保留；不导入旧字段或流程。

## C49 `apps/lina-plugins/idc-portal/plugin.yaml`

- 分支改动：将门户版本改为 v0.1.17。
- 本地冲突：本地门户沿用当前v0.1.14配置契约；不能仅为旧官网整体替换成分支v0.1.17。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。

## C50 `openspec/changes/rebuild-product-commerce-workflow/tasks.md`

- 分支改动：追加另一环境的模拟商品、旧修订及测试完成记录。
- 本地冲突：本地包含最新已完成任务与真实证据，不能写入另一环境的完成事实。
- 最终决定：保留本地。旧业务、版本、生成产物及历史验收记录不引入。
