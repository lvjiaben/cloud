import { t, locale, translateMarkup } from "./i18n.js";
import { listContent, getContent, announcementHtml } from "./content.js";
// Public website behavior: local presentation and bounded anonymous data loading.
import {
  slides,
  locations,
  defaultNavigation,
  contentKinds,
  apiTypes,
} from "./locales/zh-CN.js";
import {
  catalogBase,
  contentBase,
  request,
  requestDownload,
  localized,
  safeURL,
  customerURL,
  escapeHTML as esc,
} from "./api.js";
import {
  servicesPage,
  icon,
  shell,
  homepage,
  catalogPage,
  contentPage,
  articlePage,
  infoPage,
} from "./views.js";

const page = document.body.dataset.page;
document.documentElement.lang = locale;
const mainMarkup =
  page === "home"
    ? homepage()
    : ["products", "services"].includes(page)
      ? catalogPage()
      : page === "article"
        ? articlePage()
        : Object.hasOwn(contentKinds, page)
          ? contentPage(page)
          : ["configure", "cart"].includes(page)
            ? '<section class="container page-section" id="commerce-page"></section>'
            : infoPage(page);
document.body.innerHTML = translateMarkup(shell(mainMarkup, page === "home"));
const $ = (selector) => document.querySelector(selector);
function productText(item, key) {
  return (
    item[key + (document.documentElement.lang === "en-US" ? "EnUS" : "ZhCN")] ||
    item[key + "ZhCN"] ||
    item[key + "EnUS"] ||
    ""
  );
}
let brand;
let catalogCategories = [];
let slideIndex = 0;

function status(id, text, retry, empty = false) {
  const target = document.getElementById(id);
  if (!target) return;
  target.replaceChildren(document.createTextNode(text));
  target.classList.toggle("error", Boolean(retry));
  target.classList.toggle("empty", empty);
  if (retry) {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = t("Retry");
    button.addEventListener("click", retry);
    target.append(button);
  }
}

function renderNavigation(container, items, depth = 0) {
  container.replaceChildren();
  if (!Array.isArray(items) || depth > 2) return;
  const list = document.createElement("ul");
  for (const item of items) {
    const children = Array.isArray(item.children) ? item.children : [];
    if (item.group && !children.length) continue;
    const href = item.customer
      ? customerURL(item.href)
      : safeURL(item.href, { external: item.external });
    if (!item.group && !href) continue;
    const li = document.createElement("li");
    const control = document.createElement(item.group ? "button" : "a");
    control.textContent = localized(item.label);
    if (item.group) {
      control.type = "button";
      control.setAttribute("aria-label", localized(item.label));
      control.setAttribute("aria-expanded", "false");
    } else {
      control.href = href;
      if (item.external) {
        control.target = "_blank";
        control.rel = "noopener noreferrer";
      }
    }
    li.append(control);
    if (children.length) {
      const sub = document.createElement("div");
      renderNavigation(sub, children, depth + 1);
      const subtree = sub.firstElementChild;
      if (subtree) {
        const toggleControl = item.group
          ? control
          : document.createElement("button");
        if (!item.group) {
          toggleControl.type = "button";
          toggleControl.className = "nav-children-toggle";
          toggleControl.setAttribute(
            "aria-label",
            t("Expand") + localized(item.label),
          );
          toggleControl.textContent = "⌄";
          li.classList.add("has-link-children");
          li.append(toggleControl);
        }
        li.append(subtree);
        subtree.hidden = true;
        toggleControl.setAttribute("aria-expanded", "false");
        const toggle = (open) => {
          subtree.hidden = !open;
          toggleControl.setAttribute("aria-expanded", String(open));
        };
        toggleControl.addEventListener("click", (event) => {
          // Desktop hover has already opened the menu before the click arrives.
          const pointerClick = event.detail > 0 && matchMedia("(min-width: 1201px)").matches;
          toggle(pointerClick || subtree.hidden);
        });
        li.addEventListener("mouseenter", () => {
          if (matchMedia("(min-width: 1201px)").matches) toggle(true);
        });
        li.addEventListener("mouseleave", () => {
          if (
            matchMedia("(min-width: 1201px)").matches &&
            !li.contains(document.activeElement)
          )
            toggle(false);
        });
        li.addEventListener("focusout", (event) => {
          if (!li.contains(event.relatedTarget)) toggle(false);
        });
        li.addEventListener("keydown", (event) => {
          if (event.key === "Escape" && !subtree.hidden) {
            toggle(false);
            toggleControl.focus();
            event.stopPropagation();
          }
        });
      }
    }
    list.append(li);
  }
  container.append(list);
}

