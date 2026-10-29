# 会话体检与范围验证

在仓库根目录通过已有上下文入口运行：

```text
go run ./.agents/tools/context.go audit --thread 01a1533d-0a8d-7dd3-a0bb-c6b704a6716a
go run ./.agents/tools/context.go check --path .agents/tools --change streamline-agent-context --plan
go run ./.agents/tools/context.go check --path .agents/tools --change streamline-agent-context
```

`audit --file path/to/session.jsonl`显式选择文件；`--thread`在`CODEX_HOME/sessions`下定位精确ID，未配置时使用用户的`.codex/sessions`。多个匹配会失败。体检流式读取最多1 GiB，单条记录最多8 MiB，总输出最多8 KiB，包括最多八条重复读取路径与八条大文本记录。不会执行或复制原始命令、消息、内部推理、凭据或图片编码。损坏或超限记录返回不完整摘要及非零退出码。

用量取自日志中最新的计数，缺失项保留为`null`；累计输入包含重复上下文，不能据此计算账户费用。读取热点只识别采用`JSON`引号的命令参数中的`cat`和`sed -n`，属于启发式定位，不证明重读没有必要；最多统计4096个不同源路径，超限报告不完整。工具名称采用固定词表，其他名称合并归类。文本字节、图片块和未知序列化输出分别计数，不按字节估算`token`。

`check`必须显式指定可重复的`--path`或`--change`，不会自动纳入整个脏工作区。`--plan`不运行验证命令、不创建日志。执行时复用范围内的`Go`包测试、每个所属模块一次官方`linactl lint.go`、独立工具测试及`go vet`，并严格校验相关活跃`OpenSpec`变更。`go.mod`或`go.sum`变更选择整个所属模块。支持的文档检查`JSON`语法、空白和`Markdown`本地内联链接存在性，不验证渲染排版或锚点。

检查串行执行，完整命令输出保存在带时间前缀的`temp/YYYYMMDD/`日志中。摘要只包含状态、退出码和日志位置；已结束进程保留真实退出码，命令缺失或禁止启动时报告`not_started`及127或126。单项超时默认为`--timeout 2m`，最大接受`10m`；超时状态为`timeout`，该项采用124约定码。以上失败及前端行为、`SQL`验证等未配置类别均使整体命令非零退出，不会显示为通过。领域验收、数据库场景、外部状态和必要`E2E`仍需本任务的对应证据。

一次最多接受二十个路径、十个变更、两千个文件、每个模块三十二个显式`Go`包和二十四项检查；单份文档最多1 MiB，文档正文总量最多16 MiB。达到边界时缩小任务。计划和摘要最多8 KiB，完整源码与日志保留供定向检查。
