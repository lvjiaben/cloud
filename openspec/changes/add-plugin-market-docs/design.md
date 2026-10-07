## Context

插件市场展示需要使用插件随包资源，而不是依赖仓库根文档、开发者说明或外部页面。现有插件清单已经声明名称、描述、菜单、权限、依赖、租户范围和`i18n`语言，因此市场文档可以作为清单资源的旁路补充，先以静态文件落地。

## Decisions

- 文档目录采用`manifest/docs/<locale>/`。
- 每个插件生成`index.md`、`configuration.md`和`changelog.md`三个主题文件；该结构满足当前市场展示需要，同时不把文件名写入宿主运行时契约。
- 单个插件市场介绍不提供安装介绍文档、权限介绍文档或图片资源；插件开发、测试、打包、安装和生命周期统一放入插件管理后台的开发指南，避免每个插件重复维护。
- 开发指南使用仓库内双语静态结构化内容，通过现有插件管理页面按钮在当前页面普通`Card`和折叠区域展示，不新增接口、数据库或第二套插件管理器。
- 文档事实来源优先级为插件`plugin.yaml`、插件现有`README.md`、插件`manifest/config`、`manifest/sql`、`manifest/i18n`和前端菜单声明。
- `zh-CN`和`en-US`文档保持同一事实集合。中文使用全角标点，路径、命令、配置项、权限字符串和插件 ID 使用inline code。

## Boundaries

- 插件自身市场文档仍只新增随包资源；开发指南只修改现有插件管理前端的通用展示逻辑和双语资源，不修改宿主插件扫描、插件市场 API、数据库、权限种子或插件生命周期。
- 含`plugin.yaml`的目录视为本次插件范围；没有`plugin.yaml`的支持目录不生成市场文档。
- 文档中提到的配置只描述当前插件声明和现有页面，不新增功能承诺。

## Validation

- 静态检查所有含`plugin.yaml`的插件均存在两种语言的三个文档文件。
- 静态检查不存在`install.md`、`permissions.md`、`manifest/docs/assets`和 Markdown 图片引用。
- 开发指南执行前端单元测试、类型检查、双语检查、生产构建和真实浏览器中英文及窄屏回归，并人工复核关键截图。
- 运行`openspec validate add-plugin-market-docs --strict`。

## Rule Impact Record

- 文档治理：有影响，已读取`.agents/rules/documentation.md`和`.agents/instructions/markdown-format.instructions.md`。
- 插件资源：有影响，已读取`.agents/rules/plugin.md`。
- `i18n`：有影响，新增双语市场文档资源并同步维护开发指南的宿主前端双语资源；不修改运行时`manifest/i18n`资源。
- 前端运行时：有影响，只增加插件管理页内的开发指南入口和展示；API、数据库、缓存、数据权限、后端运行时和开发工具跨平台无影响。
- 并行协作评估：本次变更为批量静态资源生成，目录和内容由同一规则统一校验，使用本地脚本生成比拆分子代理更能保证一致性。
