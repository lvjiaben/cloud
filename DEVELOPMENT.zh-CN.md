# 官网、用户中心与 AI 开发入口

本指南面向刚拉取仓库的开发者和协作`AI`。首次安装的完整命令与配置见[首次安装](INSTALL.zh-CN.md)；本页说明源码在哪里、修改后怎样交付，以及如何接着开发。文档描述当前实现，不表示所有第三方业务均已联调。

## 安装与日常启动

准备`Go 1.25+`、`Node.js 22.12+`、`pnpm 10+`、`make`及可连接的`PostgreSQL`。在仓库根目录执行：

```text
make env.setup
```

填写生成的`.site-runtime/installation/config.yaml`中的数据库连接，再对首次空库执行：

```text
make db.init confirm=init
make dev
```

日常启动只需`make dev`，不要重复初始化已有数据库。该命令先完整构建，再在前台运行下表六个服务；不是源码热更新。修改代码后按`Ctrl+C`结束本次会话，再运行`make dev`。不要使用旧`make stop`管理新入口的会话。`make dev plan=1`只打印计划；端口冲突会报错，不会替你杀掉原进程。

| 服务 | 默认端口 | 用途 |
| --- | --- | --- |
| 后端 | `6030` | 业务、认证和插件接口 |
| 管理后台 | `6031` | `/admin/` |
| 用户中心网关 | `6032` | 当前选中客户模板及同源接口代理 |
| 官网网关 | `6680` | 根据后台选择展示当前官网 |
| 默认官网渲染器 | `6681` | 默认模板的独立`SSR`服务 |
| 简约官网渲染器 | `6682` | 简约模板的独立`SSR`服务 |

首次启动后，进入后台插件管理安装并启用所需模板，然后在“系统设置 → 官网配置”和“系统设置 → 用户中心配置”选择模板并保存。进程启动、插件安装启用、选择展示模板是三个不同步骤。只启动后端不会自动启动网页网关；安装`WASM`也不会启动`Node.js`。

实例密钥、身份与数据库配置位于`.site-runtime/installation/`，必须持久化、备份，不得提交。`temp/output/`是构建输出，不是源码。官网配置存储及独立部署环境变量详见[安装说明](INSTALL.zh-CN.md)。

## 源码与接口去哪里找

| 目标 | 仓库内路径 |
| --- | --- |
| 默认官网 | `apps/lina-plugins/website-default/frontend/site/` |
| 简约官网 | `apps/lina-plugins/website-minimal/frontend/site/` |
| 原生多页官网 | `apps/lina-plugins/website-html/frontend/public/` |
| 官网网关 | `apps/lina-plugins/website-default/gateway/server.mjs` |
| 默认用户中心 | `apps/lina-plugins/customer-default/frontend/customer/` |
| 用户中心网关 | `apps/lina-plugins/customer-default/frontend/server.mjs` |
| 公共品牌、模板配置及开发文档 | `apps/lina-plugins/idc-portal/` |
| 会员导航、商品菜单归属 | `apps/lina-plugins/idc-core/backend/internal/service/productdirectory/` |
| 标准安装启动工具 | `apps/lina-plugins/idc-compliance/hack/tools/installation/main.go` |

官网和用户中心都是独立托管动态插件：`plugin.yaml`声明能力，`WASM`交付插件及公开资源，浏览器运行`HTML/JS/CSS`；默认官网还可单独运行`SSR`。前端不限`Vue`、`React`、`jQuery`或原生脚本。旧`apps/lina-vben/apps/web-customer`已移除，旧`apps/lina-websites/sites/default`不是当前官网入口，不要继续在旧目录开发。

详细契约直接阅读以下文档，后台对应配置页面的开发说明也使用这些文件：

- [官网插件开发](apps/lina-plugins/idc-portal/docs/website-plugin-development.zh-CN.md)：清单、真实页面、导航、公开接口、构建、部署与生命周期。
- [用户中心插件开发](apps/lina-plugins/idc-portal/docs/customer-plugin-development.zh-CN.md)：认证、客户接口、会员导航、独立打包和入口代理。
- [默认客户模板](apps/lina-plugins/customer-default/README.zh-CN.md)和[默认官网模板](apps/lina-plugins/website-default/README.zh-CN.md)：插件自带构建方式。

