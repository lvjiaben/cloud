# Default Public Website

This Nuxt application owns the public homepage, product catalog and configuration display, news, announcements, knowledge base, downloads, and help pages.

Authentication, the customer account, cart, checkout, orders, finance, and verification are owned by the separate Vben `@lina/web-customer` application. Website links use the deployed `NUXT_PUBLIC_CUSTOMER_URL`; this package does not render private customer routes or call customer-only APIs.
