# 银联支付 (`idc-payment-unionpay`)

This `dynamic + managed` plugin declares payment-method configuration. `idc-payment` owns payment execution, signature verification, queries, refunds, and financial facts. Installation does not prove channel readiness or payment success.

## Development contract

Follow the [plugin development and installation guide](../README.md). The declaration uses `payment-method@v1`, `owner=idc-payment`, `role=provider`, `methods=[capabilities.describe]`, and multiple interface instances. The authoritative [public payment declaration validator](../idc-payment/backend/cap/paymentcap/dynamic_provider_contract.go) rejects missing owners, dependencies, invalid schemas, and unsupported metadata. A new provider name does not implement a payment protocol.

- Provider: `unionpay`.
- Declared modes: `unionpay_web`, `unionpay_wap`.
- Currencies: `CNY`.
- Configuration fields: `endpointOrigin`, `merchantId`, `certificateSerialNo`, `privateKey`, `publicKey`. Secrets remain inside the owner's confidential boundary and never enter public settings or logs.

## Build and operation

From the repository root run `go run ./hack/tools/linactl build dir=apps/lina-plugins/idc-payment-unionpay`. Upload, install, and enable the resulting dynamic artifact using the standard lifecycle. Dependencies are defined by `plugin.yaml`; increase the version and preview before confirming an update. Never edit installation state directly in the database.

Manage channel instances in the payment owner's configuration page. Real merchant eligibility, credentials, and callbacks must satisfy each declared mode. Use explicitly authorized test objects for real payment checks. Contract tests do not prove real money movement.
