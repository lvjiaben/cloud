# TC011 isolated peer protocol test

`TC011b` creates two randomly named local test databases, initializes host schema and required seed with the official initialization command, starts source plugins normally to initialize their required data, and runs two independent backends. It does not clone deployment metadata, customers, transactions, orders, or services, and does not modify the current development database. Success and failure both stop owned processes and remove owned databases and the temporary directory containing credentials.

The test uses the official build tool for the current `idc-peer` and a test-only provider. Public APIs install plugins, create customers, grant API access, save secrets, verify the connection, publish and import products, and pay with balance. The upstream one-time product uses normal manual delivery; downstream query completes without creating a service. The test-only ongoing provider reports accepted and completed states through the normal worker. Only the initial expiry of that upstream test entitlement is a controlled fixture; downstream mappings, debits, and renewals use the real public protocol and workers.

`TC011c` uses the same official initialization and cleanup facilities to create one isolated instance. It reproduces peer authorization loss through another plugin's official enable operation, then recovers the check by officially reenabling the already-enabled peer. It verifies stopped new-business admission, impact checks after list/detail reads, successful disable with no business, and refusal to disable with a published imported product. Connection, products, and import use public APIs; only customer identity and balanced recharge use the existing controlled fixture. No customer order, business status, or host authorization is fabricated. The initial disabled-to-enabled transition may lose authorization again and require the already-enabled reconfirmation entry. This test does not claim to remove the host synchronization defect.

Prerequisites: local PostgreSQL, `python3` with `PyYAML`, `openssl`, `ip`, Go, and `temp/bin/lina` built from current source. Use the runner's existing `E2E_DB_*` or `E2E_PG_URL`; never embed credentials in scripts. Existing local development security configuration is reused. The temporary directory has mode `0700`; configuration and certificate private keys have mode `0600`.

The real outbound policy requires public HTTPS. The test selects a public IP from a local interface, or accepts an explicitly configured local public IP through `E2E_PEER_PUBLIC_IP`, and binds a trusted temporary HTTPS proxy to its local test upstream. Private-address, loopback, and certificate restrictions remain enforced. The test fails when no suitable public IP exists. It does not contact third-party remote services.

```bash
pnpm -C hack/tests test:module plugin:idc-channel -- --grep 'TC-11b'
pnpm -C hack/tests test:module plugin:idc-channel -- --grep 'TC-11c'
```

Logs contain fixed stages, sanitized API paths, and results. They exclude AppSecret, access tokens, database passwords, SQL parameters, raw provider responses, and configuration. Backend logs retained for startup or runtime failure are private diagnostic files with mode `0600` in the current date's temporary directory, and are not public deliverables.
