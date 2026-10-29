# 原生服务器商业管理实施记录

## 根因与边界

旧附属报价与对象生命周期绑定`service_instance`，原生服务器由`service_entitlement`承载。简单替换编号会绕过归属约束或触发错误的新购履约。本次沿已有`088`权益商业主体、订单、账单、结清证明和资源执行器扩展；`151`补附属生命周期，`152`补原生IP与配置报价来源，不伪造旧服务或渠道身份。

`servercommercialcap`仅向`idc-resource`声明源码工厂，显式注入资源拥有方既有`Source`，不通过动态宿主服务暴露。资源负责真实目标、动作权限、确认、能力与任务；核心负责客户归属、发布价格、库存、财务及对象账期。核心与资源共享事务只执行本地持久化，网络调用交给既有任务租约。

## 当前证据

- 快照、备份和固定流量包已通过真实报价、下单、账单付款、结清消费者及取消回归。完整回归日志为`/tmp/native-addon-final-regression-pg.log`；归属、资源版本、动作及并发幂等补验为`/tmp/native-checkout-guards-race-pg.log`。已付款流量包即使后来退款取消仍占销售总库存，见`/tmp/native-traffic-retention-pg.log`。
- `151`真实旧数据迁移、重复迁移、免费对象额度、绑定、对象删除、未付账单关闭及实际付款时点重建周期已有数据库测试。生命周期初次验证见`/tmp/native-lifecycle-final-race.log`；后续恢复与迁移补验见`/tmp/native-ready-recovery-final-race.log`和`/tmp/native-ready-migration151-pg.log`。
- 未派发操作恢复核对实际余额退款账本及信用退款变动，部分退款保留原占用，未知执行结果不重派发；这些证据覆盖恢复拥有方，不代替资源跨插件联调。
- 退款与执行采用订单、账单、客户、父权益、操作的统一锁顺序。当前退款加锁和已提交执行状态反证通过真实数据库`-race`，见`/tmp/native-refund-lock-current.log`。
- 原生IP已经覆盖报价、付款、完成及未知结果；任务绑定只保留容量，真实换IP成功且当前IP发生变化才消费预留。商业预留与真实退款补偿已通过`/tmp/native-ready-commercial-hold-final-pg.log`；付款并发、取消与成功/失败回执见`/tmp/native-priced-final-race-pg.log`。
- `ReadCheckout`从冻结报价读取确认动作；`RejectUnsent`仅接受没有任务、没有执行起点的操作；`ListReady`按有界批次轮转。真实数据库并发、错误目标、变更证据及任务回滚反证共六项通过，日志`/tmp/native-commercial-admission-current.log`。
- `ReadPurchase`通过原始报价和幂等键取回已有操作与账单，不依赖新的供应商快照；执行中或父服务暂停后仍可由当前合法拥有方查询。真实数据库`-race`通过原请求恢复、错误客户/父权益/键拒绝及尚未下单返回空，日志`/tmp/native-commercial-purchase-read-pg.log`。

- 原生结构化目标报价保留真实客户或员工身份，`change`不套用仅适用于新购的活动/券，客户专属价仍按规则计算。客户与员工真实报价/冻结、受限权限、跨员工/跨父权益拒绝及原辅助下单回归，见`/tmp/owned-target-staff-pg.log`、`/tmp/owned-target-price-pg.log`、`/tmp/owned-target-existing-regression-pg.log`。
- 结构化升级实际计费及持久化覆盖真实目标绑定、订单账单、结清、任务与成功后权益配置投影，三周期计费和含税显示分别见`/tmp/native-priced-cycle-persistence-pg.log`及`/tmp/native-priced-display-amount-pg.log`。供应商编译在这些核心测试中仍是明确契约替身，不能替代真实资源调用验收。
- 后台恢复使用`CheckQueuedRestore`，要求共享事务且不创建客户会话；真实父状态及对象门禁见`/tmp/native-queued-restore-pg.log`。`WithQueuedEntitlement`与资源端调用使用同一事务上下文，实际供应商网络须在提交后执行。
- 普通免费操作的`native_server_write_fence`保留真实任务及已确认回执，结果未知继续阻止退款、归属变更和冲突商业操作。核心退款状态反证及商业互斥分别通过`/tmp/native-refund-lock-write-fence.log`与`/tmp/native-commercial-free-write-mutex-pg.log`。资源执行与回执恢复由资源拥有方联合验证。
- 同一免费任务后续仍未知时可以更新证据摘要，活动占用持续有效；终态只允许原证据重放。真实数据库并发验证见`/tmp/native-write-fence-unknown-pg.log`和`/tmp/native-write-fence-unknown-final-pg.log`，恢复旧实现的失败反证见`/tmp/native-write-fence-unknown-red-pg.log`。现有联调库必须重放`151`更新触发器，字段未变不需要再次生成DAO。
- 本范围六张表通过随机隔离模式的真实迁移后调用官方`make dao`生成，再精确迁回生成结果；没有初始化共享库。证据为`/tmp/native-commercial-dao-check.log`、`/tmp/native-commercial-write-fence-dao-check.log`及`/tmp/native-commercial-generated-compile.log`。数据历史与冻结报价不可变，不用软删除抹去执行、计费或退款证据；待核对任务按有界集合轮转，资源端按任务编号批量装配，不逐条查询人员或权限。
- 核心模块最终官方lint及未使用代码检查通过，见`/tmp/native-write-fence-unknown-final-lint.log`；本范围生成代码编译及OpenSpec严格校验通过，分别见`/tmp/native-commercial-generated-compile.log`和`/tmp/native-commercial-final-openspec.log`。

