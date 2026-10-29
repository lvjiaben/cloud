// Emit real multi-page HTML documents and local resources with no host dependency.
import { cp, mkdir, readFile, writeFile, access, rm } from "node:fs/promises";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "yaml";
import {
  shell,
  homepage,
  catalogPage,
  servicesPage,
  contentPage,
  articlePage,
  infoPage,
} from "../frontend/views.js";
import { translateMarkup } from "../frontend/i18n.js";
const root = fileURLToPath(new URL("..", import.meta.url));
const output = resolve(root, "frontend/dist");
const manifest = parse(await readFile(resolve(root, "plugin.yaml"), "utf8"));
const pages = [
  ...manifest.business_extensions[0].provider_metadata.values.pages,
  { id: "article", nameZh: "资讯详情", path: "/article.html" },
];
const implemented = new Set([
  "home",
  "configure",
  "cart",
  "products",
  "services",
  "article",
  "news",
  "announcements",
  "knowledge",
  "downloads",
  "datacenters",
  "help",
  "about",
  "contact",
]);
for (const page of pages) {
  if (
    !implemented.has(page.id) ||
    !(page.path === "/" || /^\/[a-z0-9-]+\.html$/u.test(page.path))
  ) {
    throw new Error("Declared page is not implemented: " + page.path);
  }
}
// Rebuild only this plugin's generated directory; retired assets must not ship.
await rm(output, { recursive: true, force: true });
await mkdir(output, { recursive: true });
for (const name of [
  "app.js",
  "i18n.js",
  "commerce.js",
  "purchase-controls.js",
  "api.js",
  "content.js",
  "locales",
  "views.js",
  "style.css",
  "media",
])
  await cp(resolve(root, "frontend", name), resolve(output, name), {
    recursive: true,
  });
for (const page of pages) {
  const main =
    page.id === "cart" || page.id === "configure"
      ? `<main class="container commerce-page" id="commerce-page"><h1>${page.nameZh}</h1></main>`
      : page.id === "home"
        ? homepage()
        : page.id === "products"
          ? catalogPage()
          : page.id === "services"
            ? servicesPage(new URLSearchParams())
            : page.id === "article"
              ? articlePage()
              : ["news", "announcements", "knowledge", "downloads"].includes(
                    page.id,
                  )
                ? contentPage(page.id)
                : infoPage(page.id);
  const filename = page.path === "/" ? "index.html" : page.path.slice(1);
  const html = `<!doctype html>\n<html lang="zh-CN"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="description" content="${page.nameZh}：云产品、企业服务与技术资讯，连接您的云端业务。"><meta name="theme-color" content="#00a6e8"><title>${page.nameZh}</title><link rel="icon" href="data:,"><link rel="stylesheet" href="./style.css"><script type="module" src="./app.js"></script></head><body data-page="${page.id}" data-title="${page.nameZh}" data-title-en="${page.nameEn || "Article"}">${translateMarkup(shell(main, page.id === "home"))}<noscript><p>请启用 JavaScript，以加载产品、资讯和导航配置。</p></noscript></body></html>\n`;
  await writeFile(resolve(output, filename), html);
}
for (const page of pages)
  await access(
    resolve(output, page.path === "/" ? "index.html" : page.path.slice(1)),
  );
console.log(`Built ${pages.length} public pages for ${manifest.id}`);
