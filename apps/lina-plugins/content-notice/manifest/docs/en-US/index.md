# Content - Announcements

This document is bundled under the plugin manifest and is intended for plugin-market display. Runtime behavior is unchanged by the documentation resource.

## Summary

Built-in source plugin for announcement management, publication, and per-user read state.

## Documents

- [Configuration](configuration.md)
- [Changelog](changelog.md)

## Feature Highlights

- Queries, creates, edits, previews, publishes, sorts, and deletes announcements.
- Drafts stay private to management; published content is shared by the customer center and public website.
- The page and backend ship with the host, are enforced at startup, and stay hidden from plugin management.
- Supports inline images and persistent user read state without type, attachment, or audience settings.

## Where It Fits

Use it to manage announcements inside the content workspace.

## Entry Points

| Name | Path |
| --- | --- |
| Announcements | `/system/notice` |

## Metadata

| Field | Description |
| --- | --- |
| Plugin ID | `content-notice` |
| Version | `v0.3.1` |
| Type | `source` |
| Distribution | `builtin` |
| Scope | `tenant_aware` |
| Multi-tenant | Yes |
