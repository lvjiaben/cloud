# 第二阶段秘密信封基础验证记录

## 范围与完成判断

本记录覆盖任务 2.7.a 与 2.7.b.1：`idc-core`内部唯一秘密信封存储、只回调明文访问、主密钥添加式轮换、从部署安全密钥环派生生产包装密钥的 KeyProvider，以及源码消费者单 owner 声明和`resource ProviderEnv`按渠道实例注入。后台管理 API、渠道秘密引用持久化和资源编排服务装配仍未完成，因此 2.7.b 与父任务保持未勾选。

## 密码学与存储边界

每个秘密版本生成独立 32 字节随机 DEK，使用 AES-256-GCM 加密负载；DEK 再由当前部署主密钥使用 AES-256-GCM 包装。负载与包装分别绑定长度前缀的 AAD，覆盖租户、owner 插件、调用范围、秘密业务编号、版本、信封类别和主密钥版本，拒绝跨范围、跨版本或替换包装。幂等指纹还绑定操作、请求摘要、父版本和乐观并发时间，同键不同输入失败关闭。

`secret`聚合仅保存当前版本水位、调用范围和脱敏尾部；`secret_version`保存不可变密文版本；`secret_key_wrap`保存可追加、按主密钥版本唯一的 DEK 包装。数据库不保存主密钥、明文、明文摘要或可枚举的全局查找值。更新追加新版本，撤销永久禁止后续访问，不物理覆盖历史密文。

## 访问与轮换边界

秘密不通过返回值交付。`secretcap.Accessor`只在已绑定 owner/scope 下接收不透明 handle 和已审查用途代码，最多调用一次 consumer；consumer 返回或 panic 后都清零明文缓冲，不传递可能含明文的原始错误。元数据只返回是否已配置、固定格式脱敏尾部和更新时间，不返回 handle、密钥版本、密文或指纹。

主密钥轮换仅追加新 DEK 包装，不重新加密负载。每个 owner/scope 使用数据库共享的 epoch、write fence、verification cursor 和 verification write fence；新活动密钥只有在有界批次中重验所有当前负载与活动包装后才进入 stable。旧密钥写入者、并发更新、中途失败和包装篡改都使验证世代失效或失败关闭，不宣称虚假完成。

源码消费者通过`secretcap/spi`的单 owner 声明期 registrar 获取工厂。`core`在`init`阶段只写声明，`resource`启动装配先永久冻结快照，再显式构造`secretcap.Source`；该接缝没有运行期`Resolve/Get`、handle 查询或明文返回。`resource`把规范化渠道引用做 SHA-256 摘要，只取固定 128 bit 作为低熵 scope 标识并绑定`idc-resource` owner；相同渠道稳定得到相同 scope，不同渠道得到不同 scope。Source authorizer 只允许该精确 scope 执行生命周期动作与`Access`，读取用途必须位于`provider.*`或`resource.*`。

`secretcap.Manager`把创建、替换、元数据、撤销和有界轮换绑定到同一 scope。创建和替换只接受同步 producer，producer 必须恰好调用一次 consumer；输入在回调后清零，服务再持有独立短期副本并在加密完成后清零，panic、重复调用、空值、超限或 producer 错误都折叠为稳定错误。Manager 不提供列表、handle 发现或序列化；调用管理器之前的 HTTP 权限、最近重新认证、数据范围和业务审计仍由拥有方管理接口负责。

`resource`为每个 Provider 同时注入 accessor 和`channelSecretSink`。重装凭据或控制台票据等机密输出经`resourcecap.StoreSecret`的一次性 producer 进入 scope manager；sink 使用绑定请求编号、能力、输出名、策略和到期时间的 SHA-256 幂等键，成功只返回不可序列化 handle，core 错误文本不会穿透 Provider 边界。

## 部署密钥 Provider

`LINAPRO_IDC_CORE_SECURITY_KEYRING`是唯一部署输入。其`reservation_key`仅用于稳定业务编号保留查找；`keys`中的有版本 32 字节根密钥通过 HKDF-SHA-256 分别派生`audit/replay`、`audit/integrity`和`secret/envelope`实际密钥，三者不共用派生结果。活动版本用于新包装，保留版本仅用于解密旧包装和完成轮换。KeyProvider 每次返回可单独清零的脱离副本；缺失、非规范 Base64、非法版本、未知字段或活动版本缺失都失败关闭。

## 验证结果

| 门禁 | 结果 |
| --- | --- |
| AES-256-GCM 已知向量、AAD、篡改和错误密钥 | 通过 |
| 范围授权、幂等指纹、脱敏元数据和 callback 清零 | 通过 |
| 真实 PostgreSQL 生命周期、回滚、聚合倒退、持久化篡改和错误密钥 | 通过 |
| 真实 PostgreSQL 并发轮换、失败恢复、旧密钥写入防护栅和活动包装篡改 | 通过 |
| `secretcap/spi`单 owner、重复声明、冻结、缺失声明和冻结后追加 | 通过 |
| `resource`渠道 scope 稳定性、跨渠道隔离、nil/错误 accessor 与 core 声明集成 | 通过 |
| source manager producer 单次调用、输入清零、panic/重复/空值失败关闭 | 通过 |
| Provider 机密输出 sink、稳定幂等摘要、handle-only 结果和错误折叠 | 通过 |
| `GOWORK=off go test ./backend/internal/service/secret ./backend/internal/runtimeconfig -count=20 -shuffle=on` | 通过；真实 PostgreSQL 随机 schema 已清理 |
| `GOWORK=off go test ./backend/... -count=1 -shuffle=on` | 通过 |
| `GOWORK=off go test ./backend/cap/resourcecap/... ./backend/internal/composition -count=1` | 通过 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core` | 通过，0 issues |

本阶段没有新增 HTTP API、前端、运行时文案、数据库结构或缓存，因此 API、前端交互、i18n、数据库和缓存无影响。DI 来源为`core backend/plugin.go`在进程初始化时声明的工厂；工厂在`resource`启动装配侧创建新服务图，复用数据库权威状态和部署密钥环，不使用包级业务单例。`secret`自有表仅由`core`内部服务访问，强制单商家 tenant 和 owner/scope 隔离；明文访问还要经过注入 authorizer 的精确用途校验。带管理员身份门禁的 HTTP 管理边界、渠道引用表和资源编排注入将在 2.7.b.2 完成，不得把当前 in-process manager 直接暴露为无授权接口。
