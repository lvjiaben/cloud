import { t } from "./i18n.js";
// Shared public markup; all server-provided text is escaped by callers.
import {
  slides,
  advantages,
  locations,
  contentKinds,
} from "./locales/zh-CN.js";

const paths = {
  shield:
    '<path d="M24 4 7 11v15c0 10 17 19 17 19s17-9 17-19V11Z"/><path d="m16 24 6 6 12-14"/>',
  cloud:
    '<path d="M13 35a10 10 0 0 1-1-20 13 13 0 0 1 25 2 9 9 0 0 1-1 18Z"/><path d="m19 25 5-5 5 5m-5-5v21"/>',
  server:
    '<rect x="8" y="6" width="32" height="12" rx="2"/><rect x="8" y="20" width="32" height="12" rx="2"/><path d="M14 12h1m5 0h14M14 26h1m5 0h14M24 32v10M12 42h24"/>',
  box: '<path d="m24 4 17 9v22l-17 9-17-9V13Zm-17 9 17 9 17-9M24 22v22M16 8l17 9v12"/>',
  globe:
    '<circle cx="24" cy="24" r="19"/><ellipse cx="24" cy="24" rx="9" ry="19"/><path d="M5 24h38M9 13h30M9 35h30"/>',
  user: '<circle cx="24" cy="16" r="8"/><path d="M9 42v-6c0-14 30-14 30 0v6M6 6h8M6 6v8m36-8h-8m8 0v8"/>',
  clock: '<circle cx="24" cy="24" r="19"/><path d="M24 11v14l9 6"/>',
  arrow: '<path d="M8 24h31M28 13l11 11-11 11"/>',
  phone:
    '<path d="m12 5 8 10-5 5c3 7 6 10 13 13l5-5 10 8-4 7C22 44 4 26 5 12Z"/>',
  chat: '<path d="M7 8h34v25H22L12 42v-9H7Z"/><path d="M14 19h20M14 25h13"/>',
};
export function icon(name, cls = "") {
  return `<svg class="icon ${cls}" viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name] || paths.box}</svg>`;
}
export function heading(title, sub = "", cls = "") {
  return `<div class="section-heading ${cls}"><h2>${title}</h2>${sub ? `<p>${sub}</p>` : ""}</div>`;
}
export function pageBanner(title, sub, tag = "CLOUD · CONNECT · CREATE") {
  return `<section class="page-banner"><div class="container"><p class="eyebrow">${tag}</p><h1>${title}</h1><p>${sub}</p></div><div class="banner-orbit" aria-hidden="true"></div></section>`;
}
export function statusBox(id, text = "Loading…") {
  return `<div id="${id}" class="status" role="status">${text}</div>`;
}
export function categoryTabs() {
  return '<nav id="catalog-tabs" class="category-tabs container" aria-label="Product types"></nav>';
}
export function mapSection(full = false) {
  return `<section class="map-section container">${heading("Global locations, connected experiences", "Deploy across regions and reach new opportunities")}<div class="stats">${[
    ["6", "Locations"],
    ["3", "Network regions"],
    ["Multi-line", "Network options"],
    ["On demand", "Flexible deployment"],
  ]
    .map(([n, t]) => `<div><strong>${n}</strong><span>${t}</span></div>`)
    .join(
      "",
    )}</div><div class="world-map" role="img" aria-label="Illustrative data center map"></div><div class="location-tabs" aria-label="Choose a location">${locations.map((l, i) => `<button class="map-pin ${i === 0 ? "selected" : ""}" data-location="${i}" aria-label="${l.name}" aria-pressed="${i === 0}"><i></i><span>${l.name}</span></button>`).join("")}</div><div class="location-detail" id="location-detail"><strong>${locations[0].name}</strong><span>${locations[0].detail}</span>${full ? "" : '<a href="/datacenters.html">Explore data centers →</a>'}</div><p class="fine-print">Locations are illustrative. Available regions are shown in product configuration.</p></section>`;
}
export function homepage() {
  return `<section class="hero" aria-label="Featured services"><div class="hero-grid" aria-hidden="true"></div><div class="hero-shield" aria-hidden="true">${icon("shield")}</div><div class="hero-glow" aria-hidden="true"></div><div class="container hero-copy"><p class="eyebrow" id="hero-tag">${slides[0].tag}</p><h1 id="hero-title">${slides[0].title}<br><strong>${slides[0].accent}</strong></h1><p id="hero-description">${slides[0].sub}</p><a class="button hero-cta" id="hero-link" href="${slides[0].href}">${slides[0].cta}${icon("arrow")}</a></div><button class="slide-arrow prev" aria-label="Previous slide">‹</button><button class="slide-arrow next" aria-label="Next slide">›</button><div class="slide-controls">${slides.map((_, i) => `<button data-slide="${i}" class="${i === 0 ? "active" : ""}" aria-label="${t("Slide")} ${i + 1}" aria-pressed="${i === 0}"></button>`).join("")}</div></section>
  <section class="products-section">${heading("Products and services", "Efficient, secure and flexible products for your cloud journey")}${categoryTabs()}<div class="product-area"><div class="container">${statusBox("catalog-status")}<div id="catalog-products" class="product-grid"></div><div class="section-bottom"><a href="/products.html">View all products ${icon("arrow")}</a></div></div></div></section>
  <section class="advantages"> <div class="container">${heading("Why choose <span data-brand></span>", "We focus on infrastructure. You focus on innovation.")}<div class="advantage-grid">${advantages.map(([i, t, d]) => `<article>${icon(i)}<div><h3>${t}</h3><p>${d}</p></div></article>`).join("")}</div></div></section>
  ${mapSection()}<section class="partners" id="partners" hidden> <div class="container">${heading("Build a connected cloud ecosystem", "Connecting partners, technology and business")}<div class="partner-grid" id="partner-grid"></div></div></section>
  <section class="news-section container">${heading("Our latest updates", "Product news, service announcements and cloud technology")}<div class="news-grid">${["news", "announcements", "knowledge"].map((kind, i) => `<article class="news-column"><div class="news-title">${icon(["box", "chat", "cloud"][i])}<h3>${contentKinds[kind]}</h3><a href="/${kind}.html" aria-label="${t("View")} ${contentKinds[kind]}">→</a></div>${statusBox(`home-${kind}-status`)}<ul id="home-${kind}-items"></ul></article>`).join("")}</div></section>`;
}
export function catalogPage() {
  return `${pageBanner("Products and services", "Find the right cloud product for your next idea", "OUR PRODUCTS")}<section class="container page-section">${categoryTabs()}<div class="catalog-toolbar"><form id="catalog-search" role="search"><label class="sr-only" for="product-query">Product name</label><input id="product-query" name="namePrefix" placeholder="Enter product name"><button class="button" type="submit">Search products</button></form><label>Product category <select id="catalog-category"><option value="">All categories</option></select></label></div>${statusBox("catalog-status")}<div id="catalog-products" class="product-grid"></div><div class="pagination"><button class="button outline" id="catalog-more" hidden>Load more products</button></div></section>`;
}
export function servicesPage() {
  return catalogPage();
}
export function contentPage(kind) {
  return `${pageBanner(contentKinds[kind], "Explore product updates and cloud technology", "INSIGHTS & UPDATES")}<section class="container page-section"><nav class="pills" aria-label="Content categories">${Object.entries(
    contentKinds,
  )
    .map(
      ([k, v]) =>
        `<a class="${k === kind ? "active" : ""}" href="/${k}.html">${v}</a>`,
    )
    .join(
      "",
    )}</nav><form ${kind === "announcements" ? "hidden" : ""} class="content-search" id="content-search" role="search"><label class="sr-only" for="content-query">Search content</label><input id="content-query" placeholder="Search articles"><button class="button">Search</button></form>${statusBox("content-status")}<div id="content-items" class="content-list"></div><div class="pagination"><button class="button outline" id="content-more" hidden>Load more articles</button></div></section>`;
}
export function articlePage() {
  return `${pageBanner("Article", "Product news and technical insights", "INSIGHTS")}<section class="container page-section article-wrap"><a id="article-back" href="/news.html">← ${t("Back to list")}</a>${statusBox("article-status")}<article id="article" hidden><h2 id="article-title"></h2><p class="article-meta" id="article-date"></p><div id="article-body" class="article-body"></div><button class="button" id="download" hidden>Download file</button></article></section>`;
}
export function infoPage(page) {
  if (page === "datacenters")
    return `${pageBanner("Global data centers", "Connect your ideas across borders", "GLOBAL INFRASTRUCTURE")}${mapSection(true)}<section class="alternate"><div class="container three-grid">${locations.map((l) => `<article class="info-card"><span class="region">${l.region}</span><h3>${l.name}</h3><p>${l.detail}</p><a href="/products.html">View available products →</a></article>`).join("")}</div></section>`;
  if (page === "contact")
    return `${pageBanner("Contact us", "Start a conversation. Find the right solution.", "CONTACT US")}<section class="container page-section"><div class="three-grid"><article class="info-card">${icon("phone")}<h2>Call us</h2><p>Product selection and service enquiries</p><p data-phone>Phone not configured</p></article><article class="info-card">${icon("chat")}<h2>Email us</h2><p>Send us your requirements</p><p data-email>Email not configured</p></article><article class="info-card">${icon("user")}<h2>Customer service</h2><p>Sign in to manage services and access support</p><a data-customer="/account" class="button" href="/help.html">Open customer center</a></article></div><div class="contact-note"><h2>Help us help you</h2><p>Tell us your use case, preferred location and configuration. Existing customers can access service details and support in the customer center.</p><p class="fine-print">Contact details come from the website configuration.</p></div></section>`;
  if (page === "about")
    return `${pageBanner("About us", "Technology that connects. Service that supports growth.", "ABOUT US")}<section class="container page-section"><div class="service-detail"><div><p class="eyebrow">FOCUS ON YOUR BUSINESS</p><h2>A reliable foundation for your cloud journey</h2><p><span data-brand></span> provides cloud services for businesses and developers. Clear product choices and convenient management help you focus on your business.</p><p class="fine-print">This introduction is sample content. Refer to published company information.</p><a class="button" href="/contact.html">Contact us</a></div><div class="service-illustration">${icon("globe")}</div></div><div class="three-grid">${[
      ["Expertise", "Focused on infrastructure and service quality"],
      ["Openness", "Connecting users, technology and partners"],
      ["Shared success", "Supporting every stage of growth"],
    ]
      .map(
        ([t, d]) =>
          `<article class="info-card"><h3>${t}</h3><p>${d}</p></article>`,
      )
      .join("")}</div></section>`;
  return `${pageBanner("Help and support", "Support at every step, from choosing to using", "HELP & SUPPORT")}<section class="container page-section"><div class="three-grid">${[
    [
      "products",
      "Choose a product",
      "Browse available products and configure your purchase here.",
    ],
    [
      "knowledge",
      "Technical documentation",
      "Read published documentation and guides.",
    ],
    ["downloads", "Downloads", "Download publicly available resources."],
  ]
    .map(
      ([p, t, d]) =>
        `<a class="info-card" href="/${p}.html"><h3>${t}</h3><p>${d}</p><span>View now →</span></a>`,
    )
    .join("")}</div><div class="faq"><h2>Frequently asked questions</h2>${[
    [
      "How do I purchase a product?",
      "Choose a product, configure it here and add it to your cart. Review your order before payment.",
    ],
    [
      "How are prices calculated?",
      "Prices depend on configuration, currency and billing term. Review the final quote before ordering.",
    ],
    [
      "How do I register my website?",
      "Registration requirements depend on location and service. Contact your provider for the applicable process.",
    ],
    [
      "Where can I manage purchased services?",
      "Open the console and sign in to manage purchased services and access support.",
    ],
  ]
    .map(([q, a]) => `<details><summary>${q}</summary><p>${a}</p></details>`)
    .join("")}</div></section>`;
}

