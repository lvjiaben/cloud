# 官网公开接口说明

## 已有接口

当前仓库已经有匿名商品目录，官网通过同源前缀`/x/idc-core/api/v1`调用：

| 方法 | 路径 | 用途 |
|---|---|---|
| `GET` | `/public/catalog/products` | 有界分页读取已发布商品 |
| `GET` | `/public/catalog/products/{publicSlug}` | 读取一个商品 |
| `GET` | `/public/catalog/products/{publicSlug}/configuration` | 读取当前配置图 |
| `POST` | `/public/catalog/products/{publicSlug}/configuration-validations` | 服务端校验完整选择 |
| `POST` | `/public/catalog/products/{publicSlug}/configuration-quotes` | 计算当前权威报价 |

金额使用服务端返回的币种和最小单位字符串，时间使用 Unix 毫秒。官网必须设置超时并服务端处理失败，不能因为接口失败输出错误价格。

官网已发布品牌投影由`idc-portal`插件提供：

| 方法 | 路径 | 用途 |
|---|---|---|
| `GET` | `/x/idc-portal/api/v1/public/brand` | 读取已发布的中英文站点名称、标识、主色、导航、页脚和 SEO 投影 |

管理员通过受保护的后台接口暂存、预览、发布品牌修订，也可以回滚到上一个已发布版本。修订保存后不可覆盖。

## 尚未实现

导航、新闻、公告、知识库、下载、协议、`SEO`内容投影和购物车交接的正式接口仍属于第十五阶段待办。完成后才能加入`site-sdk`。在正式后端 DTO、权限、缓存和测试存在前，开发者不得自行猜测接口路径，也不得用硬编码内容冒充权威数据。
