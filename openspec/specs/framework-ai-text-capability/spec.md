# framework-ai-text-capability Specification

## Purpose

定义稳定且可扩展的文本 AI 请求响应、推理强度和方法级可用状态，约束敏感输入输出的处理，并明确动态 SDK 的领域所有者及与后续多模态能力的边界。

## Requirements
### Requirement: 宿主必须发布文本 AI 抽象能力

系统 SHALL 由`ai-core/backend/cap/aicap/aitext`发布版本化文本`AI`抽象能力`plugin.ai-core.ai.text.v1`。该能力 MUST 只定义消费契约、状态查询、降级语义和 provider 接入边界，MUST NOT 在`lina-core`中持有渠道、模型、档位或调用日志业务存储。`lina-core`只通过通用 capability descriptor、依赖治理、授权快照、动态路由和审计 envelope 治理该能力。

#### Scenario: 消费方通过文本能力接口调用

- **WHEN** 宿主模块、源码插件或动态插件需要执行文本生成
- **THEN** 调用方 MUST 通过`ai-core/backend/cap/aicap/aitext`发布的`plugin.ai-core.ai.text.v1`消费接口发起调用
- **AND** 调用方 MUST NOT 直接依赖`ai-core`的`backend/internal/**`、插件表、渠道密钥结构或 provider adapter
- **AND** 调用方 MUST NOT 继续依赖`lina-core/pkg/plugin/capability/aicap/aitext`作为生产契约 owner

#### Scenario: 官方插件提供文本能力实现

- **WHEN** `ai-core`插件处于平台级可用状态并声明`plugin.ai-core.ai.text.v1`provider
- **THEN** `plugin.ai-core.ai.text.v1`的消费 service MUST 将文本生成调用委托给该 provider
- **AND** 返回值 MUST 使用该能力自有 DTO、投影和值对象

#### Scenario: 文本生成映射到固定能力方法

- **WHEN** 调用方通过`plugin.ai-core.ai.text.v1`执行`GenerateText`
- **THEN** owner 契约 MUST 将该调用视为`capabilityType=text`与`capabilityMethod=generate`
- **AND** Go 契约 MUST 使用命名类型和常量表达该能力方法
- **AND** 调用方 MUST NOT 通过请求字段把`GenerateText`改写为图片、向量、音频或其他方法

#### Scenario: 渠道存储不进入宿主

- **WHEN** 系统实现`plugin.ai-core.ai.text.v1`
- **THEN** `apps/lina-core` MUST NOT 新增渠道、模型、档位或调用日志业务表
- **AND** owner 公开契约 MUST NOT 暴露插件内部`DAO`、`DO`、`Entity`、缓存快照或密钥明文

### Requirement: 文本生成请求响应必须稳定且可扩展

系统 SHALL 为 `framework.ai.text.v1` 定义同步文本生成请求和响应契约。请求 MUST 包含 `purpose`、`tier`、`messages`、可选生成参数和可选 `thinkingEffort`；响应 MUST 返回文本、实际档位、实际渠道模型投影、用量、耗时和 Unix 毫秒时间点。

#### Scenario: 请求使用消息数组

- **WHEN** 调用方构造文本生成请求
- **THEN** 请求 MUST 使用 `messages` 数组表达输入
- **AND** 每条消息首期 MUST 至少包含 `role` 和纯文本 `content`
- **AND** 调用方 MUST NOT 通过 `metadata` 承载大段 prompt、diff、文件内容或业务原文

#### Scenario: 响应返回最小渠道投影

- **WHEN** 文本生成成功
- **THEN** 响应 MUST 包含生成文本、`tier`、`providerName`、`modelName`、`protocol`、`usage.inputTokens`、`usage.outputTokens`、`latencyMs` 和 `generatedAt`
- **AND** `generatedAt` MUST 是 Unix timestamp in milliseconds
- **AND** 响应 MUST NOT 返回 API key、secret 引用解析结果或 provider 内部配置

