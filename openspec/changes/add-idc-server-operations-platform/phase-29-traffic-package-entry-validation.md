# 第二十九阶段固定流量套餐入口验证

日期：`2026-09-04`。对应`FB-114`。本记录现已完成本地安全验收；没有执行真实客户扣款或真实上游流量变更，因此结论只覆盖正式代码、隔离数据库、受控连接器和浏览器契约，不宣称外部供应商生产环境已实际收费。

## 已确认规则与实现

后台填写固定容量、含税最终总价、适用服务器商品和销售状态；库存可选不限量或填写总可售份数。已占用与剩余由数据库事实计算，剩余最低为零。下单形成待付款账单时占用，权威取消或超时关闭后释放，已付款及结果未知保留；调低总量和下架不修改历史订单。用户结论已同步用户偏好和生命周期增量规范。

后台配置使用正式`Vben Grid/Form/Modal`，客户与员工购买使用正式`Vben Grid/Drawer/Modal`，有真实总数、数字分页和搜索。商品候选远程有界搜索，并保留当前已选名称；加载失败禁止保存，关闭后可重新打开。开关保持标准尺寸。客户确认显示后端冻结报价，不以列表价格直接下单；员工提交页面显示的套餐版本，由后端拒绝价格版本变化。

编辑边界复核发现，已选商品不再满足当前候选条件时，选择框会因缺少名称直接显示内部编号。按现有禁止泄露内部编号的规则，缺失名称只显示双语“商品名称暂不可用”，保留原内部选择值，后续搜索取得权威名称时替换提示。此处理不扩大商品可见范围、不自动移除适用商品、不绕过保存时的商品资格校验；增加独立浏览器回归证明不会把内部编号显示给用户。

客户和员工购买前均显示“有效期按上游重置规则执行，暂未提供具体到期时间”，不把报价有效期、目录过期时间或服务器到期时间当作流量到期。套餐只能增加到当前服务器，收费套餐先待付款，不自动扣余额、开机或解除暂停。

## 本轮发现并修复的重试问题

真实 PostgreSQL 验证发现员工第一次下单成功后会更新服务器版本；如果浏览器没有收到结果再次提交，同一请求会重新报价并因版本变化误报价格冲突。原结账内部的幂等复用没有被执行到。

修复仅在既有员工授权和当前服务器客户归属校验之后，按客户与请求摘要读取已提交的原附属操作，精确核对服务器、动作、套餐和版本；相同请求直接返回原订单，改变容量参数或套餐版本拒绝。查询是固定一次联表，使用现有幂等索引，不重新报价、不再次占库、不创建资金事实、不重放执行任务。没有放宽角色、数据范围或插件边界。

## 精确变更范围

相对`apps/lina-plugins/idc-core/`：

- `frontend/pages/traffic-package-client.ts`及`components/traffic-packages.vue`、`components/traffic-package-editor.vue`、`system-configuration.vue`。
- 管理流量套餐与商品候选五个 API/控制器、客户套餐 API/控制器、员工套餐列表 API/控制器，以及`server_accessory_operation.go`的套餐版本输入与转交。生成控制器使用官方生成入口，再实现对应处理方法。
- `backend/internal/service/serveroperation/serveroperation_management.go`、`serveroperation_traffic_management_postgres_test.go`、`backend/internal/controller/admin/traffic_package_http_boundary_test.go`、`backend/api/traffic_package_apidoc_i18n_test.go`及双语资源。
- `hack/tests/e2e/TC022-traffic-package-management.ts`、`hack/tests/pages/TrafficPackagePage.ts`、`hack/tests/support/traffic-package-mock.ts`。

相对`apps/lina-vben/apps/web-customer/`：`src/api/idc/traffic-packages.ts`、`customer-servers.ts`、`src/views/account/servers/traffic-package-purchase.vue`及`index.vue`、对应双语资源、`tests/support/customer-server-api-mock.ts`、`tests/pages/ServersPage.ts`和`tests/e2e/TC015-server-operations.ts`新增`k/l`。

