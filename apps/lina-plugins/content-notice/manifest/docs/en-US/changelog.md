# Changelog

## Current Release

| Version | Date | Description |
| --- | --- | --- |
| `v0.3.1` | 2026-10-28 | Unifies announcement storage and public/customer reading, adds saved sort order and isolated read state, removes notice types, attachments, audience settings, and inbox delivery, and restores masked animated editors. |
| `v0.3.0` | 2026-09-03 | Converts the plugin to a host-compiled built-in source plugin that is startup-enforced and hidden from plugin management, while preserving business menus, tenant isolation, inbox publication, and delete cascading. |
| `v0.2.1` | 2026-09-03 | Fixes invisible buttons and form borders under host HSL theme tokens, waits for host context before the single initial list request, and deduplicates resize reports to prevent layout feedback. |
| `v0.2.0` | 2026-09-02 | Converts the plugin to a managed dynamic plugin, moves notice data access behind the governed host data service, adds a standalone admin surface, and preserves tenant isolation, inbox publication, and delete cascading. |
| `v0.1.0` | 2026-07-22 | Adds a bilingual plugin-market documentation snapshot bundled with the plugin manifest, covering features, configuration, and changelog. Runtime behavior is unchanged. |

## Maintenance

- Update both `zh-CN` and `en-US` docs when plugin features, settings, or dependencies change.
