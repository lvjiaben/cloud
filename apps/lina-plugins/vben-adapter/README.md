# LinaPro Vben Plugin Adapter

This frontend-only source plugin owns the reusable contract used by plugin pages to consume official Vben page, form, grid, modal, drawer, and button adapters. It contains no IDC business page and requires no `web-antd` business-source change.

Consumer pages remain in their owning plugin's `frontend/` directory and pass host-published Vben components into `createVbenPluginAdapter`. The base contract library builds independently; the isolated runtime build reuses official Vben packages from the workspace without depending on their business pages.

Form drawers can supply secondary `actions` with localized labels and `onClick(values)` callbacks. They render official buttons, receive current unsaved values, and are disabled during submission or after closing. They do not submit or persist business data automatically.

Tables can supply localized toolbar `actions`; the runtime renders official buttons and respects disabled, loading, and destroyed states. Every isolated table and drawer app has its own Vue ID prefix, so nested drawers and search fields cannot reuse the same accessibility-label IDs.

The `isolated` entry bundles its own official Vben runtime. `mountIsolatedVbenTable` accepts optional `search` fields, initial `values`, and an `onSubmit(values)` callback. Search and reset use the grid's Vben form; reset finishes before returning its current values. Empty fields omit the search UI. The owner remains responsible for fetching rows and supplying exact pagination totals.

`mountIsolatedVbenFormDrawer` accepts `mountContent(container)`, which mounts trusted owner UI into a real DOM element and may return a cleanup function. The runtime calls cleanup when the content callback changes, the drawer closes, or the handle is destroyed. It does not parse HTML strings or own business requests. Use `readOnly: true` and `fields: []` for a content-only drawer; existing form drawers retain their form behavior. Caller-provided labels remain caller-localized; standard search controls use Vben translations.