## 变配表单上下文补充

资源和客户页面联调发现已有`QuoteTarget`缺少可编辑表单来源，新增独立`ConfigurationContextReader`，不改变既有商业服务替身接口。该能力读取真实权益当前结构化配置，复用商品公开编译器、候选求值及周期令牌，返回公开字段和不透明值；未通过求值时返回字段问题且不提供报价周期令牌。读取前后核对真实客户或员工权限、权益版本、配置及账期，不在持锁事务内调用候选拥有方。

真实商品配置读取、错误客户/权益/配置/版本/令牌拒绝及无新报价订单写入，见`/tmp/native-configuration-context-owner-pg.log`。核心边界另使用候选读取替身控制查询期间的父权益暂停及员工撤权，验证最终拒绝；这类故障注入不代表真实供应商候选查询验收。当前商品发布配置与权益冻结版本不一致时拒绝，不静默替换版本；含上传凭证的历史配置不通过该只读能力暴露。

真实公开链见`/tmp/native-configuration-context-public-fixed-pg.log`：商品配置和编译器冻结值经过该能力转换后，客户与员工分别使用返回的字段、候选、当前值及周期令牌生成真实`QuoteTarget`和`152`绑定；员工来源与真实客户归属分别验证。合法字段配伪造选项会被拒绝，真实标量越界返回问题且不提供周期令牌。首轮`/tmp/native-configuration-context-public-pg.log`失败由旧附属能力测试种子的大写计费规则编号和缺少发布履约策略引起，修正测试准备后通过，未放宽生产目录约束。

最终拥有方回归及返回令牌生成客户报价见`/tmp/native-configuration-context-owner-final-pg.log`，调用前后门禁及禁止外部事务见`/tmp/native-configuration-context-guard-pg.log`，原默认种子行为回归见`/tmp/native-configuration-context-default-fixture-pg.log`。官方模块`lint`与未使用代码检查通过，见`/tmp/native-configuration-context-final-lint.log`；`GOWORK=off`独立模块构建和启动装配编译分别见`/tmp/native-configuration-context-build.log`、`/tmp/native-configuration-context-backend-compile.log`。限定范围按`lina-review`复核，修正空上下文检查顺序及无效表单返回前的版本重验后，未发现阻塞问题。

接口与页面由资源和账户拥有方接入，`NATIVE-COMMERCE-6`保持未完成，直到完成联合验收。该只读补充不修改数据库结构，不需要新增迁移或生成DAO。

## 商业历史与对象账期补充

已有`ReadPurchase`依赖原报价和幂等键，只能恢复原下单结果；旧服务器目录历史查询绑定旧服务及执行表，不能提供原生权益商业历史。新增独立`CommercialReadAccess`提供`ListOperations`和`ListAccessoryBilling`，每页重新核对客户自有身份或既有附属管理查看权限，并在同一查询快照中限定当前权益拥有方与返回记录。暂停、到期及供应商不可达不阻断这些持久事实的读取。

分页采用稳定编号倒序、上限一百项及`NextCursor`，不返回总数。游标绑定权益、客户与筛选条件；新记录通过刷新首页获取。查询采用集合关联，不逐行读取订单、账单或权限。`151`补充原生历史和已绑定对象的两条分页索引，字段未变化，无需重新生成DAO。统一运行验证应从旧版本克隆库完成正式升级，不在已经执行旧版`151`的候选库手补索引冒充升级证据。

金额以最小币种单位字符串返回，待续费金额来自同客户、同币种真实账单的冻结总额，不能当作剩余欠款。未知结果保持真实的`manual_review`，没有成功时间则不补造时间；错误字段仅返回已知本地安全代码。对象账期只读取真实已绑定的本地对象编号，未绑定对象留在操作记录中，不猜测上游对象。服务转移不授予新客户读取原客户历史财务的权限。

