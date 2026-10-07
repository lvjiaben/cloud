import { createChannelTableRenderer } from "./channel-table-runtime.js";

let currentLocale = "zh-CN";
const english = new Map(Object.entries({
  "暂无记录": "No records", "请选择渠道": "Select a channel", "刷新当前渠道": "Refresh channel",
  "渠道": "Channels", "商品发布": "Product publications", "令牌": "Tokens",
  "额度与结算": "Finance", "健康与审计": "Health and audit", "能力清单": "Capabilities",
  "渠道账户": "Channel accounts", "渠道企业承担付款、信用与结算；服务所有者可以是渠道自身或受委托终端客户。": "The channel enterprise pays and settles. The service owner may be the channel or a delegated end customer.",
  "客户主体": "Customer", "状态": "Status", "允许新购": "New sales", "更新时间": "Updated",
  "新增渠道": "Add channel", "渠道编号创建后不能改绑客户主体。": "The customer binding cannot be changed after the channel is created.",
  "渠道编号": "Channel ID", "企业客户编号": "Enterprise customer number", "显示名称": "Display name",
  "保存待审批渠道": "Save for approval", "合作状态": "Partnership status",
  "暂停会阻止新购和危险操作，历史订单、服务查询及还款仍保留。": "Pausing blocks new sales and risky operations while preserving history, settlement, repayments, and support.",
  "批准": "Approve", "暂停停新": "Pause sales", "恢复": "Resume", "终止合作": "Close partnership",
  "已发布商品": "Published products", "渠道只看到售价与库存，不会看到核心成本、资源编号或上游凭据。": "Channels see selling price and stock only, never core cost, resource identifiers, or upstream credentials.",
  "商品版本": "Product version", "币种": "Currency", "售价": "Selling price", "库存": "Stock",
  "公开标识": "Public slug", "发布或更新商品": "Publish or update product",
  "保存前会用核心价格重新报价；价格不一致时直接拒绝。": "Core pricing is rechecked before saving; mismatches are rejected.",
  "商品版本 ID": "Product version ID", "公开商品标识": "Public product slug", "配置版本": "Configuration revision",
  "计费周期标识": "Billing term token", "购买周期数": "Purchase cycles", "数量": "Quantity",
  "发布状态": "Publication status", "已发布": "Published", "已暂停": "Paused",
  "购买配置项": "Purchase configuration", "从购买配置页面复制字段令牌，按字段类型填写值；多选、树、地址和文件令牌用英文逗号分隔。": "Copy field tokens from the purchase configuration page. Separate multi-value, tree, address, and file tokens with commas.",
  "添加配置项": "Add configuration value", "校验并保存发布": "Validate and publish",
  "字段令牌": "Field token", "字段类型": "Field type", "字段值": "Value", "由购买配置页面提供": "Provided by the purchase configuration page",
  "多值使用英文逗号分隔": "Separate multiple values with commas", "移除": "Remove",
  "单行文本": "Single-line text", "多行文本": "Multi-line text", "整数": "Integer", "小数": "Decimal",
  "布尔值": "Boolean", "单选项": "Single select", "多选项": "Multi select", "日期": "Date",
  "日期时间": "Date and time", "树选项路径": "Tree path", "地址路径": "Address path", "文件令牌": "File token",
  "访问令牌": "Access tokens", "令牌原文只显示一次；列表只保留用途、来源和状态。": "Token secrets are shown once; the list keeps metadata only.",
  "作用域": "Scopes", "来源网段": "Source CIDRs", "到期": "Expires", "操作": "Actions",
  "撤销": "Revoke", "创建令牌": "Create token", "作用域（每行一个）": "Scopes (one per line)",
  "来源网段（每行一个，可留空）": "Source CIDRs (one per line, optional)", "到期时间（可留空）": "Expiry time (optional)",
  "创建后在这里显示一次": "The new token appears here once", "创建一次性令牌": "Create one-time token",
  "逐币种资金总览": "Finance by currency", "余额、信用、应收、实收、退款和服务成本均来自核心账本。": "Balances, credit, receivables, payments, refunds, and service costs come from the core ledger.",
  "暂无资金记录": "No finance records", "最近账单": "Recent invoices", "账单": "Invoice", "订单": "Order",
  "金额": "Amount", "余额实收": "Balance paid", "信用结算": "Credit settled", "退款": "Refunded",
  "创建时间": "Created", "信用账单": "Credit statements", "未还": "Outstanding", "还款流水": "Repayments",
  "还款号": "Repayment", "渠道健康": "Channel health", "有效令牌": "Active tokens", "在售商品": "Published products",
  "待投递事件": "Pending webhooks", "最近请求": "Last request", "最近请求审计": "Recent request audit",
  "请求号": "Request ID", "动作": "Operation", "结果": "Result", "发起": "Started", "完成": "Completed",
  "已验证能力": "Verified capabilities", "未完成独立契约验证的兼容适配器保持关闭，不猜测外部系统接口。": "Compatibility adapters remain disabled until independently verified.",
  "类型": "Type", "能力": "Capability", "通用 API": "Generic API", "兼容适配器": "Compatibility adapter",
  "是": "Yes", "否": "No", "不限": "No limit", "有效": "Active", "已撤销": "Revoked", "时间": "Time",
  "渠道已保存，完成审批后才能创建令牌和发布商品。": "Channel saved. Approve it before creating tokens or publishing products.",
  "商品发布已通过核心价格校验并保存。": "The publication passed core price validation and was saved.",
  "令牌已撤销。": "Token revoked.", "令牌已创建，请现在复制并安全保存。": "Token created. Copy and store it securely now.",
  "数据已刷新。": "Data refreshed."
}));
const localize = (value, fallback) => {
  const text = String(value ?? "");
  if (text === "bridge_request_failed") {
    return currentLocale.toLowerCase().startsWith("en")
      ? "Channel data is temporarily unavailable. Configure a channel or try again later."
      : "暂时无法读取渠道数据，请先配置渠道或稍后重试。";
  }
  return currentLocale.toLowerCase().startsWith("en")
    ? (fallback || english.get(text) || text)
    : text;
};

