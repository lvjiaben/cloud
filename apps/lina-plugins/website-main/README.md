# Official Website

`website-main` is an independent managed dynamic website plugin selectively imported from commit `240aec4` on `codex/官网交付`. The current local integration is `v0.4.3`. It preserves the original pages, assets and Chinese/English presentation while using the current public contracts of `idc-portal`, `idc-core` and `content-notice`. The branch's older business implementation, other templates, simulated products and historical workspace were not merged wholesale.

All 455 original HTML, CSS, image and font files remain byte-identical to the selected branch. The added cart uses the existing configuration-page shell and styles; it has no separate stylesheet.

The website owns product configuration, individual server login choices, cart, pricing and order confirmation. Authentication uses the existing customer center; checkout selects payment methods and initiates payment within the website through the current billing and payment APIs. Unknown submissions resume by querying authoritative status without repeating payment creation. Every product has a fixed `NEW` badge without an administrative toggle. Public branding uses shared settings, including independent header/footer logos and contact and registration information. Disabled login inputs remain hidden, and passwords never enter ordinary configuration or browser persistence.

Use `Go 1.25+`, `Node.js 22.6+` and the pinned `pnpm 10.30.3`:

```text
pnpm install --frozen-lockfile
pnpm package
pnpm build:files
pnpm test
```

Packaging produces `dist/website-main.wasm`. The ordinary-file build produces `dist/site/`; choose a new `-output` directory when the destination already exists. For artifact tests, set `WEBSITE_SITE_OUTPUT` to that directory, `WEBSITE_PUBLIC_OUTPUT` to its `public` subdirectory, and `WEBSITE_API_ORIGIN` to a running local backend. Browser tests use `hack/tests/playwright.config.ts`; core local integration cases are `TC156`, `TC157`, `TC100` and `TC133`.

Upload, install and enable through existing plugin management, then select the template in website configuration. Shared branding, previous template settings and business data are retained. Project development uses the existing `make dev` supervisor; see [deployment](DEPLOYMENT.md) for independent hosting.

Local verification and branch decisions are recorded under repository `openspec/changes/integrate-website-main-current-commerce/`. Historical files in `docs/` describe the source branch and another environment; they are reference material, not proof of local runtime or verification. This integration guide takes precedence. Original licenses, attribution and provenance remain in [source and license](docs/source-and-license.md).
