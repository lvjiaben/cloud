// Local preview with fixed upstreams and method-scoped public purchase contracts.
import http from "node:http";
import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";
import { resolve, extname } from "node:path";
import { fileURLToPath } from "node:url";
import { Readable, Transform } from "node:stream";
import { pipeline } from "node:stream/promises";
import "./build.mjs";
const root = fileURLToPath(new URL("../frontend/dist/", import.meta.url));
const backend = new URL(
  process.env.WEBSITE_API_ORIGIN || "http://127.0.0.1:6030",
);
const idleTimeout = Number(process.env.WEBSITE_PROXY_IDLE_TIMEOUT_MS || 9000);
if (
  !Number.isSafeInteger(idleTimeout) ||
  idleTimeout < 1 ||
  idleTimeout > 2147483647
)
  throw new Error("Invalid proxy idle timeout");
const customerUrl = process.env.WEBSITE_CUSTOMER_URL || "http://localhost:6032";
if (
  !["http:", "https:"].includes(backend.protocol) ||
  backend.username ||
  backend.password
)
  throw new Error("Invalid API origin");
const customer = new URL(customerUrl);
if (
  !["http:", "https:"].includes(customer.protocol) ||
  customer.username ||
  customer.hostname.includes("*") ||
  customer.password
)
  throw new Error("Invalid customer origin");
// Deployment origins are explicit operator settings, never learned from a request.
const configuredCustomerOrigins = JSON.parse(
  process.env.WEBSITE_CUSTOMER_ALLOWED_ORIGINS || "[]",
);
if (!Array.isArray(configuredCustomerOrigins))
  throw new Error("Invalid customer origins");
