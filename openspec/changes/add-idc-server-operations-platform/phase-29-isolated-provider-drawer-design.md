# 独立服务器接口页面的真实 Vben 抽屉接入

## 已确认范围与根因

`frontend/src/mount.ts`仍实际挂载`src/ProviderConfiguration.vue`。前序修复已把两张表接入真实`Vben`运行时，但其新增、编辑仍使用原生表单和自绘遮罩，且缺少接口详情入口。正式`pages/provider-configuration.vue`的大抽屉不能跨不同`Vue`实例直接传入该独立入口。

本次仅补齐同一服务器接口、资源池配置和只读接口详情，不实现待确认的上游商品导入发布、定价或自动开通。

## 最小接缝

继续使用`vben-adapter`现有单份`/isolated`运行时及失败可重试加载器，增加`mountIsolatedVbenFormDrawer(container, options)`。内部使用真实`useVbenDrawer`和`useVbenForm`，不绘制第二套弹层，不导入宿主应用的私有适配器。

输入只包含当前场景需要的字段描述、普通值、已翻译标题与提示、只读标识、提交及关闭回调；返回`ready/update/close/destroy`。字段组件使用官方组件，禁止跨`Vue`实例传递`VNode`、宿主令牌、全局状态或请求对象。接口请求、权限、配置字段声明、秘密替换凭证与保存顺序继续由`idc-resource`拥有。

通过`update`更新连接器切换后的字段及服务端分页候选；初始值只在明确传入时更新，不能因加载标识变化覆盖员工正在输入的值。关闭立即失效化异步代次并清理秘密，迟到提交或详情不得重新打开已关闭抽屉。加载失败沿用现有业务错误提示，下一次打开允许重新挂载恢复。

公共运行时只增加当前字段类型的界面适配，不增加业务状态、收费规则、领域协议或网络访问。独立资源继续由现有版本化插件静态资源入口发布；不修改宿主，不读取宿主存储，不初始化文档级偏好。

## 验证与边界

- 检查插件本地`AGENTS.md`：`idc-resource`和`vben-adapter`均不存在，适用根规则。
- 命中前端、架构、插件、数据权限、接口、测试、国际化、开发工具、文档和`OpenSpec`规则；不新增数据库、后端或缓存依赖。
- 运行时通用按钮和校验复用官方语言，业务字段复用资源插件已有中英文键；英文为源文案。
- 增加自包含加载器及字段投影测试、独立构建、显式类型检查和真实运行时浏览器用例。浏览器由主线串行执行，截图必须等待动画结束再审阅。
- 审查打开、取消、重复打开、提交失败保留、成功刷新、切换连接器、候选分页、权限失效清理、资源失败重试以及模块未加载时无文档级副作用。
- 构建使用现有跨平台`Node/Vite`入口，无平台专属生产命令。当前浏览器冻结期间不改前端生产文件。

## 当前实现和验证证据

- 已接入真实`useVbenDrawer/useVbenForm`及同版`Ant Design Vue 4.2.6`字段组件，接口请求仍由原资源拥有方发出。通用抽屉接缝只有普通值和回调，不接收令牌，不创建业务请求。
- 运行时复用同一个已具失败重试的加载器；本次新增`context.ts`只提取既有隔离语言初始化，避免循环引用，没有新增文档级偏好初始化。资源页原有自定义十六进制颜色变量改用`--resource-*`前缀，防止覆盖`Vben`的`HSL`令牌。
- `pnpm run test`通过七项自包含测试：字段映射、只读禁改、零值和关闭开关有效、原有网址语法、表格与抽屉共享加载、加载前关闭或销毁、失败后再次打开重试。加载器测试执行实际源码，仅替换远程模块边界，不把测试替身当作真实页面验证。
- `pnpm run build`通过，独立公开加载器可由`Node`导入；`runtime.js`仍为单文件，不出现下级失败模块缓存造成的假重试。当前产物约`3.64 MB`，压缩约`916 KB`，增加的体积来自真实表单、抽屉和字段组件。
- 运行时全量源文件显式类型检查通过：`pnpm exec vue-tsc --noEmit -p D:/golang/cloud/temp/20260904/202500-isolated-runtime-tsconfig.json`。资源独立`vue-tsc`、生产构建和产物校验均通过。
- 资源正式五页及任务详情的显式类型检查通过，测试`TC010a-b/TC012a-f`类型检查通过。`TC010b`覆盖官方确认校验、保存失败留在抽屉、重试成功、关闭清空密码、资源池新增、文档主题与语言未改变；`TC010a`覆盖详情统计和编辑重开。
- `make.cmd i18n.check`通过。资源与核心模块`make.cmd lint.go dir=apps/lina-plugins/<plugin-id>/backend plugins=1`均通过，零问题；后端规则允许该官方入口，既有`make.cmd lint`入口并不存在。

## 本次追加文件范围

范围来源为主仓`git status --short`及未跟踪插件路径展开。当前迁机工作树中的插件不是独立子仓，不把整个未跟踪插件目录视为已审查。

- `vben-adapter/frontend`：`package.json`、`pnpm-lock.yaml`、`scripts/verify-build.mjs`、`src/isolated/types.ts`、`index.d.ts`、`loader.ts`、`runtime.ts`，新增`context.ts`、`form-drawer.ts`、`form-schema.ts`、`form-schema.test.ts`和`loader.test.mjs`。
- `vben-adapter/hack/tests/pages/IsolatedMountPage.ts`：测试挂载层级与截图动画等待，不修改宿主页面。
- `idc-resource/frontend/src/ProviderConfiguration.vue`、`src/styles.css`：独立配置接入及局部样式清理。
- `idc-resource/frontend/pages/components/provider-task-detail-drawer.vue`：详情读取与关闭代次。
- `idc-resource/hack/tests/e2e/TC010-standalone-vben-tables.ts`、`TC012-resource-pagination.ts`和`hack/tests/pages/ResourcePaginationPage.ts`：验证业务结果、失败恢复、迟到响应及截图目录。
- `idc-core/manifest/i18n/zh-CN/apidoc/server-directory-pagination.json`及`backend/api/server_directory_apidoc_i18n_test.go`；`idc-resource/manifest/i18n/zh-CN/apidoc/resource-pagination.json`及`backend/api/resource_pagination_apidoc_i18n_test.go`：修正分页翻译键并增加元数据门禁。

## 阶段审查结论

按`lina-review`检查当前追加范围；已读取规则域包括前端、测试、国际化、架构、插件、数据权限、接口、后端、文档、开发工具及`OpenSpec`。本次没有数据库结构、收费、库存、上游写入或缓存语义变更；工具入口为现有跨平台`Node/Vite`，没有宿主源文件变更。

静态和构建门禁通过，但真实浏览器和截图多模态审阅尚未执行，故不能标记交互验收完成。主线需要正常重启嵌入新运行时后，串行执行上述用例并检查浅色、暗色和窄屏下真实字段及底部按钮；独立`Ant Design`表单没有读取宿主私有主题或令牌，其暗色对比度尤其需要实测。上游商品导入、发布和定价仍按另行确认范围处理，不在本次补齐。