运行后的完整接口定义为后台同源`/api.json`，默认可访问`http://localhost:6031/api.json`；后台开发说明支持搜索、展开字段和下载。接口以当前运行代码及已安装插件为准，不把历史截图或旧文档当作新接口。认证接口在`/api/v1/auth/`，客户业务主要在`/x/idc-core/api/v1/account/`，公开官网接口在`/x/idc-core/api/v1/public/`；精确方法、参数、单位和权限必须查`OpenAPI`。用户中心不得使用管理员令牌，官网不得携带客户或管理员令牌。

## 一次修改到交付的流程

1. 先定位目标插件，读取根`AGENTS.md`、相关规则和插件本地指令，再读实际页面、接口与对应完整业务规范。
2. 在插件内修改。页面路由必须真实存在，`plugin.yaml`的`pages`只是声明，不会自动生成页面。新插件同步修改唯一标识、包入口及声明，不保留旧插件标识。
3. 调用现有公开接口；金额、报价、权限和数据归属由后端决定，不能在前端伪造。商品选择一个产品类型的会员导航，不在导航编辑里维护一份商品列表。
4. 按插件说明独立构建打包；整仓交付使用根`make build`，动态模板输出到`temp/output/`。用户中心可在插件根执行`pnpm install --frozen-lockfile`、`pnpm run build`、`pnpm run package`，输出`dist/customer-default.wasm`。独立官网步骤见其开发指南；`Windows`整仓构建优先使用根安装工具，避免复制文档中的`POSIX`环境变量前缀。
5. 后台上传`WASM`，首次安装并启用；更新已有版本时提高版本、上传、预览并确认升级，上传不等于升级完成。官网`SSR`修改还需要部署新渲染产物并重启对应服务，不能只上传`WASM`。
6. 选择模板，真实浏览器验证登录、深链刷新、菜单、商品和业务操作；检查网络错误与控制台，按影响运行测试，记录仍未验证的部分。

## 让 AI 接着开发

把仓库根目录作为工作区，向`AI`说明具体目标、页面和可复现问题，可直接使用：

```text
先读取 AGENTS.md、.agents/rules/user-preferences.md 和 DEVELOPMENT.zh-CN.md。
我要修改【官网/用户中心/具体插件】的【页面或流程】，期望【具体结果】。
先定位源码、公开接口和相关业务确认，再实现并完成匹配的测试与真实浏览器验证。
严格不改宿主，不提交、不推送；不能假造接口、价格、供应商能力或成功结果。
已有配置、数据库和其他运行实例必须保留；业务结果有歧义时用大白话先问我。
```

根`AGENTS.md`是工作入口，`.agents/CONTEXT.md`用于定位，`.agents/memory/preferences/`保存已确认业务，`openspec/changes/`记录活跃工作。不要一次加载全部规则和归档。提交之前检查源码、锁文件和公开资源是否可收录，不要把本机密钥、令牌、日志、数据库连接或`node_modules`带入提交。当前指南不授权`AI`自动提交、推送或操作第三方生产环境。

## 常见问题与已知限制

- `6680`连接被拒绝：先确认官网网关及所选模板渲染服务在运行。后台`6031`能开不代表官网已启动；不要在旧分进程实例上直接重复运行整套服务占用端口。
- 页面返回模板不可用：核对后端、插件安装启用、后台模板选择、网关后端地址及`SSR`上游，不要靠删除配置或绕过检查恢复。
- 当前版本已验证干净源码构建、空库启动、十六个动态插件安装启用、模板切换与实际页面。没有真实供应商账号的业务不算联调通过。
- 部分模板停用、卸载仍可能触发宿主扩展授权快照缺失的`owner unknown`错误。保留插件并切换展示模板；不能直接改表、删除声明或绕过检查。详见[安装验收与剩余问题](openspec/changes/repair-clean-installation/runtime-validation.md)。
- 禁止修改`apps/lina-core/`、`apps/lina-vben/`和根`hack/`。唯一安装资源例外是调用未修改的官方`pack.assets`生成`apps/lina-core/internal/packed/manifest/`，不包括修改宿主逻辑；完整边界见[主题74](.agents/memory/preferences/74-host-boundary.md)。
