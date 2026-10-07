# LinaPro Plugin Public Contracts

`apps/lina-core/pkg/plugin` contains the stable plugin-facing contracts for `lina-core`. Source plugins, dynamic plugin guests, dynamic plugin builders, and host integration code should all use this public boundary. Host-owned orchestration and persistence implementations live under `apps/lina-core/internal/service/plugin`.

This package owns the plugin kernel, source-plugin contribution API, dynamic plugin transport, core-owned capability directory, generic capability descriptor intake, owner-aware dynamic routing, authorization, audit, cache invalidation, and lifecycle governance. Non-core domain contracts that are owned by a business plugin live in that owner plugin's `backend/cap/<domain>cap` directory.

## Public Components

| Component | Responsibility |
| --- | --- |
| `capability` | Defines the unified `capability.Services` directory and subpackage contracts for core-owned capabilities such as users, files, i18n, jobs, tenant, organization, storage, cache, lock, manifest, route, plugins, and business context. Source plugins receive this directory from `pluginhost` registrar and callback inputs; dynamic plugins access only the published bridge-backed subset. Dynamic-plugin i18n resources are host-managed and no `i18n` host service is published. |
| `pluginhost` | Defines source-plugin declaration-time contracts, runtime service access, lifecycle callbacks, hook registration, HTTP route contribution, scheduled job contribution, menu filtering, permission filtering, hosted asset constants, and generic capability descriptor intake. It must not become the owner of domain-specific provider facades for plugin-owned capabilities. |
| `pluginbridge` | Provides the dynamic-plugin guest SDK, dynamic plugin declarations, core-owned host-service clients, and owner-aware host-call envelope. Guest code uses `pluginbridge.Declarations` during discovery or build flows and `pluginbridge.Services` at runtime. Plugin-owned domain guest SDKs live under the owner plugin's `backend/cap/<domain>cap/bridge` or equivalent public package. |

## Domain Capabilities

`capability.Services` is the runtime directory for core-owned domain capabilities. Source plugins consume these entries through the `capability.Services` returned by `pluginhost` registrar and callback inputs; dynamic plugins declare the matching entries that are explicitly published as dynamic `hostServices` and call them through `pluginbridge.Services`. Each core-owned domain exposes one plugin-visible `Service`; method-level contracts carry risk, authorization, data-permission, context, performance, and cache governance. Domain methods rely on the standard business `ctx` for current user, tenant, permission, and data-scope information; dynamic `hostServices` authorization stays inside the dispatcher. `I18n()` remains a source-plugin runtime capability, while dynamic-plugin i18n resources are discovered, merged, cached, and served by the host. Plugin lifecycle and plugin state belong to `Plugins()`; tenant plugin governance and tenant-filter context belong to `Tenant()`. Host-internal scope helpers are not exposed through ordinary plugin-facing access.

Plugin-owned non-core capabilities are not owned by `capability.Services`. The owner plugin publishes its ordinary consumer contract, provider `SPI`, dynamic guest SDK, descriptor helper, and version policy under `apps/lina-plugins/<plugin-id>/backend/cap/<domain>cap`. `lina-core` receives the owner descriptor, indexes `owner + service + version + method`, validates dependencies and authorization snapshots, dispatches owner-aware dynamic calls, records audit envelopes, and invalidates runtime caches after lifecycle changes. `AI` is the first plugin-owned capability family and is owned by `apps/lina-plugins/ai-core/backend/cap/aicap`.

### Unified Services and Dynamic Plugins

Plugin-visible domains no longer provide a separate management directory. Source plugins use `Services.<Domain>()` on the `capability.Services` directory returned by `pluginhost` inputs, or a narrowed injected `*cap.Service` for reads, writes, execution, and governance actions. Plugin lifecycle and enablement state are accessed through `Services.Plugins().Lifecycle()` and `Services.Plugins().State()`; tenant plugin governance and tenant filter context are accessed through `Services.Tenant().Plugins()` and `Services.Tenant().Filter()`. `capability.Services` does not expose a separate tenant service view or top-level `PluginLifecycle()`, `PluginState()`, `TenantPluginGovernance()`, `TenantFilter()`, or `TenantTableFilter()` methods. Same-process source plugins and host adapters that need to constrain GoFrame queries use `tenantspi.ApplyPluginTableFilter(ctx, Services.Tenant().Filter(), model, qualifier)` or derive the filter from an injected `tenantcap.Service.Filter()`; ordinary `tenantcap.Service` and dynamic plugins expose only serializable tenant-filter context. Dynamic plugins may call governed plugin and tenant methods only when those methods are registered, declared, authorized, and dispatched as ordinary domain `hostServices`.

Dynamic plugins may use only the concrete methods that are registered in the dynamic `host service` catalog, declared by the plugin manifest, authorized by the host, and handled by the `WASM host-service` dispatcher. If a governed action must become available to dynamic plugins, publish a narrow, versioned dynamic method in the unified domain instead of introducing a parallel management service.

