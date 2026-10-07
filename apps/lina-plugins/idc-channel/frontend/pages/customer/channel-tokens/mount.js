const make = (tag, className, text) => {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
};
const lines = (value) => String(value || "").split(/\r?\n/).map((item) => item.trim()).filter(Boolean);
const scopeOptions = [
  ["catalog.read", "查看商品目录", "View product catalog"],
  ["order.create", "创建订单", "Create orders"],
  ["order.read", "查看订单", "View orders"],
  ["service.read", "查看服务", "View services"],
  ["settlement.read", "查看结算", "View settlements"],
];
let currentLocale = "zh-CN";
const tr = (zh, en) => currentLocale.toLowerCase().startsWith("en") ? en : zh;
const time = (value) => value ? new Date(value).toLocaleString(currentLocale) : tr("不限", "No limit");
const scopeLabel = (scope) => {
  const option = scopeOptions.find(([value]) => value === scope);
  return option ? tr(option[1], option[2]) : tr("其他已授权能力", "Other authorized capability");
};

export function mount(context) {
  currentLocale = String(context?.locale || document.documentElement.lang || "zh-CN");
  const root = make("main", "token-page");
  root.dataset.testid = "channel-customer-token-page";
  root.innerHTML = `<style>
    .token-page{--ink:#17232d;--muted:#697681;--line:#dce2e6;--accent:#126f58;--soft:#eef7f4;--danger:#b34338;max-width:980px;margin:0 auto;padding:24px;color:var(--ink);font:14px/1.55 ui-sans-serif,system-ui,sans-serif}.token-page *{box-sizing:border-box}.page-head{margin-bottom:14px}.page-head h1{margin:0 0 6px;font-size:22px}.page-head p{margin:0;color:var(--muted);max-width:720px}.layout{display:grid;grid-template-columns:1.15fr .85fr;gap:14px}.card{border:1px solid var(--line);border-radius:12px;background:#fff;padding:18px}.card h2{font-size:16px;margin:0 0 4px}.sub{margin:0 0 14px;color:var(--muted)}.field{display:grid;gap:5px;margin-bottom:11px}.field span{font-size:12px;color:var(--muted)}.token-page textarea,.token-page input{width:100%;border:1px solid var(--line);border-radius:8px;padding:9px 10px;font:inherit}.scope-options{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.scope-options label{display:flex;align-items:center;gap:8px}.scope-options input{width:auto;margin:0}.token-page button{border:1px solid var(--accent);border-radius:8px;background:var(--accent);color:#fff;padding:9px 13px;font:inherit;cursor:pointer}.token-page button.secondary{background:#fff;color:var(--danger);border-color:#e3aaa4}.token{margin-top:12px;min-height:48px;padding:11px;border-radius:8px;background:#13242a;color:#d8f5eb;font-family:ui-monospace,Consolas,monospace;word-break:break-all}.list{display:grid;gap:9px}.item{border:1px solid var(--line);border-radius:9px;padding:12px}.item-head{display:flex;justify-content:space-between;gap:10px}.item small{display:block;color:var(--muted);margin-top:5px}.status{margin-top:12px;color:var(--accent)}.status.error{color:var(--danger)}.blocked{padding:24px;border:1px dashed #dfaaa5;border-radius:12px;background:#fff7f6;color:var(--danger)}
    @media(max-width:760px){.token-page{padding:12px}.layout{grid-template-columns:1fr}}
  </style><header class="card page-head"><h1>${tr("渠道访问令牌", "Channel access tokens")}</h1><p>${tr("仅商家已批准且完成实名的企业渠道可用。令牌只在创建时显示一次，请保存到安全的密钥管理工具中。", "Available only to merchant-approved, identity-verified enterprise channels. A token is shown only once when created; store it in a secure secret manager.")}</p></header><section class="layout"><article class="card create-card"><h2>${tr("创建令牌", "Create token")}</h2><p class="sub">${tr("只勾选实际需要的能力；来源网段留空表示不限制来源。", "Select only the capabilities you need. Leave source CIDRs empty to allow any source.")}</p><div class="field"><span>${tr("允许的能力", "Allowed capabilities")}</span><div class="scope-options">${scopeOptions.map(([value, zh, en], index) => `<label><input type="checkbox" name="scope" value="${value}"${index < 4 ? " checked" : ""} /> ${tr(zh, en)}</label>`).join("")}</div></div><label class="field"><span>${tr("来源网段（每行一个，可留空）", "Source CIDRs (one per line, optional)")}</span><textarea name="cidrs" rows="3"></textarea></label><label class="field"><span>${tr("到期时间（可留空）", "Expiry (optional)")}</span><input name="expires" type="datetime-local" /></label><button data-testid="create-own-token">${tr("创建并显示一次", "Create and show once")}</button><div class="token" data-testid="one-time-token">${tr("创建后在这里显示一次", "The new token will appear here once")}</div></article><article class="card"><h2>${tr("现有令牌", "Existing tokens")}</h2><p class="sub">${tr("这里不会回显令牌原文。", "Stored token values are never shown again.")}</p><div class="list" data-testid="own-token-list"></div></article></section><div class="status" role="status" aria-live="polite"></div>`;
  context.container.replaceChildren(root);
  const list = root.querySelector(".list");
  const status = root.querySelector(".status");
  const secret = root.querySelector(".token");
  const setStatus = (message, error = false) => { status.textContent = message || ""; status.classList.toggle("error", error); };
  const render = (rows) => {
    list.replaceChildren();
    if (!rows.length) { list.append(make("div", "", tr("暂无令牌", "No tokens"))); return; }
    rows.forEach((row) => {
      const item = make("div", "item");
      const head = make("div", "item-head");
      head.append(make("strong", "", tr("访问令牌", "Access token")));
      if (!row.revokedAt) { const revoke = make("button", "secondary", tr("撤销", "Revoke")); revoke.type = "button"; revoke.onclick = async () => { try { await context.request(`account/channel/tokens/${row.id}`, { method: "DELETE" }); await load(); setStatus(tr("令牌已撤销。", "Token revoked.")); } catch { setStatus(tr("撤销失败，请稍后重试。", "Revoke failed. Please try again."), true); } }; head.append(revoke); }
      item.append(head, make("small", "", `${tr("允许的能力", "Capabilities")}: ${(row.scopes || []).map(scopeLabel).join("、")}`), make("small", "", `${tr("来源", "Sources")}: ${(row.sourceCidrs || []).join(", ") || tr("不限", "No limit")}`), make("small", "", `${tr("创建", "Created")}: ${time(row.createdAt)} · ${tr("到期", "Expires")}: ${time(row.expiresAt)} · ${row.revokedAt ? tr("已撤销", "Revoked") : tr("有效", "Active")}`)); list.append(item);
    });
  };
  const load = async () => { const data = await context.request("account/channel/tokens"); render(data?.list || []); };
  root.querySelector("[data-testid=create-own-token]").onclick = async () => {
    try { const expires = root.querySelector("[name=expires]").value; const data = await context.request("account/channel/tokens", { method: "POST", body: { scopes: [...root.querySelectorAll("[name=scope]:checked")].map((item) => item.value), sourceCidrs: lines(root.querySelector("[name=cidrs]").value), expiresAt: expires ? new Date(expires).getTime() : undefined } }); secret.textContent = data?.token || ""; await load(); setStatus(tr("令牌已创建，请立即复制保存。", "Token created. Copy and store it now.")); } catch { setStatus(tr("创建失败，请确认渠道资格和填写内容。", "Creation failed. Check channel eligibility and the entered details."), true); }
  };
  load().catch(() => { root.querySelector(".layout").replaceWith(make("div", "blocked", tr("当前客户不是已批准的实名企业渠道。", "This customer is not an approved, identity-verified enterprise channel."))); setStatus(tr("此页面只对获批企业渠道开放。", "This page is available only to approved enterprise channels."), true); });
  return { unmount() { root.remove(); } };
}
