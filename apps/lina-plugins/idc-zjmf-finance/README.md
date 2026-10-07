# idc-zjmf-finance

`idc-zjmf-finance` is a managed dynamic server connector for the native ZJMF Finance API. It is separate from ZJMFCloud, ZJMF V10, and DCIM connectors and never uses the legacy `modules/addons/idcsmart_api` or `/v1` compatibility API.

English | [简体中文](README.zh-CN.md)

## Interface instances

Administrators can create multiple independent interface instances in `idc-resource`. Endpoint and credentials remain owned by that resource channel. Product releases freeze only the chosen interface projection and reviewed upstream product mapping.

This connector does not duplicate server menus or declare empty pages. Interface management, server lists, and customer server pages remain owned by `idc-resource`; a connector-owned surface may be declared only when a real independent page and its packaged static assets are delivered together.

Authentication always posts `username` and `password` to `/zjmf_api_login`. The confidential host proxy extracts `jwt` and injects `Authorization: Bearer` without exposing credentials or tokens to WASM, pages, logs, or fulfillment evidence.

The resource channel must use `login_session`, token path `["jwt"]`, header `Authorization`, scheme `Bearer`, refresh path `["status"]`, refresh value `["405"]`, and `serializeInvocations: true`. The resource owner validates these governed settings, retries authentication at most once, and holds one PostgreSQL-backed per-channel invocation lease across the complete guest call.

## Product configuration

Products come from `GET /cart/all`. Configuration comes only from `GET /cart/get_product_config?pid=<selected product>`. A missing upstream product is rejected, and lookups for different product IDs remain separate.

ZJMF Finance lets merchants name configuration options freely. This connector therefore never guesses that an option means region, network line, or operating system. Administrators explicitly map those upstream option IDs before the selected values are frozen into a product release.

## Fulfillment safety

The verified initial flow uses `/user_info`, cart reconciliation, `/cart/add_to_shop`, `/cart/settle`, and credit payment. The complete flow must hold a durable per-interface serialization lease. A failure before settlement may fail safely; a timeout after settlement without an order reference becomes an unknown result requiring manual reconciliation and is never submitted again automatically.

The connector implements this initial purchase flow with owner-only correlation carrying a governed HTTPS callback URL and numeric intent ID. It accepts exactly one operation secret slot, `downstream_token`; the confidential proxy injects its plaintext only into the supplier form. Every replay starts with `/cart/clear`, recovers an existing active server or invoice, and does not repeat cart or settlement side effects. Success is returned only after the upstream supplies a positive server ID. Renewal and resize remain unsupported because they create separate upstream financial transactions.

## Verification

Run from the plugin directory:

```powershell
$env:GOWORK = "off"
go test ./... -count=1
go vet ./...
go build ./...
$env:GOOS = "wasip1"
$env:GOARCH = "wasm"
go build -o idc-zjmf-finance.wasm .
```

Production write verification must use a dedicated test account and test product. Do not use existing customer orders or servers for destructive verification.
