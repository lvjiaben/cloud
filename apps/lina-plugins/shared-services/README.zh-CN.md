# LinaPro 共享服务

`shared-services`是一个内建源码插件，拥有两项与`IDC`业务无关的可复用能力：

- `challengecap`：按用途绑定的人机挑战，以及短期、单次使用的验证证明。
- `inboxcap`：向明确的宿主用户编号幂等发布已经渲染好的站内信。

本插件不读取`IDC`客户、员工、工单、账务或服务表。消费插件必须声明硬插件依赖，并且只能使用`backend/cap/`下的公开契约。能力拥有方独立保存自己的记录；插件或所选提供方不可用时，调用失败关闭。

## 人机验证

默认提供方生成本地算术图片，调用方也可以让用户主动切换到可被辅助技术读取的算术题。挑战五分钟后过期，最多允许尝试五次。完成挑战后签发的证明有效两分钟；证明只允许按原调用插件、原用途和原绑定原子消费一次。

部署方可以通过环境配置选择`Cloudflare Turnstile`：

| 配置项 | 含义 |
| --- | --- |
| `LINAPRO_SHARED_CHALLENGE_PROVIDER` | 填写`local`或`turnstile` |
| `LINAPRO_SHARED_TURNSTILE_SITE_KEY` | `Turnstile`公开站点密钥 |
| `LINAPRO_SHARED_TURNSTILE_SECRET` | `Turnstile`私密校验密钥 |

`Turnstile`验证固定使用`Cloudflare`官方校验地址。缺少配置、请求超时、响应格式异常、动作不匹配或令牌被拒绝时，统一失败关闭。

## 站内信

发布方提交明确的宿主用户编号、稳定来源、幂等号、已经渲染好的标题与正文、业务类别，以及可选的当前站点内跳转路径。插件在一个事务中保存一条权威消息和对应发布证据。相同内容的并发或重复请求返回原结果；同一幂等身份改换内容时直接拒绝。

发布成功只代表消息及证据已经保存，不代表用户已经阅读。已读状态投影和业务通知策略不属于本能力。

## 验证

在仓库根目录执行插件治理检查：

```powershell
.\make.cmd plugins.check
.\make.cmd lint.go dir=apps/lina-plugins/shared-services plugins=1
```

在本插件目录执行`Go`验证：

```powershell
$env:GOWORK = 'off'
go test ./...
go vet ./...
```

设置`LINAPRO_SHARED_SERVICES_TEST_DSN`后，可以运行使用隔离`PostgreSQL`模式的持久化和并发测试。测试只创建并删除一个随机命名的模式。
