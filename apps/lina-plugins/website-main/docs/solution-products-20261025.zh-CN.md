# 解决方案关联商品实施验收

## 展示位与配置

已盘点当前官网七个相关产品展示位，统一使用官网模板配置。商品名称、简介、默认配置月费、公开性及可售状态由本方公开目录提供，不从参考站复制，不再读取目录前三件兜底。

| 方案 | 页面 | 配置项 |
| --- | --- | --- |
| 电商 | `/plan/electricity.html` | `relatedproductselectricity` |
| 网站 | `/plan/web.html` | `relatedproductsweb` |
| 游戏 | `/plan/game.html` | `relatedproductsgame` |
| 出海 | `/plan/Goingtosea.html` | `relatedproductsgoingsea` |
| 金融 | `/plan/financial.html` | `relatedproductsfinancial` |
| 政企、工业入口 | `/plan/government.html` | `relatedproductsgovernment` |
| 安全 | `/plan/safety.html` | `relatedproductssafety` |

部署后在“官网配置”中设置各方案关联商品，每行粘贴一个本方商品页面链接，例如`/products.html?product=starter-cloud`。各方案独立，最多二十四件，去重后保留行顺序。留空显示空态，不擅自替运营选商品。非法或外站链接提示配置错误，不回退查询全目录。

下架、禁用及不满足公开条件的商品不展示；仍公开但暂不可售的商品标明状态并保留详情入口。所有卡片先进入本方商品详情，继续使用已有购买链路。

## 实现与范围审查

- 前端新增`frontend/public/lina/solution-products.js`，由`site.js`统一接入七页，移除`web-solution.js`中的旧推荐读取；配置声明位于`plugin.yaml`。
- 后端仅扩展`idc-core`公开目录的`publicSlugs[]`筛选，最多二十四项。数据库参数化集合筛选在分页前执行，游标绑定集合；保留单商品详情和未筛选分页行为。
- 复用平台租户、发布、分类和定价边界；这是现有匿名公开商品投影，不增加管理数据权限豁免。官网只消费公开接口，没有跨插件私有实现访问。
- 一次有界目录请求复用既有批量价格装配，不按商品逐项请求详情。没有新增数据库结构、服务实例、宿主能力或服务端缓存；浏览器状态随既有三十秒及窗口恢复刷新，失败清除旧结果并可重试，旧请求不能覆盖新配置。
- 本轮未修改宿主；其他会话已存在的宿主及插件改动不属于本轮范围。新增固定界面文案提供中英文，商品内容遵循本方公开字段。

## 验证证据

| 验证 | 结果 |
| --- | --- |
| `idc-core`目录服务、公开控制器与公开接口包测试 | 通过 |
| `TestPublicCatalogAssociation`参数、游标及真实PostgreSQL集合查询测试 | 通过；随机独立测试模式清理，覆盖公开边界、分页前筛选、详情及原编号分页 |
| `website-main`的`go test ./... -count=1` | 通过 |
| 接口英文源文案及翻译覆盖两项治理测试 | 通过 |
| `TC133`七处推荐行为 | 四项通过 |
| `TC118`网站方案回归 | 四项通过 |
| 桌面、英文手机、空态、失败重试截图 | 已视审 |

浏览器证据目录为`temp/20261025/1792898790058-website-test-results/`。本次用源码拦截和本方接口模拟验证行为，不能作为运行后端与后台配置字段已部署的证据。数据库测试直接覆盖新增查询路径；既有全量数据库夹具缺少当前价格查询字段，未宣称全量数据库集成通过。

## 门禁与部署缺口

按`lina-review`完成本任务源码、接口、配置、规范及测试范围审查；本次实现未发现新的阻塞缺陷，但治理与部署尚未闭环：

- `make lint.go dir=apps/lina-plugins/idc-core plugins=1`被既有并发改动的格式问题阻塞：`backend/api/admin/v1/catalog_product_description.go:8`、`backend/internal/controller/admin/admin_v1_catalog_product_create.go:22`、`backend/internal/service/catalog/catalog_management_persistence.go:228`。未擅自格式化其他任务文件。
- 全仓`i18n.check`报告旧官网五文件共二十三项运行时文案问题；本次新增推荐模块未被报告，运行时消息及前端键覆盖检查通过。
- 当前`6680`运行目录未同步本次脚本，后端尚未部署新筛选，已安装模板尚未升级新配置字段，不能声称线上已生效。应后端、模板配置声明与官网脚本配套更新后，再验证真实配置保存、七页读取和下架刷新。
- 偏好主题`72-customer-website-visual.md`的“官网修复持续升级授权”明确“不升级后端……不停止或重启现有服务”。本轮需单独取得本地后端更新及必要重启授权；未执行半套静态发布。

OpenSpec严格校验结果记录在对应变更任务中。治理门禁和实际部署任务保持未完成。
