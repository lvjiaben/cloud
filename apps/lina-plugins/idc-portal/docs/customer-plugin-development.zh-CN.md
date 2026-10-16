# 用户中心模板开发

用户中心可以使用原生 JavaScript、jQuery、Vue、React 或其他技术栈。交付物是 HTML、JavaScript、CSS，不要求使用 Vben。内置参考源码为 `apps/lina-plugins/customer-default/frontend/customer`。

## 品牌与模板配置

GET `/x/idc-portal/api/v1/public/customer-center/configuration` 无需登录。响应 `data` 中 `active` 表示当前模板是否可用，`selectedPlugin` 为模板标识，`common` 为所有客户模板共享的品牌资料，`config` 为当前模板自己的公开设置，`entry` 为版本化静态入口。名称、Logo、标题、公司、联系方式在后台“用户中心配置”填写。切换模板保留公共资料，各模板设置独立保存。禁止将密钥放入公开设置。

GET `/x/idc-portal/api/v1/admin/customer-center/configuration` 读取公共配置、插件候选和配置结构；PUT 同路径保存 {`selectedPlugin`,`common`,`config`}，需要 idc:admin:portal:customer-template 权限。客户端只能选择服务端提供的已安装启用模板。

## 会员导航与业务接口

登录后 GET `/x/idc-core/api/v1/account/product-directories?page=1&pageSize=100` 获取当前客户的启用导航。产品类型路径为 `/account/product-directories/{directoryNo}`；其他类型使用 `targetUrl`。显示 `nameZhCn/nameEnUs`、icon、`sortOrder`、`serviceCount`；按 `total` 继续翻页。导航入口不能替代后端权限判断，所有客户接口均按当前登录人校验。

完整的、与当前安装插件一致的接口定义在本站 `/api.json`。后台本页提供实时文档入口；开发时以该文档中的请求方法、字段、响应和权限为准，不自行猜测接口。认证使用 POST `/api/v1/auth/login`（`username`、`password`、`clientType=web`）；成功取 `data.accessToken` 和 `refreshToken`。若返回 `preToken` 和 `tenants`，调用 `POST /x/tenant-core/api/v1/auth/select-tenant`，提交 `{preToken, tenantId}`完成登录后再进入业务。续期 POST `/api/v1/auth/refresh`，退出 POST `/api/v1/auth/logout`；刷新提交 `{refreshToken}`，退出携带当前访问令牌并提交 `{}`，字段以运行中的 OpenAPI 为准。业务请求携带登录响应中的 `Bearer accessToken`。不要把管理员令牌、`AppSecret` 或其他客户身份放入模板。

## 插件声明和安装

参考 customer-default 的 `plugin.yaml`：type=dynamic，`business_extensions` 的 point=customer-template、owner=idc-portal、version=v1、role=provider、methods=[capabilities.describe]。`provider_metadata.type=customer-template-contract`、version=v1，values.entry=index.html、spa=true；`public_assets.source` 指向构建目录，mount=/、index=index.html。`entry` 必须为包内相对 HTML 路径，不能填写外部地址。

`interface_configuration` 为模板自己的公开配置，`cardinality=singleton`，最多30个字段，支持 single_line、multi_line、boolean、integer。pages 声明已经实现的客户路由和中英文名称；不能声明后台或接口路径。框架无关模板提供 `entry` 即可，不需要声明内置 renderer。

内置模板整个目录可复制到仓库外；安装标准 Node.js 22+、pnpm 10.30.3 和 Go 1.25+ 后，在插件根运行 `pnpm install --frozen-lockfile`、`pnpm run build`、`pnpm run package`，输出 `dist/customer-default.wasm`。必要公开组件和客户页面源码已带许可复制到插件 vendor，无需宿主源码、源码插件前端或仓库构建工具。将该包在后台“插件管理”上传安装并启用，再到“用户中心配置”选择保存。升级保留配置，停用/卸载不删除订单、余额、服务和工单。

## 独立用户中心入口

运行 node `apps/lina-plugins/customer-default/frontend/server.mjs`，`CUSTOMER_PORT` 默认6032，`LINAPRO_BACKEND_PROXY_TARGET` 默认 http://127.0.0.1:6030。同一端口只运行一个入口；旧 web-customer 应用已由独立插件替代。网关跟随后台选择的任意模板，不绑定内置框架。SPA 支持深层地址刷新；非SPA仅根入口，其它资源使用版本化 `/x-assets` 地址。

读取配置的最小例子：

```javascript
const response = await fetch('/x/idc-portal/api/v1/public/customer-center/configuration');
const body = await response.json();
if (!response.ok || body.code !== 0) throw new Error(body.message || 'Request failed');
const data = body.data;
document.title = data.common.siteName;
```

接口失败提供重试，不伪造成功；分页使用服务端 `total`；时间字段按各接口文档使用毫秒时间戳。历史配置不含客户业务数据。

最小合法声明（字段必须至少1个，最多30个）：

```yaml
id: my-customer
name: My customer template
version: v0.1.0
type: dynamic
distribution: managed
scope_nature: platform_only
supports_multi_tenant: false
default_install_mode: global
dependencies:
  plugins:
    - { id: idc-portal, version: ">=0.1.11 <1.0.0" }
business_extensions:
  - point: customer-template
    version: v1
    role: provider
    owner: idc-portal
    methods: [capabilities.describe]
    interface_configuration_cardinality: singleton
    interface_configuration:
      version: v1
      fields:
        - key: primarycolor
          type: single_line
          label: { en-US: Theme color, zh-CN: 主题颜色 }
          maximum_length: 7
    provider_metadata:
      type: customer-template-contract
      version: v1
      values: { entry: index.html, spa: true }
public_assets:
  - { source: frontend/dist, mount: /, index: index.html }
```

复制示例的 Go provider 入口并调整插件标识，参考本插件以标准库实现公开版本化 Bridge 协议，保持插件标识及能力声明一致；默认模板的 `go.mod` 不依赖宿主或 idc-portal 源码；把 HTML 放到 `frontend/dist/index.html` 后，从仓库根执行：

```bash
make wasm dir=apps/lina-plugins/my-customer
```

产物为 `temp/output/my-customer.wasm`。后台上传该 WASM（不是 ZIP）并安装启用。公开上传接口 POST `/api/v1/plugins/dynamic/package` 使用 multipart file；首次 POST `/api/v1/plugins/my-customer/install` {}，PUT `/api/v1/plugins/my-customer/enable` {}；升级必须增加版本、重新打包上传，再 POST `/api/v1/plugins/my-customer/upgrade` {"confirmed":true}。停用和卸载走插件管理既有影响检查，不要直接删服务器文件。

当前部署的生命周期限制：模板选择、配置、正式上传安装和升级已验证；部分模板的停用/卸载影响检查仍可能被宿主返回“owner unknown”。遇到该结果应保留插件并切换展示模板，不得绕过检查、直接删除文件或声称卸载成功。该宿主限制不通过修改宿主规避。

开发文档在本页按保留换行的纯文本显示；代码和 YAML 可直接复制。公共配置接口只返回展示资料，登录返回的访问令牌、刷新令牌与其他秘密不属于模板公开配置。


## 整仓启动与开发交接

首次安装见根[安装说明](../../../../INSTALL.zh-CN.md)，源码定位、交付步骤和`AI`协作提示见[开发入口](../../../../DEVELOPMENT.zh-CN.md)。根`make dev`会统一启动客户网关及官网，无需另开占用相同端口的进程。独立模板升级需提高版本并走正式上传、预览、确认升级流程。
