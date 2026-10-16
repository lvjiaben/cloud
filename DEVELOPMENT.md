# Website, customer center, and AI development

This guide helps developers and coding agents continue work after cloning the repository. See [First installation](INSTALL.md) for the complete setup and deployment instructions. Documented startup validation does not imply that every external provider has been tested.

## Installation and daily startup

Prepare `Go 1.25+`, `Node.js 22.12+`, `pnpm 10+`, `make`, and an accessible `PostgreSQL` database. From the repository root, run:

```text
make env.setup
```

Fill in the database connection in `.site-runtime/installation/config.yaml`. Initialize a new, empty database once, then start:

```text
make db.init confirm=init
make dev
```

For daily startup, run only `make dev`; do not initialize an existing database again. The command builds the delivery and runs six foreground services. It does not provide hot reload. After code changes, stop this session with `Ctrl+C` and rerun `make dev`. Do not use the old `make stop` for this session. `make dev plan=1` prints the plan without starting anything. Port conflicts fail explicitly without killing other processes.

| Service | Default port | Purpose |
| --- | --- | --- |
| Backend | `6030` | Business, authentication, and plugin APIs |
| Admin | `6031` | `/admin/` |
| Customer gateway | `6032` | Selected customer template and same-origin API proxy |
| Website gateway | `6680` | Website selected in admin configuration |
| Default website renderer | `6681` | Independent SSR service |
| Minimal website renderer | `6682` | Independent SSR service |

On first startup, install and enable the required templates in plugin management, then select and save them in the website and customer-center settings under system settings. Starting processes, installing/enabling a plugin, and selecting the displayed template are separate operations. Starting only the backend does not start web gateways; installing WASM does not start Node.js.

Persist and back up `.site-runtime/installation/`, which contains instance keys, identity, and private database configuration. Never commit it. `temp/output/` is generated delivery output, not source. See [installation](INSTALL.md) for website configuration storage and deployment environment variables.

## Source and API map

| Target | Repository path |
| --- | --- |
| Default website | `apps/lina-plugins/website-default/frontend/site/` |
| Minimal website | `apps/lina-plugins/website-minimal/frontend/site/` |
| Native multipage website | `apps/lina-plugins/website-html/frontend/public/` |
| Website gateway | `apps/lina-plugins/website-default/gateway/server.mjs` |
| Default customer application | `apps/lina-plugins/customer-default/frontend/customer/` |
| Customer gateway | `apps/lina-plugins/customer-default/frontend/server.mjs` |
| Shared branding, template settings, developer documentation | `apps/lina-plugins/idc-portal/` |
| Member navigation and product assignment | `apps/lina-plugins/idc-core/backend/internal/service/productdirectory/` |
| Installation and startup tool | `apps/lina-plugins/idc-compliance/hack/tools/installation/main.go` |

Website and customer templates are independent managed dynamic plugins. `plugin.yaml` declares capabilities; WASM delivers the plugin and public assets; browsers run HTML, JavaScript, and CSS. Default website templates can also run independent SSR services. Vue, React, jQuery, and plain scripts are all valid. The old `apps/lina-vben/apps/web-customer` has been removed; `apps/lina-websites/sites/default` is not the current website entry. Do not develop against those old locations.

The following detailed contracts are also displayed by the corresponding admin settings pages:

- [Website plugin development](apps/lina-plugins/idc-portal/docs/website-plugin-development.zh-CN.md): manifests, real pages, navigation, public APIs, packaging, deployment, and lifecycle.
- [Customer plugin development](apps/lina-plugins/idc-portal/docs/customer-plugin-development.zh-CN.md): authentication, customer APIs, member navigation, independent packaging, and proxy entry.
- [Customer template](apps/lina-plugins/customer-default/README.md) and [website template](apps/lina-plugins/website-default/README.md): plugin-local build instructions.

