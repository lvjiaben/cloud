# content-notice

`content-notice` is the official built-in source plugin for announcement management.

English | [简体中文](README.zh-CN.md)

## Scope

This plugin owns:

- announcement CRUD, draft publication, and inline images
- persistent sort order and per-user read state
- shared published content for the customer center and public website
- the default workspace entry under the `content` host catalog

## Host Boundary

The host keeps the `content` catalog skeleton, authentication, tenant boundaries, and menu governance. The plugin is compiled with the host, keeps business data in its own table, and uses published host capabilities for user projections. Published announcements share one source without audience settings; read state is isolated by the authenticated user. It is startup-enforced, hidden from plugin management, and cannot be disabled or uninstalled.

## Plugin Metadata

| Field | Value |
| --- | --- |
| Plugin ID | `content-notice` |
| Type | `source` |
| Distribution | `builtin` |
| Scope | `tenant_aware` |
| Install mode | `global` |
