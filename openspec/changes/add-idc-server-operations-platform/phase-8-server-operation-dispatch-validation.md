# 第八阶段服务器操作派单底座验证记录

## 完成范围

- 在`idc-core`新增`serveroperationresourcecap`版本化源码契约，覆盖开机、关机、重启、重置密码、控制台、重装、变配和换`IP`，请求摘要绑定操作、服务、渠道、资源池、服务版本、规范参数及秘密句柄引用。
- 在`idc-resource`新增启动期单向声明工厂和资源拥有方实现；只读取当前启用渠道、资源池、活跃映射及最新完整能力快照，不把本地服务编号当作上游资源编号。
- 派单前严格检查完整能力矩阵、新鲜度、支持级别、必填参数、枚举、整数上下限、正则约束、未知字段和秘密绑定。重置密码只接受名称为`password`的单个不透明句柄，其他本轮日常动作拒绝多余秘密输入。
- Provider 任务继续复用现有持久化、映射解析、受保护写执行、任务查询、资源查询、未知结果冻结和人工接管路径；本轮没有修改智简魔方端点、表单字段、任务号、凭据或未知结果语义，也没有开放客户或后台写接口。
- 内部摘要以固定字节数组写入任务拥有方元数据，避免随机十六进制摘要偶发触发秘密扫描，同时保持跨 owner 投影使用固定 64 位小写十六进制摘要。

## 验证结果

- `serveroperationresourcecap`契约测试连续`10`轮通过。
- 资源参数门禁和真实 PostgreSQL 幂等派单、意图冲突、终态读取连续`20`轮通过；数据库使用每轮随机隔离 schema，未修改正式业务库。
- `make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1`通过，`0 issues`。
- `make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1`通过，`0 issues`。
- `make.cmd plugins.check format=json`通过，扫描`2781`个文件和`13`份插件清单，`0 findings`。
- `make.cmd i18n.check plugins=1`通过，运行时、多语言消息及前端键覆盖均为`0`违规。
- `openspec validate add-idc-server-operations-platform --strict`通过。

## 未完成边界

本记录只证明资源派单底座完成，不代表`8.2`、`8.4`、`8.5`或`8.6`父任务完成。`core`业务操作、客户与后台权限接口、互斥状态收敛、一次性凭据读取、客户页面和真实浏览器旅程仍需继续实现。可信`HTTPS`入口未就绪前，不执行智简魔方生产写测试。