| Domain capability | Responsibility boundary | Runtime and validation path |
| --- | --- | --- |
| `APIDoc` | Resolves route text and title operation keys in localized API documentation. | Served through the capability directory or `apidoc` host calls; validation focuses on route and operation-key payloads. |
| `Auth` | Handles tenant token selection/switch, impersonation, authz, and external-identity login via `Token()`, `Authz()`, and `ExternalLogin()`. **Installed dynamic and source plugins share the same trust and rights model**: the host stamps caller plugin ID; source ownership uses `ProvideExternalIdentity`; dynamic ownership uses `auth` hostService `resources.ref` (provider ID); wire method `external_login.login_by_verified_identity`. Token/session minting stays host-owned. SPA handoff is closed over `extlogin-core`. | Runtime checks enablement, method authorization, and provider ownership. External-identity engine is `extlogin-core`; without it, external login fails closed. |
| `AI` | Plugin-owned by `ai-core`; aggregates typed AI sub-capabilities for text, image, embedding, audio, vision, document, safety, and video, including method-level status projections. | Source plugins import `ai-core/backend/cap/aicap`; dynamic plugins declare `owner: ai-core`, `service: ai`, and `version: v1`. `lina-core` governs descriptor discovery, dependency checks, authorization snapshots, owner-aware routing, audit, and cache invalidation. |
| `Users` | Provides base user reads, bounded listing, visibility, governed writes, credentials, role assignment, and `CreateFromExternal` (least-privilege mint from external identity). Dynamic `users` publishes `users.create_from_external` among other authorized methods. | Host keeps visibility boundaries. `CreateFromExternal` is operator-less and least-privilege; dynamic callers must declare and be authorized for the method. |
| `BizCtx` | Projects the current business request context. | Used as a read-only runtime context bridge for request user, tenant, locale, and request metadata. |
| `Dict` | Resolves dictionary value labels, lists bounded value candidates, and validates typed value visibility. | Host validation stays within visible dictionary types and values. |
| `Files` | Provides host file-center projections, bounded search, visibility enforcement, content reads, governed uploads, and creation from plugin storage into `sys_file`. | Host validation prevents plugins from probing or using file IDs outside their visible boundary. Uploads create host file-center metadata while plugin-private objects remain owned by `Storage()`. |
| `HostConfig` | Reads host configuration values through the host priority chain and exposes governed `SysConfig()` projections and writes for `sys_config` rows. `SetValue(ctx, key, value, options)` writes one key; `BatchSetValue(ctx, items, options)` writes many keys in one transaction and one runtime-config revision (prefer batch for multi-field settings saves). `options.SystemManageable` nil on insert defaults to false / plugin closed-loop. | Dynamic declarations must list `resources.keys` for `get` and single-key `sys_config` methods. This capability is separate from plugin-scoped business config. Plugin-owned settings MUST pass `options.SystemManageable: gconv.PtrBool(false)`. |
| `I18n` | Reads locale and translates messages for source plugins. | Source plugins receive this through `capability.Services` from `pluginhost` inputs; dynamic plugins do not receive an `i18n` host service because their i18n resources are host-managed. |
| `Jobs` | Reads scheduled-job metadata, searches bounded job candidates, validates job visibility, and performs governed runtime job actions. | Declaration-time job contracts are submitted through `pluginbridge.Declarations.Jobs().Register(...)`; runtime services do not expose `Register`. |
| `Notifications` | Lists and reads typed notification message projections, batch-loads messages by business source, validates visibility, deletes messages, updates read state, and sends governed notifications. | Actor-scoped read, delete, and read-state calls do not require resource declarations; `messages.send` requires a `resources[].ref` boundary. |
| `Plugins` | Exposes current plugin projection, plugin registry projections, tenant plugin pages, plugin enablement state, provider enablement state, plugin-scoped config, and governed plugin lifecycle orchestration. | Runtime checks cover plugin visibility, plugin enablement/provider state, plugin-scoped config source isolation, lifecycle preconditions, and dynamic `hostServices` authorization for published governance methods. |
| `Route` | Exposes dynamic-route metadata for the current execution. | Used by runtime route dispatch without exposing host router internals. |
| `Sessions` | Reads the current online-session projection, searches sessions, batch-loads session projections, validates session visibility, and batch-reads user online status. | Host validation keeps session and user visibility scoped to the caller. |
| `Storage` | Provides plugin-private object storage operations for plugin-owned attachments, binary objects, import/export temporaries, and uninstall cleanup, including explicit batch stat, cursor list, and batch delete. | Source plugins receive plugin-scoped `Storage()` through `capability.Services` from `pluginhost` inputs; dynamic declarations use `service: storage` with `resources.paths`; writes do not create `sys_file` rows or expose provider keys or local paths. |
| `Cache` | Provides plugin-scoped cache get, set, delete, multi-key get/set/delete, increment, and expiry operations. | Dynamic declarations use `resources[].ref`; runtime dispatch validates namespace, key, value size, and positive TTL payloads. Cache remains non-authoritative plugin runtime data. |
| `Lock` | Provides plugin-visible distributed lock acquire, renew, and release operations. | Dynamic declarations use `resources[].ref`; runtime dispatch validates lock resource and lease payloads. |
| `Manifest` | Reads plugin-scoped manifest or artifact resources, including bounded multi-get and path listing. | Dynamic declarations use `resources.paths`; source and dynamic paths are resolved through plugin-scoped resource views. |
| `Org` | Provides optional organization projections such as user organization profiles, bounded department trees, department search, post options, visibility checks, department assignments, department names, and post IDs. | Provider availability is explicit; fallback services return safe neutral values when the organization provider is absent. |
| `Tenant` | Provides optional tenant context, tenant info, tenant batch/search projections, visibility checks, membership validation, accessible tenant lists, tenant plugin governance, and plugin-owned table tenant-filter context. | Provider availability is explicit; lifecycle writes and membership replacement stay in the tenant owner or host-internal SPI. Same-process Go callers can pass the ordinary filter context to `tenantspi.ApplyPluginTableFilter(...)` when they own a GoFrame model. Dynamic plugins receive only serializable filter context and must use `RecordStore` or owner-side filters for tenant isolation. |

### Plugin Distribution

Plugin manifests and lifecycle callback snapshots include `distribution`, which is normalized by the host to `managed` or `builtin`. Omitted values are treated as `managed`. `builtin` is a source-plugin-only governance mode for project components compiled with the host: the host installs, enables, and safely upgrades them during startup, ordinary plugin-management write actions are rejected, and the plugin-management list excludes them before filtering, pagination, and total calculation. Read-only detail lookup remains available to trusted host integrations that must inspect required capabilities. Dynamic plugins must remain `managed` and cannot self-declare built-in governance.

### Business Extensions and Frontend Surfaces

Plugin manifests keep four independent dimensions. `type` selects the runtime (`source` or `dynamic`), `distribution` selects lifecycle governance (`builtin` or `managed`), `business_extensions` declares business placement, and frontend surfaces declare presentation. That fourth dimension is split into separately governed `admin_surfaces` and `customer_surfaces`. The host never infers any dimension from a plugin name.

