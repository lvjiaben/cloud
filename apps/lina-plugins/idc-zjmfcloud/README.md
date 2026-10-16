# idc-zjmfcloud

`idc-zjmfcloud` is a managed dynamic connector for ZJMF public-cloud product configuration and server operations governed by `idc-resource`.

English | [简体中文](README.zh-CN.md)

## Delivery model

Version `v0.2.6` is delivered only as `dynamic + managed`. It has no source-plugin registration, built-in distribution mode, or compatibility path. Build the installable WASM artifact from the repository root:

```bash
go run ./hack/tools/linactl build dir=apps/lina-plugins/idc-zjmfcloud
```

The artifact declares the `product-fulfillment@v1` provider contract owned by `idc-core` so the host can render product and customer configuration and freeze the server contract on a published product version. The declared `configuration.candidates`, `provision`, and `query` methods are authorization metadata; the guest rejects all direct execution through the product-owner context. Candidate discovery, provisioning, and later server operations use the `resource-provider@v1` extension owned by `idc-resource`. The resource owner resolves the selected `pfi_*` interface to its channel, creates the persistent task, binds the generated server password, and then invokes this connector.

## Structured configuration

`endpoint`, `username`, and `password` are managed only on the resource channel page and are never copied into product automation. Server-only fields belong to this connector rather than the generic product model: region, network line, CPU, memory, operating system, system disk, data disk, and bandwidth. Region, line, and operating-system choices are loaded through the versioned `configuration.candidates` method.

Optional hostname and password generation rules are configured on the product. The pure `product.configuration.validate` method checks their combined limits during form evaluation, saving, and publication; it receives no supplier credentials or outbound context. The resource owner generates hostnames deterministically and passwords inside the existing secret consumer boundary from the frozen rules. Products without these rules retain their previous generation behavior.

Provisioning parameters accept resource-owner backup, snapshot, NAT forwarding and shared-website quotas: disabled maps to `-1`, unlimited maps to `0`, and limited quotas retain their positive count. Duplicate or contradictory quotas are rejected; legacy tasks without these facts gain no implicit parameters. This describes parameter conversion only; purchase options, supplier limits and corresponding prices must be configured before sale.

## Governed supplier boundary

Candidate discovery, server provisioning, querying, and ongoing operations accept only the `resource-provider@v1` context issued by `idc-resource`. They use the one-use `confidentialoutbound.execute` service. Candidate discovery logs in and reads only `/areas`, `/nodes`, and `/image`, then returns bounded bilingual labels and values. Provisioning fails before any supplier request when the resource owner has not bound the generated server password. Supplier URLs, credentials, password values, request headers, login tokens, and raw confidential failures never enter the guest request, response, page, log, or evidence payload. Final supplier execution remains asynchronous and resource-owner controlled.

## Frontend surfaces

The managed artifact publishes separate entries under `admin.resource-operations` and `customer.products`. Both entries run in the governed sandbox, inherit host theme and language, synchronize title and height, and never read host storage, cookies, or access tokens. Server data access and lifecycle actions remain controlled by the resource owner and host page bridge.

## Legacy protocol conformance code

`backend/internal/provider/zjmfcloud` contains the protocol projection used by the managed WASM guest and its capability conformance tests. It is not registered as a source plugin. The managed-release contract test reuses its capability declaration so the dynamic manifest and the proven server action matrix cannot drift silently.

Production write validation still requires a trusted HTTPS supplier endpoint and isolated test objects. Existing customer resources must never be used for destructive verification.

## Page bridge contract

The admin `server-management` surface borrows the read-only `zjmfcloud-servers.admin.v1` projection from `idc-resource >= 0.1.26`: `GET admin/resource/zjmfcloud-servers` and `GET admin/resource/zjmfcloud-servers/{serverNo}`. Every request checks the current staff identity, single-merchant scope, and `idc:admin:resource:server-batch:view` permission. Queries are fixed to this connector, bounded to 100 rows per page, and return an exact scoped total. Details share the same boundary. The projection excludes channel credentials and action references. The customer surface retains the resource owner's customer directory and action gates.

Build runs the plugin's locked dependency install, type check, and both page entry builds before assembling WASM. Installation or upgrade must be followed by enablement; live supplier operations additionally require a configured resource channel.
