# Default customer center template

An independent dynamic template with shared branding (logo, name, title, company and contacts) and separately retained template settings.

Copy this entire directory anywhere. With Node.js 22+, pnpm 10.30.3 and Go 1.25+, run `pnpm install --frozen-lockfile`, `pnpm run build` and `pnpm run package`. Upload `dist/customer-default.wasm`. No host source, external source symlinks or repository build tools are required.

`frontend/customer/vendor/` contains only the 29 required Vben package dependencies and customer integration source closure. See `DEPENDENCIES.json` and the retained `vben/LICENSE` (MIT). Business features consume public HTTP APIs. These copies are maintained independently.

Run `node frontend/server.mjs` with `LINAPRO_BACKEND_PROXY_TARGET` and `CUSTOMER_PORT`. The framework-independent gateway resolves the active template and forwards API/authentication/assets, including deep-link refreshes. Third-party templates may use vanilla JavaScript, jQuery, Vue or React.

Select the installed template under System settings / Customer center configuration. Template switches preserve shared and per-template settings and all customer business data. Some host versions have an owner authorization defect in disable/uninstall impact checks; this plugin does not bypass those checks or modify the host.
