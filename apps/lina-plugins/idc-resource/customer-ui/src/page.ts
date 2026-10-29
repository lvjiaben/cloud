// The resource plugin owns the purchased-server columns, display rules and interactions.
import {
  mountIsolatedVbenTable,
  mountIsolatedVbenFormDrawer,
  type IsolatedFormDrawerHandle,
  type IsolatedTableColumn,
} from "@linapro/vben-plugin-adapter/isolated";
import zh from "../../manifest/i18n/zh-CN/customer-services.json";
import en from "../../manifest/i18n/en-US/customer-services.json";
import { listen, request, send } from "./bridge";
import { expiry, systemIcon } from "./format";
import "@linapro/vben-plugin-adapter/isolated.css";
import "./appearance.css";
type Row = Record<string, unknown> & {
  entitlementNo: string;
  productNameZhCn: string;
  productNameEnUs: string;
  statusCode: string;
  orderNo: string;
  managementPreparation?: string;
  requiresManualProcessing?: boolean;
  expiresAt: number | null;
  customerRemark: string;
  customerRemarkRevision: number;
  currentPriceAmountMinor?: string | null;
  currencyCode?: string;
  periodUnitCode?: string;
  periodCount?: number;
  chargeCycles?: number;
};
type Fact = {
  entitlementNo: string;
  hostname: string;
  addresses: string[];
  addressesConfirmed: boolean;
  osName: string;
};
const serviceCapability = "purchased-services.customer.v1",
  factsCapability = "native-server-facts.customer.v1";
let words: Record<string, string> = zh.plugins.idc_resource.purchasedServices,
  locale = "zh-CN",
  initialized = false,
  busy = false,
  page = 1,
  size = 20,
  keyword = "",
  sequence = 0,
  rows: Row[] = [],
  total = 0,
  facts = new Map<string, Fact>();