const el = (tag, className, text) => {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = localize(text);
  return node;
};

const money = (minor, currency = "") => `${currency ? `${currency} ` : ""}${(Number(minor || 0) / 100).toFixed(2)}`;
const date = (value) => value ? new Date(value).toLocaleString(currentLocale) : "—";
const lines = (value) => String(value || "").split(/\r?\n/).map((item) => item.trim()).filter(Boolean);

function field(label, name, options = {}) {
  const wrap = el("label", "field");
  wrap.append(el("span", "field-label", label));
  const input = options.type === "textarea" ? el("textarea", "control") : el("input", "control");
  input.name = name;
  if (options.type && options.type !== "textarea") input.type = options.type;
  if (options.placeholder) input.placeholder = localize(options.placeholder);
  if (options.value !== undefined) input.value = options.value;
  if (options.type === "textarea") input.rows = options.rows || 3;
  wrap.append(input);
  return { wrap, input };
}

function selectField(label, name, options) {
  const wrap = el("label", "field");
  wrap.append(el("span", "field-label", label));
  const input = el("select", "control");
  input.name = name;
  options.forEach(([value, text]) => input.append(new Option(localize(text), value)));
  wrap.append(input);
  return { wrap, input };
}

const configurationTypes = [
  ["single_line", "单行文本"], ["multi_line", "多行文本"], ["integer", "整数"],
  ["decimal", "小数"], ["boolean", "布尔值"], ["single_select", "单选项"],
  ["multi_select", "多选项"], ["date", "日期"], ["datetime", "日期时间"],
  ["tree_select", "树选项路径"], ["address", "地址路径"], ["file", "文件令牌"],
];

function configurationValueEditor() {
  const root = el("div", "value-editor");
  const add = (initial = {}) => {
    const row = el("div", "value-row");
    const token = field("字段令牌", "fieldToken", { placeholder: localize("由购买配置页面提供") });
    const type = selectField("字段类型", "fieldType", configurationTypes);
    const value = field("字段值", "fieldValue", { placeholder: localize("多值使用英文逗号分隔") });
    type.input.value = initial.type || "single_line";
    token.input.value = initial.fieldToken || "";
    value.input.value = initial.value || "";
    const remove = button("移除", "danger");
    remove.onclick = () => row.remove();
    row.append(token.wrap, type.wrap, value.wrap, remove);
    root.append(row);
  };
  const values = () => [...root.querySelectorAll(".value-row")].map((row) => {
    const fieldToken = row.querySelector("[name=fieldToken]").value.trim();
    const type = row.querySelector("[name=fieldType]").value;
    const raw = row.querySelector("[name=fieldValue]").value.trim();
    if (!fieldToken) throw new Error(localize("每个配置项都必须填写字段令牌", "Every configuration value needs a field token"));
    const result = { fieldToken, type };
    if (type === "single_line" || type === "multi_line") result.textValue = raw;
    else if (type === "integer") result.integerValue = raw;
    else if (type === "decimal") result.decimalValue = raw;
    else if (type === "boolean") {
      if (raw !== "true" && raw !== "false") throw new Error(localize("布尔值只能填写 true 或 false", "Boolean values must be true or false"));
      result.booleanValue = raw === "true";
    } else if (type === "date") result.dateValue = raw;
    else if (type === "datetime") result.dateTimeValue = raw;
    else {
      const tokens = raw.split(",").map((item) => item.trim()).filter(Boolean);
      if (type === "single_select" || type === "multi_select") result.optionTokens = tokens;
      if (type === "tree_select") result.treeOptionTokens = tokens;
      if (type === "address") result.addressOptionTokens = tokens;
      if (type === "file") result.fileTokens = tokens;
    }
    return result;
  });
  return { root, add, values };
}

