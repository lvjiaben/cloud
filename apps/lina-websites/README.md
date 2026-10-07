# Lina Website Packages

This workspace is independent from `apps/lina-vben/`. It contains the default public website, stable website contracts, SDK, starter template, and bilingual documentation used to build installable website packages.

The customer center remains `apps/lina-vben/apps/web-customer/` and follows the Vben structure of `web-antd`. A website package only owns public acquisition and content routes. Cart, authentication, checkout, payment, account, and after-sales routes are handed to the customer center through server-authoritative opaque references.

Website developers build a self-contained package and deliver a precompiled `.zip`. An IDC merchant uploads the package in its own administration workspace. The local installation validates, stages, previews, publishes, switches, and rolls back the package without recompiling LinaPro.

The current `sites/default/` source was preserved from the earlier combined Nuxt prototype. It is a migration baseline, not proof that the final website package contract or customer-page split is complete.

Start here:

- [Third-party development](docs/development.md)
- [Public APIs](docs/api.md)
- [Archive format](docs/package-format.md)
- [Nuxt starter](templates/nuxt-starter/README.md)