相对`apps/lina-plugins/idc-resource/`：`frontend/pages/server-accessory-client.ts`、`components/traffic-package-purchase.vue`、`server-accessories.vue`、对应双语资源，以及`hack/tests/`下`TC005-server-accessories.ts`、`ServerAccessoriesPage.ts`、`server-accessory-mock.ts`。

套餐数据库、报价、库存、取消与结清拥有方另有测试范围，本记录不把所有未跟踪插件文件都当成已审查。

## 当前验证证据

- `TC022a-d`后台管理`4/4`实际 Chromium 浏览器通过，覆盖二十五条数据分页、筛选重置、总量调低、下架、最终金额、失败恢复、重复打开、候选失败禁存，以及已选商品名称缺失时不泄露内部编号；既有六张截图已审阅，灰表头、白表体及数字分页符合要求。
- 客户`TC015k/l`两项通过，覆盖售罄、准确提示、后端报价与列表价格不同时的确认、失败保留窗口及相同请求重试；三张截图已审阅。
- 员工`TC-5f/g`两项通过，提交固定套餐版本、空业务参数，仅生成待付款账单；失败后同请求重试。保存结果截图已审阅。
- 浏览器以上八项使用明确业务 API 替身，只证明实际前端交互和请求契约，没有真实客户扣款或上游写操作；数据库、财务与连接器行为由下述独立后端门禁验证。
- 实际 GoFrame HTTP 测试验证适用商品数组零至一百零一项原样交给业务校验、空值库存、路由编号不能被查询或正文覆盖；通过。
- `serveroperation`完整后端包在真实 PostgreSQL 随机隔离模式下通过，耗时`69.451`秒。覆盖套餐历史、总量/占用/剩余、调低总量、下架保留历史、权威报价、当前商品绑定、并发下单只成功一份、价格版本冲突、订单重放、取消与超时关闭幂等、错误事实回滚，以及新增的取消后占用从一恢复为零的直接断言。
- 附属结账链定向回归通过：未付款不结清；权威结清后只为原服务器建立一条任务；并发或重复事件复用；未知、篡改或缺失证据拒绝；审计失败回滚；索引查询命中`idx_plugin_idc_core_addon_order`。测试夹具原先把包含流量包结构的`109`迁移误当成单索引迁移，已改为从真实迁移中提取精确索引语句并验证两次执行幂等。
- 魔方云连接器`TestTrafficPackage*`全部通过，覆盖容量参数、上游映射和后置校验、任务已受理不重复读取、非法或无限流量实例零写入、未证明和未授权结果保持未知且不重放、传输歧义先核对再收敛。`TestTrafficFulfillmentNeverChangesRuntimeState`同时证明流量操作不会自动开机或解除暂停。
- `idc-core`与`idc-resource`插件生产构建通过；总后台和用户中心类型检查、双语键检查及生产构建通过，用户中心 Vben 对齐检查通过。

## 运行状态与验收边界

本机此前正常插件升级已应用`idc-core v0.1.92`的`109`迁移和`idc-resource v0.1.24`。本轮在不清库、不重装插件的前提下正常重启`6030/6031/6032`，后台`4/4`、客户`2/2`、员工`2/2`浏览器回归再次通过；三项服务保持监听，总后台与用户中心首页返回`HTTP 200`，核心根路径按设计返回`404`但端口正常监听。

`FB-114`据上述证据完成。真实外部付款和上游变更不属于本地自动验收范围；生产接入仍应使用供应商沙箱或受控小额流程单独验收，不能用本轮测试替代。

独立发现框架插件升级弹窗对`riskHints:null`直接读取`length`导致渲染失败。已记录根因，未改宿主。云插件升级经正式预检确认无依赖阻断、无授权变化、无 SQL 和风险项后，通过同一正式升级 API 完成`v0.2.3`至`v0.2.4`；不是卸载、重新安装或强制绕过依赖。弹窗问题仍待单独修复授权。