Built-in business owners publish versioned `business_extensions` with `role: owner`. Managed providers use `role: provider`, name the owner plugin explicitly, and declare a versioned `dependencies.plugins` edge to that owner. Batch discovery first indexes all owner publications, then accepts only providers whose owner, point, version, and requested methods match. This is one in-memory projection over an already loaded manifest batch, not one query per plugin. The initial published point identifiers are `product-fulfillment`, `sms-channel`, `payment-method`, and `website-template`; future owner plugins may publish additional safe kebab-case points.

Provider extensions may attach independently versioned `interface_configuration`, `product_configuration`, and `customer_configuration` schemas. A provider with `interface_configuration` must also declare `interface_configuration_cardinality` as `multiple` for many named interface instances or `singleton` for one global settings record. Cardinality governs settings records only; it never limits plugin-owned business rows or CRUD features. The first protocol version supports exactly `single_line`, `multi_line`, `integer`, `decimal`, `boolean`, `single_select`, `multi_select`, `date`, `datetime`, `secret`, `tree_select`, `address`, and `file`. The only text formats are the host-owned `phone`, `email`, `domain`, and `url` validators. A `secret` field is restricted to interface configuration; product and customer schemas are available only to `product-fulfillment` providers. File fields name a safe host-governed `upload_policy` and cannot provide a URL.

Choice fields use either a bounded bilingual `static_candidates` list or one versioned `dynamic_candidates` method already present in the extension's authorized `methods`, never both. Conditions are bounded `and`/`or` trees over `equal`, `not_equal`, `contains`, numeric comparisons, `empty`, and `not_empty`. Results are restricted to `show`, `hide`, `require`, `disable`, `filter_candidates`, `requires`, and `excludes`. Unknown nested keys are rejected in both source YAML and dynamic artifact JSON, so scripts, expressions, arbitrary regular expressions, upload endpoints, and undeclared dynamic methods cannot be hidden in a manifest.

```yaml
type: dynamic
distribution: managed
dependencies:
  plugins:
    - id: commerce-core
      version: ">=0.1.0 <1.0.0"
business_extensions:
  - point: product-fulfillment
    version: v1
    role: provider
    owner: commerce-core
    methods: [fulfill, query, colors.list]
    interface_configuration_cardinality: multiple
    interface_configuration:
      version: v1
      fields:
        - key: api_secret
          type: secret
          label: { en-US: API secret, zh-CN: 接口密钥 }
          required: true
    product_configuration:
      version: v1
      fields:
        - key: model
          type: single_line
          label: { en-US: Model, zh-CN: 型号 }
    customer_configuration:
      version: v1
      fields:
        - key: color
          type: single_select
          label: { en-US: Color, zh-CN: 颜色 }
          dynamic_candidates:
            version: v1
            method: colors.list
customer_surfaces:
  - key: my-products
    version: v1
    title: My products
    slot: customer.products
    route: my-products
    entry: customer/index.html
    bridge_capabilities: [products.query-own]
public_assets:
  - source: frontend
    mount: customer
```

Release snapshots freeze requested and host-accepted business extension declarations, including all three configuration schemas, together with the separate admin/customer surfaces. Dynamic WASM artifacts must embed the same complete declarations as `plugin.yaml`; differing runtime distribution, business extensions, nested configuration, or surfaces are rejected. Discovery evidence returns detached safe schemas and calculates its declaration digest over the complete normalized declaration, so a field, candidate, condition, action, or schema-version change produces different frozen evidence.

An extension owner may define one bounded, non-executable `provider_metadata` object without moving its business schema into the host. The host understands only `type`, `version`, and JSON-compatible `values`; it normalizes, clones, freezes, authorizes, and includes the complete object in the declaration digest. The source owner registers the typed validator for the exact extension point and contract version during declaration setup:

```go
if err := declarations.BusinessExtensions().RegisterProviderValidator(
    "example-extension",
    "v1",
    validateExampleProviderDeclarations,
); err != nil {
    return err
}
```

The validator receives an isolated clone of the provider release's complete normalized declaration group, so it can enforce required metadata, typed fields, method authorization, and release-wide uniqueness without mutating the host snapshot. Source and dynamic providers use the same catalog path. Metadata with a missing owner registration, an unsupported contract version, duplicate owner registration, or a validator rejection fails installation and upgrade closed. The host never guesses the metadata's business meaning.

Business owners discover new-binding candidates through the governed `plugins.business_extensions.list` projection. It performs one bounded authoritative registry/active-release read, returns only the provider identity, display name, release/contract versions, allowed methods, and release/declaration digests, and fails closed when any selected snapshot is uncertain. An empty method set is valid for selecting and binding the provider; method authorization is required only when an owner later invokes a concrete method. `plugins.business_extensions.exact.get` verifies one frozen declaration for a retained historical release, while `plugins.business_extensions.exact.batch_get` verifies a bounded input-ordered collection with one authoritative release query; neither implies current runtime availability. A source business owner must still commit a new binding through `HTTPRegistrar.BusinessExtensions().WithNewBusinessAdmission`: the host rechecks the active release while holding the provider lifecycle lock, keeps that renewable lock across the owner's transaction callback, and serializes stop-new-business, disable, uninstall, and active-release upgrades against the callback. Background owners receive the same owner-bound, fail-closed facade through `JobsRegistrar.BusinessExtensions()`; the jobs registrar never falls back to an unscoped service when the binding is unavailable. Losing lease renewal cancels the callback context and fails admission closed so a context-aware database transaction rolls back. Lock names and registry tables are never exposed to the owner.

