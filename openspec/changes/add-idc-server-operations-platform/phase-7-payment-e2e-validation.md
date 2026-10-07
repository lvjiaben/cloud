# 第七阶段支付浏览器回归记录

## 覆盖范围

- 客户支付页面固定在`apps/lina-vben/apps/web-customer`，验证支付宝电脑网站、支付宝手机网站、微信`H5`和微信扫码四种模式；未向`web-antd`写入客户业务代码。
- IDC 后台异常资金页面继续由`idc-payment/frontend/`拥有，通过宿主插件机制挂载；验证迟到到账分配、易支付转人工原路退款、受宿主治理的凭证上传与结果未知冻结。
- 取消后的迟到到账只进入待归属资金。页面不复活旧订单，分配请求只提交目标新订单编号，退款结果未知时不显示可重放按钮。

## 浏览器结果

| 入口 | 用例 | 结果 |
| --- | --- | --- |
| `web-customer` | `TC011a`至`TC011e`、`TC011g`至`TC011j` | 真实 Chrome `9/9`通过 |
| `web-customer` | `TC011f`真实已安装通道回调 | 未设置`E2E_REAL_RECHARGE=1`时按门禁跳过，不冒充真实通道证据 |
| 支付插件后台 | `TC002a`迟到款精确分配、`TC002b`人工退款凭证、`TC002c`结果未知冻结 | 真实 Chromium `3/3`通过 |

客户测试确认四种模式提交各自的`modeCode`，微信扫码只展示二维码并等待服务端确认。扫码弹窗改为等待充值弹窗完全关闭后再打开，人工截图确认不再叠加旧弹窗或付款方式下拉层。

后台测试使用超出 JavaScript 安全整数范围的金额`9007199254740993.01`，页面仍按字符串和`BigInt`精确显示。人工退款先上传文件取得`host-file`引用和服务端摘要，再提交确认；测试等待确认响应完成，避免把请求刚发出误判为业务完成。

## 截图复核

- `temp/20260820/044849-customer-payment-wechat-native-pending.png`：客户扫码弹窗、二维码、等待服务端确认提示和“查看到账状态”按钮清晰，旧充值弹窗及下拉层已关闭。
- `temp/20260820/044918-payment-exception-fund-allocation.png`：待归属资金、原订单/支付尝试、超大金额、目标新订单和原路退款动作清晰。
- `temp/20260820/044611-payment-exception-fund-manual-refund.png`：人工退款凭证上传与确认动作清晰。
- `temp/20260820/044614-payment-exception-fund-result-unknown.png`：结果未知只显示核对状态，没有自动动作按钮。

## 其他门禁

- `pnpm --dir apps/lina-vben --filter @lina/web-customer typecheck`：通过。
- `pnpm --dir apps/lina-plugins/idc-payment/frontend build`：类型检查、构建和 bundle 校验通过。
- `node hack/tests/scripts/validate-e2e.mjs`：通过，已校验`225`个 E2E 文件。
- 本轮未修改`apps/lina-vben/apps/web-antd`业务源码、依赖、路由、状态或语言资源。
