## ADDED Requirements

### Requirement: 插件清单资源必须可携带市场文档

含`plugin.yaml`的插件 SHALL 可以在`manifest/docs/`下携带插件市场展示文档。官方插件 SHOULD 至少提供`zh-CN`和`en-US`两种语言的文档资源，并覆盖功能介绍、配置说明和更新日志。首版市场文档 SHALL 不提供安装介绍文档、权限介绍文档或图片资产，避免文档资源超出市场首版展示范围。

#### Scenario: 官方插件提供双语市场文档

- **WHEN** 插件目录包含`plugin.yaml`
- **THEN** 插件在`manifest/docs/zh-CN/`和`manifest/docs/en-US/`下提供市场文档
- **AND** 两种语言的文档描述同一组功能、配置项和版本事实
- **AND** 文档目录不包含`install.md`、`permissions.md`或`assets/`图片资源

#### Scenario: 无运行时配置的插件说明配置边界

- **WHEN** 插件没有`manifest/config`资源且没有独立设置页面
- **THEN** 插件市场配置文档说明当前版本无需额外配置
- **AND** 文档不得虚构不存在的配置项或运行时开关

#### Scenario: 支持目录不作为插件生成市场文档

- **WHEN** `apps/lina-plugins/`下的目录缺少`plugin.yaml`
- **THEN** 该目录不作为可安装插件处理
- **AND** 本轮插件市场文档补齐不要求该目录提供`manifest/docs`

### Requirement: 插件管理后台提供内嵌开发指南

系统 SHALL 在现有插件管理页面提供双语插件开发指南入口。管理员点击后 SHALL 在当前后台界面展开仓库随版本交付的指南，内容 SHALL 覆盖清单、扩展点、源码插件、动态`WASM`插件、后台与客户中心页面、安全桥、接口实例、商品履约、测试、打包、安装、升级、停止新业务、停用和卸载。系统 SHALL NOT 为指南另建插件管理器或强制跳转外部文档站点。

#### Scenario: 管理员查看插件开发指南

- **WHEN** 管理员在插件管理页点击“开发指南”
- **THEN** 当前页面使用普通`Card`、目录和折叠内容展示当前界面语言的指南
- **AND** 页面不出现营销横幅、整页跳转或第二套插件管理入口
