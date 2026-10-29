# Built-in plugin manager

`plugin-manager` is a platform-only built-in source plugin. Its published menu filter hides the original plugin-management navigation entry. The workspace lives at `/extension/plugin-manager` under **System settings → Plugin management**, attached to the stable `setting` catalog so the frontend's extension-center hiding does not conceal it. No host source changes are required.

Source-plugin builds must refresh the official temporary workspace through `plugins=1`. Do not reuse an outdated registration aggregate. Verify that the delivered binary includes `lina-plugin-plugin-manager`, then check the live category API and replacement navigation; an enabled database record alone does not prove runtime registration.

## Categories and list

Initial tabs are Payment, SMS, Servers, Cloud products, Website, Customer center, and System features. `manifest/config/config.yaml` defines `catalog`; deployment plugin configuration may override it. Ordered `categories` contain `id`, `name`, exact `plugins`, and optional `prefixes`. Exact IDs win, followed by the longest prefix; `fallback` assigns unmatched plugins. New categories require no frontend enum changes. The initial categories have matching icons; additional categories use a generic icon. The toolbar includes a development-documentation drawer using the published host guide messages.

Only managed dynamic plugins appear. Searches are limited to identifier and name. Name appears above identifier in one column, followed by version, status, business admission, runtime state, installation time, and actions limited to install, update, and uninstall. The action column fits its button content. Disabling or uninstalling automatically stops new sales before continuing. Already stopped plugins skip that step; a stop failure prevents the subsequent action. If the subsequent action fails, sales remain stopped and the administrator can retry. Existing permissions, dependency and business-impact checks, and existing-service access remain in effect.

## Installation and validation

Follow the [development guide](../README.md). Categories never grant installation permission. Public host APIs retain owner-contract, dependency, permission, authorization-scope, and business-impact checks. Install and uninstall use one short inline confirmation. Installation and reinstallation automatically enable the plugin and open new-business admission; uploading a new package completes these steps without another installation dialog. Details load on demand, and updates review target-version permissions. Uninstallation preserves storage; dependency and business blockers cannot be bypassed.

Standard `plugins=1` builds register this source plugin. `plugin_embed.go` embeds its manifest, configuration, and pages. The plugin uses Chinese as its single UI language. Custom roles must be granted the new plugin menu through existing role management; APIs retain `plugin:list` and their original action permissions without automatic privilege expansion. Catalog aggregation dispatches ordinary public GET requests through the same server's HTTP handler, repeating authentication and permissions while forwarding only required identity, locale, and tenant headers. Reads are bounded to 1000 summaries and paginated on the server after classification, without per-row detail queries, private tables, or extra caches.

## Verification

From the repository root run `GOWORK=off go -C apps/lina-plugins/plugin-manager test ./backend/... -count=1` and `make lint dir=apps/lina-plugins/plugin-manager`. `hack/tests/e2e/TC001-plugin-management.ts` covers page interactions; `TC002-live-navigation.ts` verifies deployment through real login, menu navigation, and catalog APIs without injected routes or mocked APIs. Run them through the standard plugin E2E runner. Contract-audit cases live in `hack/tests/contracts/` and need an isolated source-plugin Go workspace. Do not restart shared instances while another chat uses them.

## Business authorization recovery

When disable or uninstall encounters missing business authorization, the action automatically revalidates through the public enable endpoint and continues without another dialog or checkbox. It never resumes new sales or edits private snapshots. Server permissions, dependencies, and business-impact checks remain authoritative; failures refresh real state and allow retry. This adapts to host synchronization clearing authorization snapshots and does not claim to fix that internal cause.
