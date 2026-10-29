# First installation

For website/customer source locations, delivery, and AI collaboration, see the [development guide](DEVELOPMENT.md).

This repository already includes business plugin sources. Do not download a different generation of official plugins. Install `Go 1.25+`, `Node.js 22.12+`, `pnpm 10+`, and `PostgreSQL`, then run these commands from the repository root.

## Development installation and startup

Prepare a reachable `PostgreSQL` database, then prepare runtime dependencies and instance configuration. These commands do not install a database server, delete data, or rebuild an existing database.

```text
make env.setup
```

`make env.setup` installs frontend runtime dependencies, persists installation keys, and creates `.site-runtime/installation/config.yaml` from the official configuration template without overwriting an existing file. Edit this file to configure the database connection. Subsequent installation commands use it by default; an explicit `GF_GCFG_FILE` takes precedence. The file contains sensitive configuration such as database credentials and must not be committed to `Git`.

For a new database only, initialize it once:

```text
make db.init confirm=init
```

Once the database is ready, start the complete development environment:

```text
make dev
```

The root `Makefile` routes `env.setup`, `db.init`, `build`, and `dev` to the plugin installation tool. `env.setup` prepares only application runtime dependencies and installation material. `dev` builds the complete delivery, then runs the backend, admin gateway, customer gateway, two website renderers, and website gateway in the current terminal. Default addresses are `http://127.0.0.1:6031/admin/`, `http://127.0.0.1:6032/`, and `http://127.0.0.1:6680/`. Press `Ctrl+C` in the startup terminal to stop services created by this invocation; do not use legacy `make stop` for this session. Occupied ports cause an explicit error without terminating another instance. Run `make dev` again after source changes; this command does not provide source hot reloading.

Exited services restart independently with delays increasing from one to thirty seconds; healthy services stay running. A stable minute resets the retry delay. Use `make dev rebuild=0` to reuse a complete existing delivery with the same supervisor; missing entries cause an explicit error. After selecting a website or customer template in the administration, refresh the corresponding site to read the new selection without restarting processes.

After the first startup, install and enable the required dynamic plugins in the administration UI, then select templates in website and customer center configuration. Building artifacts or starting processes does not perform plugin installation, activation, or template selection for the administrator.

The build and database initialization invoke official `pack.assets` to generate metadata, `SQL`, and language resources under `apps/lina-core/internal/packed/manifest/` for compile-time `Go` embedding. This official resource generation is a precise exception to the read-only host rule; host source logic and root `hack/` remain unchanged. Other build artifacts go to `temp/output/`.

Inspect the startup plan without generating keys, building, or starting services:

```text
make dev plan=1
```

Override ports using portable `make` variable arguments, for example `make dev LINA_CORE_PORT=16030 LINA_VBEN_PORT=16031 LINA_CUSTOMER_PORT=16032 LINA_WEBSITE_PORT=16680`.

| Variable | Default | Purpose |
| --- | --- | --- |
| `LINA_CORE_PORT` | `6030` | Backend APIs |
| `LINA_VBEN_PORT` | `6031` | Administration entry |
| `LINA_CUSTOMER_PORT` | `6032` | Customer center entry |
| `LINA_WEBSITE_PORT` | `6680` | Website entry; the two renderers use the next two ports, `6681` and `6682` by default |

Browser testing and code inspection tools are not runtime prerequisites. If needed, install the full official development toolchain separately with `go run ./hack/tools/linactl env.setup`. This includes `Playwright` and system dependencies and may not support every operating system. Failure to install this optional toolchain does not mean application runtime dependency installation failed.

## Preparation and build

```text
make env.setup
make build
```

The plugin installation tool compiles the unmodified host with all source business plugins through the public `official_plugins` registration contract. Admin assets go to `temp/output/admin/public` without writing host sources. Customer and website templates use their own builders; dynamic artifacts go to `temp/output`. Neither the removed customer application nor the old website workspace is used. Use this plugin delivery command rather than the host `linactl build` entry, which still expects the old customer application.

`make build` performs a complete build by default. For a plan or a limited build, use `go run ./apps/lina-plugins/idc-compliance/hack/tools/installation/main.go build plugins=1 plan=1`; the plan generates no keys and starts no services. Replace `plan=1` with `target=backend` for admin/backend only, or `target=templates` for dynamic packages and templates only. Use `out=directory` with the plugin command or `make build output_dir=directory` to change the output location; source directories are rejected.

## Database and startup

The following is the separate-process deployment flow; use `make dev` above for unified development startup. Reuse the database configuration prepared earlier. Initialize only a new database, never an existing installation.

```text
make db.init confirm=init
go run ./apps/lina-plugins/idc-compliance/hack/tools/installation/main.go run ./temp/output/lina
```