function renderHeaderNavigation() {
  const configured = Array.isArray(brand?.navigation) && brand.navigation.length;
  const navigation = configured ? brand.navigation : structuredClone(defaultNavigation);
  if (!configured) {
    navigation[0].children.push(
      ...catalogCategories.map((category) => ({
        label: { zhCN: category.nameZhCN, enUS: category.nameEnUS },
        href: `/services.html?category=${encodeURIComponent(category.categoryNo)}`,
      })),
    );
  }
  renderNavigation($("#header-nav"), navigation);
}

function bindCustomerLinks() {
  document.querySelectorAll("[data-customer]").forEach((link) => {
    const href = customerURL(link.dataset.customer);
    link.href = href || "/help.html";
    if (!href)
      link.title = t(
        "Customer center is not configured. Contact the administrator.",
      );
  });
}

function applyBrand() {
  const name = localized(brand?.siteName) || "";
  document.querySelectorAll("[data-brand]").forEach((node) => {
    node.textContent = name;
  });
  document.querySelectorAll("[data-company]").forEach((node) => {
    node.textContent = brand?.companyName || localized(brand?.footer) || "";
  });
  document.querySelectorAll("[data-phone]").forEach((node) => {
    node.textContent = brand?.contactPhone || t("Phone not configured");
  });
  document.querySelectorAll("[data-email]").forEach((node) => {
    node.textContent = brand?.contactEmail || t("Email not configured");
  });
  $(".brand-mark").replaceChildren();
  const logo = safeURL(brand?.logoUrl, { external: true });
  $('link[rel="icon"]').href = logo || "data:,";
  if (logo) {
    const image = document.createElement("img");
    image.src = logo;
    image.alt = name;
    $(".brand-mark").replaceChildren(image);

  }
  renderHeaderNavigation();
  renderNavigation($("#footer-nav"), brand.footerNavigation || []);
  $("#footer-nav").hidden = !brand.footerNavigation?.length;
  document.querySelectorAll("[data-default-footer]").forEach((node) => {
    node.hidden = Boolean(brand.footerNavigation?.length);
  });
  const registration = $("#registration-links");
  registration.replaceChildren();
  for (const [number, url] of [
    [brand.icpNumber, brand.icpUrl],
    [brand.policeNumber, brand.policeUrl],
  ]) {
    if (!number) continue;
    const href = safeURL(url, { external: true });
    const node = document.createElement(href ? "a" : "span");
    node.textContent = number;
    if (href) {
      node.href = href;
      node.target = "_blank";
      node.rel = "noopener noreferrer";
    }
    registration.append(node);
  }
  if ($("#partners")) {
    const config = brand.templateEntry?.assetBase?.includes("/website-qycn/")
      ? brand.templateConfig || {}
      : {};
    const partners = String(
      config[locale === "en-US" ? "partnersen" : "partners"] ||
        config.partners ||
        "",
    )
      .split("\n")
      .map((v) => v.trim())
      .filter(Boolean)
      .slice(0, 20);
    $("#partner-grid").replaceChildren(
      ...partners.map((text) => {
        const node = document.createElement("div");
        node.textContent = text;
        return node;
      }),
    );
    $("#partners").hidden = !partners.length;
  }
  $(".footer-main").classList.toggle(
    "configured",
    Boolean(brand.footerNavigation?.length),
  );
  if (page !== "article")
    document.title =
      (page === "home"
        ? localized(brand.seoTitle) || t("Home")
        : t(document.body.dataset.titleEn || document.body.dataset.title)) +
      " - " +
      name;
  const description = localized(brand.seoDescription);
  if (description && page === "home")
    $('meta[name="description"]').content = description;
  if (page === "home") setSlide(slideIndex);
}