The complete API contract is the running admin origin's `/api.json`, normally `http://localhost:6031/api.json`. Admin developer documentation supports search, field expansion, and download. Definitions reflect the running code and installed plugins; old screenshots are not API contracts. Authentication uses `/api/v1/auth/`, customer business APIs mainly use `/x/idc-core/api/v1/account/`, and anonymous website APIs use `/x/idc-core/api/v1/public/`. Check OpenAPI for exact methods, fields, units, and permissions. Customer templates must not use admin tokens; websites must not carry customer or admin tokens.

## Change-to-delivery workflow

1. Locate the owning plugin. Read root `AGENTS.md`, applicable rules, plugin-local instructions, actual pages/APIs, and the relevant complete business requirements.
2. Modify the plugin. Routes must actually exist: `plugin.yaml` page declarations do not create pages. A new plugin needs a unique identifier consistently applied to its manifest and entry points.
3. Consume existing public APIs. Backend pricing, authorization, and ownership remain authoritative. A product selects one product-type member menu; navigation editing does not maintain a duplicate product list.
4. Build/package following plugin instructions. For repository delivery use root `make build`, which writes dynamic templates to `temp/output/`. The customer plugin supports `pnpm install --frozen-lockfile`, `pnpm run build`, and `pnpm run package` from its root, producing `dist/customer-default.wasm`. Follow the website guide for independent website builds. On Windows, prefer the root installation tool for repository builds rather than copying POSIX environment-variable prefixes.
5. Upload WASM and install/enable on first use. For updates, increment the version, upload, preview, and confirm the upgrade. Upload alone does not activate an upgrade. SSR changes also require deployment of the new renderer and restarting its service.
6. Select the template and verify login, deep-link refresh, menus, products, and affected operations in a real browser. Check network/console errors, run relevant tests, and record unverified areas.

## Continuing with an AI agent

Open the repository root as the workspace and supply a concrete target, page, and reproduction. For example:

```text
Read AGENTS.md, .agents/rules/user-preferences.md, and DEVELOPMENT.md first.
Change [website/customer/plugin] at [page or workflow] to achieve [specific result].
Locate source, public APIs, and confirmed business rules before implementation.
Complete relevant tests and real browser verification.
Do not modify the host, commit, or push. Do not invent APIs, prices, provider capabilities, or success results.
Preserve existing configuration, databases, and unrelated running instances.
Ask in plain language before implementing ambiguous business outcomes.
```

Root `AGENTS.md` is the work entry, `.agents/CONTEXT.md` locates topics, `.agents/memory/preferences/` preserves confirmed decisions, and `openspec/changes/` records active work. Do not load every rule or archive at once. Before a later commit, ensure source, lockfiles, and public assets are trackable; exclude private keys, tokens, logs, database connections, and `node_modules`. This guide does not authorize automatic Git publication or third-party production actions.

## Troubleshooting and known limitations

- Connection refused on `6680`: check the website gateway and the selected template's renderer. A working admin on `6031` does not establish website availability. Do not start duplicate services over an existing split-process instance.
- Template unavailable: check backend reachability, installation/enabled state, selected template, gateway backend address, and SSR upstreams. Do not delete configuration or bypass checks.
- Clean-source builds, empty-database startup, installation/enabling of sixteen dynamic plugins, template switching, and actual pages have been validated. External provider transactions without real accounts remain unverified.
- Some template disable/uninstall operations may still fail with `owner unknown` due to a host extension-authorization snapshot defect. Keep the plugin and switch the displayed template; do not edit tables, remove declarations, or bypass checks. See [validation and remaining issues](openspec/changes/repair-clean-installation/runtime-validation.md).
- Do not modify `apps/lina-core/`, `apps/lina-vben/`, or root `hack/`. The precise installation exception permits the unchanged official `pack.assets` to generate `apps/lina-core/internal/packed/manifest/`; it does not permit host logic changes. See [host boundary](.agents/memory/preferences/74-host-boundary.md).
