# TDesign Customer Center

[中文说明](README.zh-CN.md)

An independent, Chinese customer-center template built with React and TDesign React. It registers the `customer-template/v1` extension of `idc-portal`. It does not replace `customer-default` or import host frontend source.

## Features

- Merchant-owned navigation loaded atomically, grouped by customer tasks, with only merchant-defined product entries and a mobile drawer.
- Login, verified email registration, password reset, first-login password changes, password changes after checking the current password, and isolated session recovery.
- Website handoff for purchasing and cart; orders under fees, recharge, payment preparation, balance and credit payment, browser payment handoff and authoritative payment-result queries.
- Purchased services, supported native-server actions, explicit credential reveal, renewal quotations, invoices and transactions.
- Tickets, attachments, replies, closure, reopening and feedback.
- Actual user avatar and avatar upload, nickname editing and login-contact status with account-security entry points, notification contacts, addresses, public keys, personal and enterprise verification, invoice profiles, tax invoices, refunds, credit applications and repayments, notification history, preferences, quiet hours and referral records.
- Account export and closure requests, with server-owned authorization, recent-login and retention checks.

Optional features depend on the corresponding installed business plugins. This template renders its declared routes; arbitrary third-party customer page renderers require a matching template adapter. A visible operation is never a substitute for backend authorization.

## Build and verify

Requirements: Node.js 20.19+ or 22.12+, pnpm 10.30.3 and Go 1.25+ with WASI reactor support.

```sh
pnpm install --ignore-workspace --frozen-lockfile
pnpm test
pnpm run build
GOWORK=off go test ./... -count=1
pnpm run package
node scripts/verify-package.mjs
pnpm run test:e2e
```

The package is `dist/customer-tdesign.wasm`. The verifier initializes it in an isolated WASI runtime, checks descriptor dispatch and rejection, and verifies embedded frontend assets. Browser tests use controlled API fixtures; they do not charge real money or operate external servers.

```sh
CUSTOMER_API_ORIGIN=http://127.0.0.1:6032 pnpm run dev
```

Development listens on `127.0.0.1:16742`. Production assets use same-origin public APIs. No credentials or merchant payment configuration belong in this plugin.

## Installation

Upload the artifact through the deployment's normal plugin installation flow, satisfy its declared `idc-portal` dependency, then enable it. Select the new template in the existing customer-center template settings when ready. The local instance has installed and enabled this package through the normal lifecycle while retaining its active template selection. The previous template remains available for rollback.

## Website and avatar integration

Set `websiteurl` to the website origin and configure `websitecatalogpath`, `websiteconfigurepath` and `websitecartpath`. The configuration path uses `{product}`. The website must implement `POST /__website/session` with customer-origin checks; `/auth/website-session` supplies the origin-restricted login bridge. Missing settings show a recoverable error. Isolated previews can inject these public settings using `CUSTOMER_WEBSITE_CONFIG`.

Avatar changes use the customer-owned `idc-core` endpoint `POST /account/customer/avatar-uploads`, followed by the existing self-profile update API. Deploy the matching business-plugin backend; customers do not receive backoffice file permissions.

## Verification limits

Local real API reads and isolated browser workflows cover the implemented pages. A dedicated local test account also verifies profile saves; address, contact and invoice-title creation, editing and removal; and signing in after a password change. External payment settlement, supplier actions, identity providers, tax issuance require acceptance in the merchant's configured test environment. Browser payment supports web redirects, form posts and QR codes; native app and WeChat JSAPI payment modes need an environment-specific adapter. Arbitrary third-party page embedding still requires a dedicated adapter.

## Sources and licensing

The UI uses the official [TDesign React](https://github.com/Tencent/tdesign-react) component library under MIT. It does not use the legacy TDesign Starter. The [Magic Cube reference](https://github.com/aazooo/zjmf-manger-decoded) was inspected only to understand merchant menus, product association and sales categories; no code or private account data was copied from it. This plugin and its public API DTO adapters follow the project's Apache-2.0 license in `LICENSE`. Runtime dependency notices are in `THIRD-PARTY-NOTICES.txt`; both are included in the published assets.

## Login-contact binding status

Phone and email are no longer written through ordinary profile editing. The matching `idc-core` middleware authenticates the caller and rejects customer contact changes that bypass verification, while preserving full staff-profile submissions, nicknames, avatars and the existing password flow. Deploy and enable the matching business-plugin backend for this protection.

The self-service flow verifies the existing contact, verifies the new contact, and saves automatically without staff approval. The server selects the existing contact. First-time binding verifies the other existing contact. Customers who cannot receive a code or have neither contact use the existing support-ticket page.

The matching `idc-core` and `idc-notify` plugins wrap the existing public profile route without modifying host sources or accessing private host tables. Proofs bind the account, session, field and original profile, expire after ten minutes, limit failed attempts and cannot be replayed after saving. Profile writes and proof consumption share one transaction. Deploy notification migration `010-login-contact-change.sql` and configure working email/SMS channels and matching verification templates.

## Resource workspace

The overview displays the authenticated account ID and at most five recent resources. Merchant-defined product tabs provide server-side search and independent, versioned customer notes. Runtime status, IP addresses and operating systems come from one bounded local resource-fact request per page; missing observations remain unknown. The fee column shows the current effective price record, not a cash receipt or a renewal quote.

The management page groups instance information, systems and connections, snapshots and backups, network security, operation history and fees. It uses resource-owned capabilities, exact choices, current-session confirmation and durable operation status. Passwords and console output are revealed only by an explicit one-time request and disappear when their dialog closes. Rescue uses a customer-entered temporary password and never invents a generated password. Commercial operations require the matching resource and core implementations; a frontend build alone does not confirm their availability. Deploy core migration `150-customer-service-workspace.sql` through the normal plugin lifecycle with the matching declared dependencies.

Commercial actions use separate quote eligibility, authoritative final inclusive prices, explicit confirmation and actual payment documents. Configuration changes reevaluate the current entitlement's complete typed fields and freeze a dedicated target before calculating the difference; they do not change the website cart. Persisted commercial history and verified-object billing periods use bounded cursor pages, including suspended services. Unknown object readiness never enables restoration.

The inbox synchronizes the current customer's real host deliveries and renders bodies as plain text. Login and visible/focus events refresh reminders with a thirty-second bound; there is no continuous polling. Same-session refresh failures retain the last confirmed unread count, while identity changes clear it. Sending status is shown separately from actual message content.

Provider limits are deliberate: the current Magic Cube adapter cannot prove that its IP-count update performs an address replacement, so it does not offer that action. Snapshot/backup restoration remains unavailable when upstream readiness cannot be verified. Monitoring and boot-order writes require a verified provider contract and are not represented by fake controls.