async function loadBrand() {
  try {
    const result = await request("/x/idc-portal/api/v1/public/brand");
    if (!result.brand) throw new Error(t("Not configured"));
    brand = result.brand;
    applyBrand();
    $("#brand-status").hidden = true;
  } catch {
    const target = $("#brand-status");
    target.hidden = false;
    status(
      "brand-status",
      t("Website details are unavailable. Please retry."),
      loadBrand,
    );
  }
}

function setSlide(index) {
  const hero = $(".hero");
  if (!hero) return;
  slideIndex = (index + slides.length) % slides.length;
  const slide = slides[slideIndex];
  hero.dataset.theme = slide.theme;
  const ownConfig = brand?.templateEntry?.assetBase?.includes("/website-qycn/");
  const customTitle =
    ownConfig && slideIndex === 0
      ? localized(
          locale === "en-US" && brand.templateConfig?.herotitleen
            ? brand.templateConfig.herotitleen
            : brand.heroTitle,
        )
      : "";
  $("#hero-title").innerHTML = customTitle
    ? esc(customTitle)
    : `${esc(slide.title)}<br><strong>${esc(slide.accent)}</strong>`;
  $("#hero-description").textContent =
    (ownConfig &&
      slideIndex === 0 &&
      localized(
        locale === "en-US" && brand.templateConfig?.herodescriptionen
          ? brand.templateConfig.herodescriptionen
          : brand.heroDescription,
      )) ||
    slide.sub;
  $("#hero-tag").textContent = slide.tag;
  $("#hero-link").href = slide.href;
  $("#hero-link").innerHTML = esc(slide.cta) + icon("arrow");
  const background =
    ownConfig && slideIndex === 0
      ? safeURL(brand.heroImage, { external: true })
      : "";
  hero.style.backgroundImage = background
    ? `linear-gradient(#15234f44,#15234f44),url(${JSON.stringify(background)})`
    : "";
  hero.style.backgroundSize = background ? "cover" : "";
  document.querySelectorAll("[data-slide]").forEach((button) => {
    const selected = Number(button.dataset.slide) === slideIndex;
    button.classList.toggle("active", selected);
    button.setAttribute("aria-pressed", String(selected));
  });
}

