# IDC BT Panel Connector

This managed plugin integrates **BT Panel** itself. It does not reuse or claim evidence from the unrelated `btHost` virtual-hosting server module.

The [official BT Panel API overview](https://docs.bt.cn/api/) proves that the panel accepts form-encoded `POST` requests and requires `request_time` plus `request_token = md5(request_time + md5(api_sk))` on every call. The official [`getData`](https://docs.bt.cn/api/data/getData) and [`AddSite`](https://docs.bt.cn/api/site/AddSite) pages define the exact site lookup and creation forms used here.

The plugin declares repeatable interface instances with a panel endpoint and secret API key, and keeps website fields in plugin-owned product and customer schemas. `BuildSitePlan` freezes a reviewed PHP or static-site plan without receiving the API key.

## Host-confidential signing

The manifest selects the strict `bt_panel_double_md5` profile. The trusted `idc-resource` host reads the frozen interface's `api_key` secret handle, generates the current Unix second, derives the lowercase double-MD5 token, and injects only `request_time` and `request_token` into the outgoing form. The API key, timestamp, and token never enter WASM input, output, logs, or evidence. Guest attempts to supply either derived field are rejected before network access.

Provisioning first queries `/data` for an exact, case-insensitive primary-domain match. An existing site is recovered without another create call. A missing site is submitted once through `/site` `AddSite`; transport loss, HTTP `5xx`, or a successful response without `siteId` becomes a non-retryable unknown result for manual reconciliation. Explicit HTTP `4xx` or `siteStatus: false` fails safely. This first contract creates only websites and deliberately excludes database creation and destructive deletion.
