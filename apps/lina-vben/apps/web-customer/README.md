# LinaPro Customer Center

`@lina/web-customer` is the independent customer-facing Vben application. It is a sibling of `@lina/web-antd`, not a page inside the administration workspace and not a Nuxt website.

The application uses `apps/lina-vben/apps/web-antd/` as its mother template. Its Vben, Vue 3, Vite, Ant Design Vue, `src/` layout, bootstrap, router, request client, stores, preferences, theme, layouts, locales, components, page containers, type checks, and build style must remain aligned with `web-antd`.

Only the package and application identity, development port `6032`, storage namespace, customer routes, customer APIs, branding values, customer pages, and output boundary may differ. Run `pnpm check:alignment` in this directory before accepting structural changes.

Public websites do not live here. Website development, the default Nuxt website, SDK, contracts, starter template, documentation, and installable website packages belong to `apps/lina-websites/`.
