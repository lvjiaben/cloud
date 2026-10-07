import { createChannelTableRenderer } from "./channel-table-runtime.js";

const copy = (context, key, fallback) => {
  try { return context.t(`plugin.idc-channel.${key}`, fallback); } catch { return fallback; }
};

const inputField = (label, type = "text", placeholder = "") => {
  const wrap = document.createElement("label");
  wrap.style.cssText = "display:grid;gap:5px";
  const title = document.createElement("span");
  title.textContent = label;
  const input = type === "textarea" ? document.createElement("textarea") : document.createElement("input");
  if (type !== "textarea") input.type = type;
  if (placeholder) input.placeholder = placeholder;
  input.style.cssText = "width:100%;box-sizing:border-box;padding:8px;border:1px solid #cbd5e1;border-radius:6px;font:inherit";
  wrap.append(title, input);
  return { wrap, input };
};

const button = (label, primary = false) => {
  const element = document.createElement("button");
  element.type = "button";
  element.textContent = label;
  element.style.cssText = `border:1px solid ${primary ? "#176b4d" : "#cbd5e1"};border-radius:6px;background:${primary ? "#176b4d" : "#fff"};color:${primary ? "#fff" : "#172033"};padding:9px 16px;cursor:pointer;font:inherit`;
  return element;
};

