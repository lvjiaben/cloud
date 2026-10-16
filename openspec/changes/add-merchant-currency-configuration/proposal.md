## Why

后台缺少货币配置，协助下单仅读取默认币种，商品币种与运营配置脱节。

## What Changes

新增插件自有货币管理与系统配置入口；统一启用币种和新报价校验，保持同币种定价、余额结算及历史快照。

## Capabilities

### New Capabilities
- `merchant-currency`：单商户货币配置及业务联动。

### Modified Capabilities

无。

## Impact

仅`idc-core`插件及项目记录，禁止修改宿主；新增权限、迁移、双语文案和测试。
