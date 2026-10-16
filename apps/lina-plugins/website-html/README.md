# Native HTML website example

This plugin includes HTML, JavaScript, CSS, a standard-library Go guest and a standalone packager. No repository source or frontend framework is required.

```sh
make install build
# Upload dist/website-html.wasm, install or upgrade, enable and select it in website configuration.
node gateway/server.mjs
```

The gateway listens on `6680` without a separate renderer. Configure `WEBSITE_API_ORIGIN` and `WEBSITE_CUSTOMER_URL` in the process environment when using nondefault backend or customer ports. `frontend/public/index.html` and `products.html` are actual source pages; packaging checks every declared HTML page exists. Add real HTML files and update `plugin.yaml` pages when extending the template. Public brand data, per-template settings and navigation use the public brand API. Submenu groups expand without navigation. This is a native integration example; use `website-default` or `website-minimal` for the full cloud-service website.