Start the admin entry in another terminal:

```text
node ./temp/output/admin/server.mjs
```

Admin defaults to `http://127.0.0.1:6031/admin/` and forwards backend requests to `http://127.0.0.1:6030`. Configure `ADMIN_PORT`, `ADMIN_HOST` and `LINAPRO_BACKEND_PROXY_TARGET` as needed. Use a reverse proxy and process manager in production. SPA deep-link refreshes work; keep backend and admin processes running separately.

On `Windows`, use `./temp/output/lina.exe` for the backend executable. Install and enable built managed plugins in the administration plugin list. Supplier integrations require your own supplier accounts; successful installation does not imply a supplier connection.

## Website configuration

Install and enable a website plugin in the administration plugin list, then open “System settings → Website configuration”. Multiple website plugins can be installed, but only one is selected for display. Switching the selection removes the previous website from display without disabling or uninstalling its plugin; each template retains its settings. Set shared information here; the selected plugin defines its image, menu, and other fields. A website without an enabled template displays “Website is not enabled”.

A full installation build already places website WASM and SSR services under `temp/output/websites/`. Each website plugin carries its own application source and build tools. The retired `apps/lina-websites` workspace has been removed. Enter `apps/lina-plugins/website-default`, or copy the entire plugin outside the repository, then run:

```text
make install build
```

Upload `dist/website-default.wasm` in plugin management, preview and confirm the upgrade. For a separate full `SSR` deployment, set `PORT=6681` and `NUXT_API_ORIGIN=http://127.0.0.1:6030` in the renderer's environment or process manager, then run:

```text
node frontend/site/.output/server/index.mjs
```

For the separate gateway process, set `WEBSITE_SSR_ORIGINS` to `{"website-default":"http://127.0.0.1:6681"}`, then run:

```text
node gateway/server.mjs
```

The delivery equivalents are `temp/output/websites/website-default/renderer/server/index.mjs` and `temp/output/websites/website-default/gateway/server.mjs`; run them with Node and the same environment variables from the repository root. The gateway listens on port6680 and routes to the SSR service for the selected template. Build `website-minimal` independently, run it on port6682, and add its operator-controlled origin to `WEBSITE_SSR_ORIGINS` when needed. Configure renderers with `NUXT_API_ORIGIN`, `NUXT_PUBLIC_SITE_URL`, and `NUXT_PUBLIC_CUSTOMER_URL`; configure the gateway with `WEBSITE_API_ORIGIN` and `WEBSITE_CUSTOMER_URL`. Never derive SSR upstream origins from visitor input.

Without an SSR mapping, the gateway serves the complete browser application embedded in the dynamic package; business pages remain usable, but this static mode does not provide full server-rendered SEO. The native HTML example includes real HTML pages and uses the same `make install build` package command. Installing the backend or WASM does not start Node processes automatically.

## Customer template configuration and startup

Install and enable `customer-default` in the administration plugin list, then select it under “System settings → Customer center configuration”. Save shared branding and template-specific settings separately. Manage member navigation in the unified administration page; each product belongs to one product menu.

`make dev` already starts the customer entry. For separate-process deployment, the customer site uses a plugin-owned entry; running only the backend does not start it. From the repository root, run:

```text
go run ./apps/lina-plugins/idc-compliance/hack/tools/installation/main.go customer.run
```

The default listener is `127.0.0.1:6032`. Configure `CUSTOMER_PORT` and `LINAPRO_BACKEND_PROXY_TARGET` for other ports or backend origins. The old host customer application has been removed. Do not run another service on the same port. A conflicting listener causes startup to fail; no unrelated process is terminated. In production, supervise `node temp/output/customer/server.mjs` or the plugin's `frontend/server.mjs`, and retain installed template packages and their public assets. Templates can use any frontend framework through the documented public APIs and asset entry protocol. Developer documentation is available in the corresponding administration pages.

## Preserve installation material

`.site-runtime/installation` contains this installation's configuration, random keys, and identity. Persist and back it up, and reuse it across restarts. Never commit it to `Git` or delete and regenerate it: existing encrypted data would become unreadable. Explicit deployment configuration takes precedence; damaged files stop preparation and require backup recovery.

Website configuration uses file storage. Multiple read instances must share `LINA_SITE_STORAGE`; only one administration instance may write configuration. Concurrent writes from multiple instances are unsupported. The preparation tool does not replace shared storage configuration.

`plugins.install` downloads development sources; it does not install plugins in the administration UI. Skip it when this repository already contains the complete sources. When downloads are needed, the plugin-side `official-plugins.yaml` uses this project's `main` branch; do not mix in obsolete plugins from another repository.
