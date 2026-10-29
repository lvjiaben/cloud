# 金额展示验证

- [x] TC009：合同详情及已有自动签授权额度保持精确大金额和币种精度，只读验收不签署或创建授权，覆盖CNY、JPY、KWD及完整额度列宽。证据见`production-money-rendering-validation.md`。

# 2026-10-22 功能退役与材料留存

- TC004、TC006、TC009 合同/独立隐私治理用例随功能删除，原金额验收记录保留。
- [x] TC003f：企业认证页材料留存抽屉覆盖中文文案、期限更新、人工保全生效后不可删除、解除保全及二次删除确认；完整列表权限与仅材料权限两变体均通过，窄权限不挂载企业指标/列表且不发企业查询。独立冻结生产构建与受控启动/业务 API 下通过（2026-10-22，2 passed，12.1 秒），真实后端约束另由隔离 PostgreSQL 测试证明。截图见`temp/20261022/131831-material-retention-opened.png`、`131832-material-retention-held.png`、`131834-material-retention-confirmed.png`及`131836-material-retention-only-opened.png`、`131837-material-retention-only-held.png`、`131839-material-retention-only-confirmed.png`。