const customerOrigins = new Set([customer.origin]);
for (const value of configuredCustomerOrigins) {
  if (typeof value !== "string") throw new Error("Invalid customer origin");
  const origin = new URL(value);
  if (
    !["http:", "https:"].includes(origin.protocol) ||
    origin.username ||
    origin.hostname.includes("*") ||
    origin.password ||
    origin.pathname !== "/" ||
    origin.search ||
    origin.hash
  )
    throw new Error("Invalid customer origin");
  customerOrigins.add(origin.origin);
}
const mime = {
  ".json": "application/json",
  ".html": "text/html; charset=utf-8",
  ".js": "application/javascript",
  ".css": "text/css",
  ".jpg": "image/jpeg",
  ".png": "image/png",
  ".svg": "image/svg+xml",
};
const handoffs = new Map();
const server = http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url, "http://preview.invalid");
    if (/%2f|%5c|%00/iu.test(url.pathname)) {
      res.writeHead(400);
      return res.end();
    }
    if (url.pathname === "/__website/session") {
      const cookie = "qycn_website_handoff";
      const flags =
        "; Path=/__website/session; HttpOnly; SameSite=Lax" +
        (req.socket.encrypted ? "; Secure" : "");
      for (const [id, item] of handoffs)
        if (item.expires < Date.now()) handoffs.delete(id);
      if (req.method === "POST") {
        if (!customerOrigins.has(req.headers.origin)) {
          res.writeHead(403);
          return res.end();
        }
        let body = "";
        for await (const chunk of req) {
          body += chunk;
          if (Buffer.byteLength(body) > 16384) {
            res.writeHead(413);
            return res.end();
          }
        }
        const form = new URLSearchParams(body),
          returnTo = form.get("returnTo") || "/cart.html",
          accessToken = form.get("accessToken") || "";
        if (
          !/^\/(?:cart|configure|products|services)\.html(?:\?[^#\r\n]*)?$/u.test(
            returnTo,
          ) ||
          /[\r\n]/u.test(accessToken) ||
          accessToken.length > 8192
        ) {
          res.writeHead(400);
          return res.end();
        }
        if (accessToken) {
          const check = await fetch(
            new URL("/x/idc-core/api/v1/account/commerce/cart", backend),
            {
              headers: { Authorization: "Bearer " + accessToken },
              signal: AbortSignal.timeout(10000),
              redirect: "error",
            },
          );
          if (!check.ok) {
            await check.body?.cancel();
            res.writeHead(403);
            return res.end();
          }
          const payload = await check.json();
          if (String(payload.code) !== "0") {
            res.writeHead(403);
            return res.end();
          }
        }
        if (handoffs.size >= 100) {
          res.writeHead(503);
          return res.end();
        }
        const id = randomUUID();
        handoffs.set(id, { accessToken, expires: Date.now() + 30000 });
        res.writeHead(303, {
          location: returnTo,
          "cache-control": "no-store",
          "set-cookie": cookie + "=" + id + flags + "; Max-Age=30",
        });
        return res.end();
      }
      if (req.method === "GET") {
        if (
          req.headers["sec-fetch-site"] &&
          !["same-origin", "none"].includes(req.headers["sec-fetch-site"])
        ) {
          res.writeHead(403);
          return res.end();
        }
        const raw = String(req.headers.cookie || "")
          .split(";")
          .map((v) => v.trim())
          .find((v) => v.startsWith(cookie + "="));
        const id = raw?.slice(cookie.length + 1),
          item = handoffs.get(id);
        handoffs.delete(id);
        res.writeHead(200, {
          "content-type": "application/json",
          "cache-control": "no-store",
          "set-cookie": cookie + "=" + flags + "; Max-Age=0",
        });
        return res.end(
          JSON.stringify({
            checked: Boolean(item),
            accessToken: item?.accessToken || "",
          }),
        );
      }
      res.writeHead(405);
      return res.end();
    }
    if (url.pathname === "/__website/configuration" && req.method === "GET") {
      res.writeHead(200, {
        "content-type": "application/json",
        "cache-control": "no-store",
      });
      return res.end(JSON.stringify({ customerOrigin: customer.origin }));
    }
    if (
      url.pathname.startsWith("/x/") ||
      url.pathname.startsWith("/x-assets/")
    ) {
      const upload =
        /^\/x\/idc-core\/api\/v1\/account\/catalog\/(products\/[^/]+\/purchase-configuration\/file-uploads|purchase-configuration\/file-uploads\/CFU_[A-Fa-f0-9]{32}(?:\/content)?)$/u.test(
          url.pathname,
        );
      const account =
        upload ||
        url.pathname === "/x/idc-payment/api/v1/account/payment-channels" ||
        url.pathname === "/x/idc-core/api/v1/account/billing/overview" ||
        /^\/x\/idc-core\/api\/v1\/account\/(marketing\/coupons(?:\/claims)?|commerce\/(cart|carts|cart-items(?:\/[^/]+)?|guest-cart-merge|quotes|orders|quote-items\/[^/]+\/purchase-configuration)|catalog\/products\/[^/]+\/purchase-configuration\/(quote|evaluate|server-login))$/u.test(
          url.pathname,
        );
      const guestCart =
        /^\/x\/idc-core\/api\/v1\/public\/commerce\/(cart|carts|cart-items(?:\/[^/]+)?)$/u.test(
          url.pathname,
        );
      const purchase =
        /^\/x\/idc-core\/api\/v1\/(public|account)\/catalog\/products\/[^/]+\/purchase-configuration\/(evaluate|quote)$/u.test(
          url.pathname,
        );
      const allowed =
        account ||
        guestCart ||
        /^\/x-assets\/[a-z0-9-]+\/[^/]+\//u.test(url.pathname) ||
        /^\/x\/content-notice\/api\/v1\/public\/announcements(?:\/\d+)?$/u.test(url.pathname) ||
        url.pathname === "/x/idc-portal/api/v1/public/brand" ||
        /^\/x\/idc-core\/api\/v1\/public\/(catalog\/products|content\/items|content\/downloads)(\/|$)/u.test(
          url.pathname,
        );
      const postAllowed =
        (upload && /\/products\//u.test(url.pathname)) ||
        purchase ||
        (account && /\/purchase-configuration\/server-login$/u.test(url.pathname)) ||
        (account && url.pathname.endsWith("/marketing/coupons/claims")) ||
        (/\/(carts|cart-items|guest-cart-merge|quotes|orders|purchase-configuration)$/u.test(
          url.pathname,
        ) &&
          (account || guestCart)) ||
        /^\/x\/idc-core\/api\/v1\/public\/content\/items\/[^/]+\/download-tickets$/u.test(
          url.pathname,
        );
      if (!allowed) {
        res.writeHead(404);
        return res.end();
      }
      if (
        !["GET", "HEAD"].includes(req.method) &&
        !(req.method === "POST" && postAllowed) &&
        !(
          req.method === "PUT" &&
          ((upload && url.pathname.endsWith("/content")) ||
            (account && /\/commerce\/cart-items\/[^/]+$/u.test(url.pathname)))
        ) &&
        !(
          req.method === "DELETE" &&
          (account || guestCart) &&
          /\/cart-items\/[^/]+$/u.test(url.pathname)
        )
      ) {
        res.writeHead(405);
        return res.end();
      }
      if (
        !["GET", "HEAD"].includes(req.method) &&
        req.headers.origin &&
        req.headers.origin !== `http://${req.headers.host}`
      ) {
        res.writeHead(403);
        return res.end();
      }
      let body;
      if (req.method === "POST" || req.method === "PUT") {
        const chunks = [];
        let size = 0;
        for await (const chunk of req) {
          size += chunk.length;
          if (
            size >
            (upload && url.pathname.endsWith("/content")
              ? 64 * 1024 * 1024
              : 65536)
          ) {
            res.writeHead(413);
            return res.end();
          }
          chunks.push(chunk);
        }
        body = Buffer.concat(chunks);
      }
      const controller = new AbortController();
      const timer = setTimeout(
        () =>
          controller.abort(
            new DOMException("Upstream stalled", "TimeoutError"),
          ),
        idleTimeout,
      );
      const cancel = () => controller.abort();
      res.on("close", cancel);
      try {
        const headers = {
          Accept: "application/json",
          "Accept-Language":
            req.headers["accept-language"] === "en-US" ? "en-US" : "zh-CN",
          "Accept-Encoding": "identity",
        };
        for (const name of ["range", "if-range"]) {
          if (typeof req.headers[name] === "string")
            headers[name] = req.headers[name];
        }

        if (body)
          headers["Content-Type"] =
            req.method === "PUT"
              ? "application/octet-stream"
              : "application/json";
        if (
          account &&
          /^Bearer [^\r\n]+$/u.test(req.headers.authorization || "")
        )
          headers.Authorization = req.headers.authorization;
        if (account || guestCart) {
          const cookie = String(req.headers.cookie || "")
            .split(";")
            .map((v) => v.trim())
            .find((v) => /^lina_idc_guest_cart=[A-Za-z0-9_.%-]+$/u.test(v));
          if (cookie) headers.Cookie = cookie;
        }
        const upstream = await fetch(
          new URL(url.pathname + url.search, backend),
          {
            method: req.method,
            body,
            headers,
            signal: controller.signal,
            redirect: "error",
          },
        );
        timer.refresh();
        const responseHeaders = { "cache-control": "no-store" };
        for (const name of [
          "content-type",
          "content-disposition",
          "content-range",
          "accept-ranges",
          "etag",
          "last-modified",
          "retry-after",
        ]) {
          if (upstream.headers.has(name))
            responseHeaders[name] = upstream.headers.get(name);
        }
        // Fetch decodes compressed bodies; their wire length no longer applies.
        if (
          !upstream.headers.has("content-encoding") &&
          upstream.headers.has("content-length")
        ) {
          responseHeaders["content-length"] =
            upstream.headers.get("content-length");
        }
        if (account || guestCart) {
          const cookies = upstream.headers
            .getSetCookie()
            .filter((value) => value.startsWith("lina_idc_guest_cart="));
          if (cookies.length) responseHeaders["set-cookie"] = cookies;
        }
        res.writeHead(upstream.status, responseHeaders);
        if (req.method === "HEAD" || !upstream.body) return res.end();
        const progress = new Transform({
          transform(chunk, encoding, callback) {
            timer.refresh();
            callback(null, chunk);
          },
        });
        await pipeline(Readable.fromWeb(upstream.body), progress, res);
      } finally {
        clearTimeout(timer);
        res.off("close", cancel);
        controller.abort();
      }
      return;
    }
    if (!["GET", "HEAD"].includes(req.method)) {
      res.writeHead(405);
      return res.end();
    }
    const path = decodeURIComponent(url.pathname);
    if (path.includes("..") || path.includes("\\") || path.includes("\0")) {
      res.writeHead(404);
      return res.end();
    }
    const file = resolve(root, path === "/" ? "index.html" : "." + path);
    let body = await readFile(file);
    if (extname(file) === ".html") {
      const runtime = JSON.stringify({ customerUrl }).replaceAll(
        "<",
        "\\u003c",
      );
      body = Buffer.from(
        body
          .toString()
          .replace(
            "<head>",
            `<head><script>window.__LINA_TEMPLATE__=${runtime}</script>`,
          ),
      );
    }
    res.writeHead(200, {
      "content-type": mime[extname(file)] || "application/octet-stream",
      "cache-control": "no-store",
    });
    res.end(req.method === "HEAD" ? undefined : body);
  } catch (error) {
    if (res.destroyed) return;
    if (res.headersSent) {
      res.destroy();
      return;
    }
    if (error.code === "ENOENT") {
      res.writeHead(404, { "content-type": "text/plain; charset=utf-8" });
      res.end("页面不存在");
    } else {
      const code = error.name === "TimeoutError" ? 504 : 502;
      res.writeHead(code, { "content-type": "application/json" });
      res.end(JSON.stringify({ code, message: "公开接口暂时不可用" }));
    }
  }
});
server.listen(
  Number(process.env.PORT || 16880),
  process.env.HOST || "127.0.0.1",
  () =>
    console.log("Website preview: http://127.0.0.1:" + server.address().port),
);
