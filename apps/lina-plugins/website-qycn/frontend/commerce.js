// Website-owned purchase flow. Only published contracts cross plugin boundaries.
import { t, locale } from "./i18n.js";
import { escapeHTML as esc, customerURL, request, catalogBase } from "./api.js";
import { operatingSystemControl, regionControl, quantitySlider, generatedPassword } from "./purchase-controls.js";
let token = "";
let pendingSession;
const text = (row, key, language = locale) =>
  row?.[key + (language === "en-US" ? "EnUS" : "ZhCN")] ||
  row?.[key + "ZhCN"] ||
  row?.[key + "EnUS"] ||
  "";
const href = (path) =>
  path + (path.includes("?") ? "&" : "?") + "lang=" + locale;
const base = () => (token ? "account" : "public");
const draftsKey = "website-qycn-purchase-drafts";
function drafts() {
  try {
    return JSON.parse(sessionStorage.getItem(draftsKey) || "{}");
  } catch {
    return {};
  }
}
function saveDraft(reference, draft) {
  if (!reference) return;
  try {
    const items = drafts();
    items[reference] = draft;
    sessionStorage.setItem(
      draftsKey,
      JSON.stringify(Object.fromEntries(Object.entries(items).slice(-100))),
    );
  } catch {
    /* Checkout offers configuration again when storage is unavailable. */
  }
}
export function amount(value, currency) {
  const digits = new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
  }).resolvedOptions().maximumFractionDigits;
  const minor = BigInt(value || "0"),
    scale = 10n ** BigInt(digits),
    absolute = minor < 0n ? -minor : minor;
  return `${currency} ${minor < 0n ? "-" : ""}${absolute / scale}${digits ? "." + String(absolute % scale).padStart(digits, "0") : ""}`;
}
export async function syncSession() {
  if (pendingSession) return pendingSession;
  pendingSession = (async () => {
    const response = await fetch("/__website/session", {
      credentials: "same-origin",
      signal: AbortSignal.timeout(10000),
      redirect: "error",
    });
    if (response.ok) {
      const data = await response.json();
      if (data.checked) {
        token = typeof data.accessToken === "string" ? data.accessToken : "";
        return Boolean(token);
      }
    }
    return new Promise((resolve, reject) => {
      const target = customerURL("/auth/website-session");
      if (!target) {
        reject(
          new Error(
            t("Customer center is not configured. Contact the administrator."),
          ),
        );
        return;
      }
      const origin = new URL(target).origin,
        nonce = crypto.randomUUID(),
        frame = document.createElement("iframe");
      frame.hidden = true;
      frame.title = t("Customer session");
      frame.referrerPolicy = "no-referrer";
      const cleanup = () => {
        clearTimeout(timer);
        window.removeEventListener("message", receive);
        frame.remove();
      };
      const receive = (event) => {
        if (
          event.origin !== origin ||
          event.source !== frame.contentWindow ||
          event.data?.type !== "lina-website-session" ||
          event.data?.nonce !== nonce
        )
          return;
        token =
          typeof event.data.accessToken === "string"
            ? event.data.accessToken
            : "";
        cleanup();
        resolve(Boolean(token));
      };
      const timer = setTimeout(() => {
        cleanup();
        reject(
          new Error(t("Could not read your sign-in status. Please retry.")),
        );
      }, 10000);
      window.addEventListener("message", receive);
      frame.src =
        target +
        "?" +
        new URLSearchParams({ nonce, websiteOrigin: location.origin });
      document.body.append(frame);
    });
  })()
    .catch((error) => {
      token = "";
      throw error;
    })
    .finally(() => {
      pendingSession = undefined;
    });
  return pendingSession;
}
async function signIn() {
  if (await syncSession()) return true;
  const target = customerURL("/auth/website-session");
  location.assign(
    target +
      "?" +
      new URLSearchParams({
        login: "1",
        returnTo: location.pathname + location.search,
        websiteOrigin: location.origin,
      }),
  );
  return false;
}
export async function commerce(
  path,
  method = "GET",
  body,
  signal,
  owner = "idc-core",
) {
  const binary = body instanceof Blob;
  const paymentChannels =
    owner === "idc-payment" &&
    method === "GET" &&
    /^account\/payment-channels\?currencyCode=[A-Z]{3}$/u.test(path);
  const core =
    owner === "idc-core" &&
    (/^(public|account)\/(commerce|catalog)\//u.test(path) ||
      /^account\/marketing\/coupons(?:\/claims)?$/u.test(path) ||
      (method === "GET" && path === "account/billing/overview"));
  if (!core && !paymentChannels) throw new Error("Invalid commerce path");
  const account = path.startsWith("account/");
  if (account && !token) throw new Error(t("Sign in before checking out"));
  const response = await fetch(`/x/${owner}/api/v1/` + path, {
    method,
    credentials: "same-origin",
    redirect: "error",
    headers: {
      Accept: "application/json",
      "Accept-Language": locale,
      ...(body === undefined
        ? {}
        : {
            "Content-Type": binary
              ? "application/octet-stream"
              : "application/json",
          }),
      ...(account ? { Authorization: "Bearer " + token } : {}),
    },
    body: body === undefined ? undefined : binary ? body : JSON.stringify(body),
    signal: signal
      ? AbortSignal.any([signal, AbortSignal.timeout(12000)])
      : AbortSignal.timeout(12000),
  });
  if (response.status === 401) {
    token = "";
    throw new Error(t("Your session expired. Sign in again."));
  }
  const payload = await response.json();
  if (!response.ok || String(payload.code ?? 0) !== "0" || !payload.data)
    throw new Error(t("Could not complete the operation. Refresh and retry."));
  return payload.data;
}
function button(label, action, cls = "button") {
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
}
function options(items, prefix = "", language = locale) {
  return (items || []).flatMap((item) => [
    { ...item, display: prefix + text(item, "label", language) },
    ...options(
      item.children,
      prefix + text(item, "label", language) + " / ",
      language,
    ),
  ]);
}
function fieldInput(field, value) {
  const wrap = document.createElement("div");
  wrap.className = "purchase-field";
  const label = document.createElement("label");
  label.textContent = text(field, "label") + (field.required ? " *" : "");
  const id = "field-" + field.fieldToken;
  label.htmlFor = id;
  wrap.append(label);
  let input;
  const choices = options(field.options);
  const numericChoices = field.type === "integer" && ["radio", "select"].includes(field.displayControl);
  const choiceItems = numericChoices
    ? (field.numericChoices || []).map((number) => ({
        optionToken: number,
        display: (locale === "en-US" ? field.numericLabels?.[number]?.enUS : field.numericLabels?.[number]?.zhCN) || number,
      }))
    : choices;
  const currentChoice = numericChoices ? value.integerValue : (value.optionTokens || [])[0];
  const firstChoice = choiceItems.find((item) => !item.children?.length)?.optionToken;
  const choiceValue = choiceItems.some((item) => item.optionToken === currentChoice)
    ? currentChoice
    : firstChoice;
  if (field.type === "single_select" && choices.length && field.presentation === "operating_system") {
    input = operatingSystemControl(field, value);
  } else if (field.type === "single_select" && choices.length && field.presentation === "region") {
    input = regionControl(field, value);
  } else if (
    (numericChoices && field.displayControl === "radio") || (
    field.type === "single_select" &&
    choices.length > 0 &&
    (field.displayControl === "radio" ||
      ((!field.displayControl || field.displayControl === "auto") && choices.length <= 10))
  )) {
    input = document.createElement("fieldset");
    input.className = "purchase-options";
    input.dataset.control = "choices";
    input.setAttribute("aria-label", text(field, "label"));
    for (const [index, item] of choiceItems.entries()) {
      const option = document.createElement("label");
      const radio = document.createElement("input");
      radio.type = "radio";
      radio.id = id + "-choice-" + index;
      radio.name = id;
      radio.value = item.optionToken;
      radio.checked = (choiceValue ?? "") === item.optionToken;
      radio.required = Boolean(field.required);
      radio.disabled = Boolean(field.disabled);
      const caption = document.createElement("span");
      caption.textContent = item.display;
      option.append(radio, caption);
      input.append(option);
    }
  } else if (numericChoices) {
    input = document.createElement("select");
    for (const item of choiceItems) {
      const option = new Option(item.display, item.optionToken);
      option.selected = choiceValue === item.optionToken;
      input.add(option);
    }
  } else if (
    ["single_select", "multi_select", "tree_select", "address"].includes(
      field.type,
    )
  ) {
    input = document.createElement("select");
    input.multiple = ["multi_select", "tree_select", "address"].includes(
      field.type,
    );
    const available = new Set(choices.map((item) => item.optionToken));
    let selected = (
      value.optionTokens || value.treeOptionTokens || value.addressOptionTokens || []
    ).filter((token) => available.has(token));
    if (field.type === "single_select" && !selected.length && firstChoice) selected = [firstChoice];
    for (const item of choices) {
      const option = new Option(item.display, item.optionToken);
      option.selected = selected.includes(item.optionToken);
      input.add(option);
    }
  } else if (field.type === "multi_line") {
    input = document.createElement("textarea");
    input.value = value.textValue || "";
  } else {
    input = document.createElement("input");
    input.type =
      {
        boolean: "checkbox",
        date: "date",
        datetime: "datetime-local",
        integer: "number",
        decimal: "number",
        file: "file",
      }[field.type] || "text";
    if (field.type === "boolean") input.checked = Boolean(value.booleanValue);
    else if (field.type !== "file")
      input.value =
        value.textValue ??
        value.integerValue ??
        value.decimalValue ??
        (field.type === "integer" ? field.minimumInteger : field.type === "decimal" ? field.minimumDecimal : undefined) ??
        value.dateValue ??
        (value.dateTimeValue
          ? new Date(
              new Date(value.dateTimeValue).getTime() -
                new Date(value.dateTimeValue).getTimezoneOffset() * 60000,
            )
              .toISOString()
              .slice(0, 16)
          : "");
    if (field.type === "integer" || field.type === "decimal") {
      input.min = field.minimumInteger ?? field.minimumDecimal ?? "";
      input.max = field.maximumInteger ?? field.maximumDecimal ?? "";
      input.step = field.type === "integer" ? field.quantityStep || "1" : "any";
    }
    if (field.type === "file") {
      input.multiple = (field.uploadPolicy?.maximumFiles || 1) > 1;
      input.accept = (field.uploadPolicy?.allowedExtensions || []).join(",");
      input.required = Boolean(field.required) && !value.fileTokens?.length;
    }
  }
  input.id = id;
  input.dataset.token = field.fieldToken;
  input.dataset.wasDisabled = String(Boolean(field.disabled));
  input.required =
    Boolean(field.required) &&
    !["boolean", "integer", "decimal"].includes(field.type) &&
    !(field.type === "file" && value.fileTokens?.length);
  input.disabled ||= Boolean(field.disabled);
  if (field.maximumLength) input.maxLength = field.maximumLength;
  if (field.minimumLength) input.minLength = field.minimumLength;
  const sliderControl = field.type === "integer" && (field.displayControl === "slider" ||
    ((!field.displayControl || field.displayControl === "auto") && field.minimumInteger !== undefined && field.maximumInteger !== undefined && BigInt(field.maximumInteger) > BigInt(field.minimumInteger)));
  if (sliderControl) {
    wrap.append(quantitySlider(field, input));
  } else wrap.append(input);
  const description = document.createElement("small");
  description.textContent = [text(field, "description"), sliderControl ? "" : field.unit]
    .filter(Boolean)
    .join(" · ");
  wrap.append(description);
  return wrap;
}
function collect(form, view) {
  const existing = new Map(
    (view.values || []).map((value) => [value.fieldToken, value]),
  );
  return view.fields
    .filter((field) => field.visible)
    .map((field) => {
      if (field.disabled || field.type === "file")
        return (
          existing.get(field.fieldToken) || {
            fieldToken: field.fieldToken,
            type: field.type,
          }
        );
      const input = [...form.querySelectorAll("[data-token]")].find(
        (item) => item.dataset.token === field.fieldToken,
      );
      const value = { fieldToken: field.fieldToken, type: field.type };
      const arms = {
        integer: "integerValue",
        decimal: "decimalValue",
        date: "dateValue",
        datetime: "dateTimeValue",
        boolean: "booleanValue",
        single_select: "optionTokens",
        multi_select: "optionTokens",
        tree_select: "treeOptionTokens",
        address: "addressOptionTokens",
        file: "fileTokens",
      };
      const arm = arms[field.type] || "textValue";
      value[arm] =
        field.type === "boolean"
          ? input.checked
          : field.type === "integer" && input.dataset.control === "choices"
            ? input.querySelector("input:checked")?.value || ""
          : arm.endsWith("Tokens")
            ? Array.from(
                input.dataset.control === "choices"
                  ? input.querySelectorAll("input:checked")
                  : input.selectedOptions || [],
              )
                .map((option) => option.value)
                .filter(Boolean)
            : field.type === "datetime" && input.value
              ? new Date(input.value).toISOString()
              : input.value;
      return value;
    })
    .filter((value) => {
      // A customer may clear a number while editing. Omit the empty draft;
      // evaluation restores the first sellable value before authoritative pricing.
      return (
        !["integer", "decimal"].includes(value.type) ||
        (value.integerValue ?? value.decimalValue ?? "") !== ""
      );
    });
}
// Compare the submitted configuration, ignoring only object/field ordering.
// Never reuse a speculative quote when server evaluation changes values.
function configurationKey(revision, values) {
  return JSON.stringify([
    revision,
    [...values]
      .sort((left, right) => left.fieldToken.localeCompare(right.fieldToken))
      .map((value) =>
        Object.fromEntries(
          Object.entries(value).sort(([left], [right]) =>
            left.localeCompare(right),
          ),
        ),
      ),
  ]);
}
// One bounded catalog page per selection; independent from configuration pricing.
function configureCatalog(root, current) {
  const sidebar = root.querySelector(".purchase-catalog");
  const categories = sidebar.querySelector(".purchase-categories");
  const products = sidebar.querySelector(".purchase-catalog-products");
  const status = sidebar.querySelector(".catalog-status");
  const more = sidebar.querySelector(".catalog-more");
  let category = current.categoryNo || "",
    cursor = "",
    generation = 0,
    controller;
  async function load(append = false) {
    const run = ++generation;
    controller?.abort();
    controller = new AbortController();
    message(status, t("Loading products…"));
    more.hidden = true;
    if (!append) products.replaceChildren();
    const query = new URLSearchParams({ limit: "20" });
    if (category) query.set("categoryNo", category);
    if (append && cursor) query.set("cursor", cursor);
    try {
      const result = await request(`${catalogBase}?${query}`, {
        signal: AbortSignal.any([controller.signal, AbortSignal.timeout(8000)]),
      });
      if (run !== generation) return;
      categories.replaceChildren(
        ...[
          { categoryNo: "", nameZhCN: t("All categories") },
          ...(result.categories || []),
        ].map((item) => {
          const node = document.createElement("button");
          node.type = "button";
          node.textContent = text(item, "name");
          node.setAttribute(
            "aria-pressed",
            String(item.categoryNo === category),
          );
          node.addEventListener("click", () => {
            if (item.categoryNo === category) return;
            category = item.categoryNo;
            cursor = "";
            load();
          });
          return node;
        }),
      );
      for (const item of result.products || []) {
        const link = document.createElement("a");
        link.textContent = text(item, "name");
        if (item.availability === "unavailable") {
          link.setAttribute("aria-disabled", "true");
          link.title = t("Currently unavailable");
        } else {
          link.href = href(
            `/configure.html?product=${encodeURIComponent(item.publicSlug)}`,
          );
        }
        if (item.publicSlug === current.publicSlug)
          link.setAttribute("aria-current", "page");
        products.append(link);
      }
      cursor = result.nextCursor || "";
      more.hidden = !cursor;
      message(
        status,
        products.childElementCount ? "" : t("No products found."),
      );
    } catch (error) {
      if (run !== generation) return;
      message(status, error.message, true);
      status.append(button("Retry", () => load(append), "button outline"));
    }
  }
  more.addEventListener("click", () => load(true));
  load();
}

