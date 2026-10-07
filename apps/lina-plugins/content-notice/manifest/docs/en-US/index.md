# Content - Notices

This document is bundled under the plugin manifest and is intended for plugin-market display. Runtime behavior is unchanged by the documentation resource.

## Summary

Built-in source plugin for notice management, publication, and inbox delivery.

## Documents

- [Configuration](configuration.md)
- [Changelog](changelog.md)

## Feature Highlights

- Queries, creates, edits, previews, publishes, and deletes notices.
- Drafts do not notify users; publication sends an inbox message.
- The page and backend ship with the host, are enforced at startup, and stay hidden from plugin management.
- Supports tenant-aware usage scenarios.

## Where It Fits

Use it to manage tenant-aware notices and announcements inside the content workspace.

## Entry Points

| Name | Path |
| --- | --- |
| Notices | `/system/notice` |

## Metadata

| Field | Description |
| --- | --- |
| Plugin ID | `content-notice` |
| Version | `v0.3.0` |
| Type | `source` |
| Distribution | `builtin` |
| Scope | `tenant_aware` |
| Multi-tenant | Yes |
