# Default Public Website

This Nuxt application owns the public homepage, product catalog and configuration display, news, announcements, knowledge base, downloads, and help pages.

Authentication, the customer account, cart, checkout, orders, finance, and verification are owned by the separate Vben `@lina/web-customer` application. Website links use the deployed `NUXT_PUBLIC_CUSTOMER_URL`; this package does not render private customer routes or call customer-only APIs.

Development and production proxy anonymous website APIs to `NUXT_API_ORIGIN` (default `http://127.0.0.1:6030`). Configure this origin when the backend uses another port. Administrative APIs are excluded; see [Public website APIs](../../docs/api.md).

Install and enable `website-default` or `website-minimal`, then select and configure it in System settings → Website configuration. The two plugins use different built-in layouts and the active common information and template fields.