function setupPresentation() {
  const nav = $("#header-nav");
  $(".menu-toggle").addEventListener("click", () => {
    const open = nav.classList.toggle("open");
    $("#header-menu").classList.toggle("open", open);
    $(".menu-toggle").setAttribute("aria-expanded", String(open));
    $(".menu-toggle").setAttribute(
      "aria-label",
      open ? t("Close navigation") : t("Open navigation"),
    );
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && nav.classList.contains("open")) {
      nav.classList.remove("open");
      $("#header-menu").classList.remove("open");
      $(".menu-toggle").setAttribute("aria-expanded", "false");
      $(".menu-toggle").setAttribute("aria-label", t("Open navigation"));
      $(".menu-toggle").focus();
    }
  });
  $("#back-top").addEventListener("click", () =>
    window.scrollTo({
      top: 0,
      behavior: matchMedia("(prefers-reduced-motion: reduce)").matches
        ? "instant"
        : "smooth",
    }),
  );

  document.querySelectorAll("[data-location]").forEach((button) =>
    button.addEventListener("click", () => {
      const selected = locations[Number(button.dataset.location)];
      document.querySelectorAll("[data-location]").forEach((node) => {
        node.classList.toggle("selected", node === button);
        node.setAttribute("aria-pressed", String(node === button));
      });
      $("#location-detail").innerHTML =
        `<strong>${selected.name}</strong><span>${selected.detail}</span><a href="/products.html">${t("Browse products")} →</a>`;
    }),
  );
  if (page !== "home") return;
  let timer;
  function restartTimer() {
    clearInterval(timer);
    timer = setInterval(() => {
      // Background tabs and keyboard interaction pause rotation. A resting
      // pointer or mouse-click focus must not silently stop the entire banner.
      if (
        !document.hidden &&
        !$(".hero a:focus-visible, .hero button:focus-visible")
      ) setSlide(slideIndex + 1);
    }, 6500);
  }
  function selectSlide(index) {
    setSlide(index);
    restartTimer();
  }
  $(".prev").addEventListener("click", () => selectSlide(slideIndex - 1));
  $(".next").addEventListener("click", () => selectSlide(slideIndex + 1));
  document.querySelectorAll("[data-slide]").forEach((button) =>
    button.addEventListener("click", () => selectSlide(Number(button.dataset.slide))),
  );
  restartTimer();
  window.addEventListener("pagehide", () => clearInterval(timer));
  window.addEventListener("pageshow", (event) => {
    if (event.persisted) restartTimer();
  });
}

function productCard(product) {
  const name = productText(product, "name");
  const summary =
    productText(product, "summary") ||
    t("Choose the configuration that suits your needs.");
  const href = `/configure.html?product=${encodeURIComponent(product.publicSlug)}`;
  const available = product.availability !== "unavailable";
  return `<article class="product-card live-product"><span class="product-tag">${esc(productText(product, "categoryName") || t("Cloud products"))}</span><h3>${esc(name)}</h3><p>${esc(summary)}</p><div class="product-cta"><span>${available ? (product.availability === "manual_confirmation" ? t("Manual confirmation required") : t("Available products")) : t("Currently unavailable")}</span>${available && href ? `<a href="${esc(href)}">${t("Configure and quote")} →</a>` : `<a href="/contact.html">${t("Contact sales")} →</a>`}</div></article>`;
}

function setupCatalog() {
  if (!$("#catalog-products")) return;
  let cursor = "";
  let category = new URLSearchParams(location.search).get("category") || "";
  let generation = 0;
  const more = $("#catalog-more");
  async function load(append = false) {
    const run = ++generation;
    status("catalog-status", t("Loading products…"));
    if (more) more.hidden = true;
    const query = new URLSearchParams({ limit: page === "home" ? "6" : "12" });
    if ($("#product-query")?.value.trim())
      query.set("namePrefix", $("#product-query").value.trim());
    if (category) query.set("categoryNo", category);
    if (append && cursor) query.set("cursor", cursor);
    if (!append) $("#catalog-products").replaceChildren();
    try {
      const result = await request(`${catalogBase}?${query}`);
      if (run !== generation) return;
      const products = Array.isArray(result.products) ? result.products : [];
      $("#catalog-products").insertAdjacentHTML(
        "beforeend",
        products.map(productCard).join(""),
      );
      cursor = result.nextCursor || "";
      if (more) more.hidden = !cursor;
      if (Array.isArray(result.categories)) {
        const categories = result.categories;
        const select = $("#catalog-category");
        if (select) {
          select.innerHTML =
            '<option value="">' +
            t("All categories") +
            "</option>" +
            categories
              .map(
                (c) =>
                  `<option value="${esc(c.categoryNo)}">${esc(productText(c, "name"))}</option>`,
              )
              .join("");
          select.value = category;
        }
        $("#catalog-tabs").innerHTML = [
          {
            categoryNo: "",
            nameZhCN: t("All products"),
            nameEnUS: "All products",
          },
          ...categories,
        ]
          .map(
            (c) =>
              `<a class="${c.categoryNo === category ? "active" : ""}" href="/services.html?category=${encodeURIComponent(c.categoryNo)}" data-category="${esc(c.categoryNo)}">${esc(productText(c, "name"))}</a>`,
          )
          .join("");
        $("#catalog-tabs")
          .querySelectorAll("a")
          .forEach((link) =>
            link.addEventListener("click", (event) => {
              event.preventDefault();
              category = link.dataset.category;
              const url = new URL(location.href);
              category
                ? url.searchParams.set("category", category)
                : url.searchParams.delete("category");
              history.replaceState(null, "", url);
              load();
            }),
          );
        $("#footer-categories").innerHTML = categories
          .map(
            (c) =>
              `<a href="/services.html?category=${encodeURIComponent(c.categoryNo)}">${esc(productText(c, "name"))}</a>`,
          )
          .join("");
        catalogCategories = categories;
        renderHeaderNavigation();
      }
      status(
        "catalog-status",
        $("#catalog-products").children.length
          ? ""
          : t("No products are currently available."),
        null,
        true,
      );
    } catch {
      if (run !== generation) return;
      status(
        "catalog-status",
        t("Products could not be loaded. Please retry."),
        () => load(append),
      );
    }
  }
  $("#catalog-search")?.addEventListener("submit", (event) => {
    event.preventDefault();
    load();
  });
  $("#catalog-category")?.addEventListener("change", (event) => {
    category = event.target.value;
    load();
  });
  more?.addEventListener("click", () => load(true));
  load();
}

