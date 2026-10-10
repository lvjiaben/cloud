# 验证记录（2026-10-09）

本次范围为已识别支付/交付/认证/CI问题、商家服务菜单与购买销售分类导航。保留既有上下文优化变更；未提交、推送或归档。

## 业务与实现

按用户最新纠正，对照参考仓库`aazooo/zjmf-manger-decoded`的`MenusController::addProductPage/setNavList`：后台配置关联商品的客户中心菜单，`service?groupid=菜单ID`选中对应服务列表。购买页`cart/default/sidebar-categories.tpl`独立使用销售分类。此前依据`nav_group/nav_group_user`实现客户显示偏好属于误解，已移除入口、写接口及查询；历史偏好表保留但不再参与生产读写。当前使用`service_entitlement`权威事实查询本人已购服务，菜单不改变账单或所有权。当前销售分类契约为平级，本轮未凭空新增魔方的两级分类数据模型。

## 已完成的验证

- 目录SQL111重复执行、授权/客户隔离、并发归属冲突、乐观版本、审计失败事务回滚、123目录分页、自动时间戳均通过真实隔离PostgreSQL测试；GoFrame API/DAO生成与双语资源检查通过。
- 最新`TC023`四个Chromium用例通过：商家关联保存/冲突；菜单选择/深链接刷新/历史/双语切换且无偏好写入；失败后全部服务可用并重试恢复侧栏；第101个菜单自动定位tab并刷新保留。输出`temp/20261009/merchant-menu-final`。UI传输使用fixture，客户隔离与旧偏好不影响导航由真实PostgreSQL独立验证。
- 新增`TC024`购买分类E2E与3项状态单测通过，覆盖销售分类API筛选、清游标、URL刷新、错误恢复、窄屏和复用同一商品选购tab。截图`temp/20261009/purchase-categories/`。
- 递归门禁发现密钥轮换失败会将raw SQL、加密封装及触发器内部错误带入公开错误链；现仅保留本服务稳定业务码并剥离cause，无新增raw日志。新单测及11项真实数据库secret回归通过，覆盖轮换失败回滚、并发与重试。
- 资源凭据Create/Replace/Revoke现与binding及immutable revision同事务追加不含秘密的审计；Rotate以有界batch加入ambient事务，与聚合审计原子提交。生命周期PostgreSQL回归、审计失败恢复/重试、SQL写入fake验证轮换审计失败回滚、四操作审计非秘密字段断言和安全错误单测通过，资源全模块lint为0。根独立检查事务位置、幂等键、权限及快照，未发现阻塞；最新宿主已包含修复。
- 新旧支付状态、早到回调/提交未知、查询租约、停用渠道历史交易通过真实数据库回归；魔方财务未付/已付契约使用本地TLS服务回归，不连接供应商。
- 双前端类型检查、编辑器4项回归、原前端643项测试以及source插件模式的双端生产构建通过。后续新增菜单组织2项回归通过，覆盖移除extension容器后保留PluginSandboxView路径/权限/桥元数据；管理端类型和生产构建已重跑。
- Go lint由linactl执行：核心、支付/魔方财务、资源、工具相关模块零问题。最终核心全模块lint包含目录新包与修复后的集成fixture。
- 动态插件完整构建：全部11个WASM产物有效；SSL自有前端先构建再打包；缺资产/缺surface入口打包必须失败。魔方云、迁移使用插件自身双语隔离页面，19项前端单测与3项真实编译页面浏览器测试通过。
- 新宿主启动后 idc-core v0.1.94、idc-resource v0.1.26 自动SQL升级成功，真实管理员目录API和固定魔方云资源API均返回200及准确空列表。真实迁移 v0.2.2 安装启用成功，宿主iframe加载并经页面桥完成v10来源预检，页面无JS异常；未执行正式导入。
- 已人工检查目录编辑/商家菜单选中服务分类/加载错误与迁移真实页面截图。

## 剩余风险及边界

前端审计从177项降至3项（2高、1中，0严重）：braces、node-forge、sprintf-js，当前无上游修复版本，来源为开发/构建工具链。记录 `temp/project-audit/frontend-audit-after.json`。不能描述为零漏洞。

未配置真实支付、云服务器或SSL供应商凭据，未执行真实支付扣款、证书签发、服务器开通或来源数据库迁移；这些不属于本地fixture与隔离数据库证据。

## 集成收尾

已修复归档解析污染desired索引的问题：仅storage发现扫描维护待升级索引，按路径读取归档保持独立缓存。catalog包与真实数据库 preview→旧归档读取→Execute 新包回归通过，宿主linactl lint零问题。

本地真实升级成功且installed/enabled=1、runtimeState=normal：SSL v0.3.6、魔方云 v0.2.6、魔方财务 v0.1.5；finance实际升版确保修复进入已安装系统。SSL和魔方云真实宿主iframe加载正确版本，前者证书空态、后者准确空表和分页均正常，pageErrors=[]。证据 `temp/project-audit/runtime-upgrade-final.log`、`runtime-plugin-pages.log` 与对应截图。

当前AST必跑清单共69包454项均有本次PASS证据：core 39包331项、payment 1包9项、resource 10包33项、compliance 16包46项、notify 3包35项；`idc-required-validation.json`核对missing={}。分段修复后复跑受影响包并复用未变化包证据，**不声称一次完整门禁命令退出0**。串行fixture避免默认PostgreSQL锁容量耗尽，测试内部并发竞态仍执行。唯一精确排除的性能manifest有单独opt-in契约；codegen fixture亦按原边界排除。身份fixture调用缺少PostgreSQL命名将被AST门禁直接拒绝；missing/skip/fail/同名跨包/失败子测试负向回归通过。

修复的测试前提包括：隔离schema的GoFrame元数据缓存；通用商品发布取代旧server模型并恢复审计回滚；明确已被通用entitlement覆盖的旧删除表测试移除；当前migration集合与佣金断言；身份SQL参数/106个人注册模型；当前UTC有效期；合规和通知独立DSN与projection fixture。未放宽生产约束或跳过有效业务回归。

最新core/resource/compliance/notify全module linactl均exit0，宿主/支付/财务/构建工具此前本次相关模块证据复用。最终官方插件聚合宿主编译成功并再次重启；HTTP只读插件/菜单检查确认已启用版本正常，前后端6030/6031/6032可用。OpenSpec严格校验和diff检查通过，lina-review结论见review.md。

## 本轮目录反馈纠正

后端真实`productdirectory` PostgreSQL回归、API/controller编译、插件启动绑定、`linactl`核心lint/deadcode、全仓i18n检查通过。前端当前客户端类型检查、目录改动eslint通过；客户端最终生产构建记录见`temp/20261009/merchant-menu-final-build.log`。已重新编译官方插件聚合宿主并重启至PID1465788，公开前端配置返回200。新增菜单无新运行期DI实例；复用现有access刷新队列与插件服务owner；查询客户权限与分页集合装配保持既有边界。

根人工检查最新商家保存、SSL菜单选中与失败恢复截图；文案已翻译、菜单和分页清晰，无显示设置入口。独立复审发现的重试未恢复侧栏、超过100项未定位tab、语言切换菜单滞后三项均已修复并纳入当前TC023。
