# Website Deployment

Run `pnpm package` to produce `dist/website-main.wasm`, then upload, install and enable it through existing plugin management without mock data. Select it in website configuration while retaining shared information and previous template settings. Current dependencies are `idc-portal >=0.1.14` and `idc-core >=0.1.0`; announcements consume the existing public `content-notice` API.

For independent ordinary-file hosting, use `pnpm build:files` or `go run ./hack/tools/build/main.go -output <new-directory>`. The directory contains pages, gateway and resource checksums, excluding the host, business backend, QA data and plugin binaries. Verify WASM and ordinary-file delivery separately. Set `WEBSITE_API_ORIGIN` and `WEBSITE_CUSTOMER_URL` before running `node gateway/server.mjs`; optional `HOST` and `PORT` select the listener.

The customer center must trust the actual website origin. Authentication uses the existing session bridge. The proxy accepts only the public reads and authenticated account transactions required by this website. Order confirmation opens the customer-center invoice rather than creating another payment state. Project development is managed by `make dev`, not a permanent temporary preview process.

The initial branding cache contains only public identity data. It defaults to `.public-brand-cache`; `WEBSITE_BRAND_CACHE_DIR` may specify a shared writable directory. Entries expire after thirty seconds and refresh every ten seconds. Expiration does not prevent page delivery. Never store credentials or private business data there.

Use the current integration records for installation and runtime evidence. Do not overwrite immutable releases, delete business data or modify the host. See [source and license](docs/source-and-license.md) for attribution.
