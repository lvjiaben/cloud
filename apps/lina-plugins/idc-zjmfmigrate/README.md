# idc-zjmfmigrate

`idc-zjmfmigrate` is the managed dynamic plugin for ZJMF Finance and Business Management V10 migration checks.

English | [简体中文](README.zh-CN.md)

## Scope

The plugin owns read-only source preflight, staging, mappings, conflict decisions, import reconciliation, and cutover checks.

## Dependencies

The plugin depends on the public import contracts from `idc-core` and `idc-resource`.

## Delivery

Version `v0.2.2` is delivered as `dynamic + managed` with `platform_only`, `supports_multi_tenant: false`, and `global` governance. It runs as isolated WASM, is entered from System Configuration, and requires manual enablement during a migration window. The conflict list uses the `vben-adapter` isolated runtime with bounded pages and an exact host-scoped total. Upgrading preserves configuration and does not add SQL migrations.

## Phase Boundary

The current workflow covers source-parameter preflight, migration batches, conflict decisions, reconciliation, cutover checks, guarded rollback, and evidence reports. Real source connectivity and production import still require source credentials, backups, and an acceptance window; read-only preflight has no external side effects.

## Build and page bridge

Run from the repository root:

```bash
go run ./hack/tools/linactl build dir=apps/lina-plugins/idc-zjmfmigrate
```

The frontend ships `admin/migration/index.html` and compiled assets using the official `vben-adapter` isolated table and form drawer. The `migration-checks` surface calls existing `admin/migration/*` contracts through `idc-migration.admin.v1`, with write payloads in `body`. Host and owner authorization remain mandatory. Successful source preflight gates batch creation; import, execution, cutover, and rollback additionally require page confirmation. The page never reads host storage or tokens.
