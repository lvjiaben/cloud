# 本次范围质量审查

按`lina-feedback`处理全业务简化反馈，按`lina-review`审查本轮自身改动及未跟踪测试。官网价格和侧栏沿用`add-qycn-website-template`第11节；账户改绑由并行账户任务维护，未覆盖其文件。宿主禁止目录工作区状态为空。

## 实现与反证

- 解释性预览只从已发布商品/配置/价格及资源拥有方快照读取；真实购买冻结仍执行当前准入与修订校验，没有跨请求价格缓存。现有构造和公开拥有方能力不变，没有新DI或跨插件私有数据访问。
- 资源候选保持当前绑定授权方法、连接修订、规范化请求和共享数据库租约围栏。实际刷新仍通过受治理调用。PG反证覆盖停用供应商不能Freeze、无GetProviderExact的快照读取、连接变化隔离、刷新失败保留及租约并发。
- 内容范围批量写保留原授权事务、500项上限及参数绑定；岗位占用集合查询保留租户过滤、首个输入冲突和整批拒绝；资源附件及支付对账批写共用原事务，保留唯一约束、稳定编号、金额和归属，不创建SQL迁移或生成模型。
- 支付1000来源/1000差异分别2次写，已匹配支付仍为`none`且客户摘要一致，其余999差异，金额共500500分；后续差异批失败不会留下头、来源、差异或改变渠道健康；幂等重放不重复保存。
- 全量候选复查及不改动理由见`audit-report.md`。未发现可以单凭静态引用安全删除的生产Go入口；无用代码检查不能替代动态插件、语言、注册和路由的使用关系。

## 验证证据

`temp/20261023/qycn-under1/evidence/`保存当前验证日志：core/resource/org/payment最新官方lint及U1000通过，另27插件结果在`lint/results.json`；catalog/productautomation、interfacecandidate、contenthub、org岗位、accessorydirectory及支付真实PostgreSQL测试通过。resource与payment的backend整包也通过。没有执行生产或第三方付款、退款及资源开通。

官网触发E2E质量审查，原因是新增分类侧栏和核价行为反馈。最终39项用例及14项单测通过。TC009的等待门证明目录慢时价格仍可用，断言分类商品返回值、分页、对应href、英文错误/重试/空态及手机宽度；TC008保留并行、同值去重和规范化重新报价，TC007保留乱序、失败和无效配置保护，TC005保留购物车/结算契约。新增用例通过同插件POM调用分类/分页/重试，未增加宿主测试工具。

## 联合产物与隔离运行

最终无overlay正式二进制：`temp/20261023/qycn-under1/lina-pricing-final`，SHA256为`6b0daac1e3c22682fb883095a2a4362c71ae534636e9990635da1e9ec29479ea`。5922项实际Go/Embed/module输入构建前后哈希相同，构建后重新枚举输入集合及哈希一致，未包含测试文件；详见`final-build-inputs.json`、`final-inputs-verification.json`及`final-workspace-inputs.json`。

同产物隔离API另测10轮评估与报价，组合耗时7–27ms，初次配置读取48ms（`final-api-timings.json`）。

同产物隔离页面两种宽度实测：桌面235/224/221ms，手机221/222/223ms，依次为CPU4、CPU2、套餐默认，价格CNY31/21/11。证据`final-isolated-ui-timings.json`。非法CPU步长禁止加购。有效隔离登录后管理API确认core162、notify8、resource70、org1、payment13全部normal/enabled/installed且有效版本一致，证据`isolated-lifecycle-final.json`；先前复用共享令牌的Unauthorized失败未算通过。

## 共享运行与第二轮验收

账户负责人按主协调窗口唯一更新6030，PID1740803，运行精确6b0产物；本方没有重启共享服务。5个builtin有效版本与发现版本一致，normal/installed/enabled，记录`shared-v018-lifecycle.json`。账户任务另证旧版副本官方升级、010索引和重复启动；副本存在既存其他业务FK恢复异常，未把该副本当作全业务完整性证明，未修改源业务数据。

官网先正式升级v017，二轮手机复查发现返回顶部与固定购买栏重叠，导致工具实际点击进入加购。只移除了本次创建的唯一测试条目，未清空其他条目。回归反证为按钮底部824px大于购买栏顶部754.25px。随后独立升级v018：单个ResizeObserver观察购买栏border-box（含状态和安全区padding），CSS将全局按钮放到其上方，保留正常点击；不增加HTTP请求或改身份可信来源。最初content-box未响应padding变化被安全区用例捕获，修为border-box后通过。

当前v018完整40项E2E和14项单测通过，独立包清单/前端内容相同，25个已安装资源逐字节一致；官方升级v017→v018执行成功normal，selected website-default及common/config/navigation完整不变。证据`website-v018-upgrade-result.json`、`installed-assets-v018.json`及`evidence/qycn-v018-*.log`。TC009c在390/320窄屏和46px底部安全区模拟中实际从页面底部点击返回顶部，scrollY=0，加购请求为0且原购物车内容不变，随后正常加购仅1次且保留原条目。

正式16882页面再次实测v018：桌面226/226/223ms，手机223/221/218ms，对应CPU4/2/默认价格CNY31/21/11（`shared-v018-ui-timings.json`）。实际手机返回顶部scrollY由2837到0，按钮底部734.25px、购买栏顶部754.25px；原购物车仍为空。随后实际主动加购到官网购物车、再仅移除测试条目恢复为空，没有提交订单、付款或开通。英文侧栏和控件显示正确，首页真实产品菜单可展开、自动轮播内容变化且没有播放按钮。

共享稳定后API配置GET45ms、10轮评估/报价组合7–20ms（`shared-v018-api-timings.json`）。切换刚结束的一次配置GET为1020ms及首个报价375ms，保留在`shared-api-timings.json`；稳定页面交互样本均低于一秒，未将本地样本宣称为高并发或任意网络条件下的SLA。

最终按lina-review复查本轮当前文件及新增测试，E2E断言覆盖实际行为和异常，未发现阻塞问题。后端输入5922项保持冻结，官网当前v018，未修改宿主、提交、推送或归档。
