# idc-zjmfcloud

`idc-zjmfcloud`是用于智简魔方云商品配置和服务器操作的托管动态连接器，服务器操作由`idc-resource`统一管理。

[English](README.md) | 简体中文

## 交付方式

`v0.2.1`只以`dynamic + managed`方式交付，不再保留源码插件注册、内置分发或旧兼容路径。在仓库根目录执行下面的命令可构建可安装的 WASM 产物：

```powershell
go run ./hack/tools/linactl wasm dir=apps/lina-plugins/idc-zjmfcloud out=temp/output/idc-zjmfcloud
```

产物声明由`idc-core`拥有的`product-fulfillment@v1`扩展点，供宿主渲染商品配置和客户购买配置，并把服务器契约冻结到已经发布的商品版本。清单中的`configuration.candidates`、`provision`和`query`只作为授权和冻结证据，客体会拒绝从商品拥有方上下文直接执行它们。候选加载、真实开通和后续服务器操作都使用`idc-resource`拥有的`resource-provider@v1`扩展点。资源插件负责把选中的`pfi_*`接口解析到资源渠道、创建持久任务并绑定生成的服务器密码，然后再调用本连接器。

## 结构化配置

`endpoint`、`username`和`password`只在资源渠道页面管理，不能复制到商品自动化配置。地区、线路、处理器、内存、操作系统、系统盘、数据盘和带宽是本连接器声明的服务器专用字段，不进入通用商品模型。地区、线路和操作系统选项通过有版本的`configuration.candidates`方法加载。

## 受控供应商边界

候选加载、服务器开通、查询和后续操作只接受`idc-resource`签发的`resource-provider@v1`上下文，并使用一次性的`confidentialoutbound.execute`服务。候选加载登录后只读取`/areas`、`/nodes`和`/image`，只返回有数量限制的中英文名称和值。资源插件没有绑定生成的服务器密码时，开通会在任何供应商请求发生前失败。供应商地址、账号密码、服务器密码明文、请求头、登录令牌和原始机密错误不会进入客体请求、客体响应、插件页面、日志或证据数据。最终供应商调用仍通过持久异步任务执行，并由资源插件控制。

## 前端页面入口

托管产物分别在`admin.resource-operations`和`customer.products`受控位置发布页面。两个页面都运行在宿主治理的隔离容器内，继承宿主主题和语言，同步页面标题和高度，并且不会读取宿主存储、Cookie 或访问令牌。服务器数据访问和生命周期操作仍由资源业务拥有方及宿主页面桥控制。

## 旧协议校验代码

`backend/internal/provider/zjmfcloud`包含托管 WASM 客体实际使用的协议投影和能力校验测试。它不会注册成源码插件。托管版本契约测试会复用其中的能力声明，避免动态清单和已经验证过的服务器动作矩阵悄悄产生偏差。

生产写入验证仍需要可信的 HTTPS 供应商入口，并且只能操作专门创建的隔离测试对象，严禁使用已有客户资源做破坏性验证。
