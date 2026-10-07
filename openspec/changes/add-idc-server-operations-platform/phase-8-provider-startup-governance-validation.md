# Provider 启动装配治理验证记录

## 完成范围

- `resourcecap/spi`只允许按`pluginID + providerID + contractVersion`声明工厂，精确重复声明、空工厂、非规范版本和当前 owner 不支持的主版本均失败关闭。
- 首次冻结得到与声明顺序无关的不可变快照；冻结后继续声明被拒绝，生产代码中唯一一次`Freeze`固定在`idc-resource`的`pluginhost.Jobs`启动回调。
- `idc-zjmfcloud`清单显式硬依赖`idc-core`和`idc-resource`，宿主依赖解析器对缺失、版本范围不满足和依赖环均在安装或启动副作用前拒绝。
- 运行时每次执行前重新读取启用渠道。渠道停用时保留声明和历史，但不会构造`ProviderEnv`、秘密访问器、秘密写入器或 Provider 客户端，也不会接受新任务。
- 静态治理测试扫描全部 IDC 插件，消费插件不得导入`idc-resource/backend/internal`；Provider SPI 不提供运行期`Get`、`Resolve`、`Lookup`或业务调用入口。

## 验证结果

- `GOWORK=off go test ./backend/cap/resourcecap/... ./backend/internal/composition ./backend -count=10 -shuffle=on`：通过。
- 新增的渠道停用回归用例确认读取门禁执行一次、工厂调用为零、秘密访问器与写入器绑定均为零。
- `GOWORK=off go test ./internal/service/plugin/internal/dependency -run 'TestCheckInstallBlocksUnsatisfiedFrameworkVersion|TestCheckInstallReportsMissingAndVersionUnsatisfied|TestCheckInstallBlocksDependencyCycle' -count=10`：通过。
- `make.cmd plugins.check format=json`：扫描 13 份清单和 2778 个文件，`0 findings`。

## 边界说明

本项只补充并收口启动装配、依赖和目录边界测试，没有修改`apps/lina-core`或`web-antd`，没有新增 HTTP 接口、页面、数据库结构、缓存或用户文案，因此 API、数据权限、SQL、前端和 i18n 无新增影响。具体服务器操作编排和页面仍由 8.5—8.14 分别验收，不能因本项完成而提前勾选。
