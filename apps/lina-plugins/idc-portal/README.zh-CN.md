# idc-portal

`idc-portal`管理官网和用户中心模板、公共品牌配置、模板独立配置、导航和密码重置桥接。前端模板是可独立安装的插件。

[English](README.md) | 简体中文

## 能力范围

- 拥有`website-template`和`customer-template`扩展点，唯一硬插件依赖为`idc-core`。
- 提供已安装模板、公共品牌、模板配置、官网导航和会员导航的后台管理页面。
- 通过`GET /x/idc-portal/api/v1/public/brand`提供当前官网配置投影，直接读取现行官网配置；已删除另一套废弃的品牌修订存储及暂存、发布、回滚接口。
- 保留仍被使用的客户外链菜单配置和公开配置接口。

官网源码和渲染资源位于`apps/lina-plugins/website-default/`及`apps/lina-plugins/website-minimal/`，独立用户中心模板位于`apps/lina-plugins/customer-default/`。旧`apps/lina-websites/`工作区已经删除。本插件负责模板管理，不嵌入模板应用。

模板契约、构建和启动方式见[开发说明](../../../DEVELOPMENT.zh-CN.md)及[安装说明](../../../INSTALL.zh-CN.md)。

## 密码重置桥接

仅为现有密码重置邮件注册`GET /admin/auth/reset-password`。客户应用地址来自`LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN`；生产要求`HTTPS`，本地允许明确的环回`HTTP`地址。通过`302`跳转到`/auth/reset-password`，只保留一个非空`token`，忽略请求主机、来源和其他参数。其余`/admin/*`路由仍由管理后台负责。
