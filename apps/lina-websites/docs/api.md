# Public website APIs

## Available now

The anonymous catalog already exists under the same-origin prefix `/x/idc-core/api/v1`:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/public/catalog/products` | Bounded page of published products |
| `GET` | `/public/catalog/products/{publicSlug}` | One published product |
| `GET` | `/public/catalog/products/{publicSlug}/configuration` | Current configuration graph |
| `POST` | `/public/catalog/products/{publicSlug}/configuration-validations` | Server validation of a complete selection |
| `POST` | `/public/catalog/products/{publicSlug}/configuration-quotes` | Current authoritative quote |

Use the returned currency and minor-unit strings. Timestamps are Unix milliseconds. Website requests need a timeout and server-side failure handling; an API failure must never produce a guessed price.

The published website brand projection is available under the portal plugin:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/x/idc-portal/api/v1/public/brand` | Published bilingual site name, logo, color, navigation, footer and SEO projection |

Administrators stage and publish code-free brand revisions through protected portal routes. Revisions are immutable and the last published revision can be rolled back.

## Not implemented yet

Navigation, news, announcement, knowledge-base, download, agreement, SEO content projection, and cart-handoff APIs remain Phase 15 work. They will enter `site-sdk` only after backend DTOs, permissions, cache rules, and tests exist. Developers must not invent paths or present hard-coded data as authoritative.
