# idc-ai

`idc-ai` is the built-in source plugin for read-only IDC AI assistance.

English | [简体中文](README.zh-CN.md)

## Scope

The plugin owns merchant AI configuration, read-only tools, insights, retrieval, diagnostics, prediction, and drafts. It never exposes a dangerous-operation tool.

## Dependencies

The plugin depends on the public `idccap` and `resourcecap` read contracts from `idc-core` and `idc-resource`.

## Delivery

Version `v0.1.0` is delivered as `source + builtin` with `platform_only`, `supports_multi_tenant: false`, and `global` governance.

## Phase Boundary

The plugin exposes a merchant-scoped, encrypted OpenAI-compatible configuration, bounded read-only copilot APIs, customer self-data assistant, safety audit and feedback records. Dangerous operations are rejected before any model call; when the model is disabled or unavailable, users are sent back to normal pages.
