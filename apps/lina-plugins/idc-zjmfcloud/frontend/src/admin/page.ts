import {
  mountIsolatedVbenTable,
  mountIsolatedVbenFormDrawer,
} from "@linapro/vben-plugin-adapter/isolated";
import {
  businessLabel,
  displayName,
  formatTime,
  parseServerDetail,
  parseServerPage,
  type Locale,
  type ServerItem,
} from "../customer/servers/model";
import { createBridge } from "../shared/bridge";

export function startAdminServersPage(
  win: Window = window,
  doc: Document = document,
) {
  const bridge = createBridge(
    win,
    "server-management",
    "zjmfcloud-servers.admin.v1",
  );
  let locale: Locale = "zh-CN",
    initialized = false,
    sequence = 0,
    currentPage = 1,
    pageSize = 20,
    total = 0,
    rows: ServerItem[] = [],
    busy = false;
  const words = (cn: string, en: string) => (locale === "en-US" ? en : cn);
  doc.body.innerHTML =
    '<main class="zjmfcloud-servers"><div class="toolbar"><strong data-testid="admin-title"></strong><button data-testid="admin-refresh" class="button"></button></div><p role="status" data-testid="admin-notice"></p><div data-testid="admin-table"></div><div data-testid="admin-drawer"></div></main>';
  const node = (id: string) =>
    doc.querySelector<HTMLElement>(`[data-testid="${id}"]`)!;
  const resize = () =>
    bridge.resize(Math.ceil(doc.documentElement.scrollHeight));
  const notice = (value: string) => {
    node("admin-notice").textContent = value;
    resize();
  };
  const table = mountIsolatedVbenTable<ServerItem & Record<string, unknown>>(
    node("admin-table"),
    { columns: [], rows: [], rowKey: "serverNo", height: 480, locale },
  );
  void table.ready
    .then(resize)
    .catch(() =>
      notice(
        words(
          "表格加载失败，请刷新页面。",
          "The table could not load. Refresh the page.",
        ),
      ),
    );
  function render() {
    const title = words("魔方云服务器管理", "ZJMFCloud servers");
    node("admin-title").textContent = title;
    doc.title = title;
    bridge.setTitle(title);
    node("admin-refresh").textContent = words("刷新", "Refresh");
    table.update({
      locale,
      rows: rows as (ServerItem & Record<string, unknown>)[],
      loading: busy,
      columns: [
        {
          field: "product",
          title: words("商品", "Product"),
          minWidth: 220,
          cell: (row) => displayName(row, locale),
        },
        { field: "channelName", title: words("接口", "Interface") },
        { field: "poolName", title: words("资源池", "Resource pool") },
        {
          field: "state",
          title: words("状态", "Status"),
          cell: (row) => businessLabel(row.state, locale),
        },
        {
          field: "latestOperation",
          title: words("最近活动", "Latest activity"),
          cell: (row) => businessLabel(row.latestOperation, locale),
        },
        {
          field: "actions",
          title: words("操作", "Actions"),
          fixed: "right",
          width: 120,
          cell: (row) => {
            const button = doc.createElement("button");
            button.className = "button";
            button.textContent = words("查看详情", "View details");
            button.onclick = () => {
              void detail(row.serverNo);
            };
            return button;
          },
        },
      ],
      pagination: {
        currentPage,
        pageSize,
        total,
        onChange: (page) => {
          void load(page.currentPage, page.pageSize);
        },
      },
    });
    (node("admin-refresh") as HTMLButtonElement).disabled = busy;
    resize();
  }
  async function load(page = currentPage, size = pageSize) {
    const requestSequence = ++sequence;
    busy = true;
    render();
    try {
      const response = await bridge.request(
        "GET",
        "admin/resource/zjmfcloud-servers",
        { params: { page, limit: size } },
      );
      const result =
        response.ok === true ? parseServerPage(response.result) : null;
      const count = (response.result as { total?: unknown } | undefined)?.total;
      if (
        !result ||
        typeof count !== "number" ||
        !Number.isSafeInteger(count) ||
        count < result.items.length
      )
        throw new Error("invalid");
      if (requestSequence !== sequence) return;
      rows = result.items;
      total = count;
      currentPage = page;
      pageSize = size;
      notice("");
    } catch {
      if (requestSequence === sequence) {
        rows = [];
        total = 0;
        notice(
          words(
            "服务器加载失败，请检查权限或稍后重试。",
            "Servers could not load. Check access or try again.",
          ),
        );
      }
    } finally {
      if (requestSequence === sequence) {
        busy = false;
        render();
      }
    }
  }
  async function detail(serverNo: string) {
    const response = await bridge.request(
      "GET",
      `admin/resource/zjmfcloud-servers/${serverNo}`,
      {},
    );
    const result =
      response.ok === true ? parseServerDetail(response.result) : null;
    if (!result || result.serverNo !== serverNo) {
      notice(
        words(
          "无法读取该服务器，请检查权限或刷新列表。",
          "The server is unavailable. Check access or refresh the list.",
        ),
      );
      return;
    }
    const drawer = mountIsolatedVbenFormDrawer(node("admin-drawer"), {
      title: words("服务器详情", "Server details"),
      locale,
      readOnly: true,
      values: {
        product: displayName(result, locale),
        location: `${result.channelName} · ${result.poolName}`,
        state: businessLabel(result.state, locale),
        operation: businessLabel(result.latestOperation, locale),
        boundAt: formatTime(result.boundAt, locale),
        updatedAt: formatTime(result.updatedAt, locale),
      },
      fields: [
        {
          name: "product",
          label: words("商品", "Product"),
          component: "Input",
        },
        {
          name: "location",
          label: words("接口 / 资源池", "Interface / resource pool"),
          component: "Input",
        },
        { name: "state", label: words("状态", "Status"), component: "Input" },
        {
          name: "operation",
          label: words("最近活动", "Latest activity"),
          component: "Input",
        },
        {
          name: "boundAt",
          label: words("交付时间", "Delivered"),
          component: "Input",
        },
        {
          name: "updatedAt",
          label: words("更新时间", "Updated"),
          component: "Input",
        },
      ],
      onClosed: () => drawer.destroy(),
    });
    void drawer.ready
      .then(resize)
      .catch(() =>
        notice(
          words("详情加载失败，请重试。", "Details could not load. Try again."),
        ),
      );
  }
  node("admin-refresh").onclick = () => {
    void load();
  };
  const off = bridge.subscribeContext((context) => {
    locale = context.locale === "en-US" ? "en-US" : "zh-CN";
    const theme = context.theme as Record<string, unknown> | undefined;
    doc.documentElement.dataset.theme = String(theme?.mode ?? "light");
    for (const [key, value] of Object.entries(theme?.cssVariables ?? {}))
      if (typeof value === "string")
        doc.documentElement.style.setProperty(key, value);
    render();
    if (!initialized) {
      initialized = true;
      void load();
    }
  });
  render();
  bridge.ready();
  const observer = new ResizeObserver(resize);
  observer.observe(doc.body);
  return () => {
    sequence++;
    off();
    table.destroy();
    observer.disconnect();
    bridge.dispose();
  };
}
