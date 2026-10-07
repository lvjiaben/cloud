# 第二十阶段反馈修复、界面与插件持久化验证记录

日期：2026-09-01  
对应任务：`FB-62`

## 反馈范围与根因

本轮接收了用户对客户中心、管理后台菜单、SSL 插件空白、辅助报价表格高度、销售渠道页面层级、VxeGrid 装饰条以及本地插件重启后反复安装的反馈。静态检查和真实浏览器复核确认：

- 客户中心沿用了旧的圆角/浅色缓存偏好，和企业云页面令牌混用；后台和客户端需要各自作用域隔离。
- AI、副驾驶、下游渠道和官网品牌菜单的 manifest 文案、父级归属或同版本投影没有完全刷新，导致英文、菜单层级不稳定或入口不明显。
- SSL 管理页的 HTML 使用了多一级 `admin/` 相对路径，浏览器实际请求了不存在的脚本；沙箱 iframe 的匿名资源还缺少跨源响应头。
- `use-vxe-grid` 默认渲染了横跨筛选区的装饰分隔条；辅助报价页的网格没有把剩余高度传递给表格内部滚动容器；销售渠道页多包了一层普通大 Card。
- `linactl build` 默认输出目录同时是动态插件运行时存储目录，原先整体删除目录会删除 `.capability-storage`、`.runtime`、`releases` 和根目录插件 wasm，宿主重启后只能把插件当作孤儿并要求重新安装。

## 实现收口

| 范围 | 实现与边界 |
| --- | --- |
| 客户中心与后台视觉 | `web-customer` 增加客户端独立企业云令牌（冷灰背景、深色导航、蓝色主色、直角信息面板）；客户端和管理端在偏好初始化后再次应用产品级主题，避免浏览器旧缓存重新带回圆角/串色。默认官网继续使用独立 Nuxt 样式，不与两个后台文档泄漏。 |
| 菜单 | AI、副驾驶、下游渠道和官网品牌补齐中英文 manifest 菜单资源，并把入口分别归入运营工作台、客户与销售、工单与内容等业务父级；内建插件启动时每次同步菜单/权限投影，修复同版本 manifest 更新后仍显示旧层级。资源插件的服务器续费菜单也补齐双语；官网品牌增加插件自有 `portal-brand.vue` 页面桥接静态品牌挂载入口，保证菜单可直接打开。 |
| SSL 与沙箱 | SSL 管理页改用同级 `../entry.js`；宿主为无凭据、版本化的公开插件 iframe 资源返回匿名 `Access-Control-Allow-Origin: *`，不开放带凭据的业务 API。 |
| 表格与页面结构 | 辅助报价页使用 `Page` 剩余高度、`min-height: 0` 和 VxeGrid 内部 flex 链路；销售渠道页移除外层大 Card，保留插件自有内容壳。 |
| VxeGrid | 删除默认装饰分隔条及其额外底部留白，保留公开 `separator` 类型以避免无关调用方接口破坏。 |
| 动态插件持久化 | 默认 `temp/output` 仅清理生成的宿主交付文件；显式保留 `.capability-storage`、`.runtime`、`releases` 和根目录 `.wasm`。自定义输出目录仍保持原有整体清理语义，避免改变正式交付目录行为。 |

## 重启后真实浏览器复核

使用 `D:/golang/cloud/temp/start-phase19.ps1` 停止并重启接口 `6030`、管理后台 `6031` 和用户端 `6032`，随后以已登录管理员会话访问：

- 菜单：中文显示 `AI 运营副驾驶`、`下游渠道管理`、`官网品牌设置`、`服务器续费`；英文旧文案 `AI Operations Copilot`、`Downstream channel tokens`、`Website Brand` 不再作为中文菜单显示。
- SSL：`/admin/plugins/yunsu-ssl/ssl-certificates?...` 的 iframe 正常显示“SSL 证书管理 / 共 0 张证书 / 暂无证书”，`h1` 数量为 `1`，不再空白。
- 官网品牌：`/admin/idc/portal-brand` 正常加载插件自有品牌表单，出现“官网品牌设置”、双语字段、保存/预览/发布/回滚操作和“尚未发布品牌修订”状态，不再显示动态页面 404。
- 下游渠道：页面正常显示“IDC 运营 · 销售”和“代理与销售”，没有 `.ant-card` 或 `.vben-card` 外层大卡片，插件壳存在且内容可读。
- 协助报价：`.vxe-grid` 高度 `276px`，`.vxe-grid--table-wrapper` 高度 `109px`，表格内容区非零并在页面剩余空间内滚动。
- 插件管理：重启后的动态插件 `idc-baota`、`idc-dcim`、`idc-v10`、`idc-zjmf-finance`、`idc-zjmfcloud`、`yunsu-ssl` 均显示“正常”，没有重新安装要求。
- 客户中心：`http://localhost:6032/auth/login` 正常渲染，文档带有 `data-customer-console=true`，页面背景计算值为 `rgb(245, 247, 250)`、圆角为 `0px`；本轮没有可用的真实客户账号，因此不把登录后业务流程冒充为验收通过。
- 源码扫描：`use-vxe-grid.vue` 中不再存在用户反馈的完整装饰条 class、`isSeparator` 或 `separatorBg`。

## 自动化验证

以下命令在本轮工作区执行并通过：

- `go test ./internal/service/plugin/internal/lifecycle ./internal/service/plugin/internal/integration ./internal/cmd/internal/httpstartup`
- `go test ./... -run 'TestCleanBuildOutputDirPreservesDynamicPluginRuntimeStore' -count=1`
- `pnpm -F @lina/web-antd typecheck`
- `pnpm -F @lina/web-customer typecheck` 与 `pnpm run build:customer`
- `pnpm run build:antd`
- `pnpm run build:default`
- `pnpm test`（`apps/lina-plugins/yunsu-ssl/frontend`）
- `./make.cmd i18n.check`
- `./make.cmd plugins.check plugins=1`
- `./make.cmd lint.go`
- `openspec validate add-idc-server-operations-platform --strict`
- `git diff --check`（仅共享基线已有的 CRLF 提示，无空白错误）

## 规则影响与未决边界

- 本轮命中前端、测试、i18n、架构、插件、后端 Go、开发工具、文档和 OpenSpec 规则；未修改业务 API、数据库表、数据权限边界或缓存契约，因此这些领域没有新增业务语义变化。插件构建工具的跨平台行为通过 Windows 测试覆盖；自定义输出目录的清理语义保持不变。
- 客户端真实业务登录账号、腾讯电子签、诺诺、真实魔方云对象和真实支付/上游/通知供应商的证书、主机、回调、费用与安全联调仍缺外部条件，继续按用户确认保持暂停，不能用本地假服务勾选剩余外部门禁。

## 结论

用户反馈中可在本地完成的视觉、菜单、页面加载、表格高度、页面层级和动态插件持久化问题已关闭；`FB-62`具备勾选依据。重启或正常构建不会再清理已安装动态插件的运行时存储，用户无需每次手动重新安装。