async function configure(root) {
  const query = new URLSearchParams(location.search),
    slug = query.get("product"),
    quoteItem = query.get("quoteItemNo"),
    cartItemNo = query.get("cartItemNo"),
    restoreKey =
      "website-qycn-configuration-" +
      slug +
      (cartItemNo ? "-" + cartItemNo : "");
  root.classList.add("product-configurator");
  root.innerHTML = `<div class="purchase-heading"><div><a href="${href("/products.html")}">← ${esc(t("Continue shopping"))}</a><h1>${esc(t("Configure product"))}</h1><p class="purchase-description"></p></div><a class="button outline" href="${href("/cart.html")}">${esc(t("Cart"))} →</a></div><div class="purchase-workspace"><aside class="purchase-catalog"><h2>${esc(t("Product categories"))}</h2><nav class="purchase-categories" aria-label="${esc(t("Product categories"))}"></nav><h3>${esc(t("Products"))}</h3><nav class="purchase-catalog-products" aria-label="${esc(t("Products"))}"></nav><div class="catalog-status" role="status"></div><button class="catalog-more" hidden>${esc(t("Load more products"))}</button></aside><div class="purchase-layout"><form class="purchase-form"><h2>${esc(t("Product configuration"))}</h2><div class="status" role="status"></div><div id="purchase-fields"></div><section id="server-login" class="server-login-fields" hidden></section><div class="purchase-controls"></div></form><aside class="purchase-summary"><h2>${esc(t("Order summary"))}</h2><p class="purchase-product-name"></p><div class="purchase-selection"></div><h3>${esc(t("Billing period"))}</h3><div class="purchase-terms"></div><div class="purchase-checkout"><div class="price-status" role="status"></div><div class="purchase-total"><span>${esc(t("Total"))}</span><strong id="purchase-price">—</strong></div><p class="purchase-price-note">${esc(t("Prices update automatically when you change the configuration."))}</p><button class="button purchase-add" disabled>${esc(t("Add to cart"))}</button></div></aside></div></div>`;
  // The mobile purchase bar changes height with status text and safe-area padding.
  // Keep the global utility button above its actual border box.
  const checkout = root.querySelector(".purchase-checkout");
  new ResizeObserver(() => {
    document.body.style.setProperty(
      "--purchase-checkout-height",
      `${checkout.getBoundingClientRect().height}px`,
    );
  }).observe(checkout, { box: "border-box" });
  const state = root.querySelector(".status"),
    form = root.querySelector("form"),
    fields = root.querySelector("#purchase-fields"),
    terms = root.querySelector(".purchase-terms");
  let view,
    product,
    busy = false,
    quote,
    quoteIdentity = "",
    selectedTerm,
    selectedTermToken = "",
    priceRun = 0,
    priceTimer,
    priceController,
    priceSelectionKey = "",
    pendingAddAttempt,
    editingItem,
    editingCartVersion;
  const priceState = root.querySelector(".price-status"),
    addButton = root.querySelector(".purchase-add"),
    priceLabel = root.querySelector("#purchase-price");
  const loginFields = root.querySelector("#server-login");
  let loginPolicyKey = "", loginToken = "", loginDirty = false,
    loginHostname = "", loginPasswordConfigured = false, loginPasswordNeedsReentry = false;
  function renderServerLogin() {
    const policy = view.serverLoginPolicy;
    const key = JSON.stringify(policy || null);
    if (loginPolicyKey === key) return;
    loginPolicyKey = key;
    loginFields.hidden = !policy?.hostnameEnabled && !policy?.passwordEnabled;
    if (loginFields.hidden) { loginFields.replaceChildren(); return; }
    loginFields.innerHTML = `<h3>${esc(t("Server login"))}</h3><p>${esc(t(loginPasswordConfigured ? "Leave the password blank to keep the existing password." : "Optional. Leave blank to generate automatically."))}</p><div class="server-login-status" role="status"></div>`;
    function row(name, type, value) {
      const wrap = document.createElement("div");
      wrap.className = "purchase-field";
      const label = document.createElement("label");
      const input = document.createElement("input");
      input.id = `server-${type === "password" ? "password" : "hostname"}`;
      input.type = type;
      input.value = value;
      input.autocomplete = type === "password" ? "new-password" : "off";
      input.placeholder = t(type === "password" && loginPasswordConfigured
        ? "Password is set. Leave blank to keep it."
        : "Generate automatically");
      input.maxLength = type === "password" ? 20 : 63;
      input.spellcheck = false;
      label.textContent = t(name);
      label.htmlFor = input.id;
      const controls = document.createElement("div");
      controls.className = "server-login-input";
      controls.append(input);
      input.addEventListener("input", () => {
        loginDirty = true;
        if (type !== "password") loginHostname = input.value;
        message(loginFields.querySelector(".server-login-status"), "");
      });
      wrap.append(label, controls);
      loginFields.append(wrap);
      return { input, controls };
    }
    if (policy.hostnameEnabled) row("Hostname", "text", loginHostname);
    if (policy.passwordEnabled) {
      const { input, controls } = row("Host password", "password", "");
      const reveal = button("Show password", () => {
        input.type = input.type === "password" ? "text" : "password";
        reveal.textContent = t(input.type === "password" ? "Show password" : "Hide password");
        reveal.setAttribute("aria-pressed", String(input.type === "text"));
      }, "server-password-visibility");
      reveal.setAttribute("aria-pressed", "false");
      const generate = button("Generate password", () => {
        input.value = generatedPassword(policy.password);
        input.dispatchEvent(new Event("input", { bubbles: true }));
      }, "server-password-generate");
      controls.append(reveal, generate);
      if (loginPasswordNeedsReentry)
        message(loginFields.querySelector(".server-login-status"), t("For your privacy, enter the host password again after signing in, or leave it blank to generate one."));
    }
  }
  async function saveServerLogin() {
    const hostname = loginFields.querySelector("#server-hostname")?.value || "";
    const password = loginFields.querySelector("#server-password")?.value || "";
    if (!loginDirty && loginToken) return loginToken;
    if (!hostname && !password && !loginToken) return undefined;
    const result = await commerce(
      `account/catalog/products/${encodeURIComponent(slug)}/purchase-configuration/server-login`,
      "POST", {
        configurationRevision: view.configurationRevision,
        hostname, password,
        ...(loginToken ? { previousServerLoginToken: loginToken } : {}),
      },
    );
    loginToken = result.serverLoginToken;
    loginHostname = result.hostname || "";
    loginPasswordConfigured = Boolean(result.passwordConfigured);
    loginDirty = false;
    const input = loginFields.querySelector("#server-password");
    if (input) {
      input.value = "";
      input.type = "password";
      input.placeholder = t(loginPasswordConfigured ? "Password is set. Leave blank to keep it." : "Generate automatically");
    }
    loginFields.querySelector("p").textContent = t(loginPasswordConfigured ? "Leave the password blank to keep the existing password." : "Optional. Leave blank to generate automatically.");
    message(loginFields.querySelector(".server-login-status"), t("Login settings saved."));
    return loginToken;
  }
  const setBusy = (value) => {
    busy = value;
    form.querySelectorAll("button, input, select, textarea").forEach((node) => {
      if (value) {
        node.dataset.wasDisabled = String(node.disabled);
        node.disabled = true;
      } else {
        node.disabled = node.dataset.wasDisabled === "true";
        delete node.dataset.wasDisabled;
      }
    });
    terms.querySelectorAll("input").forEach((node) => (node.disabled = value));
    addButton.disabled = value || !quote || !selectedTerm;
  };
  function renderFields() {
    renderServerLogin();
    const active = fields.contains(document.activeElement)
      ? document.activeElement.id
      : "";
    const selection = active && [
      document.activeElement.selectionStart,
      document.activeElement.selectionEnd,
    ];
    fields.replaceChildren(
      ...view.fields
        .filter((field) => field.visible)
        .map((field) =>
          fieldInput(
            field,
            view.values.find(
              (value) => value.fieldToken === field.fieldToken,
            ) || {},
          ),
        ),
    );
    if (active) {
      const restored = document.getElementById(active);
      restored?.focus({ preventScroll: true });
      if (selection && restored?.setSelectionRange && selection[0] !== null) {
        try {
          restored.setSelectionRange(...selection);
        } catch {}
      }
    }
    if (busy)
      fields
        .querySelectorAll("input, select, textarea")
        .forEach((input) => (input.disabled = true));
    fields.querySelectorAll('input[type="file"]').forEach((input) =>
      input.addEventListener("change", async () => {
        if (busy) return;
        setBusy(true);
        invalidatePrice();
        try {
          if (!(await signIn())) return;
          const field = view.fields.find(
              (field) => field.fieldToken === input.dataset.token,
            ),
            files = [...input.files],
            policy = field.uploadPolicy;
          if (
            !policy ||
            files.length > policy.maximumFiles ||
            files.some((file) => file.size > policy.maximumFileBytes) ||
            files.reduce((sum, file) => sum + file.size, 0) >
              policy.maximumTotalBytes
          )
            throw new Error(t("Files exceed the allowed size or count."));
          message(state, t("Uploading files…"));
          const fileTokens = [];
          for (const file of files) {
            const reserved = await commerce(
              `account/catalog/products/${encodeURIComponent(slug)}/purchase-configuration/file-uploads`,
              "POST",
              {
                configurationRevision: view.configurationRevision,
                fieldToken: field.fieldToken,
                fileName: file.name,
                sizeBytes: file.size,
              },
            );
            const uploaded = await commerce(
              `account/catalog/purchase-configuration/file-uploads/${encodeURIComponent(reserved.uploadToken)}/content`,
              "PUT",
              file,
            );
            if (uploaded.status !== "ready")
              throw new Error(t("File upload is not ready. Please retry."));
            fileTokens.push(uploaded.fileToken);
          }
          view.values = view.values.filter(
            (value) => value.fieldToken !== field.fieldToken,
          );
          view.values.push({
            fieldToken: field.fieldToken,
            type: "file",
            fileTokens,
          });
          input.required = false;
          message(state, t("Files uploaded. Review your configuration."));
        } catch (error) {
          input.value = "";
          message(state, error.message, true);
        } finally {
          setBusy(false);
          if (
            view.values.some(
              (value) =>
                value.fieldToken === input.dataset.token &&
                value.fileTokens?.length,
            )
          )
            queuePrice();
        }
      }),
    );
  }
  async function load() {
    message(state, t("Loading…"));
    fields.hidden = true;
    try {
      if (!slug || slug.length > 128)
        throw new Error(t("This content is unavailable or has been removed."));
      const results = await Promise.all([
        request(`${catalogBase}/${encodeURIComponent(slug)}`),
        request(
          `${catalogBase}/${encodeURIComponent(slug)}/purchase-configuration`,
        ),
      ]);
      product = results[0].product;
      configureCatalog(root, { ...product, publicSlug: slug });
      view = results[1];
      if (!view.ready || !Array.isArray(view.fields))
        throw new Error(
          t(
            "This product is not ready for configuration. Please try again later.",
          ),
        );
      view.values ||= [];
      let restored;
      try {
        restored = JSON.parse(sessionStorage.getItem(restoreKey) || "null");
      } catch {}
      if (cartItemNo) {
        if (!(await signIn())) return;
        const currentCart = (await commerce("account/commerce/cart")).cart;
        editingItem = currentCart.items.find(
          (item) => item.cartItemNo === cartItemNo && item.publicSlug === slug,
        );
        if (!editingItem)
          throw new Error(
            t("This cart item is no longer available. Refresh the cart."),
          );
        editingCartVersion = currentCart.version;
        loginToken = editingItem.serverLoginToken || "";
        loginHostname = editingItem.serverLoginHostname || "";
        loginPasswordConfigured = Boolean(editingItem.serverLoginPasswordConfigured);
        restored ||= drafts()[editingItem.configurationReference];
        addButton.textContent = t("Save changes");
      }
      if (
        restored?.configurationRevision === view.configurationRevision &&
        Array.isArray(restored.values)
      ) {
        view.values = restored.values;
        selectedTermToken = restored.billingTermToken || "";
        if (restored.serverLoginToken) loginToken = restored.serverLoginToken;
        loginHostname = restored.serverLoginHostname ?? loginHostname;
        loginPasswordConfigured = Boolean(restored.serverLoginPasswordConfigured || loginPasswordConfigured);
        loginPasswordNeedsReentry = Boolean(restored.passwordNeedsReentry);
      }
      root.querySelector("h1").textContent = text(product, "name");
      root.querySelector(".purchase-product-name").textContent = text(
        product,
        "name",
      );
      root.querySelector(".purchase-description").textContent = text(
        product,
        "summary",
      );
      renderFields();
      const currency = document.createElement("select");
      currency.id = "purchase-currency";
      currency.setAttribute("aria-label", t("Currency"));
      for (const code of product.currencies || [])
        currency.add(new Option(code, code));
      if ((product.currencies || []).includes(query.get("currency")))
        currency.value = query.get("currency");
      if ((product.currencies || []).includes(restored?.currencyCode))
        currency.value = restored.currencyCode;
      root.querySelector(".purchase-controls").replaceChildren(currency);
      fields.hidden = false;
      message(state, "");
      if (cartItemNo && !restored)
        message(state, t("Review this configuration before saving changes."));
      queuePrice(0);
    } catch (error) {
      message(state, error.message, true);
      state.append(button("Retry", load, "button outline"));
    }
  }
  function selectionSummary() {
    const selected = collect(form, view);
    return view.fields
      .filter((field) => field.visible && field.type !== "file")
      .map((field) => {
        const value = selected.find(
          (item) => item.fieldToken === field.fieldToken,
        );
        const row = {};
        for (const [language, suffix] of [
          ["zh-CN", "ZhCN"],
          ["en-US", "EnUS"],
        ]) {
          const choices = options(field.options, "", language);
          row["label" + suffix] = text(field, "label", language);
          row["value" + suffix] = !value
            ? ""
            : value.optionTokens
              ? value.optionTokens
                  .map(
                    (id) =>
                      choices.find((item) => item.optionToken === id)
                        ?.display || "",
                  )
                  .join(", ")
              : value.booleanValue !== undefined
                ? t(value.booleanValue ? "Yes" : "No", language)
                : field.type === "integer" && field.numericLabels?.[value.integerValue]
                ? (language === "zh-CN" ? field.numericLabels[value.integerValue].zhCN : field.numericLabels[value.integerValue].enUS) || value.integerValue
              : (value.integerValue ??
                  value.decimalValue ??
                  value.dateValue ??
                  value.dateTimeValue ??
                  value.textValue ??
                  "");
        }
        return row;
      });
  }
  function renderSelection() {
    root.querySelector(".purchase-selection").innerHTML = selectionSummary()
      .map(
        (field) =>
          `<div><span>${esc(text(field, "label"))}</span><strong>${esc(text(field, "value"))}</strong></div>`,
      )
      .join("");
  }
  function invalidatePrice() {
    priceSelectionKey = "";
    ++priceRun;
    clearTimeout(priceTimer);
    priceController?.abort();
    quote = undefined;
    selectedTerm = undefined;
    terms.replaceChildren();
    priceLabel.textContent = "—";
    addButton.disabled = true;
  }
  function currentSelectionKey() {
    return JSON.stringify([
      token,
      root.querySelector("#purchase-currency").value,
      configurationKey(view.configurationRevision, collect(form, view)),
    ]);
  }
  function queuePrice(delay = 120) {
    if (!view || busy) return;
    const selectionKey = currentSelectionKey();
    // Native number/select controls can fire input and change for the same edit.
    // Neither blur nor repeated events should restart an in-flight quote.
    if (selectionKey === priceSelectionKey) return;
    invalidatePrice();
    priceSelectionKey = selectionKey;
    renderSelection();
    message(priceState, t("Updating price…"));
    priceTimer = setTimeout(updatePrice, delay);
  }
  function chooseTerm(term) {
    selectedTerm = term;
    selectedTermToken = term.billingTermToken;
    priceLabel.textContent = amount(term.totalAmount, quote.currencyCode);
    addButton.textContent = t(
      term.status === "final"
        ? quoteItem
          ? "Save configuration"
          : cartItemNo
            ? "Save changes"
            : "Add to cart"
        : "Sign in to view your price",
    );
    addButton.disabled = busy;
  }
  async function updatePrice() {
    const run = ++priceRun;
    priceController?.abort();
    priceController = new AbortController();
    const signal = priceController.signal;
    if (!form.checkValidity()) {
      message(priceState, t("Complete the required options to see the price."));
      return;
    }
    message(priceState, t("Updating price…"));
    message(state, "");
    const identity = token,
      purchaseBase = base();
    try {
      const values = collect(form, view);
      const configurationRevision = view.configurationRevision;
      const currencyCode = root.querySelector("#purchase-currency").value;
      const endpoint = `${purchaseBase}/catalog/products/${encodeURIComponent(slug)}/purchase-configuration`;
      const evaluation = commerce(
        `${endpoint}/evaluate`,
        "POST",
        { configurationRevision, values },
        signal,
      );
      // The quote endpoint independently validates the full configuration.
      // Start both reads together, but publish only after evaluation agrees.
      const quoted = commerce(
        `${endpoint}/quote`,
        "POST",
        { configurationRevision, values, currencyCode },
        signal,
      ).then(
        (value) => ({ value }),
        (error) => ({ error }),
      );
      const evaluated = await evaluation;
      if (run !== priceRun || token !== identity) return;
      view = evaluated;
      view.values ||= [];
      renderFields();
      renderSelection();
      priceSelectionKey = currentSelectionKey();
      if (!view.valid) {
        priceController.abort();
        const labels = (view.issues || [])
          .map((issue) =>
            text(
              view.fields.find(
                (field) => field.fieldToken === issue.fieldToken,
              ),
              "label",
            ),
          )
          .filter(Boolean);
        message(
          priceState,
          t("Please check the product configuration.") +
            (labels.length ? " " + labels.join("、") : ""),
          true,
        );
        return;
      }
      let nextQuote;
      if (
        configurationKey(configurationRevision, values) ===
        configurationKey(view.configurationRevision, view.values)
      ) {
        const result = await quoted;
        if (result.error) throw result.error;
        nextQuote = result.value;
      } else {
        // Dynamic dependencies/defaults may normalize the submitted choices.
        nextQuote = await commerce(
          `${endpoint}/quote`,
          "POST",
          {
            configurationRevision: view.configurationRevision,
            values: view.values,
            currencyCode,
          },
          signal,
        );
      }
      if (run !== priceRun || token !== identity) return;
      quote = nextQuote;
      quoteIdentity = identity;
      terms.replaceChildren();
      const preferred =
        quote.terms?.find(
          (term) => term.billingTermToken === selectedTermToken,
        ) || quote.terms?.[0];
      for (const [index, term] of (quote.terms || []).entries()) {
        const option = document.createElement("label");
        option.className = "purchase-term";
        const radio = document.createElement("input");
        radio.type = "radio";
        radio.name = "billing-term";
        radio.value = String(index);
        radio.checked = term === preferred;
        radio.addEventListener("change", () => chooseTerm(term));
        option.append(radio);
        const caption = document.createElement("span");
        caption.innerHTML = `<strong>${esc(text(term, "label"))}</strong><small>${esc(amount(term.totalAmount, quote.currencyCode))}</small>`;
        option.append(caption);
        terms.append(option);
        if (term === preferred) chooseTerm(term);
      }
      message(
        priceState,
        quote.terms?.length
          ? ""
          : t("No purchase terms are currently available."),
      );
    } catch (error) {
      if (run !== priceRun || signal.aborted) return;
      priceSelectionKey = "";
      quote = undefined;
      addButton.disabled = true;
      message(priceState, t("Could not update the price. Please retry."), true);
      priceState.append(button("Retry", () => queuePrice(0), "button outline"));
    }
  }
  form.addEventListener("input", (event) => {
    if (event.target.type !== "file" && !loginFields.contains(event.target)) queuePrice();
  });
  form.addEventListener("change", (event) => {
    if (event.target.type !== "file" && !loginFields.contains(event.target)) queuePrice();
  });
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    queuePrice(0);
  });
  addButton.addEventListener("click", async () => {
    if (busy || !quote || !selectedTerm) return;
    if (selectedTerm.status === "final") return add(selectedTerm);
    setBusy(true);
    try {
      rememberConfiguration();
      if (await signIn()) {
        setBusy(false);
        queuePrice(0);
      }
    } catch (error) {
      message(priceState, error.message, true);
    } finally {
      setBusy(false);
    }
  });
  function rememberConfiguration() {
    const draft = {
      publicSlug: slug,
      configurationRevision: view.configurationRevision,
      values: collect(form, view),
      billingTermToken: selectedTerm?.billingTermToken || selectedTermToken,
      currencyCode: root.querySelector("#purchase-currency").value,
      purchaseCycles: selectedTerm?.minimumPurchaseCycles || 1,
      serverLoginHostname: loginFields.querySelector("#server-hostname")?.value || "",
      ...(loginToken ? { serverLoginToken: loginToken } : {}),
      serverLoginPasswordConfigured: loginPasswordConfigured,
      passwordNeedsReentry: Boolean(loginFields.querySelector("#server-password")?.value),
    };
    try {
      sessionStorage.setItem(restoreKey, JSON.stringify(draft));
    } catch {}
    return draft;
  }
  async function add(term) {
    if (busy || !quote || !quote.terms.includes(term)) return;
    setBusy(true);
    let reprice = false;
    message(state, t("Adding to cart…"));
    try {
      rememberConfiguration();
      if (!(await signIn())) return;
      if (quoteIdentity !== token) {
        reprice = true;
        quote = undefined;
        terms.replaceChildren();
        throw new Error(
          t("Your session changed. Review the configuration again."),
        );
      }
      if (editingItem?.quantity > 1 && (loginToken || loginFields.querySelector("#server-hostname")?.value || loginFields.querySelector("#server-password")?.value))
        throw new Error(t("Set this item's quantity to one in the cart, then configure the other servers separately."));
      const serverLoginToken = await saveServerLogin();
      rememberConfiguration();
      if (quoteItem) {
        await commerce(
          `account/commerce/quote-items/${encodeURIComponent(quoteItem)}/purchase-configuration`,
          "POST",
          {
            publicSlug: slug,
            configurationRevision: view.configurationRevision,
            values: view.values,
            ...(serverLoginToken ? { serverLoginToken } : {}),
          },
        );
        location.assign(href("/cart.html"));
        return;
      }
      const currencyCode = quote.currencyCode;
      const input = {
        publicSlug: slug,
        currencyCode,
        configurationRevision: quote.configurationRevision,
        values: view.values,
        billingTermToken: term.billingTermToken,
        purchaseCycles: term.minimumPurchaseCycles,
        quantity: 1,
        ...(serverLoginToken ? { serverLoginToken } : {}),
      };
      input.summary = selectionSummary();
      input.termLabelZhCN = term.labelZhCN || term.labelEnUS || "";
      input.termLabelEnUS = term.labelEnUS || term.labelZhCN || "";
      input.locale = locale;
      if (cartItemNo) {
        const { cart: updated } = await commerce(
          `account/commerce/cart-items/${encodeURIComponent(cartItemNo)}`,
          "PUT",
          {
            expectedCartVersion: editingCartVersion,
            expectedItemVersion: editingItem.version,
            quantity: editingItem.quantity,
            purchase: input,
          },
        );
        saveDraft(
          updated.items.find((item) => item.cartItemNo === cartItemNo)
            ?.configurationReference,
          input,
        );
        try {
          sessionStorage.removeItem(restoreKey);
        } catch {}
        location.assign(href("/cart.html"));
        return;
      }
      const { cart: before } = await commerce("account/commerce/carts", "POST", { currencyCode });
      // A lost response must retry with the original optimistic version. Taking
      // a fresh version would append a duplicate after an already committed add.
      const attemptKey = restoreKey + "-add-attempt";
      const selection = JSON.stringify([
        input.publicSlug,
        input.configurationRevision,
        input.values,
        input.billingTermToken,
        input.currencyCode,
        input.serverLoginToken || "",
      ]);
      let attempt = pendingAddAttempt;
      try {
        attempt ||= JSON.parse(sessionStorage.getItem(attemptKey) || "null");
      } catch {}
      if (
        attempt?.cartNo !== before.cartNo ||
        attempt?.selection !== selection
      ) {
        attempt = { cartNo: before.cartNo, version: before.version, selection };
        try {
          sessionStorage.setItem(attemptKey, JSON.stringify(attempt));
        } catch {}
      }
      pendingAddAttempt = attempt;
      const { cart: after } = await commerce("account/commerce/cart-items", "POST", {
        publicSlug: input.publicSlug,
        configurationRevision: input.configurationRevision,
        values: input.values,
        billingTermToken: input.billingTermToken,
        currencyCode: input.currencyCode,
        purchaseCycles: input.purchaseCycles,
        quantity: input.quantity,
        ...(input.serverLoginToken ? { serverLoginToken: input.serverLoginToken } : {}),
        expectedCartVersion: attempt.version,
      });
      const added = after.items.filter(
          (item) =>
            !before.items.some((old) => old.cartItemNo === item.cartItemNo),
        );
      if (added.length === 1 && after.version === before.version + 1)
        saveDraft(added[0].configurationReference, input);
      pendingAddAttempt = undefined;
      try {
        sessionStorage.removeItem(restoreKey);
        sessionStorage.removeItem(attemptKey);
      } catch {}
      location.assign(href("/cart.html"));
    } catch (error) {
      message(state, error.message, true);
      const link = document.createElement("a");
      link.href = href("/cart.html");
      link.textContent = t("Check the cart before adding again");
      state.append(link);
    } finally {
      setBusy(false);
      if (reprice) queuePrice(0);
    }
  }
  await load();
  // Failure to read authentication must not prevent anonymous configuration.
  syncSession()
    .then(() => {
      if (token !== quoteIdentity) queuePrice(0);
    })
    .catch(() => {});
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
    preferBalance = false,
    paymentError = "",
    paymentLoading = false,
    paymentRun = 0,
    assetsRun = 0;
  const savedDrafts = drafts();
  const keys = new Map();
  const quantityDrafts = new Map();
  root.classList.add("cart-workspace");
  root.innerHTML = `<div class="cart-heading"><div><a class="cart-back" href="${href("/products.html")}">← ${esc(t("Continue shopping"))}</a><h1>${esc(t("Shopping cart checkout"))}</h1></div><ol class="cart-steps"><li class="current"><b>1</b>${esc(t("Review items"))}</li><li><b>2</b>${esc(t("Checkout details"))}</li><li><b>3</b>${esc(t("Invoice payment"))}</li></ol></div><div class="status cart-status" role="status" aria-live="polite"></div><section class="cart-list-section"><h2 class="cart-section-title"></h2><div class="cart-items"></div></section><section class="cart-details" hidden></section><div class="cart-checkout"><div class="cart-checkout-inner"><div class="cart-total"></div><div class="cart-coupon"><label for="cart-coupon-code">${esc(t("Promotion code"))}</label><div><input id="cart-coupon-code" aria-label="${esc(t("Promotion code"))}" autocomplete="off" maxlength="24" placeholder="${esc(t("Enter promotion code"))}"><button type="button" class="button outline coupon-apply">${esc(t("Apply"))}</button></div><select class="cart-coupon-assets" aria-label="${esc(t("My coupons"))}" hidden></select></div><div class="cart-actions"></div></div></div>`;
  const state = root.querySelector(".cart-status"),
    items = root.querySelector(".cart-items"),
    total = root.querySelector(".cart-total"),
    actions = root.querySelector(".cart-actions"),
    detailPanel = root.querySelector(".cart-details"),
    checkout = root.querySelector(".cart-checkout");
  new ResizeObserver(() =>
    document.body.style.setProperty(
      "--cart-checkout-height",
      `${checkout.getBoundingClientRect().height}px`,
    ),
  ).observe(checkout, { box: "border-box" });
  const setBusy = (value) => {
    busy = value;
    root.querySelectorAll("button").forEach((node) => {
      node.disabled = value || node.dataset.unavailable === "true";
    });
    root.querySelector(".cart-coupon-assets").disabled = value;
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
          (savedDrafts[item.configurationReference]?.locale === locale
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
        text(field, "label") || (draft.locale === locale ? field.label : "");
      const value =
        text(field, "value") || (draft.locale === locale ? field.value : "");
      if (label && !persisted.some((existing) => existing.label === label))
        persisted.push({ label, value });
    }
    if (item.serverLoginHostname)
      persisted.push({ label: t("Hostname"), value: item.serverLoginHostname });
    return [...persisted, { label: t("Billing period"), value: period(item) }];
  }
  function render() {
    if (!validQuote()) quote = undefined;
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
        '<div class="cart-loading" aria-hidden="true"><i></i><i></i><i></i></div>';
      return;
    }
    if (!rows.length) {
      checkout.hidden = true;
      items.innerHTML = `<div class="cart-empty"><span aria-hidden="true">◇</span><h2>${esc(t("Your cart is empty"))}</h2><p>${esc(t("Choose a product and configure it to get started."))}</p><a class="button" href="${href("/products.html")}">${esc(t("Browse products"))}</a></div>`;
      return;
    }
    checkout.hidden = false;
    for (const item of rows) {
      const row = quote?.items?.find(
        (row) => row.cartItemNo === item.cartItemNo,
      );
      const unavailable = item.statusCode && item.statusCode !== "active";
      const card = document.createElement("article");
      card.className = "cart-product";
      card.dataset.item = item.cartItemNo;
      card.innerHTML = `<div class="cart-product-heading"><div class="cart-product-mark" aria-hidden="true">▤</div><div><h3>${esc(text(item, "productName"))}</h3><p>${esc(t("Product configuration"))} · ${esc(period(item))}</p></div><div class="cart-product-tools"></div></div><dl class="cart-configuration">${configRows(
        item,
      )
        .map(
          (field) =>
            `<div><dt>${esc(field.label)}</dt><dd>${esc(field.value || "—")}</dd></div>`,
        )
        .join(
          "",
        )}</dl><div class="cart-product-footer"><div class="cart-quantity"><label for="quantity-${esc(item.cartItemNo)}">${esc(t("Quantity"))}</label><div class="quantity-control"><button type="button" class="quantity-less" aria-label="${esc(t("Decrease quantity"))}">−</button><input id="quantity-${esc(item.cartItemNo)}" type="number" min="1" max="1000" step="1" value="${item.quantity}" inputmode="numeric"><button type="button" class="quantity-more" aria-label="${esc(t("Increase quantity"))}">+</button></div></div><div class="cart-item-price"><span>${esc(t("Subtotal"))}</span><strong>${row ? esc(amount(row.totalAmountMinor, cart.currencyCode)) : "—"}</strong></div></div>`;
      const tools = card.querySelector(".cart-product-tools");
      const edit = document.createElement("a");
      edit.className = "cart-edit";
      edit.textContent = "✎ " + t("Edit configuration");
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
          "cart-remove",
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
          "button outline quantity-update",
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
        issue.className = "cart-product-issue";
        issue.textContent = t(
          "This product is unavailable. Edit or remove it before checkout.",
        );
        card.append(issue);
      }
      if (row?.configurationRequired && !row.configurationFrozen) {
        const repair = document.createElement("a");
        repair.className = "cart-product-issue";
        repair.href = href(
          `/configure.html?product=${encodeURIComponent(item.publicSlug)}&quoteItemNo=${encodeURIComponent(row.quoteItemNo)}`,
        );
        repair.textContent = t("Complete configuration");
        card.append(repair);
      }
      items.append(card);
    }
    total.innerHTML = `<span>${esc(t("Total due"))}</span><strong>${quote ? esc(amount(quote.totalAmountMinor, quote.currencyCode)) : "—"}</strong><small>${quote ? `${esc(t("Subtotal"))} ${esc(amount(quote.subtotalMinor, quote.currencyCode))} · ${esc(t("Discount"))} ${esc(amount(quote.discountMinor, quote.currencyCode))} · ${esc(t("Tax"))} ${esc(amount(quote.taxAmountMinor, quote.currencyCode))}` : esc(t("Prices are confirmed by the server."))}</small>`;
    if (details) {
      detailPanel.innerHTML = `<h2 class="cart-section-title">${esc(t("Checkout details"))}</h2><div class="cart-detail-panel"><h3>${esc(t("Order summary"))}</h3><div class="cart-detail-lines">${rows
        .map((item) => {
          const line = quote?.items?.find(
            (row) => row.cartItemNo === item.cartItemNo,
          );
          return `<div><div class="cart-detail-line"><span>${esc(text(item, "productName"))}<small>${esc(period(item))} × ${item.quantity}</small></span><strong>${line ? esc(amount(line.totalAmountMinor, cart.currencyCode)) : "—"}</strong></div><dl class="cart-detail-configuration">${configRows(
            item,
          )
            .map(
              (field) =>
                `<div><dt>${esc(field.label)}</dt><dd>${esc(field.value || "—")}</dd></div>`,
            )
            .join("")}</dl></div>`;
        })
        .join("")}</div><dl class="cart-detail-totals">${[
        ["Subtotal", "subtotalMinor"],
        ["Discount", "discountMinor"],
        ["Tax", "taxAmountMinor"],
        ["Total due", "totalAmountMinor"],
      ]
        .map(
          ([label, key]) =>
            `<div><dt>${esc(t(label))}</dt><dd>${quote ? esc(amount(quote[key], quote.currencyCode)) : "—"}</dd></div>`,
        )
        .join(
          "",
        )}</dl><div class="cart-payment-handoff"><span aria-hidden="true">▣</span><div><h3>${esc(t("Invoice payment"))}</h3><p>${esc(t("Confirm the order, then choose an available payment method or account balance on your invoice."))}</p></div></div><p class="cart-quote-validity">${esc(t("Price valid until"))} ${quote ? esc(new Date(quote.expiresAt).toLocaleTimeString(locale)) : "—"}</p></div>`;
    }
    if (details) renderPaymentChoices();
    renderActions();
    setBusy(busy);
  }
  function renderPaymentChoices() {
    const panel = detailPanel.querySelector(".cart-payment-handoff");
    panel.innerHTML = `<div class="cart-payment-choices"><h3>${esc(t("Payment method"))}</h3><div class="cart-payment-options"></div><p>${esc(t("Your selection will be carried to the invoice. Payment requires your confirmation there."))}</p></div>`;
    const choices = panel.querySelector(".cart-payment-options");
    if (availableBalance !== undefined && BigInt(availableBalance) > 0n) {
      const balance = document.createElement("label");
      balance.className = "cart-payment-option";
      balance.innerHTML = `<input type="radio" name="payment-choice" value="balance"><span>${esc(t("Account balance"))}<small>${esc(t("Available"))} ${esc(amount(availableBalance, cart.currencyCode))}</small></span>`;
      balance.querySelector("input").checked = preferBalance;
      balance.querySelector("input").onchange = () => {
        preferBalance = true;
        preferredChannel = "";
      };
      // The current balance contract supports paying the complete invoice only.
      if (BigInt(availableBalance) < BigInt(quote.totalAmountMinor)) {
        balance.querySelector("input").disabled = true;
        balance.classList.add("unavailable");
        balance.append(document.createTextNode(t("Insufficient balance")));
      }
      choices.append(balance);
    }
    for (const channel of paymentChannels) {
      const label = document.createElement("label");
      label.className = "cart-payment-option";
      const input = document.createElement("input");
      input.type = "radio";
      input.name = "payment-choice";
      input.checked = !preferBalance && preferredChannel === channel.channelNo;
      input.onchange = () => {
        preferredChannel = channel.channelNo;
        preferBalance = false;
      };
      const name = document.createElement("span");
      name.textContent = channel.displayName;
      label.append(input, name);
      choices.append(label);
    }
    if (!choices.children.length) {
      const note = document.createElement("p");
      note.textContent =
        (paymentLoading ? t("Loading…") : paymentError) ||
        t("Available payment methods will be verified on the invoice.");
      choices.append(note);
    }
    if (paymentError && !paymentLoading)
      choices.append(button("Retry", () => { void loadPaymentChoices(); }, "button outline"));
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
      token !== expectedIdentity ||
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
          row.modes?.some((mode) => !["jsapi", "app"].includes(mode)),
        );
      if (results.some((result) => result.status === "rejected"))
        paymentError = t(
          "Could not load payment methods. Retry or choose a method on the invoice.",
        );
      if (!paymentChannels.some((row) => row.channelNo === preferredChannel))
        preferredChannel = "";
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
        "Could not load payment methods. Retry or choose a method on the invoice.",
      );
    }
    if (details) renderPaymentChoices();
  }
  function renderActions() {
    actions.replaceChildren();
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
          "button outline",
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
      );
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
      details ? "button cart-submit" : "button",
    );
    action.dataset.unavailable = String(blocked);
    action.disabled = blocked || busy;
    actions.append(action);
  }
  async function ensureIdentity() {
    const previous = identity;
    if (!(await signIn())) throw new Error(t("Sign in before checking out"));
    if (previous !== token) {
      cart = undefined;
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
      if (action !== submit) {
        quote = undefined;
        details = false;
      }
      render();
      message(state, error.message, true);
      state.append(button("Refresh cart", load, "button outline"));
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
      if (generation !== assetsRun || identity !== expectedIdentity || token !== expectedIdentity || cart?.currencyCode !== expectedCurrency) return;
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
            (locale === "en-US" ? asset.nameEnUs : asset.nameZhCn) ||
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
        deviceId = localStorage.getItem("website-qycn-coupon-device");
        if (!deviceId) {
          deviceId = crypto.randomUUID();
          localStorage.setItem("website-qycn-coupon-device", deviceId);
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
      identity = token;
      if (!token) {
        message(state, t("Sign in before adding products to your cart."));
        state.append(button("Sign in", () => run(signIn)));
        return;
      }
      const merged = await commerce("account/commerce/guest-cart-merge", "POST");
      cart = Object.prototype.hasOwnProperty.call(merged, "cart")
        ? merged.cart
        : (await commerce("account/commerce/cart")).cart;
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
      state.append(button("Retry", load, "button outline"));
    } finally {
      loading = false;
      if (!cart) render();
      setBusy(false);
    }
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
    const key = "website-qycn-order-" + quote.quoteNo;
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
    const { checkout: result } = await commerce(
      "account/commerce/orders",
      "POST",
      { quoteNo: quote.quoteNo, idempotencyKey: id },
    );
    const paymentQuery = new URLSearchParams({ pay: "1" });
    if (!paymentLoading && preferredChannel) paymentQuery.set("paymentChannel", preferredChannel);
    if (!paymentLoading && preferBalance) paymentQuery.set("useBalance", "1");
    const target = customerURL(
      `/account/billing/invoices/${encodeURIComponent(result.invoiceNo)}?${paymentQuery}`,
    );
    quote = undefined;
    actions.replaceChildren();
    message(state, t("Order created. Opening payment."));
    if (target) {
      const link = document.createElement("a");
      link.href = target;
      link.textContent = t("Open invoice");
      actions.append(link);
      location.assign(target);
    }
  }
  await load();
}
export async function startCommerce(page) {
  const root = document.querySelector("#commerce-page");
  if (page === "cart") await cartPage(root);
  else await configure(root);
}