Source owners invoke a concrete provider method only through the owner-bound `BusinessExtensionRegistrar.InvokeProviderMethod` facade. Every call supplies complete frozen release/declaration evidence, one authorized method, a declared `new-business` or `existing-business` purpose, and a versioned bounded JSON object. Calls that need host-resolved interface state may additionally carry one bounded, high-entropy `runtimeContext.contextId`; source and dynamic providers receive the same opaque ID, while interface instances, revisions, configuration values, credentials, secrets, and tokens remain behind an owner-aware host service whose caller plugin ID is stamped by the host. Calls that do not need interface state omit `runtimeContext`. One provider invocation may use at most eight confidential outbound steps under one deadline and total-byte budget. Steps are strictly sequenced; replay, concurrency, cross-invocation use, expiry, and budget overruns fail before the supplier handler starts, and the owner revokes the context immediately when the provider invocation ends. The host rechecks the active release and lifecycle state while holding the same provider lock: stopped providers reject new-business calls but continue exact existing-business operations; disabled, missing, mismatched, or unauthorized providers fail closed. Source providers explicitly register one method dispatcher through `Providers().ProvideBusinessExtensionMethods`. Dynamic providers receive the same envelope through the existing isolated WASM guest dispatcher using the fixed `BusinessExtensionBridgeRequest` DTO; no public HTTP self-call, raw token, identity snapshot, secret handle, or private manifest state is provided. Requests and responses have strict byte, object, array, depth, string, complexity, and five-second execution bounds.

The administration host currently publishes `admin.catalog-products`, `admin.resource-operations`, `admin.orders-finance`, `admin.notification-marketing`, `admin.compliance-governance`, `admin.support-content`, and `admin.extension-management`. The signed-in customer host publishes `customer.products`, `customer.finance`, `customer.account`, `customer.security`, `customer.compliance`, and `customer.support`. Administration surfaces must declare a non-empty host permission. The host always generates `/plugins/{plugin-id}/...` administration routes and `/account/plugins/{plugin-id}/...` customer routes, so a plugin cannot replace `/system`, `/auth`, or another host route. Dynamic surface entries must be plugin-relative and covered by `public_assets`; remote URLs and traversal are rejected. Administration surfaces are materialized into the existing permission-filtered menu tree, while customer surfaces are returned only after authentication and mounted below an existing customer business group. These gates do not yet grant page-bridge methods: bridge authorization, identity/data-scope enforcement, and isolated page loading remain separate runtime checks.

The public `GET /api/v1/plugins/dynamic` discovery endpoint uses optional authentication: requests without `Authorization` receive runtime states without customer surfaces; supplied access tokens go through the same online-session validation as protected APIs. Valid customer and staff-assisted customer sessions preserve their target user, tenant, and acting-staff context. Invalid supplied identities are rejected, not promoted or silently trusted. Dynamic surface declarations and their `public_assets` coverage are projected together from the same immutable effective release; a staged package cannot fill missing or invalid effective declarations. Discovery does not grant any page-bridge capability or business-data access.

### Isolated Page Bridge

Managed dynamic pages run in an isolated iframe without `allow-same-origin` and never receive an access token, host state, or caller-supplied headers. Pages use version `1.0` of the `linapro.plugin-page-bridge` message protocol; every message must match the current iframe, plugin ID, surface key, and protocol version exactly. `host.view.v1` is a built-in baseline capability for every governed surface. It only supports `ready`, height, unified error, title, and plugin-local route synchronization, is not listed in `bridge_capabilities`, and cannot access business data.

Business calls require three matching bindings. The surface lists a capability in `bridge_capabilities`; one exact active dynamic route or host-captured source-owner DTO route binds the same capability through `pageBridgeCapability` and `pageBridgeHost`; and the host backend issues a two-minute tamper-proof ticket for the current login session, user, tenant, provider release, server-resolved owner release, administration/customer host, surface, surface version, capability, HTTP method, and actual path. The page request cannot choose `ownerPluginId`; it is returned only after the host finds one unambiguous route. Before issuance, an administration surface rechecks both the surface permission and the exact owner-route permission. Customer surfaces reject a super administrator entering directly, tenant impersonation and another merchant context. Dynamic self-owned customer routes retain their self-only scope requirement. Source-owned customer routes must use `RouteGroup.BindCustomerPageBridge`: its authoritative owner callback receives the original authenticated context and must return the same host user ID as the requesting customer. Missing proof, rejection or a different identity fails closed. This admits a real customer without backoffice roles, including a valid staff-assisted customer session, without granting employee privileges. The result is never cached across requests. The owner route still performs ordinary staff permission/data-scope checks or current-customer object ownership checks afterwards, including revocation since issuance. The browser carries only the opaque ticket. A missing layer, disabled release, expired or modified ticket, mismatch, changed method or path, identity change, or cross-surface request is rejected. The plugin page cannot obtain ticket contents, provide arbitrary headers, select another owner namespace, or use a query capability to reach an unbound mutation route.

Each page may have at most eight bridge requests in flight. Duplicate in-flight `requestId` values are rejected and each request is cancelled after 30 seconds. Switching plugin or surface, retrying, or unmounting cancels prior requests and drops late responses from the old page. The host removes its skeleton only after the plugin sends `ready`, preventing a loaded HTML shell with slow scripts or subresources from flashing blank content.

## Dynamic-Plugin-Only Capabilities

`Runtime`, `Network`, and `RecordStore` are dynamic-plugin-only entries on the `pluginbridge.Services` directory returned by `pluginbridge.Default()` or `pluginbridge.New()`. They are not part of `capability.Services` because source plugins either already run inside the host process with native equivalents or use source-plugin data access seams instead of guest host-service wrappers.

| Capability | Public entry | Boundary reason |
| --- | --- | --- |
| `Runtime` | `pluginbridge.Default().Runtime()` or `pluginbridge.New().Runtime()` | Dynamic plugins need a WASI host-service client for logs, state, time, UUIDs, and node identity; source plugins use host-native logging and runtime context directly. |
| `Network` | `pluginbridge.Default().Network()` or `pluginbridge.New().Network()` | Dynamic plugins need governed outbound HTTP through host-service authorization; source plugins use host-native HTTP clients or injected domain services. |
| `RecordStore` | `pluginbridge.Default().RecordStore()` or `pluginbridge.New().RecordStore()`, plus `pkg/plugin/pluginbridge/recordstore` | Dynamic plugins need a guest-side facade over the data host-service protocol and typed query plans; source plugins use their own DAO or provider seams. |