客户授权以真实活动客户身份、`CurrentCustomer`及数据库权益归属自隔离，员工的数据范围标记不作为客户读取或原生商业准入前提。员工仍通过既有精确权限及可表达范围校验；代操作、租户错配、身份不一致和跨客户访问继续拒绝。该边界沿用客户身份拥有方与权益拥有方的现行契约，不读取宿主私有授权表。

审查发现原生商业准入与新读取入口都在区分客户前拒绝员工范围不可表达标记，误挡合法客户。数据库红反证见`/tmp/native-customer-scope-before-pg.log`；两个入口分别移除该全局判断后，`/tmp/native-customer-scope-after-pg.log`真实数据库并发检测通过客户自有历史、账期、报价和结账，跨客户仍拒绝；员工获得精确动作权限后，范围未知仍被原有员工校验拒绝。该补验使用显式注入的活动客户身份，身份拥有方的`CurrentCustomer`实现另经只读查证，不冒称宿主登录端到端验收。最终三文件摘要见`/tmp/native-customer-scope-files.sha256`。

边界数据库回归见`/tmp/native-commercial-read-verified-pg.log`，覆盖分页、权限重验、空页、只读事务、超过`2^53`的金额、财务归属和安全错误投影；其中根聚合表使用明确的有界替身，转移场景是读取防御验证，不代表实际转移流程验收。真实`152`报价、下单、结清、任务绑定及未知回执读取见`/tmp/native-commercial-read-real-checkout-pg.log`，验证返回原操作及财务引用，未知回执仍为`manual_review`且无完成时间。

初次官方模块`lint`与未使用代码检查见`/tmp/native-commercial-read-final-lint.log`；独立模块构建显式退出码为零，见`/tmp/native-commercial-read-backend-build-confirmed.log`。客户范围修正后的最终门禁见`/tmp/native-customer-scope-final-lint.log`和`/tmp/native-customer-scope-backend-build.log`。按`lina-review`完成限定审查，未发现阻塞问题。新增索引使旧测试根表缺失真实`id`列的问题显现，已等价补齐该测试列；早期失败日志保留，当前有效数据库证据仅引用上述通过记录。最终七文件冻结摘要见`/tmp/native-commercial-read-final-files.sha256`。

`NATIVE-COMMERCE-7`保持未完成，等待资源HTTP、产品管理页面及统一升级验收；核心只读回归不能替代这些集成证据。

## 待完成

核心报价、差价、商业预留、生成结果及模块静态检查已有上述证据；最终整包装配和资源、客户界面联合验收仍在进行。现有结构化资源编译器明确拒绝降配，不把旧固定配置路径当作原生降级能力。扣款前提醒及续费失败事件已有核心数据库验证，全部合资格员工实际收到通知仍有公开能力缺口：当前权限接口只能校验当前员工，无法可靠枚举事件发生时全部离线有权员工。登录后补发不能等价全员通知，不通过读取宿主私表或模拟身份规避。

真实供应商能力仍有独立缺口：当前魔方接口的换`IP`调用仅有调整数量的合同证据，不能据此证明支持更换地址；核心价格和占用测试不证明该供应商支持换址。资源拥有方负责撤回不具备证据的能力、报价准入和任务执行，已结清但未发送的操作按可信未发送证明转人工核对，已发送未知结果保留原任务与占用。创建快照或备份缺少可验证的任务到对象标识时，同样保留未知状态，不按名称或列表差异绑定收费对象。

恢复门禁的证据分属不同拥有方：核心`CheckRestore`及`CheckQueuedRestore`证明权益、退款、对象绑定和账期资格，不证明供应商对象已经就绪。资源在提交及出队前均先执行`ValidateParameters`，目录解析要求同一真实对象、种类、快照和`available`状态，再调用核心门禁。供应商不能无证据把目录对象一律标为可用；状态无法可靠映射时须由资源保持未知并拒绝恢复，不能仅凭对象存在或本地账期有效放行。

与并发对话分工：本方只维护核心商业与上述迁移；账户对话维护商品目录、备注、确认接缝及核心版本清单；资源对话维护资源映射、任务及供应商适配；总协调对话维护通知接缝。本方不启动共享候选、不执行真实供应商写入、不提交或推送。

本次新增源码能力本身无HTTP翻译资源变化；新增续费提醒事件的模板与本地化由通知拥有方维护。未修改宿主或根工具，不新增默认开发脚本。完整可用性结论以当前源码门禁和最终联合验收为准。
