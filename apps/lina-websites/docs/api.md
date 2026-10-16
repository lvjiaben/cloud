# Public website APIs

The default Nuxt website uses a restricted same-origin `/x/` proxy targeting `NUXT_API_ORIGIN` (default `http://127.0.0.1:6030`). Development and production share this proxy. Only HTTP(S) origins are accepted; administrative APIs are excluded.

| Plugin | Public path | Purpose |
| --- | --- | --- |
| `idc-portal` | `GET /api/v1/public/brand` | Active website common information, renderer and template fields |
| `idc-core` | `GET /api/v1/public/catalog/*` | Published products and purchase configuration |
| `idc-core` | `GET /api/v1/public/content/*` | Public content and download information |
| `idc-core` | `GET /api/v1/public/commerce/*` | Public cart and handoff information |

The complete brand URL is `/x/idc-portal/api/v1/public/brand`. Responses use the `{ code, message, data }` envelope, with brand content in `data.brand`.

Anonymous POST operations are limited to cart `carts` and `cart-items`; product `purchase-configuration/evaluate`, `purchase-configuration/quote` and `campaign-exposures`; and content `download-tickets`. Other write methods return 405; administrative and other plugin paths return 404. Authorization headers are not forwarded. Cart ownership remains controlled by the backend's protected cookie.

Administrators select an installed and enabled website plugin in System settings → Website configuration, then save common information and that template's own fields. Disabled plugins do not expose their configuration. Public pages do not cache the brand projection; refresh after saving to see changes. Legacy brand revision APIs remain available, but the public website reads only the new configuration.

The default website supports `public-standard-v1` and `public-minimal-v1`. Third-party layouts can publish a framework-neutral HTML entry through the template declaration and formal public_assets. The gateway uses the active release entry and supports SPA deep links; no frontend framework is required. Sign-in, purchase and account pages remain in the customer application. Use server-returned currencies and minor-unit strings; timestamps are Unix milliseconds. API failures must never produce guessed prices or hard-coded content presented as authoritative.