function dateText(timestamp) {
  return Number.isFinite(timestamp) && timestamp > 0
    ? new Date(timestamp)
        .toLocaleDateString(locale, {
          year: "numeric",
          month: "2-digit",
          day: "2-digit",
        })
        .replaceAll("/", "-")
    : "";
}
function articleURL(kind, item) {
  return `/article.html?kind=${kind}&id=${encodeURIComponent(item.contentNo)}`;
}

async function loadHomeContent(kind) {
  const id = `home-${kind}-status`;
  status(id, t("Loading…"));
  try {
    const result = await listContent(kind, { limit: 5 });
    const items = Array.isArray(result.items) ? result.items : [];
    $(`#home-${kind}-items`).innerHTML = items
      .map(
        (item) =>
          `<li><a href="${articleURL(kind, item)}">${esc(item.title)}</a><time>${dateText(item.publishedAt)}</time></li>`,
      )
      .join("");
    status(id, items.length ? "" : t("No published articles"), null, true);
  } catch {
    status(id, t("Articles could not be loaded"), () => loadHomeContent(kind));
  }
}

function setupContentList(kind) {
  let cursor = "";
  let generation = 0;
  const more = $("#content-more");
  async function load(append = false) {
    const run = ++generation;
    status("content-status", t("Loading content…"));
    more.hidden = true;
    if (!append) $("#content-items").replaceChildren();
    try {
      const result = await listContent(kind, { limit: 12, cursor: append ? cursor : "", query: $("#content-query").value.trim() });
      if (run !== generation) return;
      const items = Array.isArray(result.items) ? result.items : [];
      $("#content-items").insertAdjacentHTML(
        "beforeend",
        items
          .map(
            (item) =>
              `<a class="content-row" href="${articleURL(kind, item)}"><h2>${esc(item.title)}</h2><p>${esc(item.summary || "")}</p><time>${dateText(item.publishedAt)}</time></a>`,
          )
          .join(""),
      );
      cursor = result.nextCursor || "";
      more.hidden = !cursor;
      status(
        "content-status",
        $("#content-items").children.length
          ? ""
          : t("No matching content. Try another search."),
        null,
        true,
      );
    } catch {
      if (run === generation)
        status(
          "content-status",
          t("Content could not be loaded. Please retry."),
          () => load(append),
        );
    }
  }
  $("#content-search").addEventListener("submit", (event) => {
    event.preventDefault();
    load();
  });
  more.addEventListener("click", () => load(true));
  load();
}

