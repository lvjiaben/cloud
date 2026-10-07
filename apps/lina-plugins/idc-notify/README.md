# idc-notify

`idc-notify` is the built-in source plugin for IDC notifications.

English | [简体中文](README.zh-CN.md)

## Implemented Capabilities

- PostgreSQL-backed event catalog, bilingual immutable templates, event policies, delivery tasks, attempts, provider-result queries, safe retry, manual review, manual resend, callback evidence, and recoverable channel-secret changes.
- Plugin-owned administrator, customer-account, staff, callback, and public presales APIs. All routes stay under the `idc-notify` plugin prefix.
- A plugin-owned management frontend for channels, templates, event policies, and delivery records. No IDC business page is placed in `web-antd`.
- Typed adapters for TLS-only SMTP, Tencent Cloud SMS, WeChat Official Account, WeCom custom applications, and signed Webhooks. DingTalk and Feishu remain adapter extension points only.
- WeCom self-service employee binding through official OAuth or QR authorization. An administrator cannot type, guess, or bind an employee `UserID` on someone else's behalf.
- Shared-service integration for background in-app delivery and server-side human verification.
- Anonymous presales verification through email or SMS, with one-time human proofs, six-digit codes, PostgreSQL rate and cost limits, five-attempt locking, ten-minute expiry, one-minute resend spacing, and crash-safe provider-call evidence.
- Anonymous presales ticket creation and access renewal through the authoritative support capability. The access token is returned only to the current browser and is never stored by this plugin. A second access-link delivery uses only the explicitly configured `LINAPRO_IDC_NOTIFY_PUBLIC_CUSTOMER_ORIGIN`.

The primary implementation areas are [database migrations](manifest/sql/), the [backend](backend/), the [plugin frontend](frontend/), and the [notification-center E2E suite](hack/tests/e2e/TC001-notification-center.ts).

## Safety Boundaries

- Notification administration requires the plugin's dedicated permissions. Secret changes, lifecycle actions, real test sends, manual closure, and resend also enforce their recent-login, version, confirmation, and current-access checks.
- Secrets are stored through `secretcap` and never returned in full. The verification HMAC key is generated once inside the secret boundary and can only be used through the `notify.verify` callback purpose.
- Provider calls are prepared durably before the call. A timeout, broken connection, or crash after preparation is recorded as an unknown result and is never sent again automatically.
- Public verification stores only purpose-separated digests, masks, counters, and status. It does not store the email address, phone number, code, client IP, browser nonce, human-proof token, or anonymous ticket access token.
- SMTP accepts only deployment-approved endpoints and certificate-verified port `465` implicit TLS or port `587` STARTTLS. Signed Webhooks accept public HTTPS only and reject redirects, private or reserved addresses, and unsafe DNS changes.
- WeChat Official Account stays disabled until the encrypted callback self-check succeeds for the current configuration generation.

## Deployment Configuration

- Email and SMS verification require an enabled provider channel and an active local template for the selected event, channel, language, and version. Missing provider configuration fails closed.
- `LINAPRO_IDC_NOTIFY_SMS_VERIFICATION_DAILY_LIMIT` is optional. When absent, the installation-wide SMS verification limit is `1000` per day. If it is present but empty, invalid, or outside `1..100000`, startup fails.
- `LINAPRO_IDC_NOTIFY_PUBLIC_CUSTOMER_ORIGIN` is optional. Production accepts only a clean public HTTPS origin; local development may explicitly use a loopback HTTP origin. When it is missing or invalid, ticket creation and renewal can still return the current-browser token, but link delivery is marked unavailable and no link is sent.

## Dependencies and Delivery

The plugin uses only published capabilities from `idc-core` and `shared-services`; it does not import their `internal` packages. Version `v0.1.0` is delivered as `source + builtin`, with `platform_only`, `supports_multi_tenant: false`, and `global` governance.

## Still Incomplete

This README does not claim that all of Phase 11 is complete:

- DingTalk and Feishu still need real adapters before they can send.
- Real external-provider credentials, public callback reachability, SMTP allowlists, SMS charges, OAuth trusted domains, and browser acceptance must still be verified in the target deployment.
- A mocked provider or a stored configuration is evidence that the integration path works; it is not evidence that a real external message was delivered.

Affected operations remain closed when a required capability or provider configuration is unavailable.
