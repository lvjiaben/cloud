# content-notice

`content-notice` is the official built-in source plugin for notice and announcement management.

English | [简体中文](README.zh-CN.md)

## Scope

This plugin owns:

- notice and announcement CRUD
- draft publication and inbox delivery
- notice-specific dictionaries and storage schema
- the default workspace entry under the `content` host catalog

## Host Boundary

The host keeps the `content` catalog skeleton, authentication, tenant boundaries, inbox capability, and menu governance. The plugin is compiled with the host, keeps business data in its own table, and uses published host capabilities for user projection and inbox delivery. It is startup-enforced, hidden from plugin management, and cannot be disabled or uninstalled.

## Plugin Metadata

| Field | Value |
| --- | --- |
| Plugin ID | `content-notice` |
| Type | `source` |
| Distribution | `builtin` |
| Scope | `tenant_aware` |
| Install mode | `global` |
