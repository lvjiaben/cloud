# idc-portal

`idc-portal` manages website and customer-center templates, shared branding, template-specific settings, navigation, and the password-reset bridge. Frontend templates are independent installable plugins.

English | [简体中文](README.zh-CN.md)

## Scope

- Owns the `website-template` and `customer-template` extension points; its only hard plugin dependency is `idc-core`.
- Provides administrator configuration pages for installed templates, common branding, template settings, website navigation, and customer navigation.
- Publishes the active website configuration through `GET /x/idc-portal/api/v1/public/brand`; the projection reads the current site configuration. The retired separate brand revision store and its staging, publishing, and rollback APIs have been removed.
- Keeps customer external-menu configuration and public configuration APIs available to their existing consumers.

Website source and rendering assets live in `apps/lina-plugins/website-default/` and `apps/lina-plugins/website-minimal/`. The independent customer template lives in `apps/lina-plugins/customer-default/`. The retired `apps/lina-websites/` workspace has been removed. This plugin manages templates; it does not embed their applications.

See [development](../../../DEVELOPMENT.md) and [installation](../../../INSTALL.md) for template contracts, builds, and startup.

## Password-reset bridge

Only `GET /admin/auth/reset-password` is registered for existing password-reset emails. The customer origin comes from `LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN`; production requires `HTTPS`, while explicit loopback `HTTP` is permitted locally. The `302` redirect targets `/auth/reset-password`, preserves one non-empty `token`, and ignores request host, referrer, and unrelated parameters. Other `/admin/*` routes remain owned by the management application.