export function shell(main, home) {
  return `<a class="skip-link" href="#main">Skip to main content</a><header class="site-header ${home ? "over-hero" : ""}"><div class="header-inner"><a class="brand" href="/" aria-label="Home"><span class="brand-mark"></span><span data-brand></span></a><select id="language" class="language-switch" aria-label="Language"><option value="zh-CN">中文</option><option value="en-US">English</option></select><button class="menu-toggle" aria-expanded="false" aria-controls="header-menu" aria-label="Open navigation">☰</button><div id="header-menu" class="header-menu"><nav id="header-nav" aria-label="Main navigation"></nav><div class="header-tools"><a href="/cart.html">Cart</a><a data-customer="/account" href="/help.html">Console</a><a href="/help.html">Registration help</a><a class="login" data-customer="/auth/login" href="/help.html">Sign in</a><a class="register" data-customer="/auth/register" href="/help.html">Register</a></div></div></div></header><div id="brand-status" class="brand-status" role="status" hidden></div><main id="main">${main}</main><section class="registration"><h2>Join <span data-brand></span> — start your cloud journey</h2><p>Your next chapter starts here</p><a data-customer="/auth/register" class="button" href="/help.html">Register now ${icon("arrow")}</a></section><div class="service-promises container">${[
    ["user", "Professional support"],
    ["clock", "Convenient online management"],
    ["shield", "Security consultation"],
    ["box", "One-stop product selection"],
  ]
    .map(([i, t]) => `<a href="/help.html">${icon(i)}${t}</a>`)
    .join(
      "",
    )}</div><footer><div class="container footer-main"><div data-default-footer><h3>Products</h3><div id="footer-categories"></div></div><div data-default-footer><h3>Documentation</h3><a href="/knowledge.html">Knowledge</a><a href="/announcements.html">Service announcements</a><a href="/downloads.html">Downloads</a><a href="/help.html">Help center</a></div><div data-default-footer><h3>About us</h3><a href="/about.html">Company profile</a><a href="/news.html">Company news</a><a href="/contact.html">Contact us</a></div><nav id="footer-nav" aria-label="Footer navigation"></nav><div class="footer-contact"><h3>Contact us</h3><strong data-phone>Phone not configured</strong><p data-email>Email not configured</p><p>Professional services for your next step</p></div></div><div class="container copyright"><span>Copyright © ${new Date().getFullYear()} <span data-company></span> All Rights Reserved.</span><span id="registration-links"></span></div></footer><aside class="floating-tools" aria-label="Quick services"><a href="/contact.html" aria-label="Contact sales">${icon("phone")}<span>Contact sales</span></a><a data-customer="/account" href="/help.html" aria-label="Customer center">${icon("user")}<span>Customer center</span></a><a href="/help.html" aria-label="Support">${icon("chat")}<span>Support</span></a><button id="back-top" aria-label="Back to top">↑<span>Back to top</span></button></aside>`;
}