async function loadArticle() {
  const query = new URLSearchParams(location.search);
  const kind = Object.hasOwn(contentKinds, query.get("kind"))
    ? query.get("kind")
    : "news";
  $("#article-back").href = `/${kind}.html`;
  const id = query.get("id");
  if (!id || id.length > 200) {
    status(
      "article-status",
      t("This content is unavailable or has been removed."),
    );
    return;
  }
  status("article-status", t("Loading content…"));
  try {
    const item = await getContent(kind, id);
    if (!item || item.type !== apiTypes[kind])
      throw new Error(t("This content is unavailable or has been removed."));
    $("#article-title").textContent = item.title;
    $("#article-date").textContent = dateText(item.publishedAt);
    const body = String(item.body || "");
    $("#article-body").innerHTML = announcementHtml(body);
    $("#article").hidden = false;
    status("article-status", "");
    document.title = item.seoTitle || item.title;
    $('meta[name="description"]').content =
      item.seoDescription || item.summary || "";
    const download = $("#download");
    download.hidden = !item.currentFileNo;
    download.onclick = async () => {
      download.disabled = true;
      status("article-status", t("Preparing your download…"));
      try {
        const ticket = await request(
          `${contentBase}/items/${encodeURIComponent(id)}/download-tickets`,
          { method: "POST" },
        );
        const file = await requestDownload(ticket.token);
        const href = URL.createObjectURL(file.blob);
        const link = document.createElement("a");
        link.href = href;
        link.download = file.filename;
        link.click();
        setTimeout(() => URL.revokeObjectURL(href), 1000);
        status(
          "article-status",
          t("Download started. Retry if it does not begin."),
        );
      } catch {
        status(
          "article-status",
          t("The file could not be downloaded. Please retry."),
          () => download.click(),
        );
      } finally {
        download.disabled = false;
      }
    };
  } catch (error) {
    status(
      "article-status",
      error.message === t("This content is unavailable or has been removed.")
        ? error.message
        : t("Content could not be loaded. Please retry."),
      loadArticle,
    );
  }
}

$(".skip-link").href = location.pathname + location.search + "#main";
renderHeaderNavigation();
setupPresentation();
bindCustomerLinks();
loadBrand();
setupCatalog();
if (page === "home")
  ["news", "announcements", "knowledge"].forEach(loadHomeContent);
if (Object.hasOwn(contentKinds, page)) setupContentList(page);
if (page === "article") loadArticle();

$("#language").value = locale;
$("#language").addEventListener("change", (event) => {
  const value = event.target.value;
  try {
    localStorage.setItem("website-qycn-language", value);
  } catch {}
  const url = new URL(location.href);
  url.searchParams.set("lang", value);
  location.assign(url);
});
// Keep locale in internal links even when browser storage is unavailable.
document.addEventListener("click", (event) => {
  const link = event.target.closest("a[href]");
  if (!link || link.hasAttribute("download")) return;
  const url = new URL(link.href);
  if (
    url.origin === location.origin &&
    !url.pathname.startsWith("/x/") &&
    !url.pathname.startsWith("/x-assets/")
  ) {
    url.searchParams.set("lang", locale);
    link.href = url.href;
  }
});
if (["configure", "cart"].includes(page))
  import("./commerce.js").then((module) => module.startCommerce(page));

if (!$("#catalog-products"))
  request(`${catalogBase}?limit=1`)
    .then((result) => {
      const categories = Array.isArray(result.categories)
        ? result.categories
        : [];
      $("#footer-categories").innerHTML = categories
        .map(
          (c) =>
            `<a href="/services.html?category=${encodeURIComponent(c.categoryNo)}">${esc(productText(c, "name"))}</a>`,
        )
        .join("");
      catalogCategories = categories;
      renderHeaderNavigation();
    })
    .catch(() => {});