const error = document.getElementById("error")!;
let noteHandle: IsolatedFormDrawerHandle | undefined;
function node(tag: string, text = "", className = "") {
  const element = document.createElement(tag);
  element.textContent = text;
  element.className = className;
  return element;
}
let fittedHeight = 0;
function resize() {
  const height = Math.max(300, window.innerHeight - 32);
  if (height !== fittedHeight) { fittedHeight = height; table.update({ height }); }
}
function navigate(path: string) {
  send("host.view.v1", "navigate", { path });
}
function button(text: string, action: () => void, ariaLabel?: string) {
  const element = node("button", text, "link-button") as HTMLButtonElement;
  element.type = "button";
  if (ariaLabel) element.setAttribute("aria-label", ariaLabel);
  element.addEventListener("click", action);
  return element;
}
function link(text: string, path: string) {
  const element = document.createElement("a");
  element.textContent = text;
  element.href = path;
  element.addEventListener("click", (event) => {
    event.preventDefault();
    navigate(path);
  });
  return element;
}
function target(row: Row) {
  return row.statusCode === "pending_activation" ||
    (row.managementPreparation && row.managementPreparation !== "ready")
    ? `/account/orders/${row.orderNo}`
    : `/account/native-servers/by-entitlement/${row.entitlementNo}`;
}
function product(row: Row) {
  return locale === "en-US"
    ? row.productNameEnUs || row.productNameZhCn
    : row.productNameZhCn || row.productNameEnUs;
}
function money(value: string, currency = "CNY") {
  try {
    const digits =
        new Intl.NumberFormat(locale, {
          style: "currency",
          currency,
        }).resolvedOptions().maximumFractionDigits ?? 2,
      amount = BigInt(value),
      unit = 10n ** BigInt(digits);
    return `${currency} ${(amount / unit).toLocaleString(locale)}${digits ? "." + (amount % unit).toString().padStart(digits, "0") : ""}`;
  } catch {
    return "—";
  }
}
function columns(): IsolatedTableColumn<Row>[] {
  return [
    {
      field: "productNameZhCn",
      title: words.product,
      minWidth: 240,
      cell: (row) => {
        const box = node("div", "", "identity");
        box.append(
          link(product(row), target(row)),
          node(
            "small",
            facts.get(row.entitlementNo)?.hostname || row.entitlementNo,
            "subline",
          ),
        );
        return box;
      },
    },
    {
      field: "statusCode",
      title: words.status,
      width: 125,
      cell: (row) => {
        const box = node("div"),
          tone =
            row.statusCode === "active"
              ? "success"
              : ["suspended", "pending_activation"].includes(row.statusCode)
                ? "warning"
                : ["expired", "terminated"].includes(row.statusCode)
                  ? "danger"
                  : "";
        const status = node("span", "", `status ${tone}`);
        status.append(
          node("span", "", "dot"),
          document.createTextNode(words[row.statusCode] || "—"),
        );
        box.append(status);
        if (
          row.statusCode === "pending_activation" &&
          row.requiresManualProcessing
        )
          box.append(node("small", words.manual, "subline"));
        return box;
      },
    },
    {
      field: "addresses",
      title: words.addresses,
      minWidth: 150,
      cell: (row) => {
        const fact = facts.get(row.entitlementNo);
        return fact?.addresses?.length
          ? fact.addresses.join(" / ")
          : fact?.addressesConfirmed
            ? words.unassigned
            : words.unknown;
      },
    },
    {
      field: "expiresAt",
      title: words.expires,
      width: 165,
      cell: (row) => {
        const data = expiry(row.expiresAt, row.statusCode),
          box = node("div", data.date);
        if (data.days !== undefined) {
          const text =
            data.days > 0
              ? words.future.replace("{days}", String(data.days))
              : data.days === 0
                ? words.today
                : words.past.replace("{days}", String(-data.days));
          box.append(node("small", text, `subline ${data.tone}`));
        }
        return box;
      },
    },
    {
      field: "currentPriceAmountMinor",
      title: words.price,
      minWidth: 160,
      cell: (row) => {
        const box = node(
          "div",
          row.currentPriceAmountMinor != null
            ? money(row.currentPriceAmountMinor, row.currencyCode)
            : "—",
        );
        box.append(
          node(
            "small",
            row.currentPriceAmountMinor != null
              ? `${words.currentPrice}${row.periodCount && row.chargeCycles ? ` · ${row.periodCount * row.chargeCycles}${words[row.periodUnitCode || ""] || ""}` : ""}`
              : words.bill,
            "subline",
          ),
        );
        return box;
      },
    },
    {
      field: "system",
      title: words.system,
      minWidth: 145,
      cell: (row) => {
        const name = facts.get(row.entitlementNo)?.osName;
        if (!name) return "—";
        const box = node("span", "", "system"),
          icon = systemIcon(name);
        if (icon) {
          const image = document.createElement("img");
          image.src = `https://api.iconify.design/simple-icons/${icon}.svg?color=%2364748b`;
          image.alt = "";
          image.setAttribute("aria-hidden", "true");
          image.referrerPolicy = "no-referrer";
          image.onerror = () => {
            image.remove();
            box.prepend(node("span", "▣"));
          };
          box.append(image);
        } else box.append(node("span", "▣"));
        box.append(document.createTextNode(name));
        return box;
      },
    },
    {
      field: "customerRemark",
      title: words.note,
      minWidth: 150,
      cell: (row) => {
        const box = node("div", "", "note-cell");
        box.append(
          node("span", row.customerRemark || "—"),
          button("✎", () => openNote(row), `${words.edit} ${product(row)}`),
        );
        return box;
      },
    },
    {
      field: "actions",
      title: words.actions,
      width: 140,
      fixed: "right",
      cell: (row) =>
        link(
          target(row).includes("/orders/") ? words.progress : words.manage,
          target(row),
        ),
    },
  ];
}
let committedPage = page,
  committedSize = size;
