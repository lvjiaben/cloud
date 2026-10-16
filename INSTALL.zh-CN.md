# 首次安装

官网、用户中心源码定位、开发交付与`AI`协作流程见[开发与协作入口](DEVELOPMENT.zh-CN.md)。

本仓库已包含业务插件源码，不需要从其他版本的官方插件仓库重复下载。准备`Go 1.25+`、`Node.js 22.12+`、`pnpm 10+`和`PostgreSQL`，在项目根目录执行以下命令。

## 开发安装与启动

先准备好可连接的`PostgreSQL`数据库，再准备运行依赖和实例配置。以下命令不会安装数据库服务，也不会删除或重建已有数据库。

```text
make env.setup
```

`make env.setup`安装前端运行依赖，保存实例密钥，并根据官方配置模板生成`.site-runtime/installation/config.yaml`，已有配置不会覆盖。编辑该文件填写数据库连接信息；后续安装工具默认使用它，也可显式设置`GF_GCFG_FILE`使用其他配置文件。配置文件包含数据库等敏感信息，不要提交到`Git`。

首次使用全新数据库时，单独执行一次初始化：

```text
make db.init confirm=init
```

数据库已准备好后，启动整套开发服务：

```text
make dev
```

根`Makefile`将`env.setup`、`db.init`、`build`和`dev`交给插件安装工具。`env.setup`只准备应用运行依赖和实例材料；`dev`先构建全部交付产物，再在当前终端运行后端、后台、客户网关、两套官网渲染服务和官网网关。默认后台为`http://127.0.0.1:6031/admin/`，客户中心为`http://127.0.0.1:6032/`，官网为`http://127.0.0.1:6680/`。在启动终端按`Ctrl+C`结束本次启动的服务，不使用旧`make stop`管理此会话；端口已占用时明确报错，不终止其他实例。修改代码后重新运行`make dev`构建，不提供源码热更新。

首次启动后，仍需在后台插件管理安装、启用所需动态插件，再在官网配置和用户中心配置中选择模板。构建或启动进程不会代替管理员完成安装、启用和模板选择。

构建及数据库初始化前调用官方`pack.assets`，将元数据、`SQL`和语言资源生成到`apps/lina-core/internal/packed/manifest/`，供`Go`编译时内嵌。该官方资源生成是宿主只读规则的精确例外，不修改宿主源码逻辑或根`hack/`。其他构建产物写入`temp/output/`。

可只查看启动计划，不生成密钥、不构建或启动服务：

```text
make dev plan=1
```

需要调整端口时，可使用跨平台的`make`变量参数，例如`make dev LINA_CORE_PORT=16030 LINA_VBEN_PORT=16031 LINA_CUSTOMER_PORT=16032 LINA_WEBSITE_PORT=16680`。

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `LINA_CORE_PORT` | `6030` | 后端接口 |
| `LINA_VBEN_PORT` | `6031` | 后台入口 |
| `LINA_CUSTOMER_PORT` | `6032` | 客户中心入口 |
| `LINA_WEBSITE_PORT` | `6680` | 官网入口；两套渲染服务占用紧邻的后两个端口，默认`6681`和`6682` |

开发浏览器测试、代码检查等工具不属于运行前提，需要时可单独执行官方`go run ./hack/tools/linactl env.setup`。该完整开发工具链包含`Playwright`及系统依赖，可能不支持某些操作系统；其安装失败不表示应用运行依赖安装失败。

## 准备和构建

```text
make env.setup
make build
```

该插件安装工具直接构建未修改的宿主，按公开`official_plugins`注册全部源码业务插件；后台资源输出到`temp/output/admin/public`，不回填宿主。用户中心、官网使用各插件自带构建工具，动态插件包输出到`temp/output`，不依赖已删除的旧客户应用或旧官网源码。此处使用插件交付构建命令，不使用仍依赖旧客户应用的宿主`linactl build`入口。

`make build`默认完整构建。需要查看计划或限定构建范围时，使用插件命令`go run ./apps/lina-plugins/idc-compliance/hack/tools/installation/main.go build plugins=1 plan=1`；不生成密钥、不启动服务。将`plan=1`替换为`target=backend`只构建后台和后端，替换为`target=templates`只构建动态插件与模板。插件命令的`out=目录`或根命令的`make build output_dir=目录`可改变交付输出，禁止指向源码目录。

## 数据库和启动

以下是分进程部署方式；开发时优先使用上方`make dev`统一启动。沿用前面准备的数据库配置，仅对全新数据库执行初始化，已有数据库不要重复初始化。

```text
make db.init confirm=init
go run ./apps/lina-plugins/idc-compliance/hack/tools/installation/main.go run ./temp/output/lina
```

在另一个终端启动后台入口：

