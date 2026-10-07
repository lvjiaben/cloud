const runtimeURL = "/x-assets/vben-adapter/v0.1.0/isolated/runtime.js";
const stylesheetURL =
  "/x-assets/vben-adapter/v0.1.0/isolated/vben-plugin-adapter.css";

// Both delivered channel entry points use the adapter's actual Vben runtime.
// This layer owns DOM lifetime only; it does not read host state or issue APIs.
export function createChannelTableRenderer(
  root,
  { locale, translate, failure },
) {
  const tables = new Map();
  let disposed = false;
  const observer = new MutationObserver(() => {
    for (const [element, entry] of tables) {
      if (!root.contains(element)) {
        entry.cancelled = true;
        entry.handle?.destroy();
        tables.delete(element);
      }
    }
  });
  observer.observe(root, { childList: true, subtree: true });
  if (!document.querySelector("link[data-channel-vben-runtime]")) {
    const stylesheet = document.createElement("link");
    stylesheet.rel = "stylesheet";
    stylesheet.href = stylesheetURL;
    stylesheet.dataset.channelVbenRuntime = "true";
    document.head.append(stylesheet);
  }
  let runtime;
  function render(columns, rows = [], empty = "", paging) {
    const shell = document.createElement("div");
    shell.className = "channel-vben-table";
    shell.style.cssText = "min-width:0;max-width:100%;background:#fff";
    const entry = { cancelled: false, handle: undefined };
    let currentPage = 1;
    let pageSize = 10;
    let total = paging?.load ? paging.total : rows.length;
    let sequence = 0;
    const decorate = (items) =>
      items.map((row, index) => ({ ...row, _channelRowKey: index }));
    const pagination = () => ({
      currentPage,
      pageSize,
      total,
      onChange: async (next) => {
        const requestSequence = ++sequence;
        next = {
          currentPage: next.pageSize === pageSize ? next.currentPage : 1,
          pageSize: next.pageSize,
        };
        // Vxe updates its pager eagerly; restore confirmed state until the read succeeds.
        entry.handle?.update({ pagination: pagination(), loading: true });
        try {
          const result = paging?.load
            ? await paging.load(next)
            : {
                list: rows.slice(
                  (next.currentPage - 1) * next.pageSize,
                  next.currentPage * next.pageSize,
                ),
                total: rows.length,
              };
          if (disposed || entry.cancelled || requestSequence !== sequence)
            return;
          if (!Number.isSafeInteger(result.total) || result.total < 0)
            throw new Error("Invalid channel page total");
          currentPage = next.currentPage;
          pageSize = next.pageSize;
          total = result.total;
          paging?.onPage?.(result.list || []);
          entry.handle.update({
            rows: decorate(result.list || []),
            pagination: pagination(),
            loading: false,
          });
        } catch (error) {
          if (disposed || entry.cancelled || requestSequence !== sequence)
            return;
          // The opaque host bridge intentionally hides HTTP status. Unknown rejection
          // cannot be treated as a transient read while retaining authorized data.
          const status = Number(error?.status || error?.response?.status);
          const denied = status === 401 || status === 403 || error?.message === "bridge_request_failed";
          if (denied) {
            total = 0;
            currentPage = 1;
          }
          entry.handle.update({
            ...(denied ? { rows: [] } : {}),
            pagination: pagination(),
            loading: false,
          });
          paging?.onError?.();
        }
      },
    });
    tables.set(shell, entry);
    (runtime ||= import(runtimeURL))
      .then(({ mountIsolatedVbenTable }) => {
        if (disposed || entry.cancelled) return;
        if (!Number.isSafeInteger(total) || total < 0)
          throw new Error("Invalid channel page total");
        entry.handle = mountIsolatedVbenTable(shell, {
          locale,
          height: 360,
          emptyText: translate(empty),
          rowKey: "_channelRowKey",
          rows: decorate(paging?.load ? rows : rows.slice(0, pageSize)),
          pagination: pagination(),
          columns: columns.map(([title, project], index) => ({
            field: "column" + index,
            title: translate(title),
            minWidth: index ? 140 : 180,
            ...(index
              ? { width: title === "操作" || title === "Actions" ? 100 : 170 }
              : {}),
            ...(title === "操作" || title === "Actions"
              ? { fixed: "right" }
              : {}),
            cell: (row) => {
              const value = project(row);
              return value instanceof Node
                ? value
                : translate(String(value ?? "—"));
            },
          })),
        });
        return entry.handle.ready;
      })
      .catch(() => {
        runtime = undefined;
        if (!disposed && !entry.cancelled) shell.textContent = failure;
      });
    return shell;
  }
  return {
    render,
    destroy() {
      disposed = true;
      observer.disconnect();
      for (const entry of tables.values()) entry.handle?.destroy();
      tables.clear();
    },
  };
}
