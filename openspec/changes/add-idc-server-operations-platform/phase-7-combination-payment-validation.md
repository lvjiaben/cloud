# 第七阶段组合支付验证记录

## 完成内容

- `payment` 只提交支付准备编号、支付尝试号和客户选择的余额额度，不能提交客户、订单、账单、币种或应付金额。
- `core` 根据当前登录客户重新读取支付准备、订单和账单，计算当前应付金额，并只返回客户编号摘要。
- 余额与渠道必须同币种；余额额度必须小于本次应付金额，剩余部分只允许一个在线渠道。
- 余额先进入资金锁定，不提前产生消费流水；相同支付尝试重放复用同一锁定，修改金额的重放被拒绝。
- 渠道回调按“总额减锁定余额”核对渠道实收金额。
- 渠道确认后，余额消费、渠道资金、两条账单分配和唯一结清记录在同一事务完成。
- 支付失败或过期通过支付尝试号释放对应锁定，不产生余额消费流水。

## 验证结果

- `core` 真实 PostgreSQL 回归覆盖当前客户重新校验、锁定幂等、改变金额拒绝、组合支付原子结清和失败释放。
- `payment` 真实 PostgreSQL 回归覆盖渠道应收金额核对，并确认传给 `core` 的渠道金额、总额、余额额度和锁定编号完全一致。
- `core` 全仓编译门禁串行通过；支付插件全部 Go 测试通过。

验证命令：

```powershell
$env:GOWORK='off'
$env:LINAPRO_IDC_CORE_TEST_DSN='pgsql:postgres:123456@tcp(127.0.0.1:5432)/cloud_idc_phase10_e2e_v2?sslmode=disable'
go test ./backend/internal/service/commerce -run 'TestChannelPaymentOwner' -count=1 -v
go test ./... -run TestNonExistent -count=0 -p 1

$env:LINAPRO_IDC_PAYMENT_TEST_DSN='pgsql:postgres:123456@tcp(127.0.0.1:5432)/cloud_idc_phase10_e2e_v2?sslmode=disable'
go test ./... -count=1
```

## 尚未包含

客户支付页面、后台渠道页面、主动查单任务、退款和对账仍属于后续任务，不能由本记录提前判定完成。
