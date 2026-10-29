# 本地订单诊断

在仓库根目录运行：

```text
go run ./.agents/tools/context.go order --order ORD_example --session-file temp/private-session.json
```

显式选择含`accessToken`字段的私有`JSON`会话文件，或在进程环境设置`LINA_DIAG_TOKEN`并省略`--session-file`。工具不会发现凭据、登录、刷新会话或打印令牌。不要把凭据放在命令行或受版本控制的文件中。`--base-url`默认为`http://127.0.0.1:6030`，只接受本机回环来源。

摘要包含最小货币单位的精确账单金额、订单和履约状态、本地自动化阶段及结果代码、资源任务状态和已保存的上游任务及资源编号。工具读取既有员工接口，保留精确操作权限和数据范围检查。可选的`execution`投影需要更新后的核心后端；旧运行版本对自动交付中的任务报告`execution_unavailable`，不声称证据完整。

最多检查三条履约任务和三条不同资源任务，总计最多八次`GET`。单次响应限制为1 MiB，单次请求十秒，整体45秒；禁止重定向和代理。大订单、权限不足、登录失效、响应无效和执行证据缺失均返回部分摘要及非零退出码。需要更多证据时明确缩小排查目标，不扫描其他客户。

此命令只读本地已保存事实，不调用供应商、不写数据库、不重试开通，也不把上游结果未知判为成功。输出不包含联系方式、备注、原始响应或秘密。`complete`只表示有界查询均已完成，不代表订单已交付；账单已付或一次接口调用成功不能证明服务器已创建。

在仓库根目录运行自包含测试：

```text
go test apps/lina-plugins/idc-core/hack/tools/order-diagnose/main.go apps/lina-plugins/idc-core/hack/tools/order-diagnose/diagnose.go apps/lina-plugins/idc-core/hack/tools/order-diagnose/diagnose_test.go -count=1
```
