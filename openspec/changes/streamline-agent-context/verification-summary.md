# 验证记录

## 范围与结果

- 默认入口`AGENTS.md`与`user-preferences.md`由339859字节减到8297字节，减少97.6%。这是文件文本体量对比，不是实际模型计费Token测量。
- 六技能、四提示和OpenSpec配置由73411字节减到15247字节，减少79.2%。提示只指向技能，流程与加载规则不在多处复制。
- 原偏好324071字节拆为73个原始章节。逐文件校验source/migration哈希，反向重定位唯一相对链接后拼接，和`git show HEAD:.agents/rules/user-preferences.md`逐字节相同。来源Git版本和章节范围保存在`migration-manifest.json`。
- 新入口、导航和所有主题Markdown本地链接检查通过。旧全量加载与禁止复用的执行指令已从当前入口/规则/六技能/四提示移除；历史治理原文明确被当前规则替代且不默认检索。
- 现有`add-idc-server-operations-platform`及其他业务变更、`openspec/specs/`、`apps/`与`hack/`无差异，业务任务状态与正文未修改。

## 验证证据

- 六技能官方`quick_validate.py`全部通过；四提示frontmatter与OpenSpec配置YAML解析、路由链接检查通过。
- `go test .agents/tools/context.go .agents/tools/context_test.go`：13组自包含测试通过。
- `go vet .agents/tools/context.go .agents/tools/context_test.go`：通过。
- `GOOS=windows GOARCH=amd64 go build`及`GOOS=darwin GOARCH=arm64 go build`指定工具源码：通过。产物位于忽略的`temp/`，仅编译验证，未声称在对应系统运行。
- Linux真实`locate 退款`：输出不超过6 KiB，覆盖多个相关规范，排除归档及phase运行报告。
- Linux真实读取既有任务`1.7`返回完整三个嵌套验收项及证据；`2.7.a`精确读取成功。
- 无选择器读取大正文、歧义选择、混合编号、路径逸出、围栏伪任务、超长单行、显式历史读取和输出预算均有回归覆盖；超过12 KiB明确拒绝，不静默截断。
- `openspec validate streamline-agent-context --strict`及`git diff --check`：通过。

## 审查与影响

按`lina-feedback`走独立`openspec-new`治理路径，并按`lina-review`审查本次任务文件。独立审查确认偏好迁移完整，修正了混合任务ID、父任务验收被截断及围栏伪任务问题，补回归验证。

本次只改变AI上下文组织与标准库工具；无业务、UI、API、SQL、运行期DI、i18n、缓存和数据权限变化，不运行无关业务E2E。跨平台只涉及定位工具，使用Go标准库并完成交叉编译。未提交、未推送、未归档。

## 后续维护

新增偏好更新对应主题及导航，不能重新堆积到短入口。索引和工具命中只是定位证据；仍需读取相关完整条款及依赖。工具不是完整CommonMark解析器，无法唯一选出或块过大时改用普通文件工具读取完整相关逻辑单元；不得将这种限制解释为可略过规范。
