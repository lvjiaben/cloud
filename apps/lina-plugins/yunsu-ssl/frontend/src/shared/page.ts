import { mountIsolatedVbenTable, type IsolatedTableColumn } from '@linapro/vben-plugin-adapter/isolated';
import { certificateAPI } from "./api-contract.js";
import { createBridge } from "./bridge.js";
import { actionLabel, copy, statusLabel } from "./copy.js";
import {
  displayProduct,
  formatTime,
  normalizeLocale,
  parseCertificateDetail,
  parseCertificatePage,
  type Audience,
  type CertificateAction,
  type CertificateItem,
  type Locale,
} from "./model.js";

interface PageOptions {
  audience: Audience;
  capability: string;
  surfaceKey: string;
}

function idempotencyKey(action: string) {
  const random = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`;
  return `ssl-page-${action}-${random}`.slice(0, 128);
}

export function startCertificatePage(options: PageOptions, win: Window = window, doc: Document = document) {
  const api = certificateAPI(options.audience);
  const bridge = createBridge(win, options.surfaceKey, options.capability);
  let locale: Locale = "zh-CN";
  let initialized = false;
  let items: CertificateItem[] = [];
  let nextCursor = "";
  let selected: CertificateItem | null = null;
  let busy = false;
  let requestedEntitlement = "";
  let openedEntitlement = "";

  const node = (testId: string) => doc.querySelector<HTMLElement>(`[data-testid="${testId}"]`)!;
  const show = (testId: string, visible: boolean) => { node(testId).hidden = !visible; };
  const resize = () => bridge.resize(Math.ceil(doc.documentElement.scrollHeight));
  const setText = (testId: string, value: string) => { node(testId).textContent = value; };
  const safeSegment = (value: string) => /^[A-Za-z0-9_-]{1,128}$/.test(value) ? value : "";
  type CertificateRow = CertificateItem & Record<string, unknown>;
  let tableFailed = false;
  let disposed = false;
  let table = createTable();

  function createTable() {
    const handle = mountIsolatedVbenTable<CertificateRow>(node('ssl-table'), {
      columns: tableColumns(), rows: [], rowKey: 'entitlementNo', height: 480, locale,
    });
    void handle.ready.then(resize).catch(() => {
      if (disposed || table !== handle) return;
      tableFailed = true;
      show('ssl-error', true);
      show('ssl-content', false);
      show('ssl-empty', false);
      resize();
    });
    return handle;
  }

  function applyTheme(context: Record<string, unknown>) {
    const theme = (context.theme as Record<string, unknown>) ?? {};
    doc.documentElement.dataset.theme = String(theme.mode ?? "light");
    for (const [name, value] of Object.entries((theme.cssVariables as Record<string, unknown>) ?? {})) {
      if (typeof value === "string") doc.documentElement.style.setProperty(name, value);
    }
    if (theme.fontFamily) doc.body.style.fontFamily = String(theme.fontFamily);
    if (Number.isFinite(theme.fontSize)) doc.body.style.fontSize = `${Number(theme.fontSize)}px`;
  }

  function applyCopy() {
    const words = copy[locale];
    const title = options.audience === "admin" ? words.adminTitle : words.customerTitle;
    doc.documentElement.lang = locale;
    doc.title = title;
    bridge.setTitle(title);
    setText("ssl-title", title);
    setText("ssl-refresh", words.refresh);
    setText("ssl-load-more", words.loadMore);
    setText("ssl-retry", words.retry);
    setText("ssl-empty-title", words.empty);
    setText("ssl-empty-hint", words.emptyHint);
    setText("ssl-error-title", words.error);
    setText("ssl-error-hint", words.errorHint);
    setText("ssl-detail-title", words.detail);
    setText("ssl-detail-close", words.close);
    setText("ssl-method-title", words.updateValidation);
    setText("ssl-method-label", words.method);
    setText("ssl-validation-title", words.validation);
    setText("ssl-method-save", words.save);
    setText("ssl-method-close", words.close);
    const methodSelect = node("ssl-method-select") as HTMLSelectElement;
    methodSelect.options[0]!.text = words.methodDns;
    methodSelect.options[1]!.text = words.methodFile;
    renderRows();
    if (selected) renderDetail(selected);
  }

  function actionButton(action: CertificateAction, item: CertificateItem) {
    const button = doc.createElement("button");
    button.type = "button";
    button.className = action === "cancel" ? "link danger" : "link";
    button.textContent = actionLabel(action, locale);
    button.disabled = busy;
    button.addEventListener("click", () => void runAction(action, item));
    return button;
  }

  function tableColumns(): IsolatedTableColumn<CertificateRow>[] {
    const words = copy[locale];
    const compact = win.innerWidth <= 720;
    return [
      {field:'primaryDomain', title:words.domain, minWidth:compact ? 120 : 180},
      ...(!compact ? [
        {field:'product',title:words.product,minWidth:160,cell:(item:CertificateRow) => displayProduct(item,locale)},
        ...(options.audience === 'admin' ? [{field:'customerName',title:words.customer,minWidth:120,cell:(item:CertificateRow) => item.customerName || '—'}] : []),
        {field:'statusCode',title:words.status,minWidth:130,cell:(item:CertificateRow) => statusLabel(item.statusCode,locale)},
        {field:'expiresAt',title:words.expiresAt,minWidth:180,cell:(item:CertificateRow) => formatTime(item.expiresAt,locale)},
      ] : []),
      {field:'actions',title:words.actions,width:compact ? 210 : 340,fixed:'right',cell:(item) => {
        const actions = doc.createElement('div'); actions.className = 'actions';
        const detail = doc.createElement('button'); detail.type = 'button'; detail.className = 'link'; detail.textContent = words.view;
        detail.addEventListener('click', () => void loadDetail(item)); actions.append(detail);
        for (const action of item.availableActions.filter(value => value !== 'sync')) actions.append(actionButton(action,item));
        return actions;
      }},
    ];
  }

  function renderRows() {
    const words = copy[locale];
    const columns = tableColumns();
    table.update({columns, rows: items as CertificateRow[], locale});
    setText("ssl-summary", locale === "en-US" ? `${items.length} certificate${items.length === 1 ? "" : "s"}` : `共 ${items.length} 张证书`);
    show("ssl-content", !tableFailed && items.length > 0); show("ssl-empty", !tableFailed && items.length === 0); show("ssl-load-more-wrap", Boolean(nextCursor)); resize();
  }

  function detailField(label: string, value: string) {
    const wrapper = doc.createElement("div"); const term = doc.createElement("dt"); const description = doc.createElement("dd");
    term.textContent = label; description.textContent = value || "—"; wrapper.append(term, description); return wrapper;
  }

  function renderDetail(item: CertificateItem) {
    const words = copy[locale]; selected = item;
    node("ssl-detail-grid").replaceChildren(
      detailField(words.domain, item.primaryDomain), detailField(words.product, displayProduct(item, locale)),
      ...(options.audience === "admin" ? [detailField(words.customer, item.customerName)] : []),
      detailField(words.status, statusLabel(item.statusCode, locale)), detailField(words.method, item.validationMethod === "dns" ? words.methodDns : words.methodFile),
      detailField(words.issuedAt, formatTime(item.issuedAt, locale)), detailField(words.expiresAt, formatTime(item.expiresAt, locale)), detailField(words.updatedAt, formatTime(item.updatedAt, locale)),
    );
    const validations = node("ssl-validation-list");
    validations.replaceChildren(...item.validation.map((entry) => {
      const card = doc.createElement("article"); card.className = "validation-card";
      for (const [label, value] of [[words.domain, entry.domain], [words.method, entry.method === "dns" ? words.methodDns : words.methodFile], [words.host, entry.host], [words.value, entry.value], [words.fileName, entry.fileName], [words.path, entry.path]] as Array<[string, string]>) {
        if (!value) continue; card.append(detailField(label, value));
      }
      return card;
    }));
    const actions = node("ssl-detail-actions"); actions.replaceChildren(...item.availableActions.map((action) => actionButton(action, item)));
    show("ssl-detail", true); resize();
  }

  async function loadList(append = false) {
    if (busy) return; busy = true; show("ssl-loading", true); show("ssl-error", false);
    if (tableFailed) {
      table.destroy();
      tableFailed = false;
      table = createTable();
    }
    const response = await bridge.request("GET", api.list, { params: { cursor: append ? nextCursor : "", limit: 30 } });
    const page = response.ok === true ? parseCertificatePage(response.result) : null;
    busy = false; show("ssl-loading", false);
    if (!page) {
      show("ssl-empty", false);
      show("ssl-error", true);
      resize();
      return;
    }
    items = append ? [...items, ...page.items] : page.items; nextCursor = page.nextCursor; renderRows();
    await openRequestedEntitlement();
  }

  async function loadDetail(item: CertificateItem) {
    await loadDetailNumber(item.entitlementNo);
  }

  async function loadDetailNumber(entitlementNo:string) {
    const key = safeSegment(entitlementNo); if (!key || busy) return; busy = true; show("ssl-loading", true);
    const response = await bridge.request("GET", `${api.detail}/${key}`, {});
    busy = false; show("ssl-loading", false);
    const detail = response.ok === true ? parseCertificateDetail(response.result) : null;
    if (disposed) return;
    if (detail) renderDetail(detail); else show("ssl-error", true);
  }

  function openMethodDialog(item: CertificateItem) {
    selected = item; const select = node("ssl-method-select") as HTMLSelectElement; select.value = item.validationMethod;
    show("ssl-method-dialog", true); resize();
  }

  async function runAction(action: CertificateAction, item: CertificateItem, validationMethod?: "dns" | "file") {
    if (action === "update-validation" && !validationMethod) { openMethodDialog(item); return; }
    if (action === "cancel" && !win.confirm(copy[locale].cancelConfirm)) return;
    const key = safeSegment(item.entitlementNo); if (!key || busy) return; busy = true; renderRows(); setText("ssl-notice", copy[locale].working); show("ssl-notice", true);
    const path = action === "download" ? `${api.download}/${key}/downloads/certificate/tickets` : `${api.action}/${key}/actions/${action}`;
    const response = await bridge.request("POST", path, { body: { idempotencyKey: idempotencyKey(action), input: validationMethod ? { validationMethod } : {} } });
    busy = false;
    if (response.ok !== true) { setText("ssl-notice", copy[locale].actionFailed); renderRows(); return; }
    if (action === "download") {
      // The owner returns only a bounded, short-lived single-use download
      // ticket. Host login tokens and supplier credentials never cross the
      // sandbox boundary.
      const result = response.result && typeof response.result === "object" ? response.result as Record<string, unknown> : {};
      const ticket = typeof result.ticket === "string" && /^[A-Za-z0-9_-]{32,128}$/.test(result.ticket) ? result.ticket : "";
      const fileName = typeof result.fileName === "string" && /^[A-Za-z0-9._-]{1,128}$/.test(result.fileName) ? result.fileName : "certificate.zip";
      if (ticket) {
        const scope = options.audience === "admin" ? "admin" : "account";
        const downloaded = await bridge.request("GET", `${scope}/service-downloads/${ticket}`, { download: { fileName } });
        if (downloaded.ok !== true) setText("ssl-notice", copy[locale].actionFailed);
      }
    } else {
      await loadList(false);
      if (selected) await loadDetail(selected);
    }
    show("ssl-notice", false); renderRows();
  }

  // Host routes carry only a safe entitlement number; the owner rechecks every detail request.
  async function openRequestedEntitlement() {
    if (options.audience!=="admin" || !requestedEntitlement || openedEntitlement===requestedEntitlement || busy) return;
    openedEntitlement=requestedEntitlement;
    await loadDetailNumber(requestedEntitlement);
  }

  const stopContext = bridge.subscribeContext((context) => {
    locale = normalizeLocale(context.locale); applyTheme(context); applyCopy();
    const route=context.route as {path?:unknown}|undefined;
    const match=typeof route?.path==="string"? /^\/?entitlements\/([A-Za-z0-9_-]{1,128})$/.exec(route.path):null;
    requestedEntitlement=match?.[1]??"";
    if (initialized) void openRequestedEntitlement();
    if (!initialized) { initialized = true; void loadList(false); }
    resize();
  });
  node("ssl-refresh").addEventListener("click", () => void loadList(false));
  node("ssl-retry").addEventListener("click", () => void loadList(false));
  node("ssl-load-more").addEventListener("click", () => void loadList(true));
  node("ssl-detail-close").addEventListener("click", () => { selected = null; show("ssl-detail", false); resize(); });
  node("ssl-method-close").addEventListener("click", () => show("ssl-method-dialog", false));
  node("ssl-method-save").addEventListener("click", () => { const current = selected; const value = (node("ssl-method-select") as HTMLSelectElement).value as "dns" | "file"; show("ssl-method-dialog", false); if (current) void runAction("update-validation", current, value); });
  const observer = new ResizeObserver(resize); observer.observe(doc.body); win.addEventListener('resize', renderRows); applyCopy(); bridge.ready(); resize();
  return () => { disposed = true; win.removeEventListener('resize', renderRows); table.destroy(); observer.disconnect(); stopContext(); bridge.dispose(); };
}
