// Preview the packaged release on a private local port without changing active templates.
import http from "node:http";
import { readFile } from "node:fs/promises";
const wasm = await WebAssembly.compile(
  await readFile(new URL("../dist/customer-tdesign.wasm", import.meta.url)),
);
const section = (name) =>
  JSON.parse(
    Buffer.from(WebAssembly.Module.customSections(wasm, name)[0]).toString(),
  );
const manifest = section("lina.plugin.manifest"),
  assets = section("lina.plugin.frontend.assets");
const base = `/x-assets/${manifest.id}/${manifest.version}/`,
  files = new Map(
    assets.map((a) => [base + a.path.replace(/^frontend\/dist\//, ""), a]),
  );
const backend = new URL(
  process.env.CUSTOMER_API_ORIGIN || "http://127.0.0.1:6032",
);
const website = process.env.CUSTOMER_WEBSITE_CONFIG ? JSON.parse(process.env.CUSTOMER_WEBSITE_CONFIG) : null;
if (website) {
  const origin = new URL(website.websiteurl);
  if (!["http:", "https:"].includes(origin.protocol) || origin.username || origin.password || origin.pathname !== "/" || origin.search || origin.hash) throw new Error("Invalid website origin");
}
const injection = website ? `<script>window.__CUSTOMER_WEBSITE__=${JSON.stringify(website).replaceAll("<", "\\u003c")}</script>` : "";
const index = Buffer.from(
  files.get(base + "index.html").contentBase64,
  "base64",
)
  .toString()
  .replace("<head>", `<head><base href="${base}">${injection}`);
const server = http.createServer((req, res) => {
  const url = new URL(req.url, "http://localhost");
  if (
    ["/api/", "/x/", "/__website/"].some((prefix) =>
      url.pathname.startsWith(prefix),
    ) || (url.pathname.startsWith("/x-assets/") && !files.has(url.pathname))
  ) {
    const upstream = http.request(
      new URL(req.url, backend),
      { method: req.method, headers: { ...req.headers, host: backend.host } },
      (r) => {
        res.writeHead(r.statusCode, r.headers);
        r.pipe(res);
      },
    );
    upstream.on("error", () => {
      res.writeHead(502);
      res.end("API unavailable");
    });
    req.pipe(upstream);
    return;
  }
  if (!["GET", "HEAD"].includes(req.method)) {
    res.writeHead(405);
    res.end();
    return;
  }
  const asset = files.get(url.pathname);
  if (asset) {
    res.writeHead(200, {
      "content-type": asset.contentType,
      "cache-control": "no-store",
    });
    res.end(
      req.method === "HEAD"
        ? undefined
        : Buffer.from(asset.contentBase64, "base64"),
    );
    return;
  }
  if (url.pathname.startsWith("/x-assets/")) {
    res.writeHead(404);
    res.end();
    return;
  }
  res.writeHead(200, {
    "content-type": "text/html; charset=utf-8",
    "cache-control": "no-store",
  });
  res.end(req.method === "HEAD" ? undefined : index);
});
server.listen(
  Number(process.env.CUSTOMER_PREVIEW_PORT || 16745),
  "127.0.0.1",
  () =>
    console.log(
      "Packaged customer preview at http://127.0.0.1:" + server.address().port,
    ),
);