#### Scenario: 无效档位被拒绝

- **WHEN** 调用方传入不是 `basic`、`standard` 或 `advanced` 的文本档位
- **THEN** 系统 MUST 在执行渠道调用前拒绝请求
- **AND** 错误 MUST 是结构化业务错误，包含可诊断的错误码和可本地化消息键

### Requirement: 文本能力必须支持 thinkingEffort 抽象参数

系统 SHALL 在文本生成请求中预留可选 `thinkingEffort` 参数。`thinkingEffort` MUST 使用平台统一枚举 `low`、`medium`、`high`、`xhigh`、`max`。模型管理 MUST NOT 声明或预先限制支持范围，具体是否可用 MUST 由测试调用、真实运行结果或 provider adapter 的结构化错误反馈判断。

#### Scenario: 请求合法 thinkingEffort

- **WHEN** 调用方请求 `thinkingEffort: high`
- **THEN** 系统 MUST 先校验该值属于平台枚举集合
- **AND** provider adapter SHOULD 将平台枚举映射到目标渠道协议支持的字段或等价参数
- **AND** 调用日志 MUST 记录请求值和实际应用值

#### Scenario: 渠道或协议不支持请求的 thinkingEffort

- **WHEN** 测试调用或真实调用发现目标渠道、协议或模型不支持请求的 `thinkingEffort`
- **THEN** 系统 MUST 返回结构化业务错误
- **AND** 系统 MUST NOT 静默降级到其他 effort
- **AND** 系统 MUST NOT 向渠道发送不受支持的专有 thinking 参数

#### Scenario: 未传 thinkingEffort

- **WHEN** 调用方未传 `thinkingEffort`
- **THEN** 系统 MUST 使用档位默认值或模型默认行为
- **AND** 若档位默认值在实际调用中不受目标渠道、协议或模型支持，系统 MUST 返回结构化错误并指示需要修正档位配置

### Requirement: 文本能力必须提供可用性和降级状态

系统 SHALL 为`plugin.ai-core.ai.text.v1`提供`Available(ctx)`、`Status(ctx)`和方法级状态等状态能力。owner 插件禁用、卸载、provider 冲突、档位未配置、模型禁用或密钥不可用时，系统 MUST 返回明确的不可用状态或业务错误，而不是产生宿主 500。动态插件读取状态时 MUST 通过 owner-aware host service 和授权快照进入 owner handler。

#### Scenario: Provider 插件不可用

- **WHEN** `ai-core`插件被禁用、卸载或启动失败
- **THEN** `Available(ctx)` MUST 返回不可用
- **AND** `Status(ctx)` MUST 返回能力 ID、provider 插件状态和不可用原因
- **AND** 调用`GenerateText` MUST 返回结构化不可用错误

#### Scenario: 档位未配置

- **WHEN** 调用方使用未配置启用主绑定的档位生成文本
- **THEN** 系统 MUST 拒绝调用
- **AND** 错误 MUST 明确指出该档位未配置可用渠道模型

#### Scenario: 可选消费方降级

- **WHEN** 业务功能可选使用文本`AI`能力
- **AND** `Available(ctx)`返回不可用
- **THEN** 业务功能 MUST 隐藏入口、返回零值或按自身规范提示配置缺失
- **AND** 业务功能 MUST NOT 直接暴露 provider 内部错误或空白页面

### Requirement: 文本能力必须保护敏感输入输出

系统 SHALL 将文本 `AI` 调用视为敏感执行路径。宿主抽象能力和 provider adapter MUST NOT 默认记录完整输入、完整输出、隐藏思考内容、密钥、diff 或业务原文。

#### Scenario: 成功调用不记录完整输入输出

- **WHEN** 文本生成成功
- **THEN** 系统 MAY 记录 `purpose`、档位、渠道模型投影、token 用量、耗时和状态
- **AND** 系统 MUST NOT 在宿主日志、调用日志或审计摘要中保存完整 `messages` 或完整生成正文

