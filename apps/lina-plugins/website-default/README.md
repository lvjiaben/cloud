# website-default

Each website plugin carries its own application source and build tools. The old `apps/lina-websites/sites/default` application is not required. Enter `apps/lina-plugins/website-default`, or copy the entire plugin outside the repository, then run:

```sh
make install build
# Upload dist/website-default.wasm in plugin management, preview and confirm the upgrade.
# Full SSR: start the plugin renderer and gateway as separate managed processes.
PORT=6681 NUXT_API_ORIGIN=http://127.0.0.1:6030 node frontend/site/.output/server/index.mjs
WEBSITE_SSR_ORIGINS='{"website-default":"http://127.0.0.1:6681"}' node gateway/server.mjs
```

The gateway listens on port6680 and routes to the SSR service for the selected template. Build `website-minimal` independently, run it on port6682, and add its operator-controlled origin to `WEBSITE_SSR_ORIGINS` when needed. Configure renderers with `NUXT_API_ORIGIN`, `NUXT_PUBLIC_SITE_URL`, and `NUXT_PUBLIC_CUSTOMER_URL`; configure the gateway with `WEBSITE_API_ORIGIN` and `WEBSITE_CUSTOMER_URL`. Never derive SSR upstream origins from visitor input.

Without an SSR mapping, the gateway serves the complete browser application embedded in the dynamic package; business pages remain usable, but this static mode does not provide full server-rendered SEO. The native HTML example includes real HTML pages and uses the same `make install build` package command. Installing the backend or WASM does not start Node processes automatically.

Published navigation pages are validated against actual routes at build time. Navigation groups can expand without linking.