function failureMessage(cause: unknown) {
  return cause instanceof Error &&
    !["Request timed out", "Page closed", "Request failed"].includes(
      cause.message,
    )
    ? cause.message
    : words.failure;
}
const table = mountIsolatedVbenTable<Row>(document.getElementById("table")!, {
  columns: columns(),
  rows: [],
  rowKey: "entitlementNo",
  height: Math.max(300, window.innerHeight - 32),
  locale,
  search: {
    fields: [{ name: "keyword", label: words.search, component: "Input", props: { placeholder: words.searchPlaceholder, "aria-label": words.search } }],
    values: { keyword },
    onSubmit: (values) => {
      if (busy) return;
      keyword = String(values.keyword || "").trim();
      page = 1;
      return load();
    },
  },
});
function render(loading = false) {
  busy = loading;
  table.update({

    columns: columns(),
    rows,
    loading,
    emptyText: words.empty,
    locale,
    pagination: {
      currentPage: page,
      pageSize: size,
      total,
      onChange: (next) => {
        page = next.currentPage;
        size = next.pageSize;
        void load();
      },
    },

  });
  queueMicrotask(() => {
    document.querySelectorAll<HTMLButtonElement | HTMLInputElement>(".vxe-grid--form-wrapper button, .vxe-grid--form-wrapper input").forEach(control => { control.disabled = busy; });
  });
  resize();
}
async function load() {
  const current = ++sequence;
  error.hidden = true;
  render(true);
  try {
    const result = await request<{ items: Row[]; total: number }>(
      serviceCapability,
      "GET",
      "account/purchased-services",
      { params: { page, pageSize: size, keyword } },
    );
    if (current !== sequence) return;
    rows = result.items;
    total = result.total;
    committedPage = page;
    committedSize = size;
    facts = new Map();
    const ids = rows.map((row) => row.entitlementNo);
    if (ids.length) {
      try {
        const data = await request<{ items: Fact[] }>(
          factsCapability,
          "GET",
          "account/native-server-facts",
          { params: { entitlementNos: ids } },
        );
        if (current !== sequence) return;
        facts = new Map(data.items.map((fact) => [fact.entitlementNo, fact]));
      } catch (cause) {
        if (current !== sequence) return;
        error.textContent =
          cause instanceof Error ? cause.message : words.factsFailure;
        error.hidden = false;
      }
    }
  } catch (cause) {
    if (current !== sequence) return;
    page = committedPage;
    size = committedSize;
    error.textContent = failureMessage(cause);
    error.hidden = false;
  } finally {
    if (current === sequence) render();
  }
}
function openNote(row: Row) {
  if (noteHandle) return;
  const container = document.createElement("div");
  document.body.append(container);
  noteHandle = mountIsolatedVbenFormDrawer(container, {
    title: words.noteTitle,
    locale,
    fields: [
      {
        name: "remark",
        label: words.note,
        component: "Textarea",
        props: { rows: 4, maxlength: 1000 },
      },
    ],
    values: { remark: row.customerRemark || "" },
    onSubmit: async (values) => {
      try {
        await request(
          serviceCapability,
          "PUT",
          `account/service-entitlements/${row.entitlementNo}/customer-note`,
          {
            body: {
              remark: String(values.remark || ""),
              expectedRevision: row.customerRemarkRevision || 1,
            },
          },
        );
        await load();
        return true;
      } catch (cause) {
        noteHandle?.update({
          errorText: cause instanceof Error ? cause.message : words.failure,
        });
        return false;
      }
    },
    onClosed: () => {
      noteHandle?.destroy();
      noteHandle = undefined;
      container.remove();
    },
  });
  void noteHandle.ready.catch((cause) => {
    error.textContent = words.failure;
    error.hidden = false;
    noteHandle?.destroy();
    noteHandle = undefined;
    container.remove();
  });
}
const dispose = listen((context) => {
  locale = context.locale === "en-US" ? "en-US" : "zh-CN";
  words =
    locale === "en-US"
      ? en.plugins.idc_resource.purchasedServices
      : zh.plugins.idc_resource.purchasedServices;
  document.documentElement.lang = locale;
  document.title = words.title;
  send("host.view.v1", "set-title", { title: words.title });
  if (!initialized) {
    initialized = true;
    void table.ready.then(load).catch((cause) => {
      error.textContent = words.failure;
      error.hidden = false;
    });
  } else render();
});
window.addEventListener("resize", resize);
window.addEventListener("pagehide", () => {
  sequence++;
  dispose();
  window.removeEventListener("resize", resize);
  noteHandle?.destroy();
  table.destroy();
});
send("host.view.v1", "ready", {});