```text
node ./temp/output/admin/server.mjs
```

后台默认在`http://127.0.0.1:6031/admin/`，代理后端`http://127.0.0.1:6030`。可用`ADMIN_PORT`、`ADMIN_HOST`、`LINAPRO_BACKEND_PROXY_TARGET`调整；生产环境由反向代理和进程管理器运行。后台支持页面深层地址刷新，后端进程和后台入口须分别保持运行。

在`Windows`中将后端可执行文件路径替换为`./temp/output/lina.exe`。后台插件列表用于安装、启用已构建的托管插件。第三方业务须配置自己的供应商账号，安装成功不代表已经连接供应商。

## 官网配置

在后台插件列表安装并启用官网插件，然后进入“系统设置 → 官网配置”。可以安装多个官网插件，但在官网配置中只能选择一个展示。切换后旧官网自动退出展示，不需要停用或卸载旧插件，各模板配置保留。公共信息在这里统一设置；首页图片、菜单等配置由所选官网插件提供。未启用官网时显示“官网尚未启用”。

完整安装构建已输出官网WASM及SSR服务到`temp/output/websites/`。官网源码和构建工具随各官网插件提供，旧`apps/lina-websites`目录已移除。进入`apps/lina-plugins/website-default`（也可将整个插件复制到仓库外），执行：

```text
make install build
```

将`dist/website-default.wasm`上传到后台插件管理，预览并确认升级。独立完整`SSR`部署时，在运行环境或进程管理器中为渲染器设置`PORT=6681`、`NUXT_API_ORIGIN=http://127.0.0.1:6030`，再运行：

```text
node frontend/site/.output/server/index.mjs
```

为另一个网关进程设置`WEBSITE_SSR_ORIGINS`为`{"website-default":"http://127.0.0.1:6681"}`，再运行：

```text
node gateway/server.mjs
```

交付包中对应入口为`temp/output/websites/website-default/renderer/server/index.mjs`和`temp/output/websites/website-default/gateway/server.mjs`，可直接用Node运行；在项目根目录设置同样环境变量。以上两个进程分别由进程管理器保持运行；网关默认监听6680。`website-minimal`使用同样命令，可监听6682并加入`WEBSITE_SSR_ORIGINS`映射。网关按后台当前模板选择对应SSR服务，切换后自动路由。模板的SSR服务配置`NUXT_API_ORIGIN`、`NUXT_PUBLIC_SITE_URL`和`NUXT_PUBLIC_CUSTOMER_URL`；网关配置`WEBSITE_API_ORIGIN`和`WEBSITE_CUSTOMER_URL`。不要把用户输入的地址加入SSR上游映射。

没有配置SSR服务时，网关加载动态包内的完整静态应用；商品和内容仍可操作，但属于浏览器渲染，不具备完整SSR搜索收录能力。原生HTML模板直接携带实际HTML页面，使用`make install build`打包即可。后端或WASM安装不会自动启动Node服务。

## 用户中心配置与启动

在后台插件列表安装并启用`customer-default`，进入“系统设置 → 用户中心配置”选择模板并保存公共品牌及模板专属设置。会员导航在后台统一管理，商品只能关联其中一个产品菜单。

`make dev`已统一启动客户入口；分进程运行时，用户中心使用插件独立入口，单独后端进程不会自动启动它。在项目根目录执行：

```text
go run ./apps/lina-plugins/idc-compliance/hack/tools/installation/main.go customer.run
```

默认监听`127.0.0.1:6032`，使用`CUSTOMER_PORT`及`LINAPRO_BACKEND_PROXY_TARGET`配置端口与后端地址。旧宿主客户应用已移除。不要启动另一个进程占用同一端口；已有进程占用时启动失败，不会擅自终止其他进程。生产部署可运行交付包的`node temp/output/customer/server.mjs`，或使用进程管理器运行插件内`frontend/server.mjs`；保留已安装模板包及其公开资源。任意前端框架模板均通过公开接口和资源入口协议运行，开发说明在对应后台管理页。

## 保存实例资料

`.site-runtime/installation`保存本实例的配置、随机密钥和身份，必须持久化并备份，重启继续使用同一目录。不要提交到`Git`或删除后重新生成，否则已有加密资料无法读取。显式部署环境配置优先；文件损坏时工具会停止并提示恢复备份。

官网配置使用文件存储，多个读取实例需共享`LINA_SITE_STORAGE`目录；配置保存只允许单个管理实例写入，不能让多个实例并发更新。首次安装准备工具不替代共享存储配置。

`plugins.install`是下载插件源码的开发命令，不是后台安装命令。本仓库已有完整源码时无需执行；需要下载时，插件侧`official-plugins.yaml`使用当前项目的`main`分支来源，不要混入其他仓库的旧版插件。
