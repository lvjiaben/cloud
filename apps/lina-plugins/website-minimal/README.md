# website-minimal

Each website plugin carries its own application source and build tools. The retired `apps/lina-websites` workspace has been removed. Enter `apps/lina-plugins/website-minimal`, or copy the entire plugin outside the repository, then run:

```sh
make install build
# Upload dist/website-minimal.wasm in plugin management, preview and confirm the upgrade.
# Full SSR: start the plugin renderer and gateway as separate managed processes.
PORT=6681 NUXT_API_ORIGIN=http://127.0.0.1:6030 node frontend/site/.output/server/index.mjs
WEBSITE_SSR_ORIGINS='{"website-minimal":"http://127.0.0.1:6681"}' node gateway/server.mjs
```

The gateway listens on port6680 and routes to the SSR service for the selected template. Build `website-minimal` independently, run it on port6682, and add its operator-controlled origin to `WEBSITE_SSR_ORIGINS` when needed. Configure renderers with `NUXT_API_ORIGIN`, `NUXT_PUBLIC_SITE_URL`, and `NUXT_PUBLIC_CUSTOMER_URL`; configure the gateway with `WEBSITE_API_ORIGIN` and `WEBSITE_CUSTOMER_URL`. Never derive SSR upstream origins from visitor input.

Without an SSR mapping, the gateway serves the complete browser application embedded in the dynamic package; business pages remain usable, but this static mode does not provide full server-rendered SEO. The native HTML example includes real HTML pages and uses the same `make install build` package command. Installing the backend or WASM does not start Node processes automatically.

Published navigation pages are validated against actual routes at build time. Navigation groups can expand without linking.

The website owns product configuration at `/products`, the cart at `/cart`, and order confirmation at `/checkout`. A hidden `/auth/website-session` page reads the configured customer service’s existing session. Signed-out customers use the complete customer authentication flow in the current tab and return to website checkout. The gateway delegates `/auth/*`, `/account/*`, and payment return pages to the operator-configured `WEBSITE_CUSTOMER_URL` service and forwards protected APIs; backend ownership checks remain authoritative. Session messages validate the configured customer origin, source window and nonce, target only the configured website origin, and never place tokens in URLs. Local loopback website aliases use the same configured customer origin; only the configured website and its same-port loopback alias are accepted locally. Production origins remain exact. When browsers partition iframe storage, a current-tab customer session probe posts the existing token to the website gateway body. The gateway accepts only the configured customer origin, verifies the customer with the backend, and delivers it in a 30-second HttpOnly handoff cookie cleared by the website session read. Tokens stay out of URLs. Selected configurations retain opaque references across guest-cart merges and pass the existing quote-item configuration validation before order submission. Only a confirmed order opens its customer invoice and payment-method dialog. Payment providers retain channel configuration and confirmation authority. Purchasing requires both the website gateway and customer service; a generic static file server is insufficient.
