# idc-portal

`idc-portal` owns website-package management, code-free brand revisions, public and customer API assembly, customer-entry routing, cart handoff, and authentication bridging for the `IDC` operations platform. It does not own the public website or customer-center application source code.

English | [简体中文](README.zh-CN.md)

## Scope

- Publishes the built-in `website-template` business extension and keeps managed website-package delivery within this plugin boundary.
- Keeps the only hard plugin dependency on `idc-core`.
- Keeps payment, resource, compliance, notification, channel, and `AI` integrations behind runtime capability boundaries.
- Embeds the plugin manifest and its small governed brand-management page; it does not embed editable website or customer-center application source code.
- Exposes the published brand projection at `GET /x/idc-portal/api/v1/public/brand`; administrators stage, preview, publish, and roll back immutable brand revisions through protected routes.
- Registers only the exact `GET /admin/auth/reset-password` bridge needed by existing host password-reset emails.

The public website source is owned by `apps/lina-websites/`. Authenticated customer pages are owned by `apps/lina-vben/apps/web-customer/`. The brand management page owned by this plugin lives in `frontend/dist/` and is loaded through the plugin frontend mechanism.

## Password-reset bridge

The bridge reads the customer application origin only from `LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN`. Production origins must use `HTTPS`; explicit loopback `HTTP` origins are allowed for local development. The handler redirects with status `302` to `/auth/reset-password`, preserves only one non-empty `token` query value, and ignores request host, referrer, and unrelated query parameters. All other `/admin/*` paths remain owned by the LinaPro management application.

## Delivery

Website-package lifecycle and public or customer API assembly remain backend responsibilities of this plugin boundary; retiring the old frontend does not move them into either frontend application. Public website rendering comes from the active validated package. Customer authentication, cart, checkout, account, billing, compliance, and support pages come from `@lina/web-customer`; this plugin no longer builds, embeds, or serves the retired `portal/` single-page application.

## Dependencies

The only hard plugin dependency is `idc-core`. Missing optional runtime capabilities fail only their own journey closed and do not change ownership of authoritative business data.