function button(label, tone = "quiet") {
  const node = el("button", `button button-${tone}`, label);
  node.type = "button";
  return node;
}

export function mount(context) {
  if (!context?.container) throw new Error("plugin container required");
  currentLocale = String(context.locale || document.documentElement.lang || "zh-CN");
  const root = el("main", "channel-workbench");
  const tables = createChannelTableRenderer(root, {
    locale: currentLocale, translate: localize, failure: localize("bridge_request_failed"),
  });
  const table = (columns, rows, paging) => tables.render(columns, rows, "暂无记录", paging);
  root.dataset.testid = "idc-channel-admin-page";
  root.innerHTML = `
    <style>
      .channel-workbench{--chn-ink:#18212b;--chn-muted:#687482;--chn-line:#d8dee5;--chn-panel:#fff;--chn-soft:#f5f7f9;--chn-primary:#1677ff;--chn-good:#13795b;--chn-bad:#b64235;box-sizing:border-box;max-width:1280px;margin:0 auto;padding:20px;color:var(--chn-ink);font:14px/1.5 ui-sans-serif,system-ui,sans-serif;min-height:720px}.channel-workbench *{box-sizing:border-box}
      .mast{display:flex;justify-content:space-between;gap:20px;align-items:center;padding:18px 20px;border:1px solid var(--chn-line);border-radius:8px;background:#fff}.eyebrow{color:var(--chn-muted);font-size:12px}.mast h1{font-size:20px;line-height:1.3;margin:3px 0}.mast p{color:var(--chn-muted);margin:0;max-width:760px}.live{display:flex;align-items:center;gap:8px;color:var(--chn-good);white-space:nowrap}.live:before{content:"";width:8px;height:8px;border-radius:50%;background:#35a878}
      .toolbar{display:grid;grid-template-columns:minmax(260px,1fr) auto;gap:12px;margin:18px 0}.select,.control{width:100%;border:1px solid var(--chn-line);border-radius:6px;background:#fff;color:var(--chn-ink);padding:9px 11px;font:inherit}.select:focus,.control:focus{outline:2px solid #1677ff22;border-color:var(--chn-primary)}
      .tabs{display:flex;gap:5px;overflow:auto;padding:5px;border:1px solid var(--chn-line);border-radius:11px;background:#e8edf0}.tab{white-space:nowrap;border:0;border-radius:7px;background:transparent;padding:9px 14px;color:#52606d;cursor:pointer}.tab[aria-selected="true"]{background:#fff;color:var(--chn-ink);box-shadow:0 3px 10px #18212b14}
      .panel{display:none;margin-top:14px}.panel.active{display:block}.grid{display:grid;grid-template-columns:repeat(12,minmax(0,1fr));gap:14px}.card{grid-column:span 12;background:var(--chn-panel);border:1px solid var(--chn-line);border-radius:8px;padding:18px}.card-half{grid-column:span 6}.card-third{grid-column:span 4}.card h2{font-size:16px;margin:0 0 4px}.card .sub{margin:0 0 15px;color:var(--chn-muted)}.form-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.field{display:grid;gap:5px}.field-wide{grid-column:1/-1}.field-label{font-size:12px;color:#56616d}.actions{display:flex;gap:8px;flex-wrap:wrap;margin-top:14px}.button{border:1px solid var(--chn-line);border-radius:6px;background:#fff;padding:8px 13px;color:var(--chn-ink);font:inherit;cursor:pointer}.button:hover{border-color:#91caff;color:#0958d9}.button-primary{background:var(--chn-primary);border-color:var(--chn-primary);color:#fff}.button-warn{border-color:#e1a23c;color:#8a5500;background:#fff8e9}.button-danger{border-color:#e6aaa3;color:var(--chn-bad);background:#fff6f5}.button:disabled{opacity:.45;cursor:not-allowed}
      .metric-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(170px,1fr));gap:10px}.metric{border:1px solid #e5e9ed;border-radius:6px;background:var(--chn-soft);padding:12px}.metric small{color:var(--chn-muted)}.metric strong{display:block;font-size:19px;margin-top:4px}.pill{display:inline-flex;align-items:center;border-radius:999px;padding:3px 8px;background:#edf1f3;font-size:12px}.pill-good{background:#e6f4ef;color:var(--chn-good)}.pill-warn{background:#fff1d8;color:#885400}.pill-bad{background:#fde9e6;color:var(--chn-bad)}
      .value-editor{display:grid;gap:8px}.value-row{display:grid;grid-template-columns:minmax(150px,1fr) minmax(130px,.7fr) minmax(190px,1.4fr) auto;gap:8px;align-items:end;padding:10px;border:1px solid #e5e9ed;border-radius:7px;background:#fafbfc}.value-row .field{min-width:0}
      .table-shell{overflow:auto;border:1px solid var(--chn-line);border-radius:9px}.data-table{width:100%;border-collapse:collapse;min-width:680px}.data-table th,.data-table td{text-align:left;padding:10px 11px;border-bottom:1px solid #e8ecef;vertical-align:top}.data-table th{background:#f5f7f8;color:#5b6670;font-size:12px;font-weight:600}.data-table tr:last-child td{border-bottom:0}.empty{padding:26px;text-align:center;border:1px dashed #c9d1d8;border-radius:9px;color:var(--chn-muted);background:#fafbfb}.notice{position:sticky;bottom:14px;margin:14px auto 0;max-width:760px;min-height:0;border-radius:9px;padding:0;background:#fff;box-shadow:0 10px 30px #18212b20;overflow:hidden}.notice:not(:empty){padding:11px 14px;border-left:4px solid var(--chn-good)}.notice.error{border-left-color:var(--chn-bad);color:var(--chn-bad)}.secret{font-family:ui-monospace,Consolas,monospace;background:#14202a;color:#d8f3e9;border-radius:8px;padding:11px;min-height:44px;word-break:break-all}
      .channel-workbench{max-width:none;min-height:0;padding:0}.mast{padding:14px 16px;border-radius:0;border-width:0 0 1px}.toolbar{margin:12px 0}.select,.control,.tabs,.tab,.card,.metric,.pill,.value-row,.table-shell,.empty,.notice,.secret,.button{border-radius:0}.tabs{gap:0;padding:0;background:#f5f7f9}.tab[aria-selected="true"]{box-shadow:none;border-bottom:2px solid var(--chn-primary)}.panel{margin-top:12px}.grid{gap:12px}.card{padding:16px}.notice{box-shadow:none}.live:before{border-radius:50%}
      @media(max-width:800px){.channel-workbench{padding:12px}.mast{align-items:flex-start;flex-direction:column}.toolbar{grid-template-columns:1fr}.card-half,.card-third{grid-column:span 12}.form-grid{grid-template-columns:1fr}.value-row{grid-template-columns:1fr}}
    </style>
    <header class="mast"><div><div class="eyebrow">${localize("渠道运营", "CHANNEL OPERATIONS")}</div><h1>${localize("下游渠道运营台", "Downstream channel operations")}</h1><p>${localize("统一管理渠道准入、商品发布、访问令牌、资金对账和调用健康。暂停渠道仍保留历史查询与还款。", "Manage channel approval, publications, tokens, reconciliation, and request health. Paused channels retain history, settlement, repayments, and support.")}</p></div><div class="live">${localize("核心账务已接入", "Core ledger connected")}</div></header>
    <div class="toolbar"><select class="select" data-testid="channel-selector"><option value="">${localize("请选择渠道")}</option></select><button class="button button-primary" data-testid="refresh-channel">${localize("刷新当前渠道")}</button></div>
    <nav class="tabs" role="tablist"></nav><div class="panels"></div><div class="notice" role="status" aria-live="polite"></div>`;
  context.container.replaceChildren(root);

  const request = (path, options = {}) => context.request(path.replace(/^\/+/, ""), options);
  const selector = root.querySelector("[data-testid=channel-selector]");
  const tabs = root.querySelector(".tabs");
  const panels = root.querySelector(".panels");
  const notice = root.querySelector(".notice");
  const state = { channels: [], selected: "", busy: false };
  const setNotice = (message, error = false) => { notice.textContent = localize(message || ""); notice.classList.toggle("error", error); };
  const guarded = async (work) => {
    if (state.busy) return;
    state.busy = true;
    root.querySelectorAll("button").forEach((item) => { item.disabled = true; });
    try { await work(); } catch (error) { setNotice(error?.message || String(error), true); }
    finally { state.busy = false; root.querySelectorAll("button").forEach((item) => { item.disabled = false; }); }
  };
  const selected = () => {
    const channelId = selector.value;
    if (!channelId) throw new Error(localize("请先选择渠道", "Select a channel first"));
    return channelId;
  };
  const pageSource = (path, params, total, field = "list", totalField = "total", prefix = "") => ({
    total,
    load: async ({ currentPage, pageSize }) => {
      const data = await request(path, { params: { ...params, [prefix ? prefix + "Page" : "page"]: currentPage, [prefix ? prefix + "PageSize" : "pageSize"]: pageSize } });
      return { list: data[field] || [], total: data[totalField] };
    },
    onPage: path === "admin/channel/accounts" ? updateChannelOptions : undefined,
    onError: () => setNotice("bridge_request_failed", true),
  });

  const definitions = [
    ["accounts", "渠道"], ["publications", "商品发布"], ["tokens", "令牌"],
    ["finance", "额度与结算"], ["health", "健康与审计"], ["capabilities", "能力清单"],
  ];
  const panelMap = new Map();
  definitions.forEach(([key, label], index) => {
    const tab = button(label);
    tab.className = "tab";
    tab.setAttribute("role", "tab");
    tab.setAttribute("aria-selected", index === 0 ? "true" : "false");
    tab.dataset.panel = key;
    const panel = el("section", `panel${index === 0 ? " active" : ""}`);
    panel.dataset.panel = key;
    panelMap.set(key, panel);
    tabs.append(tab); panels.append(panel);
  });
  tabs.addEventListener("click", (event) => {
    const tab = event.target.closest(".tab"); if (!tab) return;
    tabs.querySelectorAll(".tab").forEach((item) => item.setAttribute("aria-selected", String(item === tab)));
    panelMap.forEach((panel, key) => panel.classList.toggle("active", key === tab.dataset.panel));
    guarded(() => loadPanel(tab.dataset.panel));
  });

  function updateChannelOptions(rows) {
    const previous = selector.value;
    const selectedOption = Array.from(selector.options).find((option) => option.value === previous);
    selector.replaceChildren(new Option(localize("请选择渠道"), ""));
    rows.forEach((item) => selector.append(new Option(`${item.displayName} · ${item.channelId}`, item.channelId)));
    if (previous && selectedOption && !rows.some((item) => item.channelId === previous)) selector.append(selectedOption);
    selector.value = previous;
    state.selected = selector.value;
  }

  async function loadChannels() {
    const data = await request("admin/channel/accounts");
    state.channels = data?.list || [];
    updateChannelOptions(state.channels);
    renderAccounts(data.total);
  }

  function renderAccounts(total) {
    const panel = panelMap.get("accounts"); panel.replaceChildren();
    const grid = el("div", "grid");
    const listCard = el("article", "card"); listCard.append(el("h2", "", "渠道账户"), el("p", "sub", "渠道企业承担付款、信用与结算；服务所有者可以是渠道自身或受委托终端客户。"));
    listCard.append(table([
      ["渠道", (row) => `${row.displayName} (${row.channelId})`], ["客户主体", (row) => row.customerNo],
      ["状态", (row) => { const labels = { pending_approval: localize("待审批", "Pending approval"), approved: localize("已批准", "Approved"), paused: localize("已暂停", "Paused"), closed: localize("已终止", "Closed") }; const p = el("span", `pill ${row.statusCode === "approved" ? "pill-good" : row.statusCode === "paused" ? "pill-warn" : row.statusCode === "closed" ? "pill-bad" : ""}`, labels[row.statusCode] || row.statusCode); return p; }],
      ["允许新购", (row) => row.newSalesEnabled ? "是" : "否"], ["更新时间", (row) => date(row.updatedAt)],
    ], state.channels, pageSource("admin/channel/accounts", {}, total)));
    const createCard = el("article", "card card-half"); createCard.append(el("h2", "", "新增渠道"), el("p", "sub", "渠道编号创建后不能改绑客户主体。"));
    const channelId = field("渠道编号", "channelId", { placeholder: "channel_primary" });
    const customerNo = field("企业客户编号", "customerNo", { placeholder: "CUS_..." });
    const displayName = field("显示名称", "displayName");
    const form = el("div", "form-grid"); form.append(channelId.wrap, customerNo.wrap, displayName.wrap);
    const save = button("保存待审批渠道", "primary"); save.dataset.testid = "save-channel";
    save.onclick = () => guarded(async () => { await request("admin/channel/accounts", { method: "POST", body: { channelId: channelId.input.value.trim(), customerNo: customerNo.input.value.trim(), displayName: displayName.input.value.trim() } }); await loadChannels(); setNotice("渠道已保存，完成审批后才能创建令牌和发布商品。"); });
    createCard.append(form, el("div", "actions")); createCard.lastChild.append(save);
    const statusCard = el("article", "card card-half"); statusCard.append(el("h2", "", "合作状态"), el("p", "sub", "暂停会阻止新购和危险操作，历史订单、服务查询及还款仍保留。"));
    const actions = el("div", "actions");
    [["approve", "批准", "primary"], ["pause", "暂停停新", "warn"], ["resume", "恢复", "quiet"], ["close", "终止合作", "danger"]].forEach(([action, label, tone]) => { const b = button(label, tone); b.onclick = () => guarded(async () => { const channelIdValue = selected(); await request(`admin/channel/accounts/${encodeURIComponent(channelIdValue)}/status`, { method: "POST", body: { action } }); await loadChannels(); selector.value = channelIdValue; setNotice(localize(`渠道状态操作“${label}”已完成。`, `Channel action “${localize(label)}” completed.`)); }); actions.append(b); });
    statusCard.append(actions); grid.append(listCard, createCard, statusCard); panel.append(grid);
  }

  async function renderPublications() {
    const panel = panelMap.get("publications"); panel.replaceChildren(); const channelIdValue = selected();
    const data = await request("admin/channel/publications", { params: { channelId: channelIdValue } });
    const grid = el("div", "grid"); const listCard = el("article", "card"); listCard.append(el("h2", "", "已发布商品"), el("p", "sub", "渠道只看到售价与库存，不会看到核心成本、资源编号或上游凭据。"));
    listCard.append(table([["商品版本", (r) => r.productVersionId], ["币种", (r) => r.currency], ["售价", (r) => money(r.priceMinor, r.currency)], ["库存", (r) => r.stock], ["状态", (r) => r.status], ["公开标识", (r) => r.publicSlug]], data?.list || [], pageSource("admin/channel/publications", { channelId: channelIdValue }, data.total)));
    const formCard = el("article", "card"); formCard.append(el("h2", "", "发布或更新商品"), el("p", "sub", "保存前会用核心价格重新报价；价格不一致时直接拒绝。"));
    const fields = [field("商品版本 ID", "productVersionId", { type: "number" }), field("公开商品标识", "publicSlug"), field("配置版本", "configurationRevision"), field("计费周期标识", "billingTermToken"), field("币种", "currency", { value: "CNY" }), field("购买周期数", "purchaseCycles", { type: "number", value: 1 }), field("数量", "quantity", { type: "number", value: 1 }), field("库存", "stock", { type: "number", value: 0 }), selectField("发布状态", "status", [["published", "已发布"], ["paused", "已暂停"]])];
    const form = el("div", "form-grid"); fields.forEach((item) => form.append(item.wrap));
    const editorTitle = el("h3", "", "购买配置项");
    const editorHint = el("p", "sub", "从购买配置页面复制字段令牌，按字段类型填写值；多选、树、地址和文件令牌用英文逗号分隔。");
    const editor = configurationValueEditor();
    const addValue = button("添加配置项"); addValue.dataset.testid = "add-publication-value"; addValue.onclick = () => editor.add();
    const save = button("校验并保存发布", "primary");
    save.dataset.testid = "save-publication";
    save.onclick = () => guarded(async () => {
      await request("admin/channel/publications", { method: "POST", body: { channelId: channelIdValue, productVersionId: Number(fields[0].input.value), publicSlug: fields[1].input.value.trim(), configurationRevision: fields[2].input.value.trim(), billingTermToken: fields[3].input.value.trim(), currency: fields[4].input.value.trim().toUpperCase(), purchaseCycles: Number(fields[5].input.value), quantity: Number(fields[6].input.value), stock: Number(fields[7].input.value), status: fields[8].input.value, values: editor.values() } });
      await renderPublications(); setNotice("商品发布已通过核心价格校验并保存。");
    });
    const actions = el("div", "actions"); actions.append(addValue, save);
    formCard.append(form, editorTitle, editorHint, editor.root, actions); grid.append(listCard, formCard); panel.append(grid);
  }

  async function renderTokens() {
    const panel = panelMap.get("tokens"); panel.replaceChildren(); const channelIdValue = selected();
    const data = await request("admin/channel/tokens", { params: { channelId: channelIdValue } });
    const grid = el("div", "grid"); const listCard = el("article", "card"); listCard.append(el("h2", "", "访问令牌"), el("p", "sub", "令牌原文只显示一次；列表只保留用途、来源和状态。"));
    listCard.append(table([["作用域", (r) => (r.scopes || []).join(", ")], ["来源网段", (r) => (r.sourceCidrs || []).join(", ") || "不限"], ["到期", (r) => date(r.expiresAt)], ["状态", (r) => r.revokedAt ? "已撤销" : "有效"], ["操作", (r) => { const b = button("撤销", "danger"); b.disabled = Boolean(r.revokedAt); b.onclick = () => guarded(async () => { await request(`admin/channel/tokens/${r.id}`, { method: "DELETE" }); await renderTokens(); setNotice("令牌已撤销。"); }); return b; }]], data?.list || [], pageSource("admin/channel/tokens", { channelId: channelIdValue }, data.total)));
    const createCard = el("article", "card"); createCard.append(el("h2", "", "创建令牌"));
    const scopes = field("作用域（每行一个）", "scopes", { type: "textarea", value: "catalog.read\norder.create\norder.read\nservice.read\nsettlement.read", rows: 5 });
    const cidrs = field("来源网段（每行一个，可留空）", "sourceCidrs", { type: "textarea", rows: 5 });
    const expires = field("到期时间（可留空）", "expiresAt", { type: "datetime-local" }); const form = el("div", "form-grid"); form.append(scopes.wrap, cidrs.wrap, expires.wrap);
    const secret = el("div", "secret", "创建后在这里显示一次"); secret.dataset.testid = "one-time-token";
    const create = button("创建一次性令牌", "primary"); create.onclick = () => guarded(async () => { const out = await request("admin/channel/tokens", { method: "POST", body: { channelId: channelIdValue, scopes: lines(scopes.input.value), sourceCidrs: lines(cidrs.input.value), expiresAt: expires.input.value ? new Date(expires.input.value).getTime() : undefined } }); secret.textContent = out?.token || ""; await renderTokenListOnly(listCard, channelIdValue); setNotice("令牌已创建，请现在复制并安全保存。"); });
    createCard.append(form, el("div", "actions"), secret); createCard.querySelector(".actions").append(create); grid.append(listCard, createCard); panel.append(grid);
  }

  async function renderTokenListOnly(card, channelIdValue) {
    const data = await request("admin/channel/tokens", { params: { channelId: channelIdValue } });
    const old = card.querySelector(".channel-vben-table"); if (old) old.remove();
    card.append(table([["作用域", (r) => (r.scopes || []).join(", ")], ["来源网段", (r) => (r.sourceCidrs || []).join(", ") || "不限"], ["到期", (r) => date(r.expiresAt)], ["状态", (r) => r.revokedAt ? "已撤销" : "有效"], ["操作", (r) => { const b = button("撤销", "danger"); b.disabled = Boolean(r.revokedAt); b.onclick = () => guarded(async () => { await request(`admin/channel/tokens/${r.id}`, { method: "DELETE" }); await renderTokenListOnly(card, channelIdValue); setNotice("令牌已撤销。"); }); return b; }]], data?.list || [], pageSource("admin/channel/tokens", { channelId: channelIdValue }, data.total)));
  }

  async function renderFinance() {
    const panel = panelMap.get("finance"); panel.replaceChildren(); const channelIdValue = selected(); const data = await request("admin/channel/finance", { params: { channelId: channelIdValue } }); const grid = el("div", "grid");
    const currencyCard = el("article", "card"); currencyCard.append(el("h2", "", "逐币种资金总览"), el("p", "sub", "余额、信用、应收、实收、退款和服务成本均来自核心账本。")); const metrics = el("div", "metric-grid");
    (data?.currencies || []).forEach((row) => { const item = el("div", "metric"); const detail = currentLocale.toLowerCase().startsWith("en") ? `Credit available ${money(row.creditAvailableMinor)} · Receivable ${money(row.receivableMinor)} · Paid ${money(row.paidMinor)} · Refunded ${money(row.refundedMinor)} · Cost ${money(row.serviceCostMinor)}` : `信用可用 ${money(row.creditAvailableMinor)} · 应收 ${money(row.receivableMinor)} · 实收 ${money(row.paidMinor)} · 退款 ${money(row.refundedMinor)} · 成本 ${money(row.serviceCostMinor)}`; item.append(el("small", "", row.currencyCode), el("strong", "", money(row.availableBalanceMinor, row.currencyCode)), el("div", "", detail)); metrics.append(item); }); if (!metrics.children.length) metrics.append(el("div", "empty", "暂无资金记录")); currencyCard.append(metrics);
    const invoiceCard = el("article", "card"); invoiceCard.append(el("h2", "", "最近账单"), table([["账单", (r) => r.invoiceNo], ["订单", (r) => r.orderNo], ["金额", (r) => money(r.totalAmountMinor, r.currencyCode)], ["余额实收", (r) => money(r.paidAmountMinor)], ["信用结算", (r) => money(r.creditAmountMinor)], ["退款", (r) => money(r.refundedAmountMinor)], ["状态", (r) => r.status], ["创建时间", (r) => date(r.createdAt)]], data?.invoices || [], pageSource("admin/channel/finance", { channelId: channelIdValue }, data.invoiceTotal, "invoices", "invoiceTotal", "invoice")));
    const statementCard = el("article", "card card-half"); statementCard.append(el("h2", "", "信用账单"), table([["账单", (r) => r.statementNo], ["金额", (r) => money(r.totalMinor, r.currencyCode)], ["未还", (r) => money(r.outstandingMinor)], ["状态", (r) => r.status], ["到期", (r) => date(r.paymentDueAt)]], data?.creditStatements || [], pageSource("admin/channel/finance", { channelId: channelIdValue }, data.statementTotal, "creditStatements", "statementTotal", "statement")));
    const repaymentCard = el("article", "card card-half"); repaymentCard.append(el("h2", "", "还款流水"), table([["还款号", (r) => r.repaymentNo], ["信用账单", (r) => r.statementNo || "—"], ["金额", (r) => money(r.amountMinor, r.currencyCode)], ["状态", (r) => r.status], ["时间", (r) => date(r.createdAt)]], data?.creditRepayments || [], pageSource("admin/channel/finance", { channelId: channelIdValue }, data.repaymentTotal, "creditRepayments", "repaymentTotal", "repayment")));
    grid.append(currencyCard, invoiceCard, statementCard, repaymentCard); panel.append(grid);
  }

  async function renderHealth() {
    const panel = panelMap.get("health"); panel.replaceChildren(); const channelIdValue = selected(); const [health, audit] = await Promise.all([request("admin/channel/health", { params: { channelId: channelIdValue } }), request("admin/channel/requests", { params: { channelId: channelIdValue } })]); const grid = el("div", "grid");
    const card = el("article", "card card-third"); card.append(el("h2", "", "渠道健康")); const metrics = el("div", "metric-grid"); [["状态", health.statusCode], ["允许新购", health.newSalesEnabled ? "是" : "否"], ["有效令牌", health.activeTokenCount], ["在售商品", health.publicationCount], ["待投递事件", health.pendingWebhookCount], ["最近请求", date(health.lastRequestAt)]].forEach(([label, value]) => { const item = el("div", "metric"); item.append(el("small", "", label), el("strong", "", String(value))); metrics.append(item); }); card.append(metrics);
    const auditCard = el("article", "card"); auditCard.append(el("h2", "", "最近请求审计"), table([["请求号", (r) => r.requestId], ["动作", (r) => r.operation], ["状态", (r) => r.status], ["结果", (r) => r.resultCode || "—"], ["发起", (r) => date(r.createdAt)], ["完成", (r) => date(r.completedAt)]], audit?.list || [], pageSource("admin/channel/requests", { channelId: channelIdValue }, audit.total))); grid.append(card, auditCard); panel.append(grid);
  }

  async function renderCapabilities() {
    const panel = panelMap.get("capabilities"); panel.replaceChildren(); const data = await request("admin/channel/capabilities"); const grid = el("div", "grid"); const card = el("article", "card"); card.append(el("h2", "", "已验证能力"), el("p", "sub", "未完成独立契约验证的兼容适配器保持关闭，不猜测外部系统接口。"));
    card.append(table([["类型", (r) => r.type], ["能力", (r) => r.name], ["状态", (r) => r.status]], [...Object.entries(data?.operations || {}).map(([name, status]) => ({ type: "通用 API", name, status })), ...Object.entries(data?.adapters || {}).map(([name, status]) => ({ type: "兼容适配器", name, status }))])); grid.append(card); panel.append(grid);
  }

  async function loadPanel(key) {
    if (key === "accounts") return loadChannels();
    if (key === "capabilities") return renderCapabilities();
    if (key === "publications") return renderPublications();
    if (key === "tokens") return renderTokens();
    if (key === "finance") return renderFinance();
    if (key === "health") return renderHealth();
  }
  selector.addEventListener("change", () => { state.selected = selector.value; const active = tabs.querySelector('[aria-selected="true"]')?.dataset.panel; if (active && active !== "accounts") guarded(() => loadPanel(active)); });
  root.querySelector("[data-testid=refresh-channel]").addEventListener("click", () => guarded(async () => { const active = tabs.querySelector('[aria-selected="true"]')?.dataset.panel || "accounts"; await loadPanel(active); setNotice("数据已刷新。"); }));
  guarded(async () => { await loadChannels(); await renderCapabilities(); });
  return { unmount() { tables.destroy(); root.remove(); } };
}