New capabilities should enter `capability.Services` only when source plugins and dynamic plugins share the same stable core-owned domain contract. Dynamic-only host-service clients and guest SDKs stay under `pluginbridge`. Plugin-owned non-core capabilities must publish their public contract and guest SDK from the owner plugin's `backend/cap/<domain>cap` boundary instead of adding domain packages, codecs, or dispatch branches in `lina-core`.

### `Storage()` and `Files()` Boundary

| Scenario | Use | Boundary |
| --- | --- | --- |
| A plugin stores its own attachment, generated export, binary object, or temporary import file. | `Storage()` / dynamic `service: storage` | The plugin passes a logical object path. The host scopes it by plugin ID and tenant before delegating to the active storage provider. The object stays outside host file-center lists and does not create `sys_file` metadata. |
| A plugin deletes, lists, stats, or purges objects it owns during record deletion or uninstall cleanup. | `Storage()` / dynamic `service: storage` | Cleanup uses `Delete` or bounded `List` by logical prefix. Plugins must not delete host upload roots, provider roots, or file-center rows directly. |
| A plugin references files that users already uploaded into the host file center. | `Files()` / dynamic `service: files` | The plugin receives `filecap.FileInfo` values and visibility checks for host-owned file IDs. The response must not expose DAO, DO, Entity, provider object keys, or local absolute paths. |
| A plugin command accepts host file IDs from a request. | `Files().EnsureVisible` / `files.visible.ensure` | The command checks all IDs before mutation. Missing and invisible files share the same rejection semantics to avoid existence probing. |
| A plugin needs to upload content and register it in the host file center. | `Files().Upload` / `files.upload` | The host writes through the file owner so `sys_file` receives tenant, uploader, scene, hash, and storage metadata. Dynamic direct upload is bounded; larger dynamic payloads should use `Storage().Put` first. |
| A plugin has already written an object to its private storage and needs a host file-center record. | `Files().CreateFromStorage` / `files.create_from_storage` | The host copies from the plugin-scoped `Storage()` object into file-center storage. Dynamic plugins must also declare `storage.get` for the source path. The operation does not move or delete the source object and does not expose provider keys or local paths. |
| A browser/client needs to transfer cloud object bytes without host proxying. | File-center `direct-upload/*` and `direct-download` HTTP APIs; plugin `Storage().CreateDirectPut` / `ConfirmDirectPut` / `CreateDirectGet` | The host issues a neutral `DirectAccess` payload (`presigned_url` / `form_post` / `temporary_credentials` / `proxy`). Permanent credentials are never returned. Object keys are host-assigned with tenant/plugin scope. Local or unsupported backends return `mode=proxy` and keep host-mediated transfer. File-center metadata is written only after successful `complete`. |
| Large object upload needs cloud multipart or host-chunked transfer. | File-center auto strategy on `direct-upload/init` plus `direct-upload/part-url` / `upload/chunked/*`; optional `storagecap.MultipartUploadProvider` on active cloud providers | Init returns neutral `strategy.channel` (`direct`/`proxy`) and `strategy.encoding` (`single`/`multipart`) from size thresholds and capability probes. Direct multipart uses part-level short-lived access; proxy chunked assembles on the host or uploads parts through the provider. Source plugins may call Service multipart methods when the provider supports them; dynamic guests keep `Put` (chunked transport) without a public Multipart API in phase one. |

`Storage()` provider selection is configuration-free. The host uses the only
enabled storage provider plugin when exactly one is serviceable, falls back to
the built-in local provider when none is serviceable, and rejects storage calls
when multiple provider plugins are serviceable. File-center object content
(upload/download/delete) uses the same provider selection rules; list and search
remain on `sys_file`. Client direct access follows the same selection rules and
never exposes long-lived access keys to the frontend. Official cloud backends
(`storage-cos`, `storage-oss`, `storage-aws`, `storage-s3`, and related plugins) register via
`storagecap.Provide` and expose credentials under the host stable **System Settings**
menu (`menu_key=setting`).

## Plugin Configuration Sources

Source plugins use `Services.Plugins().Config()` and dynamic plugins use
`plugins.config.get` to read the current plugin's business configuration. These
entries are plugin-scoped and do not expose arbitrary host configuration keys or
sibling plugin configuration.

Configuration source priority is section-level:

| Priority | Source | Runtime behavior |
| --- | --- | --- |
| 1 | Host main static config section `plugin.<plugin-id>` | The whole section becomes the effective plugin config source. Missing subkeys return absent or caller defaults and do not fall back to file sources. |
| 2 | Production file `plugins/<plugin-id>/config.yaml` under the host config root | Used only when `plugin.<plugin-id>` is absent. |
| 3 | Development file `apps/lina-plugins/<plugin-id>/manifest/config/config.yaml` | Used only when host static and production file sources are absent. |
| 4 | Dynamic artifact default `manifest/config/config.yaml` | Used as the final fallback for the active dynamic plugin execution context. |

`manifest/config/config.example.yaml` is a template only and is never loaded as
runtime defaults. `HostConfig()` remains the separate host configuration
capability; non-root host keys read the current `sys_config` snapshot, active
static host config, host defaults, then absent `nil`. Dynamic `hostconfig.get`
calls still require `resources.keys` authorization in `hostServices`.
Source plugins call `HostConfig().Get(ctx, key, defaultValue)` with an explicit
default value; pass `nil` to preserve the absent-key `nil` result after the host
priority chain.

## Consumer Contracts, Provider SPI, and Guest SDK

Plugin-facing packages use three separate boundaries so each caller imports only
the contract it can safely depend on:

| Boundary | Package shape | Intended callers | Must not contain |
| --- | --- | --- | --- |
| Core-owned ordinary consumer contract | `pkg/plugin/capability/<domain>cap` | Source plugins through `capability.Services` from `pluginhost` inputs, dynamic plugins through generated or bridge-backed clients, and host adapters | GoFrame HTTP request objects, provider factory registration, host-private implementation state, or GoFrame database builders |
| Core-owned source-plugin provider SPI | `pkg/plugin/capability/<domain>cap/<domain>spi` | Source plugins that implement a host domain provider, plus host capability assembly code | Dynamic-plugin guest SDK imports or WASM host-service wire contracts |
| Core-owned dynamic-plugin guest SDK | `pkg/plugin/pluginbridge` and its dynamic-only subpackages | WASM guest code and dynamic plugin builders | Provider SPI imports or source-plugin registration APIs |
| Plugin-owned ordinary consumer contract | `apps/lina-plugins/<plugin-id>/backend/cap/<domain>cap` | Source plugins that depend on the owner plugin, owner adapters, and host modules that receive an explicitly injected owner contract | Owner plugin `backend/internal`, DAO, DO, Entity, controller, provider secret, private cache, or host dispatcher |
| Plugin-owned provider SPI and guest SDK | `apps/lina-plugins/<plugin-id>/backend/cap/<domain>cap/spi` and `apps/lina-plugins/<plugin-id>/backend/cap/<domain>cap/bridge` | Owner plugin provider registration helpers, source provider adapters, and dynamic guests calling owner-aware host services | Core `pluginhost` domain-specific facades, core `pluginbridge` domain-specific codecs, owner internal service imports, or authorization bypasses |

Core-owned provider factory declarations belong to `pluginhost.Declarations.Providers()`.
Source provider plugins declare those factories there with domain-specific
methods such as `ProvideTenant` and `ProvideOrg`. Host startup owns the provider
manager instances and injects the shared managers into host capability services;
ordinary `capability` packages do not keep package-level provider registries.
Plugin-owned providers use owner helpers to wrap typed factories as generic
capability descriptors. `pluginhost` receives those descriptors and must not add
new `Provide<Domain>` facades, such as `ProvideAIText`, for non-core domains.

## Host Domain Implementation

`apps/lina-core/internal/service/plugin` is the host-side plugin domain component. The root package exposes a unified facade covering plugin discovery, management lists, install, enable, disable, uninstall, runtime upgrades, source upgrades, runtime route dispatch, frontend asset serving, dependency checks, and capability wiring.

Hard `dependencies.plugins` checks use two lifecycle axes:

- **Install axis** (install / uninstall / upgrade version contracts): forward checks require dependencies to be installed and version-compatible; reverse checks protect all **installed** downstream dependents.
- **Runtime axis** (enable / disable): forward enable requires dependencies to be installed, **enabled**, and version-compatible; reverse disable is blocked only by **enabled** downstream dependents. Installed-but-disabled dependents do not block disable, but still block uninstall.

The host never auto-installs, auto-enables, auto-disables, or auto-uninstalls dependency chains.

## Declaration-Time and Runtime Capabilities

### Declaration-Time Capabilities

Declaration-time capabilities are the plugin's static declarations and registration output. The host uses them before business execution to build governance state.

Source plugins express declaration-time contracts through `pluginhost.Declarations`, including `Assets()`, `Lifecycle()`, `Hooks()`, `HTTP()`, `Jobs()`, and `Access()`.

### Route registration: bind controller objects with `group.Bind`

When registering source-plugin or host HTTP routes, `group.Bind` **defaults to controller objects** (for example `group.Bind(controller.NewV1(svc))` or `group.Bind(ctrl)`). GoFrame discovers route methods with metadata on the bound object.

Within one middleware group, avoid listing individual methods unless necessary.

When methods on the same logical controller must sit in **different middleware groups** (for example public login vs authenticated management APIs), the host keeps the original split-registration design: bind the public methods on the public group and the protected methods on the protected group, instead of adding Public/Protected controller wrapper types.

### Lifecycle preconditions (target vs global)

`Lifecycle()` supports two precondition styles:

| Kind | Registration | Input | Semantics |
|------|--------------|-------|-----------|
| Target | `RegisterBeforeInstallHandler` / `RegisterBeforeEnableHandler`, etc. | `SourcePluginLifecycleInput` | Invoked only when **this** plugin is installed/enabled/disabled/uninstalled; may veto self |
| Global | `RegisterGlobalBeforeInstallHandler` / `RegisterGlobalBeforeEnableHandler` / `RegisterGlobalBeforeDisableHandler` / `RegisterGlobalBeforeUninstallHandler` | `SourcePluginGlobalLifecycleInput` (includes `TargetPluginID`) | Invoked when **another** plugin is installed/enabled/disabled/uninstalled; owners implement cross-plugin governance (for example singleton kind slots) |

Orchestration:

- Install, enable, disable, and uninstall aggregate **target Before\*** plus explicitly registered **GlobalBefore\*** before state writes.
- Enable exposes `BeforeEnable` / `AfterEnable`; `AfterEnable` is best-effort and does not roll back a successful enable.
- Global participant lists include only plugins that registered that global hook (no empty fan-out).
- The host never hard-codes domain rules (such as mail transport kinds); owners implement conflict logic inside global hooks.
- Do not simulate global intercept by broadcasting self-scoped `BeforeInstall` to every plugin.

Dynamic plugins express declaration-time contracts through `plugin.yaml`, WASM custom sections, `pluginbridge.Declarations.Routes().Group(...)`, `pluginbridge.Declarations.Jobs().Register(...)`, and embedded `protocol` contracts, such as routes, jobs, lifecycle handlers, backend resources, frontend assets, SQL, i18n resources, and `hostServices`.

### Stop New Business, Disable, and Uninstall

These are separate lifecycle stages:

- `accepting` allows the plugin to be selected for new interfaces, product versions, purchases, and fulfillment tasks. `stopped` blocks only those new-business paths; the plugin remains enabled and existing customer menus and service operations remain available.
- Disable and uninstall require `stopped`, followed by trusted aggregate checks from stable source business owners registered with `RegisterBusinessImpactHandler`. The bounded categories cover selling product versions, in-flight tasks, active services, and pending historical actions.
- Required owners come only from authorized provider declarations frozen in the currently effective release. A staged artifact cannot weaken that set, and a missing required owner handler fails closed.
- Owners return aggregate counts only. Provider errors, unsupported categories, duplicate categories, or invalid counts fail closed and block the action.
- Stop/resume admission, disable, and uninstall share a per-plugin cross-instance lifecycle lock so admission cannot reopen between the drain check and the final state write.
- `force` may bypass only plugin-owned lifecycle callback vetoes when host policy permits it. It cannot bypass stop-new-business, trusted business impact, dependency, or historical-evidence gates.
- Uninstall preserves data by default. Plugins declaring business extensions cannot purge historical products, orders, configuration snapshots, interface references, fulfillment evidence, or audit records; missing dynamic artifacts do not weaken this rule.

