# QYCN-style website template

An independent bilingual, platform-scoped, managed dynamic website plugin. Its layout follows the [requested reference](https://www.qycn.com/index.html): campaign carousel, backend-defined product categories, product cards, benefits, regional nodes, partner presentation, news and footer. It does not change another template or the host.

## Build and preview

Requirements: `Node.js >= 22`, `pnpm 10.30.3`, and `Go >= 1.25`. Run from this directory, including when copied outside the repository:

```console
pnpm install --frozen-lockfile
pnpm build
pnpm dev
pnpm test
pnpm exec playwright install chromium
pnpm test:e2e
pnpm package
```

Preview defaults to `http://127.0.0.1:16880`. Set environment variables in your process manager or shell:

| Variable                           | Default                   | Purpose                                                                      |
| ---------------------------------- | ------------------------- | ---------------------------------------------------------------------------- |
| `PORT`                             | `16880`                   | Local preview port                                                           |
| `HOST`                             | `127.0.0.1`               | Preview bind address                                                         |
| `WEBSITE_API_ORIGIN`               | `http://127.0.0.1:6030`   | Existing backend public API origin                                           |
| `WEBSITE_CUSTOMER_URL`             | `http://localhost:6032`   | Customer application origin                                                  |
| `WEBSITE_CUSTOMER_ALLOWED_ORIGINS` | `[]`                      | Additional explicitly trusted customer origins as a JSON array; no wildcards |
| `WEBSITE_PROXY_IDLE_TIMEOUT_MS`    | `9000`                    | Upstream idle timeout in milliseconds for local preview                      |
| `E2E_OUTPUT_DIR`                   | Repository `temp/<date>/` | Optional external test evidence directory                                    |

The preview server is a development tool, not a production gateway. It proxies method-scoped public and purchase contracts to fixed upstreams; only account purchase routes forward authorization. `frontend/dist/` contains 14 actual HTML pages and local assets. `dist/website-qycn.wasm` contains the dynamic guest, manifest, and public assets. There is no SSR service and no private repository dependency.

## Install

Upload the WASM using the existing plugin installation interface, satisfy the declared `idc-portal` and `idc-core` dependencies, install and enable it, then select **群英风格官网** in Website Configuration. Configure shared brand, logo, company, contacts, ICP/public-security registration numbers and lookup links, and navigation there. The template declares optional Chinese/English hero text, a background image, and Chinese/English partner names (one per line). Template settings are applied only when this template is selected. Switching back to a previous template preserves its settings.

The existing website gateway serves the selected plugin assets and injects `window.__LINA_TEMPLATE__.customerUrl`. This plugin does not register admin menus, alter authentication, or change the active template itself. Static navigation uses the declared `.html` pages; article details use `article.html?kind=news&id=<contentNo>`.

## Data boundaries

| Content                                   | Source and behavior                                                                                                                                 |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| Brand, header/footer navigation           | `GET /x/idc-portal/api/v1/public/brand`; nonempty configured navigation takes priority; empty navigation uses template menus and catalog categories |
| Products                                  | `GET /x/idc-core/api/v1/public/catalog/products`; bounded cursor pagination, server-side `namePrefix` and `categoryNo` filters                      |
| Product configuration                     | Website `/configure.html?product=<publicSlug>`; website cart `/cart.html`; invoice payment in the customer center                                   |
| News, announcements, knowledge, downloads | `GET /x/idc-core/api/v1/public/content/items?type=...`; detail via `/items/<contentNo>`                                                             |
| Download                                  | `POST /x/idc-core/api/v1/public/content/items/<contentNo>/download-tickets`, then `GET /x/idc-core/api/v1/public/content/downloads/<token>`         |
| Sign in / registration                    | Configured customer application `/auth/login` and `/auth/register`                                                                                  |
| Regional nodes and company introduction   | Explicitly labelled static examples; partner names are configurable and product prices come from server quotes                                      |

Catalog responses do not provide authoritative final prices. Real product cards therefore show configuration/quotation links rather than fabricated prices. Downloads validate the file response before saving. Browser transfers use a 30-second idle timeout with no fixed total deadline while data continues; file errors, interruptions and stalls remain retryable on the article page. Files are prepared as browser `Blob` objects without an additional business size limit. API failures show retry controls; an empty or failed real list is never replaced by fake records. Contact details are never fabricated. Published article bodies are rendered as text, not executable HTML. The plugin has no database tables or private source dependency. Session tokens remain in page memory only, and bilingual resources ship with the plugin.

## Maintenance and evidence

`frontend/locales/zh-CN.js` owns static presentation content; `frontend/views.js` owns markup; `frontend/app.js` owns interactions and API projections; `frontend/style.css` owns the isolated website document styles. `scripts/build.mjs` generates page documents from the actual page declarations. `scripts/package.mjs` validates every declared page before packaging.

`TC003` covers file-stage failures, offline recovery, ticket validation, anonymous downloads and duplicate-submit protection; controlled-clock tests distinguish long transfers from inactivity. `TC001` covers desktop/mobile presentation and routes; `TC002` covers data, cursor/filter parameters, empty navigation, unsafe links, errors and download recovery. Test fixtures stay in `hack/tests/` and are not packaged. Node delivery tests exercise the actual preview download proxy, anonymous request forwarding and stale-asset removal. Go descriptor tests validate the read-only bridge identity and malformed invocations. Run repository Go lint through `make lint dir=apps/lina-plugins/website-qycn plugins=1` when working in the main repository.

The descriptor guest and public WASM packager were adapted from this project's `website-html` example and are maintained locally. The hero, icons, typography and interactions are implemented locally. Two decorative reference images were downloaded on `2026-10-22` and included locally: `frontend/media/world-map.jpg` from `https://www.qycn.com/home/img/map_dark.jpg`, and `frontend/media/city.jpg` from `https://www.qycn.com/home/img/why_huicui.jpg`. No third-party tracking scripts, contact data, logos, credentials or remote runtime dependencies are included.

## Bilingual catalog and website checkout

The template now provides 14 pages with Chinese/English selection persisted across navigation. Product categories and products come from the published catalog; no fixed demo products or prices are used. Configuration is hosted at `/configure.html?product=<publicSlug>` and the cart at `/cart.html`. The website evaluates configuration, requires sign-in before adding account cart items, merges the guest cart after sign-in, obtains server quotes, and confirms orders with stable idempotency keys. Invoice payment remains in the configured customer center.

Shared website settings in `idc-portal` own the logo, site and company names, ICP and public-security registration numbers and links. Run the updated backend to persist the new registration fields. Empty company/registration values are not invented. Template fields include Chinese/English hero text, background image and `partners`/`partnersen` (one partner name per line, hidden when empty). Public catalog prices are not assumed; configuration quotes provide authoritative price previews.

The trusted gateway injects `window.__LINA_TEMPLATE__.customerUrl`. The client verifies the configured origin, iframe source and random nonce for `/auth/website-session`. Customer templates must allow only the administratively configured website origin. Sign-in handoff uses `POST /__website/session` with `accessToken` and `returnTo`; the local adapter accepts only its primary customer origin, explicitly configured additional origins, and the four purchase/catalog HTML routes. One-use handoffs last 30 seconds, keep credentials in server memory and use an HTTP-only opaque browser handle. The production gateway must implement the same protocol; these are not automatic host endpoints. Tokens remain in page memory, never URLs or browser persistent storage. Draft configuration is not pricing authority.

The development proxy scopes methods, paths, guest cookies and account authorization to a fixed backend. Anonymous display/download requests do not forward credentials. Authenticated file fields use governed upload APIs; the local proxy allows up to 64 MiB per upload, while production follows owner policies and gateway limits.

`TC004` verifies bilingual navigation, configured brand/company/registration/partners and live categories including failures. `TC005` verifies configuration, account cart, historical guest merge, quoting and idempotent order retries. Node/Go delivery tests and the existing navigation/download regressions remain required. The template remains independently buildable with Node >=22, pnpm 10.30.3 and Go >=1.25 using `pnpm install --frozen-lockfile`, `pnpm test`, `pnpm test:e2e`, and `pnpm package`.

The carousel rotates every 6.5 seconds by default, resets its timer after manual selection, without play/pause controls. Reduced-motion preferences disable animation rather than rotation; keyboard focus within slide content and background tabs pause advancement. `TC006` covers empty navigation, response ordering, failure recovery, automatic rotation and keyboard-focus protection, and mobile/small-desktop English navigation.

Before brand data arrives, the page name, logo and favicon contain no sample brand. Configuration, billing periods and totals appear together; defaults and edits automatically evaluate and quote through the server. Each visible numeric or enumerated specification starts with its first sellable choice; continuous numbers start at their declared minimum, including zero. Existing valid selections and restored cart configurations are retained. Cleared numbers are omitted only while editing; server evaluation restores the first value and obtains a fresh quote. There is no synthetic package-default choice. New edits cancel stale requests; failures show retry controls and purchasing stays disabled until a fresh quote is ready. Mobile layouts pin the total and add-to-cart action at the bottom. `TC007` covers delayed branding, automatic pricing, slow session lookup, rapid edits, failure recovery and bilingual mobile layouts.

Configuration evaluation and quoting run concurrently with a 120ms input debounce; repeated input/blur events for unchanged selections do not issue duplicate requests. A speculative quote is used only when evaluation is valid and the revision and values match. Server-normalized configurations receive a fresh quote; transaction prices are not cached. `TC008` verifies concurrent startup, deduplication, normalized values and invalid-configuration protection.

The configuration sidebar reads live categories and products from the public catalog, with category filtering, pagination, retry, localization and a mobile layout. Catalog loading does not block configuration or quoting; `TC009` covers slow catalogs, recovery states and mobile rendering. Pricing uses published backend data and resource candidates use database snapshots. Purchase admission and prices are checked again at purchase time.

On mobile configuration pages, the back-to-top button tracks the actual purchase-bar height, including status text and device safe areas, to prevent overlapping actions. `TC009` verifies scrolling, an unchanged cart and normal add-to-cart behavior on narrow screens.

The cart follows the supplied Xiaobaiyun references with configuration cards, quantity controls, atomic item editing/removal, a fixed total/promotion bar and checkout details. Add-to-cart requires authentication before any write; safe return restores configuration/currency/term without auto-submitting. Lost write responses retry the original cart version to prevent duplicate additions. Requires `idc-core >=0.1.167` for owned atomic updates and bounded persisted price-choice/period projections. Updates invalidate the previous quote; all amounts come from server pricing. Local drafts supplement configuration display only. Coupons use current account assets and owner rules; checkout still covers the complete cart.

Checkout details read same-currency balance and available browser payment channels from published account APIs. Insufficient balance cannot be selected. Preferences pass to the customer invoice for fresh validation and explicit confirmation; the website never starts payment. When channels are unavailable, the invoice offers the remaining choice without invented providers. `TC010` covers login and restoration, item details, quantity retry, coupons, payment preference, English mobile layout, lost responses and editing the original item.

`TC011` verifies bilingual radio/select/slider rendering, first-choice initialization, typed values and summaries, zero, clearing and normalization, disabled/hidden fields, period availability and price-failure recovery on mobile.

The add-to-cart flow reuses the authoritative cart returned by open and add writes. Cart loading reuses the current cart returned by merge, including an explicit null for an empty cart; only older owners omitting the cart field need a compatibility read. Ordinary quote creation advances to the returned cart version without reloading it; legacy configuration freezing still reloads the final quote. Coupon lists, balances and payment channels load independently of checkout availability and can recover separately. The invoice still validates payment choices and requires confirmation; no transaction prices or authorization decisions are cached. `TC010g–j` verify request counts, post-quote optimistic versions, gated optional reads, retry and the necessary legacy reload.

A still-valid quote restored on cart load also freezes required configurations from this page’s saved drafts before proceeding, then reads the authoritative cart once. It does not create a replacement quote; already-frozen quotes cause no extra reads, and missing drafts still require configuration. `TC010k` covers restoration, missing drafts and freeze failure recovery.