export function mount(context) {
  if (!context?.container) throw new Error("plugin container required");
  const root = document.createElement("section");
  const tables = createChannelTableRenderer(root, {
    locale: context.locale || "zh-CN",
    translate: (value) => String(value ?? ""),
    failure: copy(context, "workspace.loadFailed", "暂时无法读取渠道数据，请稍后重试。"),
  });
  root.dataset.testid = "idc-channel-page";
  root.style.cssText = "padding:24px;max-width:1080px;margin:0 auto;color:#172033";
  const heading = document.createElement("h1");
  heading.textContent = copy(context, "title", "下游渠道管理");
  heading.style.cssText = "font-size:24px;margin:0 0 6px";
  const hint = document.createElement("p");
  hint.textContent = copy(context, "hint", "令牌只在创建时显示一次；页面不会回显已保存的令牌原文。");
  hint.style.cssText = "color:#64748b;margin:0 0 20px";
  const notice = document.createElement("p");
  notice.setAttribute("role", "status");
  notice.style.cssText = "min-height:22px;margin:8px 0;color:#176b4d";

  const capabilitiesCard = document.createElement("div");
  capabilitiesCard.dataset.testid = "channel-capabilities";
  capabilitiesCard.style.cssText = "border:1px solid #e2e8f0;border-radius:10px;background:#fff;padding:20px;margin-bottom:16px";
  const capabilitiesTitle = document.createElement("h2");
  capabilitiesTitle.textContent = copy(context, "capabilities", "当前能力");
  capabilitiesTitle.style.cssText = "font-size:18px;margin:0 0 12px";
  const capabilitiesBody = document.createElement("div");
  capabilitiesBody.textContent = copy(context, "loading", "正在读取能力...");
  capabilitiesCard.append(capabilitiesTitle, capabilitiesBody);

  const catalogCard = document.createElement("div");
  catalogCard.dataset.testid = "channel-product-publication";
  catalogCard.style.cssText = "border:1px solid #e2e8f0;border-radius:10px;background:#fff;padding:20px;margin-bottom:16px";
  const catalogTitle = document.createElement("h2");
  catalogTitle.textContent = copy(context, "catalogTitle", "商品发布");
  catalogTitle.style.cssText = "font-size:18px;margin:0 0 8px";
  const catalogHint = document.createElement("p");
  catalogHint.textContent = copy(context, "catalogHint", "查看已发布给指定渠道的商品价格和库存，不显示成本或上游凭据。");
  catalogHint.style.cssText = "color:#64748b;margin:0 0 12px";
  const catalogTools = document.createElement("div");
  catalogTools.style.cssText = "display:flex;gap:8px;align-items:end;flex-wrap:wrap";
  const catalogChannel = inputField(copy(context, "catalogChannelId", "渠道编号"), "text", "例如 channel_primary");
  catalogChannel.input.name = "catalogChannelId";
  catalogChannel.wrap.style.flex = "1 1 240px";
  const loadCatalog = button(copy(context, "loadCatalog", "读取商品目录"), true);
  catalogTools.append(catalogChannel.wrap, loadCatalog);
  const catalogBody = document.createElement("div");
  catalogBody.dataset.testid = "channel-product-catalog";
  catalogBody.style.cssText = "margin-top:16px;color:#475569";
  catalogBody.textContent = copy(context, "catalogEmpty", "填写渠道编号后读取已发布商品。");
  catalogCard.append(catalogTitle, catalogHint, catalogTools, catalogBody);

  const operationsCard = document.createElement("div");
  operationsCard.dataset.testid = "channel-operations-overview";
  operationsCard.style.cssText = "border:1px solid #e2e8f0;border-radius:10px;background:#fff;padding:20px;margin-bottom:16px";
  const operationsTitle = document.createElement("h2");
  operationsTitle.textContent = copy(context, "operationsTitle", "额度、健康与请求记录");
  operationsTitle.style.cssText = "font-size:18px;margin:0 0 12px";
  const operationsGrid = document.createElement("div");
  operationsGrid.style.cssText = "display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:12px";
  const operationCards = [
    ["quota", "quotaTitle", "quotaHint"],
    ["health", "healthTitle", "healthHint"],
    ["audit", "auditTitle", "auditHint"],
    ["settlement", "settlementTitle", "settlementHint"],
    ["customer-token", "customerTokenTitle", "customerTokenHint"],
  ];
  operationCards.forEach(([testid, titleKey, hintKey]) => {
    const item = document.createElement("article");
    item.dataset.testid = `channel-${testid}`;
    item.style.cssText = "border:1px solid #e2e8f0;border-radius:8px;padding:14px;background:#f8fafc";
    const itemTitle = document.createElement("strong");
    itemTitle.textContent = copy(context, titleKey, titleKey);
    const itemHint = document.createElement("p");
    itemHint.textContent = copy(context, hintKey, "该能力尚未开放");
    itemHint.style.cssText = "margin:6px 0 0;color:#64748b";
    item.append(itemTitle, itemHint);
    operationsGrid.append(item);
  });
  operationsCard.append(operationsTitle, operationsGrid);

  const tokenCard = document.createElement("div");
  tokenCard.dataset.testid = "channel-token-management";
  tokenCard.style.cssText = "border:1px solid #e2e8f0;border-radius:10px;background:#fff;padding:20px";
  const tokenTitle = document.createElement("h2");
  tokenTitle.textContent = copy(context, "tokenTitle", "访问令牌");
  tokenTitle.style.cssText = "font-size:18px;margin:0 0 12px";
  const tokenList = document.createElement("div");
  tokenList.dataset.testid = "channel-token-list";
  tokenList.textContent = copy(context, "loadingTokens", "正在读取令牌...");
  tokenList.style.cssText = "margin-bottom:16px;color:#475569";
  const grid = document.createElement("div");
  grid.style.cssText = "display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:14px";
  const channel = inputField(copy(context, "channelId", "渠道编号"), "text", "例如 channel_primary");
  channel.input.name = "channelId";
  const scopes = inputField(copy(context, "scopes", "作用域（每行一个）"), "textarea");
  scopes.input.name = "scopes";
  scopes.input.rows = 3;
  const sources = inputField(copy(context, "sourceCIDRs", "来源网段（每行一个，可选）"), "textarea");
  sources.input.name = "sourceCidrs";
  sources.input.rows = 3;
  const expires = inputField(copy(context, "expiresAt", "过期时间（可选）"), "datetime-local");
  expires.input.name = "expiresAt";
  [channel, scopes, sources, expires].forEach((field) => grid.append(field.wrap));
  const actions = document.createElement("div");
  actions.style.cssText = "display:flex;gap:8px;flex-wrap:wrap;margin-top:14px";
  const create = button(copy(context, "create", "创建一次性令牌"), true);
  const revoke = button(copy(context, "revoke", "撤销令牌"));
  const revokeId = inputField(copy(context, "revokeId", "要撤销的令牌编号"));
  revokeId.input.name = "revokeId";
  revokeId.input.style.maxWidth = "220px";
  actions.append(create, revoke, revokeId.wrap);
  const tokenOutput = inputField(copy(context, "tokenOutput", "新令牌（仅显示这一次）"), "textarea");
  tokenOutput.input.readOnly = true;
  tokenOutput.input.rows = 2;
  tokenOutput.input.dataset.testid = "one-time-token";
  tokenOutput.wrap.style.marginTop = "14px";
  tokenCard.append(tokenTitle, tokenList, grid, actions, tokenOutput.wrap);
  root.append(heading, hint, capabilitiesCard, catalogCard, operationsCard, tokenCard, notice);
  context.container.replaceChildren(root);

  const request = async (path, options = {}) => {
    if (typeof context.request === "function") {
      const bridgePath = path.replace(/^\/x\/idc-channel\/api\/v1\/?/, "").replace(/^\/+/, "");
      return context.request(bridgePath, options);
    }
    const response = await fetch(new URL(path, location.origin), {
      ...options,
      credentials: "same-origin",
      headers: {
        Accept: "application/json",
        ...(options.body ? { "Content-Type": "application/json" } : {}),
        ...(context.accessToken ? { Authorization: `Bearer ${context.accessToken}` } : {}),
      },
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok || String(payload.code ?? 0) !== "0") throw new Error(payload.message || `HTTP ${response.status}`);
    return payload.data;
  };
  const lines = (value) => value.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
  const setNotice = (message, error = false) => { notice.textContent = message; notice.style.color = error ? "#b91c1c" : "#176b4d"; };
  const pageSource = (path, params, total) => ({ total,
    load: ({ currentPage, pageSize }) => request(path, { params: { ...params, page: currentPage, pageSize } }),
    onError: () => setNotice(copy(context, "workspace.loadFailed", "暂时无法读取渠道数据，请稍后重试。"), true),
  });
  const renderCapabilities = (data) => {
    capabilitiesBody.replaceChildren();
    const summary = document.createElement("p");
    summary.textContent = `${copy(context, "genericApi", "LinaPro 通用接口")}: ${data?.genericApi ? copy(context, "enabled", "已启用") : copy(context, "disabled", "未启用")}`;
    capabilitiesBody.append(summary);
    const list = document.createElement("ul");
    Object.entries(data?.adapters || {}).forEach(([name, state]) => { const item = document.createElement("li"); item.textContent = `${name}: ${state}`; list.append(item); });
    Object.entries(data?.operations || {}).forEach(([name, state]) => { const item = document.createElement("li"); item.textContent = `${name}: ${state}`; list.append(item); });
    capabilitiesBody.append(list);
  };
  const renderTokens = (data) => {
    tokenList.replaceChildren();
    const rows = data?.list || [];
    tokenList.append(tables.render([
      ["ID", (item) => item.id],
      [copy(context, "channelId", "渠道编号"), (item) => item.channelId],
      [copy(context, "scopes", "作用域"), (item) => (item.scopes || []).join(", ")],
      [copy(context, "tokenStatus", "状态"), (item) => item.revokedAt ? copy(context, "revokedStatus", "已撤销") : copy(context, "active", "有效")],
      [copy(context, "createdAt", "创建时间"), (item) => item.createdAt || ""],
    ], rows, copy(context, "emptyTokens", "暂无令牌"), pageSource("/x/idc-channel/api/v1/admin/channel/tokens", {}, data.total)));
  };
  const renderCatalog = (data) => {
    catalogBody.replaceChildren();
    const rows = data?.list || [];
    catalogBody.append(tables.render([
      [copy(context, "catalogCurrency", "币种"), (item) => item.currency],
      [copy(context, "catalogPrice", "售价"), (item) => item.price],
      [copy(context, "catalogStock", "库存"), (item) => item.stock],
    ], rows, copy(context, "catalogNoItems", "这个渠道暂无已发布商品。"), pageSource("/x/idc-channel/api/v1/admin/channel/catalog", { channelId: catalogChannel.input.value.trim() }, data.total)));
  };
  const run = async (action) => {
    create.disabled = revoke.disabled = true;
    try { await action(); } catch (error) { setNotice(error?.message || String(error), true); } finally { create.disabled = revoke.disabled = false; }
  };
  create.addEventListener("click", () => run(async () => {
    if (!channel.input.value.trim()) throw new Error(copy(context, "channelRequired", "请填写渠道编号"));
    const expiry = expires.input.value.trim();
    const data = await request("/x/idc-channel/api/v1/admin/channel/tokens", { method: "POST", body: JSON.stringify({ channelId: channel.input.value.trim(), scopes: lines(scopes.input.value), sourceCidrs: lines(sources.input.value), expiresAt: expiry ? new Date(expiry).getTime() : undefined }) });
    tokenOutput.input.value = data?.token || "";
    setNotice(copy(context, "created", "令牌已创建。请立即复制，关闭页面后不会再次显示。"));
  }));
  revoke.addEventListener("click", () => run(async () => {
    const id = Number(revokeId.input.value);
    if (!Number.isSafeInteger(id) || id <= 0) throw new Error(copy(context, "revokeIdRequired", "请填写有效的令牌编号"));
    await request(`/x/idc-channel/api/v1/admin/channel/tokens/${id}`, { method: "DELETE" });
    tokenOutput.input.value = "";
    renderTokens(await request("/x/idc-channel/api/v1/admin/channel/tokens"));
    setNotice(copy(context, "revoked", "令牌已撤销"));
  }));
  loadCatalog.addEventListener("click", () => run(async () => {
    const channelId = catalogChannel.input.value.trim();
    if (!channelId) throw new Error(copy(context, "channelRequired", "请填写渠道编号"));
    renderCatalog(await request("/x/idc-channel/api/v1/admin/channel/catalog", { params: { channelId } }));
    setNotice(copy(context, "catalogLoaded", "商品目录已更新"));
  }));
  run(async () => { renderCapabilities(await request("/x/idc-channel/api/v1/admin/channel/capabilities")); renderTokens(await request("/x/idc-channel/api/v1/admin/channel/tokens")); });
  return { unmount() { tables.destroy(); root.remove(); }, update(next) { context = next; } };
}