Stable built-in business owners register the impact projection through:

```go
plugin.Lifecycle().RegisterBusinessImpactHandler(handler)
```

Every new product binding, purchase, and fulfillment write path must use the host's authoritative `CanAcceptNewBusiness` gate. Hiding a UI candidate is not a security control.

### Runtime Capabilities

Runtime capabilities are the services available while plugin business logic is executing.

Source plugins access runtime capabilities through the `capability.Services` directory returned by `pluginhost` inputs. Plugin lifecycle and state are exposed through `Services.Plugins().Lifecycle()` and `Services.Plugins().State()`. Tenant plugin governance and tenant filtering context are exposed through `Services.Tenant().Plugins()` and `Services.Tenant().Filter()`. Same-process table filtering is an explicit `tenantspi` helper call, not a separate source-plugin service-directory method or tenant-service mirror.

Dynamic plugins access published runtime capabilities through `pluginbridge.Services`. Calls are encoded through `pluginbridge/protocol`, transported through `WASI host call`, authorized by derived `HostCapabilities` and confirmed `HostServices`, then dispatched by `apps/lina-core/internal/service/plugin/internal/wasm`. Dynamic plugins do not receive the top-level compatibility shortcuts or `I18n()`, but they can call the registered governance methods under `Plugins()` and `Tenant()` when authorized by `hostServices`.

For each guest execution, the host builds a request-local `HostServices` authorization snapshot and every host call still checks `service`, `method`, and resource identity against that snapshot. Owner-aware plugin-owned calls additionally check `owner`, `version`, the calling plugin's `dependencies.plugins` declaration, owner plugin enablement, descriptor registration, and method authorization before dispatching to the owner handler.

Supplier operations that require credentials must not use the ordinary `network.request` bridge. A source business owner issues one short-lived runtime context for the exact provider plugin, release, declaration, interface instance, business-extension method, and purpose. The dynamic provider receives only that opaque context ID and may pass it only to an owner-aware host service explicitly declared by its manifest. The host accepts a runtime-context resource only when the current business-extension invocation carries the same context ID, owner, and provider plugin; a different context, owner, plugin, method, ordinary route execution, or static resource declaration fails before the owner handler runs. The owner service resolves endpoint and credential state inside the trusted host process, applies its own bounded operation contract, and returns only a safe projection. For example, `idc-core/supplier-interface-runtime@v1` can execute a bounded same-origin login session without returning the password or access token to the guest. Legacy `WithConfidentialOutboundContext` bindings remain an owner-specific implementation for resource operations and retain their one-use, expiry, replay, and field-whitelist protections. Runtime contexts are revoked when the surrounding invocation ends and are never persisted or cached.

## Dynamic Plugin Host Service Declarations

Minimal shape:

```yaml
hostServices:
  - service: runtime
    methods:
      - log.write
```

Resource-scoped shape:

```yaml
hostServices:
  - service: storage
    methods: [get, put, put.init, put.chunk, put.commit, put.abort]
    resources:
      paths:
        - reports/
  - service: data
    methods: [list, get]
    resources:
      tables:
        - plugin_acme_demo_report
  - service: hostconfig
    methods: [get]
    resources:
      keys:
        - i18n.default
  - service: network
    methods: [request]
    resources:
      - url: https://*.example.com/api
  - service: notifications
    methods: [messages.send]
    resources:
      - ref: inbox
        attributes:
          channel: inbox
```

Owner-aware plugin-owned shape:

```yaml
dependencies:
  plugins:
    - id: ai-core
      version: ">=0.1.0"

hostServices:
  - service: ai
    owner: ai-core
    version: v1
    methods:
      - text.generate
      - text.method_status.get
```

Rows without `owner` are core-owned services. Plugin-owned services must declare
`owner` and `version`; the host merges core-owned static catalog entries with
owner descriptor projections for validation, authorization, upgrade previews,
and runtime dispatch. The generated catalog below is refreshed from the host
catalog implementation and only lists core-owned services.

## Declarable Host Services

