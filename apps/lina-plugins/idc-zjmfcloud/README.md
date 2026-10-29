# idc-zjmfcloud

`idc-zjmfcloud` is a managed dynamic connector for ZJMF public-cloud product configuration and server operations governed by `idc-resource`.

English | [简体中文](README.zh-CN.md)

## Delivery model

Version `v0.3.15` is delivered only as `dynamic + managed`. It has no source-plugin registration, built-in distribution mode, or compatibility path. Build the installable WASM artifact from the repository root:

```bash
go run ./hack/tools/linactl build dir=apps/lina-plugins/idc-zjmfcloud
```

The artifact declares the `product-fulfillment@v1` provider contract owned by `idc-core` so the host can render product and customer configuration and freeze the server contract on a published product version. The declared `configuration.candidates`, `provision`, and `query` methods are authorization metadata; the guest rejects all direct execution through the product-owner context. Candidate discovery, provisioning, and later server operations use the `resource-provider@v1` extension owned by `idc-resource`. The resource owner resolves the selected `pfi_*` interface to its channel, creates the persistent task, binds the generated server password, and then invokes this connector.

## Structured configuration

`endpoint`, `username`, and `password` are managed only on the resource channel page and are never copied into product automation. Server-only fields belong to this connector rather than the generic product model: region, network line, CPU, memory, operating system, system disk, data disk, and bandwidth. Region, line, and operating-system choices are loaded through the versioned `configuration.candidates` method. Optional `Relationships` retain explicit node `area_id` values in a bounded, secret-free parent/child graph. The graph uses the same nodes response, adds no per-region calls, and never infers relationships from labels. Missing identifiers omit the relationship rather than asserting an unrestricted combination. Directory transport alone does not enable merchant or storefront filtering.

Optional hostname and password generation rules are configured on the product. The pure `product.configuration.validate` method checks their combined limits during form evaluation, saving, and publication; it receives no supplier credentials or outbound context. The resource owner generates hostnames deterministically and passwords inside the existing secret consumer boundary from the frozen rules. Products without these rules retain their previous generation behavior.

Provisioning parameters accept resource-owner backup, snapshot, NAT forwarding and shared-website quotas: disabled maps to `-1`, unlimited maps to `0`, and limited quotas retain their positive count. Duplicate or contradictory quotas are rejected; legacy tasks without these facts gain no implicit parameters. This describes parameter conversion only; purchase options, supplier limits and corresponding prices must be configured before sale.

## Governed supplier boundary

Candidate discovery, server provisioning, querying, and ongoing operations accept only the `resource-provider@v1` context issued by `idc-resource`. They use the one-use `confidentialoutbound.execute` service. Candidate discovery logs in and reads only `/areas`, `/nodes`, and `/image`, then returns bounded bilingual labels and values. Provisioning fails before any supplier request when the resource owner has not bound the generated server password. Supplier URLs, credentials, password values, request headers, login tokens, and raw confidential failures never enter the guest request, response, page, log, or evidence payload. Final supplier execution remains asynchronous and resource-owner controlled.

## Frontend surfaces

The managed artifact publishes a customer entry under `customer.products` and no standalone administration surface. Server administration is provided by the resource owner’s product-management drawer. The customer entry runs in the governed sandbox, inherits host theme and language, synchronizes title and height, and never reads host storage, cookies, or access tokens. Data access and lifecycle actions remain controlled by the resource owner and host page bridge.

## Legacy protocol conformance code

`backend/internal/provider/zjmfcloud` contains the protocol projection used by the managed WASM guest and its capability conformance tests. It is not registered as a source plugin. The managed-release contract test reuses its capability declaration so the dynamic manifest and the proven server action matrix cannot drift silently.

Production write validation still requires a trusted HTTPS supplier endpoint and isolated test objects. Existing customer resources must never be used for destructive verification.

## Page bridge contract

Server administration is available in the corresponding customer product management drawer. The connector does not declare a standalone administration surface. Server details, accessories and task history use the resource owner's existing permission and ownership checks. The customer surface retains the resource owner's customer directory and action gates.

Build runs the plugin's locked dependency install, type check, and both page entry builds before assembling WASM. Installation or upgrade must be followed by enablement; live supplier operations additionally require a configured resource channel.

The interface address is the supplier admin root, for example `https://host/W5SHNaHntv/`. The connector declares `apiPathPrefix: /v1`; the resource owner adds it to login and business requests exactly once. Operators do not enter login routes or API version suffixes.

Merchant configuration supports node selection priority, traffic reset cycle, and editable disk performance profiles (`read MB/s,write MB/s,read IOPS,write IOPS`). Profile alternatives are validated in one pure provider call before saving and mapped to native v1 fields without an extra unit multiplication. Customer-supplied GiB memory explicitly replaces the MiB product parameter; the editor does not show both as separate choices. `format_data_disk` remains a reinstall-only option, disabled by default.

The isolated guest validates the complete result shape without trusting its wall clock. Observation dates are neutralized only in local validation copies; returned bytes remain unchanged, and the resource owner supplies actual receipt time. Definite supplier rejection therefore retains its failure category instead of becoming an unconfirmed result solely because of an epoch clock.

## Merchant provisioning parameters

Direct provisioning accepts explicitly named, non-secret merchant parameters frozen with the order. Their native names and scalar contents reach the connector form; unbound customer fields stay private to the order. Credentials, the upstream customer identity and paid unit quantity remain owner controlled. Supplier acceptance is determined by its actual response. Upstream resale configurations remain read only.

Managed release `v0.3.20` requires `idc-resource >=0.1.73`. Extended guest connection fields are requested only by the exact resource-detail query. Power-operation prechecks keep the ordinary identity/status whitelist, so customer detail reads and queued actions do not borrow each other's field permissions. Both paths are covered by local TLS tests through the real managed bridge and resource confidential handler; they do not perform real supplier writes.