#### Scenario: 失败调用脱敏

- **WHEN** 渠道调用失败
- **THEN** 错误摘要 MUST 脱敏 API key、认证头、请求体和响应体中的敏感片段
- **AND** 返回给调用方的错误 MUST 保留足够诊断信息，但不得包含密钥或业务原文

### Requirement: 文本能力版本必须为后续多模态能力保留边界

系统 SHALL 将 `framework.ai.text.v1` 限定为同步文本生成能力。图片、音频、向量、重排、工具调用、多模态消息和流式输出 MUST 通过后续独立能力、独立方法或新版本契约扩展，MUST NOT 破坏 `v1` 文本同步响应语义。

#### Scenario: 能力方法不复用文本契约字段

- **WHEN** 后续新增 `image.generate`、`embedding.create`、`audio.transcribe` 或 `audio.synthesize`
- **THEN** 新能力 MUST 使用独立 capability method、host service method 或 `framework.ai.*` 新契约
- **AND** 新能力 MUST NOT 复用 `thinkingEffort`、`messages` 或文本响应字段作为跨模态通用参数

#### Scenario: 后续新增图片能力

- **WHEN** 系统后续新增图片生成或图片理解能力
- **THEN** 新能力 MUST 使用独立 capability type、host service method 或 `framework.ai.*` 新契约
- **AND** `framework.ai.text.v1` 的请求响应字段和错误语义 MUST 保持兼容

#### Scenario: 后续新增流式文本

- **WHEN** 系统后续需要流式文本输出
- **THEN** 系统 MUST 新增独立 streaming 方法或 `framework.ai.text.v2`
- **AND** 现有 `GenerateText` MUST 继续保持同步调用和单次响应语义

### Requirement: 文本 AI 能力必须提供方法级状态

系统 SHALL 由`ai-core/backend/cap/aicap/aitext`为文本`AI`能力提供`Text().MethodStatus`或等价方法级状态读取，并通过 owner descriptor 动态发布为`text.method_status.get`或等价冻结名称。该方法 MUST 与其他`AI`子能力状态语义一致，响应不得暴露 provider 私有模型配置。

#### Scenario: 读取文本生成方法状态

- **WHEN** 插件读取文本生成方法状态
- **THEN** 系统返回该方法是否可用、不可用原因和稳定状态码
- **AND** 不返回 provider 私有模型配置

#### Scenario: 文本 provider 不可用

- **WHEN** 文本 provider 未启用或当前 purpose 不可用
- **THEN** 系统返回结构化 unavailable 状态
- **AND** 插件可以据此降级而不触发 provider 内部错误

### Requirement: 文本 AI 动态 SDK 必须由 owner 插件发布

系统 SHALL 将文本`AI`动态 guest SDK、声明 helper、codec 和 DTO 复用入口迁移到`ai-core/backend/cap/aicap/bridge`或 owner 插件内等价公开包。动态 SDK MUST 复用 owner 契约 DTO，MUST NOT 在 core `pluginbridge/protocol`中继续维护文本 AI 专属请求别名、codec 或常量作为生产 owner。

#### Scenario: 动态插件调用文本生成

- **WHEN** 动态插件调用 owner SDK 的文本生成 helper
- **THEN** SDK MUST 编码 owner、service、version、method 和 payload
- **AND** SDK MUST 通过通用 host call 进入宿主授权和转发路径
- **AND** SDK 不得直接调用 owner 插件内部实现或绕过授权快照

#### Scenario: 文本 DTO 不分裂

- **WHEN** 源码插件和动态插件同时调用文本生成
- **THEN** 两者 MUST 使用同一 owner 契约 DTO 或由 owner 契约定义的等价投影
- **AND** 动态 DTO 不得在 core 中形成与源码 DTO 并行演化的第二套 owner