<!-- BEGIN generated:host-services -->
| Service | Resource declaration | Derived capability | Methods |
| --- | --- | --- | --- |
| `runtime` | None | `host:runtime` | `log.write`<br/>`state.get`<br/>`state.get_many`<br/>`state.set`<br/>`state.set_many`<br/>`state.delete`<br/>`state.delete_many`<br/>`info.now`<br/>`info.uuid`<br/>`info.node` |
| `storage` | `resources.paths` | `host:storage` | `put`<br/>`put.init`<br/>`put.chunk`<br/>`put.commit`<br/>`put.abort`<br/>`get`<br/>`delete`<br/>`delete.batch`<br/>`list`<br/>`list.cursor`<br/>`stat`<br/>`stat.batch` |
| `network` | `resources[].url` | `host:http:request` | `request` |
| `confidentialoutbound` | None | `host:confidential-outbound` | `execute` |
| `data` | `resources.tables` | `host:data:read`<br/>`host:data:mutate` | `list`<br/>`get`<br/>`batch_get`<br/>`create`<br/>`update`<br/>`delete`<br/>`transaction` |
| `cache` | `resources[].ref` | `host:cache` | `get`<br/>`get_many`<br/>`set`<br/>`set_many`<br/>`delete`<br/>`delete_many`<br/>`incr`<br/>`expire` |
| `lock` | `resources[].ref` | `host:lock` | `acquire`<br/>`renew`<br/>`release` |
| `hostconfig` | `resources.keys` | `host:hostconfig` | `get`<br/>`sys_config.get`<br/>`sys_config.value.set`<br/>`sys_config.reset` |
| `manifest` | `resources.paths` | `host:manifest` | `get`<br/>`get_many`<br/>`list` |
| `apidoc` | None | `host:apidoc` | `route_text.resolve`<br/>`route_texts.resolve`<br/>`route_title_operation_keys.find` |
| `auth` | None | `host:auth:token`<br/>`host:auth:external_login`<br/>`host:auth:authz` | `token.tenant.select`<br/>`token.tenant.switch`<br/>`token.impersonation_token.issue`<br/>`token.impersonation_token.revoke`<br/>`external_login.login_by_verified_identity`<br/>`authz.permissions.batch_get`<br/>`authz.permissions.batch_has`<br/>`authz.permissions.has`<br/>`authz.users.platform_admin.check`<br/>`authz.role_permissions.replace` |
| `users` | None | `host:users` | `users.current.get`<br/>`users.batch_get`<br/>`users.resolve.batch`<br/>`users.list`<br/>`users.visible.ensure`<br/>`users.create`<br/>`users.create_from_external`<br/>`users.update`<br/>`users.delete`<br/>`users.status.set`<br/>`users.password.reset`<br/>`users.assignment.roles.replace` |
| `bizctx` | None | `host:bizctx` | `current.get` |
| `dict` | None | `host:dict` | `dict.refresh`<br/>`dict.type.get`<br/>`dict.type.batch_get`<br/>`dict.type.list`<br/>`dict.type.visible.ensure`<br/>`dict.type.keys.visible.ensure`<br/>`dict.type.create`<br/>`dict.type.update`<br/>`dict.type.delete`<br/>`dict.value.get`<br/>`dict.value.batch_get`<br/>`dict.value.labels.resolve`<br/>`dict.value.list`<br/>`dict.value.visible.ensure`<br/>`dict.value.values.visible.ensure`<br/>`dict.value.create`<br/>`dict.value.update`<br/>`dict.value.delete`<br/>`dict.value.by_type.delete` |
| `files` | None | `host:files` | `files.batch_get`<br/>`files.list`<br/>`files.visible.ensure`<br/>`files.upload`<br/>`files.create_from_storage`<br/>`files.metadata.update`<br/>`files.delete`<br/>`files.delete_many` |
| `jobs` | None | `host:jobs` | `jobs.batch_get`<br/>`jobs.list`<br/>`jobs.visible.ensure`<br/>`jobs.create`<br/>`jobs.update`<br/>`jobs.delete`<br/>`jobs.run`<br/>`jobs.status.set`<br/>`jobs.register` |
| `notifications` | None except `messages.send`, which uses `resources[].ref` | `host:notifications` | `messages.batch_get`<br/>`messages.list`<br/>`messages.by_source.batch_get`<br/>`messages.visible.ensure`<br/>`messages.send`<br/>`messages.delete`<br/>`messages.by_source.delete`<br/>`messages.mark_read`<br/>`messages.mark_unread` |
| `plugins` | None | `host:plugins` | `plugins.current.get`<br/>`plugins.batch_get`<br/>`plugins.registry.list`<br/>`plugins.tenant.list`<br/>`plugins.business_extensions.list`<br/>`plugins.business_extensions.exact.get`<br/>`plugins.business_extensions.exact.batch_get`<br/>`config.get`<br/>`plugins.state.enabled.check`<br/>`plugins.state.provider_enabled.check`<br/>`plugins.state.enabled_authoritative.check`<br/>`plugins.lifecycle.tenant_plugin_disable.ensure`<br/>`plugins.lifecycle.tenant_plugin_disabled.notify`<br/>`plugins.lifecycle.tenant_delete.ensure`<br/>`plugins.lifecycle.tenant_deleted.notify` |
| `route` | None | `host:route` | `metadata.get` |
| `sessions` | None | `host:sessions` | `sessions.current.get`<br/>`sessions.list`<br/>`sessions.batch_get`<br/>`sessions.users.online.batch_get`<br/>`sessions.visible.ensure`<br/>`sessions.revoke`<br/>`sessions.revoke_many` |
| `org` | None | `host:org` | `capability.available`<br/>`capability.status`<br/>`org.assignment.user_profiles.batch_get`<br/>`org.department.tree.list`<br/>`org.department.batch_get`<br/>`org.department.list`<br/>`org.post.batch_get`<br/>`org.post.options.list`<br/>`org.department.visible.ensure_many`<br/>`org.post.visible.ensure_many`<br/>`org.department.create`<br/>`org.department.update`<br/>`org.department.delete`<br/>`org.post.create`<br/>`org.post.update`<br/>`org.post.delete`<br/>`org.assignment.by_user.replace`<br/>`org.assignment.by_user.cleanup` |
| `tenant` | None | `host:tenant` | `capability.available`<br/>`capability.status`<br/>`tenant.context.current`<br/>`tenant.context.info`<br/>`tenant.context.platform_bypass`<br/>`tenant.directory.batch_get`<br/>`tenant.directory.list`<br/>`tenant.membership.validate`<br/>`tenant.membership.list_by_user`<br/>`tenant.directory.visible.ensure_many`<br/>`tenant.plugins.enabled.set`<br/>`tenant.plugins.defaults.provision`<br/>`tenant.filter.context` |
<!-- END generated:host-services -->

## Maintenance Notes

When plugin public contracts or dynamic `host service` descriptors change, update both `README.md` and `README.zh-CN.md` in this directory.

### Host Service Payloads and Constants

- New core-owned host-service methods must use the unified JSON envelope (`HostServiceJSONRequest` / `HostServiceJSONResponse` or empty payloads). Do not introduce dedicated binary codecs for new methods.
- Existing dedicated codec methods are frozen; catalog governance tests reject dedicated expansion outside that allowlist.
- Service and method wire constants are maintained once in `pluginbridge/protocol/hostservices/wire_constants.go`. The catalog must reference those constants (not string literals); catalog tests enforce this.
- New WASM JSON host-service methods should reuse the `decodeCapabilityJSONRequest` and `capabilityJSONResponse` helpers.
