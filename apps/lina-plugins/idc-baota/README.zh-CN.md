# IDC 宝塔面板连接器

这个托管插件只对接**宝塔面板本身**。它不会复用无关的 `btHost` 虚拟主机模块，也不会把那个模块当成完成证据。

[宝塔官方 API 概览](https://docs.bt.cn/api/)明确说明，面板接口统一使用表单`POST`，每次请求都必须带`request_time`，并按`request_token = md5(request_time + md5(api_sk))`签名。官方[`getData`](https://docs.bt.cn/api/data/getData)和[`AddSite`](https://docs.bt.cn/api/site/AddSite)页面给出了本插件使用的网站查询与创建表单。

本插件允许管理员重复添加“面板地址 + API 密钥”的接口实例；网站字段只放在插件自己的商品配置和购买配置中。`BuildSitePlan`会冻结经过检查的 PHP 或静态网站计划，而且不会接触 API 密钥。

## 宿主机密签名

插件清单固定选择`bt_panel_double_md5`认证方式。可信的`idc-resource`宿主读取当前冻结接口实例的`api_key`机密句柄，生成当前 Unix 秒，计算全小写双重 MD5，再只把`request_time`和`request_token`注入出站表单。API 密钥、时间戳和签名都不会进入 WASM 输入、输出、日志或履约证据。插件如果自行提交时间戳或签名，宿主会在联网前直接拒绝。

开通时先调用`/data`，按主域名进行不区分大小写的精确匹配。已有网站会直接恢复，不会再次创建。未找到网站时只调用一次`/site`的`AddSite`；提交后断网、HTTP `5xx`或成功响应没有`siteId`时，结果进入“未知、人工核对”，不自动重试。明确 HTTP `4xx`或`siteStatus: false`会安全失败。首版只创建网站，不创建数据库，也不提供不可恢复的删除动作。
