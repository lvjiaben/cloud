import {getLocale} from './locale.js';
import {t} from './commerce-copy.js';
import {createCartPayment,paymentStorageKey,paymentURL,browserModes} from './cart-payment.js';
import qrcode from './payment-qr.js';
import {syncSession,signIn,commerce,customerURL,getAccessToken,takeCart} from './commerce-session.js';
const esc=value=>String(value??'').replace(/[&<>"']/gu,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const text=(row,key,language=getLocale())=>row?.[key+(language==='en-US'?'EnUS':'ZhCN')]||row?.[key+'ZhCN']||row?.[key+'EnUS']||'';
const href=path=>path+(path.includes('?')?'&':'?')+'lang='+getLocale();
function drafts(){try{const value=JSON.parse(sessionStorage.getItem('website-main-purchase-drafts')||'{}');return value&&typeof value==='object'?value:{};}catch{return {};}}
export function amount(value, currency) {
  const digits = new Intl.NumberFormat(getLocale(), {
    style: "currency",
    currency,
  }).resolvedOptions().maximumFractionDigits;
  const minor = BigInt(value || "0"),
    scale = 10n ** BigInt(digits),
    absolute = minor < 0n ? -minor : minor;
  return `${currency} ${minor < 0n ? "-" : ""}${absolute / scale}${digits ? "." + String(absolute % scale).padStart(digits, "0") : ""}`;
}
function button(label, action, cls = "btn btn-primary") {
  const node = document.createElement("button");
  node.type = "button";
  node.className = cls;
  node.textContent = t(label);
  node.addEventListener("click", action);
  return node;
}
function message(target, value, error = false) {
  target.textContent = value;
  target.classList.toggle("error", error);
  target.classList.toggle("text-danger", error);
}
async function cartPage(root) {
  let cart,
    quote,
    busy = false,
    loading = false,
    identity = "",
    details = false,
    assets = [],
    selectedAsset = "",
    paymentChannels = [],
    availableBalance,
    preferredChannel = "",
    preferredMode = "",
    preferBalance = false,
    paymentError = "",
    paymentLoading = false,
    paymentRun = 0,
    assetsRun = 0;
  let paymentFlow;
  const savedDrafts = drafts();
  const keys = new Map();
  const quantityDrafts = new Map();
  root.innerHTML = `<div class="allocation-header cart-heading flex-wrap"><div class="allocation-header-title"><h1>${esc(t("Shopping cart checkout"))}</h1></div><a class="cart-back allocation-header-shoppingCart" href="${href("/products.html")}">← <span>${esc(t("Continue shopping"))}</span></a><ol class="cart-steps list-inline w-100 mb-0 mt-3 small"><li class="list-inline-item current"><b>1</b> ${esc(t("Review items"))}</li><li class="list-inline-item"><b>2</b> ${esc(t("Checkout details"))}</li><li class="list-inline-item"><b>3</b> ${esc(t("Invoice payment"))}</li></ol></div><div class="allocation-mian pr-3"><div class="cart-status mb-3 text-break" role="status" aria-live="polite"></div><section class="cart-list-section"><h2 class="cart-section-title h6 mb-4"></h2><div class="cart-items"></div></section><section class="cart-details" hidden></section></div><div class="allocation-footer cart-checkout pr-3"><div class="cart-coupon form-group"><label for="cart-coupon-code">${esc(t("Promotion code"))}</label><div class="input-group"><input id="cart-coupon-code" class="form-control" aria-label="${esc(t("Promotion code"))}" autocomplete="off" maxlength="24" placeholder="${esc(t("Enter promotion code"))}"><div class="input-group-append"><button type="button" class="btn btn-outline-primary coupon-apply">${esc(t("Apply"))}</button></div></div><select class="cart-coupon-assets form-control mt-2" aria-label="${esc(t("My coupons"))}" hidden></select></div><div class="configoption_total flex-column flex-md-row align-items-stretch align-items-md-center"><div class="cart-total allocation-footer-l"></div><div class="cart-actions allocation-footer-r flex-wrap justify-content-end mt-2 mt-md-0"></div></div></div>`;
  const state = root.querySelector(".cart-status"),
    items = root.querySelector(".cart-items"),
    total = root.querySelector(".cart-total"),
    actions = root.querySelector(".cart-actions"),
    detailPanel = root.querySelector(".cart-details"),
    checkout = root.querySelector(".cart-checkout");
  const setBusy = (value) => {
    busy = value;
    root.querySelectorAll("button").forEach((node) => {
      node.disabled = value || node.dataset.unavailable === "true";
    });
    const paymentLocked=Boolean(paymentFlow?.state);
    root.querySelector('.coupon-apply').disabled=value||paymentLocked;
    root.querySelector('#cart-coupon-code').disabled=value||paymentLocked;
    root.querySelector(".cart-coupon-assets").disabled = value||paymentLocked;
  };
  function validQuote() {
    return quote && quote.expiresAt > Date.now();
  }
  function period(item) {
    const unit = { hour: "Hour", day: "Day", month: "Month", year: "Year" }[
      item.periodUnitCode
    ];
    if (item.chargeTypeCode === "one_time") return t("One-time");
    return unit
      ? `${item.periodCount * item.purchaseCycles} ${t(unit)}`
      : text(savedDrafts[item.configurationReference], "termLabel") ||
          (savedDrafts[item.configurationReference]?.locale === getLocale()
            ? savedDrafts[item.configurationReference]?.termLabel
            : "") ||
          t("Billing period unavailable");
  }
  function configRows(item) {
    const persisted = (item.configuration || []).map((field) => ({
      label: text(field, "label"),
      value: text(field, "value"),
    }));
    const draft = savedDrafts[item.configurationReference];
    // Local declarations supplement persisted price choices, but never prices.
    for (const field of draft?.summary || []) {
      const label =
        text(field, "label") || (draft.locale === getLocale() ? field.label : "");
      const value =
        text(field, "value") || (draft.locale === getLocale() ? field.value : "");
      if (label && !persisted.some((existing) => existing.label === label))
        persisted.push({ label, value });
    }
    if (item.serverLoginHostname)
      persisted.push({ label: t("Hostname"), value: item.serverLoginHostname });
    return [...persisted, { label: t("Billing period"), value: period(item) }];
  }
  function render() {
    if (!paymentFlow?.state && !validQuote()) quote = undefined;
    root.dataset.cartStep = details ? "details" : "items";
    items.replaceChildren();
    total.replaceChildren();
    actions.replaceChildren();
    root.querySelector(".cart-section-title").textContent =
      `${t("Order items")} (${(cart?.items || []).reduce((sum, item) => sum + item.quantity, 0)} ${t("units")})`;
    root.querySelector(".cart-list-section").hidden = details;
    detailPanel.hidden = !details;
    root
      .querySelectorAll(".cart-steps li")
      .forEach((node, index) =>
        node.classList.toggle("current", index === (details ? 1 : 0)),
      );
    const rows = cart?.items || [];
    if (!cart && loading) {
      checkout.hidden = true;
      root.querySelector(".cart-section-title").textContent = t("Loading…");
      items.innerHTML =
        '<div class="cart-loading" aria-hidden="true"></div>';
      return;
    }
    if (!rows.length) {
      checkout.hidden = true;
      items.innerHTML = `<div class="cart-empty mb-4"><h2 class="h6">${esc(t("Your cart is empty"))}</h2><p>${esc(t("Choose a product and configure it to get started."))}</p><a class="btn btn-primary" href="${href("/products.html")}">${esc(t("Browse products"))}</a></div>`;
      return;
    }
    checkout.hidden = false;
    for (const item of rows) {
      const row = quote?.items?.find(
        (row) => row.cartItemNo === item.cartItemNo,
      );
      const unavailable = item.statusCode && item.statusCode !== "active";
      const card = document.createElement("article");
      card.className = "cart-product border-bottom pb-3 mb-4";
      card.dataset.item = item.cartItemNo;
      card.innerHTML = `<div class="cart-product-heading d-flex flex-wrap justify-content-between align-items-start"><div><h3 class="h6">${esc(text(item, "productName"))}</h3><p>${esc(t("Product configuration"))} · ${esc(period(item))}</p></div><div class="cart-product-tools d-flex flex-wrap align-items-center"></div></div><dl class="cart-configuration sky-config-form">${configRows(
        item,
      )
        .map(
          (field) =>
            `<div class="sky-form-group sky-config-row mb-2"><dt class="sky-md sky-form-label">${esc(field.label)}</dt><dd class="sky-md-100-p mb-0 text-break">${esc(field.value || "—")}</dd></div>`,
        )
        .join(
          "",
        )}</dl><div class="cart-product-footer"><div class="cart-quantity form-group"><label for="quantity-${esc(item.cartItemNo)}">${esc(t("Quantity"))}</label><div class="quantity-control input-group mb-2"><div class="input-group-prepend"><button type="button" class="quantity-less btn btn-outline-secondary" aria-label="${esc(t("Decrease quantity"))}">−</button></div><input id="quantity-${esc(item.cartItemNo)}" class="form-control" type="number" min="1" max="1000" step="1" value="${item.quantity}" inputmode="numeric"><div class="input-group-append"><button type="button" class="quantity-more btn btn-outline-secondary" aria-label="${esc(t("Increase quantity"))}">+</button></div></div></div><div class="cart-item-price"><span class="mr-2">${esc(t("Subtotal"))}</span><strong class="text-danger">${row ? esc(amount(row.totalAmountMinor, cart.currencyCode)) : "—"}</strong></div></div>`;
      const tools = card.querySelector(".cart-product-tools");
      const edit = document.createElement("a");
      edit.className = "cart-edit btn btn-link btn-sm pl-0 mr-2 mb-2";
      edit.textContent = t("Edit configuration");
      edit.href = href(
        `/configure.html?product=${encodeURIComponent(item.publicSlug)}&cartItemNo=${encodeURIComponent(item.cartItemNo)}`,
      );
      tools.append(edit);
      tools.append(
        button(
          "Remove",
          () =>
            run(async () => {
              await ensureIdentity();
              quote = undefined;
              details = false;
              const query = new URLSearchParams({
                expectedCartVersion: String(cart.version),
                expectedItemVersion: String(item.version),
              });
              cart = (
                await commerce(
                  `account/commerce/cart-items/${encodeURIComponent(item.cartItemNo)}?${query}`,
                  "DELETE",
                )
              ).cart;
              render();
              if (cart.items.length) await review();
            }),
          "cart-remove btn btn-link btn-sm text-danger mb-2",
        ),
      );
      const input = card.querySelector("input"),
        update = button(
          "Update",
          () =>
            run(async () => {
              if (!input.reportValidity()) return;
              const quantity = Number(input.value);
              if (
                !Number.isInteger(quantity) ||
                quantity < 1 ||
                quantity > 1000
              )
                return;
              if (quantity === item.quantity) return;
              await ensureIdentity();
              quote = undefined;
              details = false;
              cart = (
                await commerce(
                  `account/commerce/cart-items/${encodeURIComponent(item.cartItemNo)}`,
                  "PUT",
                  {
                    expectedCartVersion: cart.version,
                    expectedItemVersion: item.version,
                    quantity,
                  },
                )
              ).cart;
              quantityDrafts.delete(item.cartItemNo);
              render();
              await review();
            }),
          "btn btn-outline-primary btn-sm quantity-update",
        );
      card.querySelector(".cart-quantity").append(update);
      if (quantityDrafts.has(item.cartItemNo))
        input.value = quantityDrafts.get(item.cartItemNo);
      card.querySelector(".quantity-less").onclick = () => {
        input.stepDown();
        input.dispatchEvent(new Event("input"));
      };
      card.querySelector(".quantity-more").onclick = () => {
        input.stepUp();
        input.dispatchEvent(new Event("input"));
      };
      input.addEventListener("input", () => {
        quantityDrafts.set(item.cartItemNo, input.value);
        // Uncommitted edits must never leave the previous payable quote enabled.
        if (Number(input.value) !== item.quantity) {
          actions.replaceChildren();
          message(state, t("Update the quantity to refresh the total."));
        } else {
          quantityDrafts.delete(item.cartItemNo);
          renderActions();
          message(state, "");
        }
      });
      if (item.hasServerLogin) {
        input.disabled = true;
        input.max = "1";
        for (const node of card.querySelectorAll(".quantity-less, .quantity-more")) {
          node.disabled = true;
          node.dataset.unavailable = "true";
        }
        update.hidden = true;
      }
      if (unavailable) {
        const issue = document.createElement("p");
        issue.className = "cart-product-issue text-danger mt-3";
        issue.textContent = t(
          "This product is unavailable. Edit or remove it before checkout.",
        );
        card.append(issue);
      }
      if (row?.configurationRequired && !row.configurationFrozen) {
        const repair = document.createElement("a");
        repair.className = "cart-product-issue btn btn-outline-primary mt-3";
        repair.href = href(
          `/configure.html?product=${encodeURIComponent(item.publicSlug)}&quoteItemNo=${encodeURIComponent(row.quoteItemNo)}`,
        );
        repair.textContent = t("Complete configuration");
        card.append(repair);
      }
      items.append(card);
    }
    total.innerHTML = `<span class="d-block small">${esc(t("Total due"))}</span><strong class="d-block h4 text-danger">${quote ? esc(amount(quote.totalAmountMinor, quote.currencyCode)) : "—"}</strong><small class="d-block">${quote ? `${esc(t("Subtotal"))} ${esc(amount(quote.subtotalMinor, quote.currencyCode))} · ${esc(t("Discount"))} ${esc(amount(quote.discountMinor, quote.currencyCode))} · ${esc(t("Tax"))} ${esc(amount(quote.taxAmountMinor, quote.currencyCode))}` : esc(t("Prices are confirmed by the server."))}</small>`;
    if (details) {
      detailPanel.innerHTML = `<h2 class="cart-section-title h6 mb-4">${esc(t("Checkout details"))}</h2><div class="cart-detail-panel"><h3 class="h6">${esc(t("Order summary"))}</h3><div class="cart-detail-lines">${rows
        .map((item) => {
          const line = quote?.items?.find(
            (row) => row.cartItemNo === item.cartItemNo,
          );
          return `<div class="border-bottom pb-3 mb-3"><div class="cart-detail-line d-flex flex-wrap justify-content-between mb-3"><span>${esc(text(item, "productName"))}<small class="d-block">${esc(period(item))} × ${item.quantity}</small></span><strong>${line ? esc(amount(line.totalAmountMinor, cart.currencyCode)) : "—"}</strong></div><dl class="cart-detail-configuration sky-config-form">${configRows(
            item,
          )
            .map(
              (field) =>
                `<div class="sky-form-group sky-config-row mb-2"><dt class="sky-md sky-form-label">${esc(field.label)}</dt><dd class="sky-md-100-p mb-0 text-break">${esc(field.value || "—")}</dd></div>`,
            )
            .join("")}</dl></div>`;
        })
        .join("")}</div><dl class="cart-detail-totals sky-config-form">${[
        ["Subtotal", "subtotalMinor"],
        ["Discount", "discountMinor"],
        ["Tax", "taxAmountMinor"],
        ["Total due", "totalAmountMinor"],
      ]
        .map(
          ([label, key]) =>
            `<div class="sky-form-group sky-config-row mb-2"><dt class="sky-md sky-form-label">${esc(t(label))}</dt><dd class="sky-md-100-p mb-0">${quote ? esc(amount(quote[key], quote.currencyCode)) : "—"}</dd></div>`,
        )
        .join(
          "",
        )}</dl><div class="cart-payment-handoff mt-4"><div><h3 class="h6">${esc(t("Invoice payment"))}</h3><p>${esc(t("Confirm the order, then choose an available payment method or account balance on your invoice."))}</p></div></div><p class="cart-quote-validity small text-muted">${esc(t("Price valid until"))} ${quote ? esc(new Date(quote.expiresAt).toLocaleTimeString(getLocale())) : "—"}</p></div>`;
    }
    if (details) renderPaymentChoices();
    renderActions();
    setBusy(busy);
  }
  function renderPaymentChoices() {
    const panel = detailPanel.querySelector(".cart-payment-handoff");
    if(paymentFlow?.state){renderPaymentProgress(panel);return;}
    panel.innerHTML = `<div class="cart-payment-choices"><h3 class="h6">${esc(t("Payment method"))}</h3><div class="cart-payment-options"></div><p>${esc(t("Choose a payment method here, then confirm the order to continue payment."))}</p></div>`;
    const choices = panel.querySelector(".cart-payment-options");
    if (availableBalance !== undefined && BigInt(availableBalance) > 0n) {
      const balance = document.createElement("label");
      balance.className = "cart-payment-option d-flex align-items-start mb-3";
      balance.innerHTML = `<input class="mr-2 mt-1" type="radio" name="payment-choice" value="balance"><span>${esc(t("Account balance"))}<small class="d-block">${esc(t("Available"))} ${esc(amount(availableBalance, cart.currencyCode))}</small></span>`;
      balance.querySelector("input").checked = preferBalance;
      balance.querySelector("input").onchange = () => {
        preferBalance = true;
        preferredChannel = "";preferredMode="";renderActions();
      };
      // The current balance contract supports paying the complete invoice only.
      if (BigInt(availableBalance) < BigInt(quote.totalAmountMinor)) {
        balance.querySelector("input").disabled = true;
        balance.classList.add("unavailable");
        balance.append(document.createTextNode(t("Insufficient balance")));
      }
      choices.append(balance);
    }
    for (const channel of paymentChannels) for(const mode of browserModes(channel)) {
      const label = document.createElement("label");
      label.className = "cart-payment-option d-flex align-items-center mb-3";
      const input = document.createElement("input");
      input.type = "radio";
      input.className = "mr-2";
      input.name = "payment-choice";
      input.checked = !preferBalance && preferredChannel === channel.channelNo && preferredMode === mode;
      input.onchange = () => {
        preferredChannel = channel.channelNo;
        preferredMode=mode;preferBalance = false;renderActions();
      };
      const name = document.createElement("span");
      name.textContent = channel.displayName+(browserModes(channel).length>1?" · "+modeName(mode):"");
      label.append(input, name);
      choices.append(label);
    }
    if (!choices.children.length) {
      const note = document.createElement("p");
      note.textContent =
        (paymentLoading ? t("Loading…") : paymentError) ||
        t("No online payment methods are currently available.");
      choices.append(note);
    }
    if (paymentError && !paymentLoading)
      choices.append(button("Retry", () => { void loadPaymentChoices(); }, "btn btn-outline-primary"));
  }
  async function loadPaymentChoices() {
    const generation = ++paymentRun,
      expectedIdentity = identity,
      expectedCurrency = cart.currencyCode,
      expectedQuote = quote.quoteNo;
    paymentLoading = true;
    paymentError = "";
    paymentChannels = [];
    availableBalance = undefined;
    if (details) renderPaymentChoices();
    const results = await Promise.allSettled([
      commerce("account/billing/overview"),
      commerce(
        "account/payment-channels?currencyCode=" + cart.currencyCode,
        "GET",
        undefined,
        undefined,
        "idc-payment",
      ),
    ]);
    if (generation !== paymentRun) return;
    paymentLoading = false;
    if (
      identity !== expectedIdentity ||
      getAccessToken() !== expectedIdentity ||
      cart?.currencyCode !== expectedCurrency ||
      quote?.quoteNo !== expectedQuote
    ) {
      preferredChannel = "";
      preferBalance = false;
      if (details) renderPaymentChoices();
      return;
    }
    try {
      if (results[0].status === "fulfilled")
        availableBalance = results[0].value.overview.balances.find(
          (row) =>
            row.accountTypeCode === "customer_balance" &&
            row.currencyCode === cart.currencyCode &&
            row.statusCode === "active",
        )?.availableBalanceMinor;
      if (results[1].status === "fulfilled")
        paymentChannels = (results[1].value.channels || []).filter((row) =>
          browserModes(row).length,
        );
      if (results.some((result) => result.status === "rejected"))
        paymentError = t(
          "Could not load payment methods. Please retry.",
        );
      if (!paymentChannels.some((row) => row.channelNo === preferredChannel && browserModes(row).includes(preferredMode))){preferredChannel = "";preferredMode="";}
      if (
        preferBalance &&
        (availableBalance === undefined ||
          BigInt(availableBalance) < BigInt(quote.totalAmountMinor))
      )
        preferBalance = false;
    } catch {
      availableBalance = undefined;
      paymentChannels = [];
      preferBalance = false;
      preferredChannel = "";
      paymentError = t(
        "Could not load payment methods. Please retry.",
      );
    }
    if (details) renderPaymentChoices();
    renderActions();
  }
  function renderActions() {
    actions.replaceChildren();
    if(paymentFlow?.state){renderPaymentActions();return;}
    if (
      cart?.items?.some(
        (item) =>
          quantityDrafts.has(item.cartItemNo) &&
          Number(quantityDrafts.get(item.cartItemNo)) !== item.quantity,
      )
    )
      return;
    if (details)
      actions.append(
        button(
          "Previous step",
          () => {
            details = false;
            render();
          },
          "btn btn-outline-primary",
        ),
      );
    if (!quote) {
      actions.append(button("Refresh pricing", () => run(review)));
      return;
    }
    const blocked =
      cart.items.some(
        (item) => item.statusCode && item.statusCode !== "active",
      ) ||
      quote.items.some(
        (row) => row.configurationRequired && !row.configurationFrozen,
      ) || (details && (paymentLoading || (BigInt(quote.totalAmountMinor)>0n && !preferBalance && (!preferredChannel||!preferredMode))));
    const nextStep = async () => {
      await ensureIdentity();
      if (!validQuote()) {
        await review();
        return;
      }
      details = true;
      paymentLoading = true;
      paymentError = "";
      paymentChannels = [];
      availableBalance = undefined;
      render();
      // Preferences are optional; the invoice revalidates and confirms payment.
      void loadPaymentChoices();
      root.scrollIntoView({ behavior: "smooth" });
    };
    const action = button(
      details ? "Place order" : "Proceed to checkout",
      () => run(details ? submit : nextStep),
      details ? "btn btn-primary cart-submit" : "btn btn-primary",
    );
    action.dataset.unavailable = String(blocked);
    action.disabled = blocked || busy;
    actions.append(action);
  }
  async function ensureIdentity() {
    const previous = identity;
    if (!(await signIn())) throw new Error(t("Sign in before checking out"));
    if (previous !== getAccessToken()) {
      cart = undefined;
      paymentFlow=undefined;
      quote = undefined;
      details = false;
      throw new Error(t("Your session changed. Refresh the cart."));
    }
  }
  async function run(action) {
    if (busy) return;
    setBusy(true);
    message(state, "");
    try {
      await action();
    } catch (error) {
      if (action !== submit && !paymentFlow?.state) {
        quote = undefined;
        details = false;
      }
      render();
      message(state, paymentFlow?.state?.phase==='unknown'?progressText():error.message, true);
      if(!paymentFlow?.state)state.append(button("Refresh cart", load, "btn btn-outline-primary"));
    } finally {
      setBusy(false);
    }
  }
  async function review() {
    if (!cart?.items?.length) return;
    await ensureIdentity();
    quote = undefined;
    render();
    message(state, t("Updating price…"));
    quote = (
      await commerce("account/commerce/quotes", "POST", {
        expectedCartVersion: cart.version,
        assetNumbers: selectedAsset ? [selectedAsset] : [],
      })
    ).quote;
    if (!(await repairQuoteConfiguration())) {
      // Quote creation returns the authoritative post-write cart version.
      if (Number.isSafeInteger(quote.cartVersion)) cart.version = quote.cartVersion;
      cart.latestQuote = quote;
    }
    message(state, "");
    render();
  }
  async function repairQuoteConfiguration() {
    let repaired = false;
    for (const row of quote.items || []) {
      if (!row.configurationRequired || row.configurationFrozen) continue;
      const item = cart.items.find(
          (item) => item.cartItemNo === row.cartItemNo,
        ),
        draft = savedDrafts[item?.configurationReference];
      if (draft) {
        await commerce(
          `account/commerce/quote-items/${encodeURIComponent(row.quoteItemNo)}/purchase-configuration`,
          "POST",
          {
            publicSlug: draft.publicSlug,
            configurationRevision: draft.configurationRevision,
            values: draft.values,
            ...(draft.serverLoginToken ? { serverLoginToken: draft.serverLoginToken } : {}),
          },
        );
        repaired = true;
      }
    }
    if (repaired) {
      cart = (await commerce("account/commerce/cart")).cart;
      quote = cart.latestQuote || quote;
    }
    return repaired;
  }
  async function loadAssets() {
    const generation = ++assetsRun,
      expectedIdentity = identity,
      expectedCurrency = cart.currencyCode;
    try {
      const result = await commerce("account/marketing/coupons");
      if (generation !== assetsRun || identity !== expectedIdentity || getAccessToken() !== expectedIdentity || cart?.currencyCode !== expectedCurrency) return;
      assets = result.items || [];
      const select = root.querySelector(".cart-coupon-assets");
      select.replaceChildren(new Option(t("Do not use a coupon"), ""));
      for (const asset of assets.filter(
        (asset) =>
          asset.currencyCode === cart.currencyCode &&
          asset.status === "active" &&
          asset.validFrom <= Date.now() &&
          asset.validUntil > Date.now() &&
          asset.consumedUses < asset.usageLimit,
      ))
        select.add(
          new Option(
            (getLocale() === "en-US" ? asset.nameEnUs : asset.nameZhCn) ||
              asset.nameZhCn,
            asset.assetNo,
          ),
        );
      select.value = selectedAsset;
      select.hidden = select.options.length < 2;
    } catch {
      /* Coupon entry still reports its own errors; cart pricing remains available. */
    }
  }
  root
    .querySelector(".cart-coupon-assets")
    .addEventListener("change", (event) =>
      run(async () => {
        selectedAsset = event.target.value;
        details = false;
        await review();
      }),
    );
  root.querySelector(".coupon-apply").addEventListener("click", () =>
    run(async () => {
      const input = root.querySelector("#cart-coupon-code"),
        code = input.value.trim();
      if (!/^[A-Za-z0-9]{8,24}$/u.test(code))
        throw new Error(
          t("Enter a valid promotion code (8–24 letters or digits)."),
        );
      await ensureIdentity();
      let deviceId;
      try {
        deviceId = localStorage.getItem("website-main-coupon-device");
        if (!deviceId) {
          deviceId = crypto.randomUUID();
          localStorage.setItem("website-main-coupon-device", deviceId);
        }
      } catch {
        deviceId = crypto.randomUUID();
      }
      const { item } = await commerce(
        "account/marketing/coupons/claims",
        "POST",
        { code, deviceId },
      );
      selectedAsset = item.assetNo;
      details = false;
      await review();
      void loadAssets();
      input.value = "";
      message(state, t("Promotion applied. The total has been updated."));
    }),
  );
  async function load() {
    if (busy) return;
    loading = true;
    setBusy(true);
    quote = undefined;
    cart = undefined;
    details = false;
    message(state, t("Loading…"));
    render();
    try {
      await syncSession();
      identity = getAccessToken();
      if (!getAccessToken()) {
        message(state, t("Sign in before adding products to your cart."));
        state.append(button("Sign in", () => run(signIn)));
        return;
      }
      cart = await takeCart() || (await commerce("account/commerce/cart")).cart;
      quote = cart?.latestQuote;
      message(state, "");
      render();
      if (cart?.items?.length) {
        void loadAssets();
        if (!validQuote()) await review();
        else if (await repairQuoteConfiguration()) render();
      }
    } catch (error) {
      message(state, error.message, true);
      state.append(button("Retry", load, "btn btn-outline-primary"));
    } finally {
      loading = false;
      if (!cart) render();
      setBusy(false);
    }
  }
  function modeName(mode){return t(({alipay_page:'Alipay',alipay_wap:'Alipay',wechat_h5:'WeChat Pay',wechat_native:'WeChat Pay',epay_alipay:'Alipay',epay_wechat:'WeChat Pay',epay_cashier:'Online payment',unionpay_web:'UnionPay',unionpay_wap:'UnionPay',native:'Scan to pay',qr:'Scan to pay',qrcode:'Scan to pay',h5:'Mobile payment'})[mode]||'Online payment');}
  async function getPaymentFlow(){
   if(!paymentFlow){let storage;try{storage=sessionStorage;}catch{}paymentFlow=createCartPayment(commerce,{storage,key:await paymentStorageKey(getAccessToken()),authenticated:()=>Boolean(getAccessToken())});}
   return paymentFlow;
  }
  function progressText(){
   const progress=paymentFlow.state;
   return t(({paid:'Payment confirmed.',awaiting_identity:'Complete identity verification before paying, then check again.',awaiting_order_review:'This order is awaiting review. Check again after approval.',expired:'The payment period expired. Check the order status.',cancelled:'The order has been cancelled.',closed:'This order cannot currently be paid.',balance_unavailable:'Balance payment is unavailable for this invoice. Choose another payment method.',failed:'The payment was closed or failed. You can choose a method and try again.',unknown:'The payment submission result is unknown. Check status before taking any further action.',waiting:'Complete payment, then check the server confirmation.',login_required:'Your session expired. Sign in again.'})[progress.phase]||'Order created. Continue payment.');
  }
  function renderPaymentProgress(panel){
   const progress=paymentFlow.state;
   panel.innerHTML=`<div class="cart-payment-choices"><h3 class="h6">${esc(t('Payment method'))}</h3><div class="cart-payment-options"></div><p>${esc(progressText())}</p></div>`;
   const content=panel.querySelector('.cart-payment-options'),created=progress.created;
   if(created&&['waiting','unknown'].includes(progress.phase)){
    const target=paymentURL(created.redirectUrl);
    if(created.redirectMethod==='QR'&&created.redirectUrl){
     try{qrcode.stringToBytes=qrcode.stringToBytesFuncs['UTF-8'];const code=qrcode(0,'M');code.addData(created.redirectUrl);code.make();const image=document.createElement('img');image.width=image.height=240;image.alt=t('Payment QR code');image.src=code.createDataURL(4,16);content.append(image);}catch{panel.querySelector('p').textContent=t('Could not display the payment QR code. Check payment status before continuing.');}
    }else if(created.redirectMethod==='GET'&&target){const link=document.createElement('a');link.href=target;link.target='_blank';link.rel='noopener noreferrer';link.className='btn btn-primary';link.textContent=t('Open payment page');content.append(link);}
    else if(created.redirectMethod==='POST'&&target){const form=document.createElement('form');form.method='POST';form.action=target;form.target='_blank';form.rel='noopener noreferrer';for(const [name,value]of Object.entries(created.redirectFields||{})){const input=document.createElement('input');input.type='hidden';input.name=name;input.value=String(value);form.append(input);}const submit=button('Open payment page',()=>{});submit.type='submit';form.append(submit);content.append(form);}
   }
   if(progress.phase==='awaiting_identity'&&customerURL('/account/identity')){const link=document.createElement('a');const target=new URL(customerURL('/account/identity'));if(progress.preparation?.contextNo)target.searchParams.set('payment_context',progress.preparation.contextNo);target.searchParams.set('return_to','/account/orders/'+progress.orderNo);link.href=target.href;link.target='_blank';link.rel='noopener noreferrer';link.textContent=t('Verify identity');content.append(link);}
   if(['failed','balance_unavailable'].includes(progress.phase)){
    // Reuse the existing payment radio nodes so an explicit retry can select another enabled mode.
    const holder=paymentFlow;paymentFlow=undefined;renderPaymentChoices();paymentFlow=holder;
    panel.querySelector('p').textContent=progressText();
    if(progress.phase==='balance_unavailable'){preferBalance=false;const input=panel.querySelector('input[value=balance]');if(input){input.checked=false;input.disabled=true;}}
   }
   const validity=detailPanel.querySelector('.cart-quote-validity');if(validity){validity.textContent=created?.expiresAt?t('Payment valid until')+' '+new Date(created.expiresAt).toLocaleString(getLocale()):'';}
  }
  function renderPaymentActions(){
   const progress=paymentFlow.state;
   if(progress.phase==='paid'){message(state,t('Payment confirmed.'));return;}
   if(progress.phase==='login_required'){actions.append(button('Sign in',()=>run(signIn)));return;}
   if(!['cancelled','closed','expired'].includes(progress.phase)&&!progress.submitted){
    const next=button('Continue payment',()=>run(async()=>{await ensureIdentity();await paymentFlow.pay({balance:preferBalance,channelNo:preferredChannel,modeCode:preferredMode});render();}));next.dataset.unavailable=String(!preferBalance&&(!preferredChannel||!preferredMode));next.disabled=next.dataset.unavailable==='true'||busy;actions.append(next);
   }
   actions.append(button('Check payment status',()=>run(async()=>{await ensureIdentity();await paymentFlow.check();render();}),'btn btn-outline-primary'));
  }
  async function resumePayment(){
   if(!getAccessToken())return;const flow=await getPaymentFlow();
   if(!flow.state){paymentFlow=undefined;return;}
   if(flow.state.choice){preferBalance=Boolean(flow.state.choice.balance);preferredChannel=flow.state.choice.channelNo||'';preferredMode=flow.state.choice.modeCode||'';}
   if(flow.state.ui){cart=flow.state.ui.cart;quote=flow.state.ui.quote;details=true;render();}
   try{await flow.check();if(['failed','balance_unavailable'].includes(flow.state.phase))await loadPaymentChoices();render();}catch(error){render();message(state,error.message,true);}
  }
  async function submit() {
    if (!validQuote()) {
      quote = undefined;
      throw new Error(t("The quote expired. Refresh pricing."));
    }
    await ensureIdentity();
    if (
      cart.items.some(
        (item) => item.statusCode && item.statusCode !== "active",
      ) ||
      quote.items.some(
        (row) => row.configurationRequired && !row.configurationFrozen,
      )
    )
      throw new Error(t("Complete configuration"));
    const key = "website-main-order-" + quote.quoteNo;
    let id = keys.get(key);
    try {
      id ||= sessionStorage.getItem(key);
    } catch {}
    if (!id) {
      id = crypto.randomUUID();
      keys.set(key, id);
      try {
        sessionStorage.setItem(key, id);
      } catch {}
    }
    if(BigInt(quote.totalAmountMinor)>0n&&!preferBalance&&(!preferredChannel||!preferredMode))throw new Error(t('Choose a payment method.'));
    const { checkout: result } = await commerce('account/commerce/orders','POST',{quoteNo:quote.quoteNo,idempotencyKey:id});
    const flow=await getPaymentFlow();
    const safeCart={...cart,items:cart.items.map(({serverLoginToken,...item})=>item)};
    flow.begin(result,{cart:safeCart,quote});
    if(BigInt(quote.totalAmountMinor)===0n)await flow.check();
    else await flow.pay({balance:preferBalance,channelNo:preferredChannel,modeCode:preferredMode});
    render();

  }
  document.addEventListener("lina:locale-changed", render);
  await load();
  await resumePayment();
}

export async function startCart(root=document.querySelector('#commerce-page')){if(root)await cartPage(root);}
if(typeof document!=='undefined'&&document.body?.dataset.page==='cart')await startCart();
