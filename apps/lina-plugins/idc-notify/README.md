# idc-notify

`idc-notify` is the built-in source plugin for IDC notifications.

English | [简体中文](README.zh-CN.md)

## Implemented Capabilities

- PostgreSQL-backed event catalog, bilingual immutable templates, event policies, delivery tasks, attempts, provider-result queries, safe retry, manual review, manual resend, and recoverable channel-secret changes.
- Plugin-owned administrator, account-security, and public presales APIs. All routes stay under the `idc-notify` plugin prefix.
- A plugin-owned management frontend for channels, templates, event policies, and delivery records. No IDC business page is placed in `web-antd`.
- Typed adapters for TLS-only SMTP, Aliyun SMS, SMSBao, and Tencent Cloud SMS. External notifications support email and SMS only.
- Shared-service integration for background in-app delivery and server-side human verification.
- Anonymous presales verification through email or SMS, with one-time human proofs, six-digit codes, PostgreSQL rate and cost limits, five-attempt locking, ten-minute expiry, one-minute resend spacing, and crash-safe provider-call evidence.
- Anonymous presales ticket creation and access renewal through the authoritative support capability. The access token is returned only to the current browser and is never stored by this plugin. A second access-link delivery uses only the explicitly configured `LINAPRO_IDC_NOTIFY_PUBLIC_CUSTOMER_ORIGIN`.

The primary implementation areas are [database migrations](manifest/sql/), the [backend](backend/), the [plugin frontend](frontend/), and the [notification-center E2E suite](hack/tests/e2e/TC001-notification-center.ts).

## Safety Boundaries

- Notification administration requires the plugin's dedicated permissions. Secret changes, lifecycle actions, real test sends, manual closure, and resend also enforce their recent-login, version, confirmation, and current-access checks.
- Secrets are stored through `secretcap` and never returned in full. The verification HMAC key is generated once inside the secret boundary and can only be used through the `notify.verify` callback purpose.
- Provider calls are prepared durably before the call. A timeout, broken connection, or crash after preparation is recorded as an unknown result and is never sent again automatically.
- Public verification stores only purpose-separated digests, masks, counters, and status. It does not store the email address, phone number, code, client IP, browser nonce, human-proof token, or anonymous ticket access token.
- SMTP accepts only deployment-approved endpoints and certificate-verified port `465` implicit TLS or port `587` STARTTLS.

## Deployment Configuration

- Email and SMS verification require an enabled provider channel and an active local template for the selected event, channel, language, and version. Missing provider configuration fails closed.
- `LINAPRO_IDC_NOTIFY_SMS_VERIFICATION_DAILY_LIMIT` is optional. When absent, the installation-wide SMS verification limit is `1000` per day. If it is present but empty, invalid, or outside `1..100000`, startup fails.
- `LINAPRO_IDC_NOTIFY_PUBLIC_CUSTOMER_ORIGIN` is optional. Production accepts only a clean public HTTPS origin; local development may explicitly use a loopback HTTP origin. When it is missing or invalid, ticket creation and renewal can still return the current-browser token, but link delivery is marked unavailable and no link is sent.

## Dependencies and Delivery

The plugin uses only published capabilities from `idc-core` and `shared-services`; it does not import their `internal` packages. Version `v0.1.0` is delivered as `source + builtin`, with `platform_only`, `supports_multi_tenant: false`, and `global` governance.

## Still Incomplete

This README does not claim that all of Phase 11 is complete:

- Real external-provider credentials, SMTP allowlists, SMS charges, and browser acceptance must still be verified in the target deployment.
- A mocked provider or a stored configuration is evidence that the integration path works; it is not evidence that a real external message was delivered.

Affected operations remain closed when a required capability or provider configuration is unavailable.

## Customer contact verification

Authenticated customers can request a code at `POST /x/idc-notify/api/v1/account/security/contact-codes` and verify it at `POST /x/idc-notify/api/v1/account/security/contact-codes/{verificationNo}/verification`. The server reads the current email or phone through the authenticated public self-profile endpoint. First-time binding verifies the other existing contact. After both old and new proofs pass, the customer template saves automatically through the existing profile route; customers who cannot verify use the existing support-ticket page.

The matching `idc-core >=0.1.162` guard and `010-login-contact-change.sql` are required. Account locks and proof consumption share the original profile-write transaction. Configure an enabled email/SMS channel and active templates for `account.login_contact_verification_code`. Default SMS templates target SMSBao; Aliyun and Tencent Cloud require the matching approved provider template and mapping. Codes expire after ten minutes, allow five failed attempts, and use persistent sending limits. This feature does not change the account's independent username or add phone/email password-login aliases.

## Accessory renewal notifications

Version `v0.1.10` adds `012-accessory-renewal-notifications.sql` with three billing events. The migration only seeds absent catalog, policy, and template rows; rerunning it preserves administrator changes, including disabled templates.

| Event | Recipient | Default channels |
| --- | --- | --- |
| `billing.accessory_renewal_due` | `customer_contact` | `email` |
| `billing.accessory_renewal_failed` | `customer_contact` | `email` |
| `billing.accessory_renewal_attention` | `staff_user` | `in_app` |

All three use the `billing` business object. Due reminders require `invoice_no`, `amount`, `currency`, `tax_amount`, `period_unit`, and `period_count`. The business owner supplies frozen tax-inclusive totals and tax components as ISO currency major-unit strings, with `period_unit` set to `day`, `month`, or `year` and `period_count` a decimal integer. Bilingual templates show the full period and explicitly describe an unpaid invoice, without claiming a successful charge. Chinese period units are localized only in the rendering copy (for example, `12个月`); frozen event and delivery variables retain `month` and the original amounts. Failed-renewal notices require `invoice_no` and explain retained data, paused usage, payment recovery, and the customer's option to delete the object.

Staff attention requires `pending_count` and the latest `invoice_no`; optional `locale_code` selects `zh-CN` or `en-US`, defaulting to `en-US`. It directs authorized staff to the renewal worklist. Staff materialization accepts only this `in_app` event from `idc-core`, using the trusted staff recipient; enabling an external channel policy does not authorize staff delivery. This seed does not grant staff permissions or resolve staff contact addresses. Other supported policy channels remain disabled; employee WeCom is not restored. Email also requires an enabled and configured channel. Migration, rendering, and materialization tests queue isolated delivery records without sending external email.


### Email and SMS notifications

Version `v0.1.11` adds migration `013-email-sms-notifications.sql` and removes retired customer channels and their policies. Existing delivery evidence is retained; pending provider calls are moved to manual review without inventing successful delivery. The private employee renewal inbox remains available. Customer `/account/notifications` APIs and the notification-preferences page have been removed.
