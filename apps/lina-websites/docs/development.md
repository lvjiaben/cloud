# Third-party website development

## What you need

Use an independent repository with published versions of `@lina/site-contracts`, `@lina/site-sdk`, and `templates/nuxt-starter/`. You do not need the LinaPro administration app, customer app, or plugin backend source.

The first supported package type is a precompiled Nuxt Node website. Developers control public pages and visual design while following the package manifest, public API, route, security, bilingual, and SEO rules.

## Allowed scope

- Home, products, news, announcements, knowledge base, downloads, contact, and public agreements.
- Versioned public projections for published brand, content, catalog, and prices.
- Add-to-cart initiation followed by an opaque server-issued handoff to the customer app.
- Chinese and English server-rendered HTML, canonical and alternate links, Open Graph, sitemap, and applicable structured data.

## Forbidden scope

- Never own `/admin`, `/auth`, `/account`, `/cart`, `/checkout`, `/api`, `/x`, or `/x-assets`.
- Never call administration or customer-private APIs, databases, providers, or installation secrets.
- Never persist authoritative prices, customer numbers, cart snapshots, orders, or payment results in URLs, cookies, or browser storage.
- Never ship install scripts or require `npm install` or online downloads after upload.

## Local development

1. Copy `templates/nuxt-starter/` to an independent repository.
2. Replace both `workspace:*` dependencies with published pinned versions.
3. Point `NUXT_API_ORIGIN` at a local LinaPro backend.
4. Run type, SSR, no-JavaScript, bilingual, SEO, 404, and route-isolation checks.
5. Use the official packer to create the manifest and `.zip` from Nuxt `.output`. The production packer and administration installer are not implemented yet; the example manifest is not a production package.

## Publication

The merchant uploads the `.zip` in its own administration app. The local installation validates, stages, starts with low privilege, health-checks, and previews the candidate. Publication atomically changes the active pointer; failure keeps the previous version available for rollback. LinaPro is not rebuilt, and traffic never depends on the software vendor's business gateway.
