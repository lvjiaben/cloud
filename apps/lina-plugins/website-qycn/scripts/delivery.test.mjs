// Exercise the actual preview proxy and builder in an isolated source copy.
import { test } from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import { once } from "node:events";
import { spawn, execFile } from "node:child_process";
import { promisify } from "node:util";
import {
  cp,
  mkdtemp,
  symlink,
  rm,
  writeFile,
  access,
  readFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const source = fileURLToPath(new URL("..", import.meta.url));
const run = promisify(execFile);
async function fixture(t, options = {}) {
  const directory = await mkdtemp(resolve(tmpdir(), "qycn-delivery-"));
  let backend, preview;
  const requests = [];
  t.after(async () => {
    if (preview && preview.exitCode === null && preview.signalCode === null) {
      const exited = once(preview, "exit");
      preview.kill();
      await exited;
    }
    if (backend) await new Promise((done) => backend.close(done));
    await rm(directory, { recursive: true, force: true });
  });
  for (const name of ["frontend", "scripts", "plugin.yaml", "package.json"])
    await cp(resolve(source, name), resolve(directory, name), {
      recursive: true,
      filter: (path) => !relative(source, path).split(sep).includes("dist"),
    });
  await symlink(
    resolve(source, "node_modules"),
    resolve(directory, "node_modules"),
    process.platform === "win32" ? "junction" : "dir",
  );
  backend = http
    .createServer((req, res) => {
      requests.push({
        path: req.url,
        headers: req.headers,
        method: req.method,
      });
      if (options.handler) return options.handler(req, res);
      res.writeHead(req.headers.range ? 206 : 200, {
        "content-type": "application/octet-stream",
        "content-disposition": "attachment; filename=guide.pdf",
        "accept-ranges": "bytes",
        etag: '"guide-v1"',
        ...(req.headers.range ? { "content-range": "bytes 0-3/8" } : {}),
      });
      res.end(req.headers.range ? "guid" : "guide-v1");
    })
    .listen(0, "127.0.0.1");
  await once(backend, "listening");
  preview = spawn(process.execPath, ["scripts/preview.mjs"], {
    cwd: directory,
    env: {
      ...process.env,
      PORT: "0",
      HOST: "127.0.0.1",
      WEBSITE_PROXY_IDLE_TIMEOUT_MS: String(options.timeout ?? 9000),
      WEBSITE_CUSTOMER_URL: options.customerUrl || "http://localhost:6032",
      WEBSITE_CUSTOMER_ALLOWED_ORIGINS: JSON.stringify(
        options.customerOrigins || [],
      ),
      WEBSITE_API_ORIGIN: `http://127.0.0.1:${backend.address().port}`,
    },
    stdio: ["ignore", "pipe", "pipe"],
  });
  const origin = await new Promise((resolveOrigin, reject) => {
    const timer = setTimeout(
      () => reject(new Error("Preview did not start")),
      15000,
    );
    let output = "";
    preview.once("exit", (code) => {
      clearTimeout(timer);
      reject(new Error(`Preview exited ${code}: ${output}`));
    });
    preview.stderr.on("data", (data) => {
      output += data;
    });
    preview.stdout.on("data", (data) => {
      output += data;
      const match = output.match(
        /Website preview: (http:\/\/127\.0\.0\.1:\d+)/u,
      );
      if (match) {
        clearTimeout(timer);
        resolveOrigin(match[1]);
      }
    });
  });
  return { directory, origin, requests };
}

test("downloads retain filenames, range semantics and anonymous upstream requests", async (t) => {
  const { origin, requests } = await fixture(t);
  const path = "/x/idc-core/api/v1/public/content/downloads/ticket";
  const response = await fetch(origin + path, {
    headers: {
      Range: "bytes=0-3",
      "If-Range": '"guide-v1"',
      Cookie: "session=secret",
      Authorization: "Bearer secret",
    },
  });
  assert.equal(response.status, 206);
  assert.equal(
    response.headers.get("content-disposition"),
    "attachment; filename=guide.pdf",
  );
  assert.equal(response.headers.get("content-range"), "bytes 0-3/8");
  assert.equal(response.headers.get("accept-ranges"), "bytes");
  assert.equal(await response.text(), "guid");
  const upstream = requests.at(-1);
  assert.equal(upstream.headers.range, "bytes=0-3");
  assert.equal(upstream.headers["if-range"], '"guide-v1"');
  assert.equal(upstream.headers.cookie, undefined);
  assert.equal(upstream.headers.authorization, undefined);
  const head = await fetch(origin + path, { method: "HEAD" });
  assert.equal(head.status, 200);
  assert.equal(
    head.headers.get("content-disposition"),
    "attachment; filename=guide.pdf",
  );
  assert.equal(await head.text(), "");
});

test("preview rejects private endpoints and mutations except public download tickets", async (t) => {
  const { origin, requests } = await fixture(t);
  const count = requests.length;
  assert.equal(
    (await fetch(origin + "/x/idc-core/api/v1/admin/orders")).status,
    404,
  );
  assert.equal(
    (
      await fetch(origin + "/x/idc-core/api/v1/public/catalog/products", {
        method: "POST",
      })
    ).status,
    405,
  );
  assert.equal(requests.length, count);
  const response = await fetch(
    origin + "/x/idc-core/api/v1/public/content/items/item-1/download-tickets",
    { method: "POST" },
  );
  assert.equal(response.status, 200);
  await response.text();
  assert.equal(requests.at(-1).method, "POST");
});

test("announcements proxy exposes only public list and detail reads", async (t) => {
  const { origin, requests } = await fixture(t);
  for (const path of ["/x/content-notice/api/v1/public/announcements?pageNum=1&pageSize=12", "/x/content-notice/api/v1/public/announcements/321"]) {
    const response = await fetch(origin + path);
    assert.equal(response.status, 200);
    await response.text();
    assert.equal(requests.at(-1).path, path);
  }
  const count = requests.length;
  assert.equal((await fetch(origin + "/x/content-notice/api/v1/public/announcements/321", { method: "PUT" })).status, 405);
  assert.equal(requests.length, count);
});

test("rebuild removes retired assets and refuses unimplemented declared pages", async (t) => {
  const { directory } = await fixture(t);
  const retired = resolve(directory, "frontend/dist/retired.js");
  await writeFile(retired, "retired source");
  await run(process.execPath, ["scripts/build.mjs"], { cwd: directory });
  await assert.rejects(access(retired), { code: "ENOENT" });
  await access(resolve(directory, "frontend/dist/locales/zh-CN.js"));
  const manifestPath = resolve(directory, "plugin.yaml");
  const manifest = await readFile(manifestPath, "utf8");
  await writeFile(
    manifestPath,
    manifest.replace("id: home", "id: missing-page"),
  );
  await assert.rejects(
    run(process.execPath, ["scripts/build.mjs"], { cwd: directory }),
    /Declared page is not implemented/u,
  );
});

test(
  "stalled response headers and bodies time out and release the upstream",
  { timeout: 5000 },
  async (t) => {
    let closed;
    const disconnected = new Promise((done) => {
      closed = done;
    });
    const { origin } = await fixture(t, {
      timeout: 200,
      handler(req, res) {
        if (req.url.endsWith("/body")) {
          res.writeHead(200, { "content-type": "application/octet-stream" });
          res.write("partial");
          res.on("close", closed);
        }
      },
    });
    const headers = await fetch(
      origin + "/x/idc-core/api/v1/public/content/downloads/headers",
    );
    assert.equal(headers.status, 504);
    await headers.text();
    const body = await fetch(
      origin + "/x/idc-core/api/v1/public/content/downloads/body",
    );
    await assert.rejects(body.text());
    await disconnected;
  },
);

test(
  "active streams may exceed idle timeout and downstream cancellation closes upstream",
  { timeout: 5000 },
  async (t) => {
    let closed;
    const disconnected = new Promise((done) => {
      closed = done;
    });
    const { origin } = await fixture(t, {
      timeout: 200,
      handler(req, res) {
        res.writeHead(200, { "content-type": "application/octet-stream" });
        res.write("start");
        let chunks = 0;
        const ticker = setInterval(() => {
          res.write("x");
          if (++chunks === 10 && req.url.endsWith("/complete")) res.end("end");
        }, 50);
        res.on("close", () => {
          clearInterval(ticker);
          if (req.url.endsWith("/cancel")) closed();
        });
      },
    });
    const complete = await fetch(
      origin + "/x/idc-core/api/v1/public/content/downloads/complete",
    );
    assert.equal(await complete.text(), "start" + "x".repeat(10) + "end");
    const controller = new AbortController();
    const cancel = await fetch(
      origin + "/x/idc-core/api/v1/public/content/downloads/cancel",
      { signal: controller.signal },
    );
    const reader = cancel.body.getReader();
    await reader.read();
    controller.abort();
    await assert.rejects(reader.read());
    await disconnected;
  },
);

test(
  "upstream truncation cannot become a successful file and HTTP errors keep their metadata",
  { timeout: 5000 },
  async (t) => {
    const { origin } = await fixture(t, {
      handler(req, res) {
        if (req.url.endsWith("/broken")) {
          res.writeHead(200, { "content-length": "100" });
          res.write("short");
          setTimeout(() => res.destroy(), 25);
        } else {
          res.writeHead(req.headers.range ? 416 : 503, {
            "content-type": "application/json",
            "content-length": "2",
            "retry-after": "30",
            "content-range": "bytes */8",
          });
          res.end("{}");
        }
      },
    });
    const broken = await fetch(
      origin + "/x/idc-core/api/v1/public/content/downloads/broken",
    );
    await assert.rejects(broken.text());
    for (const method of ["GET", "HEAD"]) {
      const response = await fetch(
        origin + "/x/idc-core/api/v1/public/content/downloads/error",
        { method, headers: { Range: "bytes=999-" } },
      );
      assert.equal(response.status, 416);
      assert.equal(response.headers.get("content-length"), "2");
      assert.equal(response.headers.get("content-range"), "bytes */8");
      assert.equal(await response.text(), method === "HEAD" ? "" : "{}");
    }
    const unavailable = await fetch(
      origin + "/x/idc-core/api/v1/public/content/downloads/error",
    );
    assert.equal(unavailable.status, 503);
    assert.equal(unavailable.headers.get("retry-after"), "30");
    await unavailable.text();
  },
);

test("upstream redirects cannot escape the fixed public API origin", async (t) => {
  let followed = false;
  const { origin } = await fixture(t, {
    handler(req, res) {
      if (req.url === "/private") {
        followed = true;
        res.end("private");
        return;
      }
      res.writeHead(302, { location: "/private" });
      res.end();
    },
  });
  const response = await fetch(origin + "/x/idc-portal/api/v1/public/brand");
  assert.equal(response.status, 502);
  await response.text();
  assert.equal(followed, false);
});

test("purchase proxy scopes credentials, cookies, methods and request bodies", async (t) => {
  const bodies = [];
  const { origin, requests } = await fixture(t, {
    handler: async (req, res) => {
      let body = "";
      for await (const chunk of req) body += chunk;
      bodies.push(body);
      res.writeHead(200, {
        "content-type": "application/json",
        "set-cookie": [
          "lina_idc_guest_cart=owned; HttpOnly; Path=/",
          "unrelated=secret; Path=/",
        ],
      });
      res.end(
        JSON.stringify({ code: 0, data: { cart: { version: 1, items: [] } } }),
      );
    },
  });
  const headers = {
    authorization: "Bearer customer-token",
    cookie: "admin_session=private; lina_idc_guest_cart=owned",
    "content-type": "application/json",
  };
  const guest = await fetch(
    origin + "/x/idc-core/api/v1/public/commerce/cart-items",
    { method: "POST", headers, body: '{"quantity":1}' },
  );
  assert.equal(guest.status, 200);
  assert.equal(requests.at(-1).headers.authorization, undefined);
  assert.equal(requests.at(-1).headers.cookie, "lina_idc_guest_cart=owned");
  assert.equal(bodies.at(-1), '{"quantity":1}');
  assert.deepEqual(guest.headers.getSetCookie(), [
    "lina_idc_guest_cart=owned; HttpOnly; Path=/",
  ]);
  const account = await fetch(
    origin + "/x/idc-core/api/v1/account/commerce/quotes",
    { method: "POST", headers, body: '{"expectedCartVersion":1}' },
  );
  assert.equal(account.status, 200);
  assert.equal(requests.at(-1).headers.authorization, "Bearer customer-token");
  assert.equal(requests.at(-1).headers.cookie, "lina_idc_guest_cart=owned");
  for (const [path, method, body] of [
    [
      "/x/idc-core/api/v1/account/commerce/cart-items/ITEM",
      "PUT",
      '{"quantity":2}',
    ],
    [
      "/x/idc-core/api/v1/account/marketing/coupons/claims",
      "POST",
      '{"code":"TEST2026","deviceId":"fixture-device-0001"}',
    ],
    ["/x/idc-core/api/v1/account/billing/overview", "GET", undefined],
    [
      "/x/idc-payment/api/v1/account/payment-channels?currencyCode=CNY",
      "GET",
      undefined,
    ],
  ]) {
    const response = await fetch(origin + path, { method, headers, body });
    assert.equal(response.status, 200);
    assert.equal(
      requests.at(-1).headers.authorization,
      "Bearer customer-token",
    );
  }
  assert.equal(
    (
      await fetch(origin + "/x/idc-payment/api/v1/account/payments", {
        method: "POST",
        headers,
        body: "{}",
      })
    ).status,
    404,
  );
  assert.equal(
    (
      await fetch(origin + "/x/idc-core/api/v1/account/billing/overview", {
        method: "POST",
        headers,
        body: "{}",
      })
    ).status,
    405,
  );
  assert.equal(
    (
      await fetch(
        origin + "/x/idc-core/api/v1/account/commerce/cart-items/ITEM",
        { method: "PUT", headers, body: "x".repeat(65537) },
      )
    ).status,
    413,
  );
  const before = requests.length;
  assert.equal(
    (
      await fetch(origin + "/x/idc-core/api/v1/account/commerce/quotes", {
        method: "POST",
        headers: { ...headers, origin: "https://other.example" },
        body: "{}",
      })
    ).status,
    403,
  );
  assert.equal(
    (await fetch(origin + "/x/idc-core/api/v1/account/profile", { headers }))
      .status,
    404,
  );
  assert.equal(requests.length, before);
});

test("session handoff requires configured origin, a valid account and a bounded local return path", async (t) => {
  const { origin } = await fixture(t, {
    handler: (req, res) => {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(JSON.stringify({ code: 0, data: { cart: null } }));
    },
  });
  const send = (source, returnTo) =>
    fetch(origin + "/__website/session", {
      method: "POST",
      redirect: "manual",
      headers: {
        origin: source,
        "content-type": "application/x-www-form-urlencoded",
      },
      body: new URLSearchParams({ accessToken: "test-token", returnTo }),
    });
  assert.equal((await send("https://other.example", "/cart.html")).status, 403);
  assert.equal(
    (await send("http://localhost:6032", "//evil.example/")).status,
    400,
  );
  const response = await send("http://localhost:6032", "/cart.html?lang=en-US");
  assert.equal(response.status, 303);
  assert.equal(response.headers.get("location"), "/cart.html?lang=en-US");
  const cookie = response.headers.get("set-cookie");
  assert.ok(cookie.includes("HttpOnly"));
  assert.ok(!cookie.includes("test-token"));
  const session = await fetch(origin + "/__website/session", {
    headers: { cookie: cookie.split(";")[0] },
  });
  assert.deepEqual(await session.json(), {
    checked: true,
    accessToken: "test-token",
  });
  const repeated = await fetch(origin + "/__website/session", {
    headers: { cookie: cookie.split(";")[0] },
  });
  assert.deepEqual(await repeated.json(), { checked: false, accessToken: "" });
});

test("customer session trusts only the explicit deployment origins", async (t) => {
  const { origin, requests } = await fixture(t, {
    customerUrl: "http://127.0.0.1:6032",
    customerOrigins: ["http://127.0.0.1:16745"],
    handler: (_req, res) => {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(JSON.stringify({ code: 0, data: { cart: null } }));
    },
  });
  for (const customerOrigin of [
    "http://127.0.0.1:6032",
    "http://127.0.0.1:16745",
  ]) {
    const response = await fetch(origin + "/__website/session", {
      method: "POST",
      redirect: "manual",
      headers: {
        Origin: customerOrigin,
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: new URLSearchParams({
        accessToken: "customer-token",
        returnTo: "/configure.html?product=11",
      }),
    });
    assert.equal(response.status, 303);
    assert.equal(
      response.headers.get("location"),
      "/configure.html?product=11",
    );
    const cookie = response.headers.get("set-cookie").split(";")[0];
    const session = await fetch(origin + "/__website/session", {
      headers: { cookie },
    });
    assert.deepEqual(await session.json(), {
      checked: true,
      accessToken: "customer-token",
    });
  }
  const before = requests.length;
  for (const customerOrigin of [
    "http://127.0.0.1:6033",
    "http://localhost:6032",
    "https://127.0.0.1:6032",
    "http://127.0.0.1:6032.evil.example",
  ]) {
    const response = await fetch(origin + "/__website/session", {
      method: "POST",
      headers: { Origin: customerOrigin },
      body: new URLSearchParams({ accessToken: "customer-token" }),
    });
    assert.equal(response.status, 403);
  }
  assert.equal(requests.length, before);
  assert.deepEqual(
    await (await fetch(origin + "/__website/configuration")).json(),
    { customerOrigin: "http://127.0.0.1:6032" },
  );
});

test("customer origin settings reject wildcards and path-based origins", async (t) => {
  for (const value of [
    "*",
    "http://*.example",
    "http://127.0.0.1:6032/account",
    "http://user:pass@127.0.0.1:6032",
    "http://127.0.0.1:6032?source=evil",
  ]) {
    await assert.rejects(
      () => fixture(t, { customerOrigins: [value] }),
      /Invalid (?:customer|URL)/,
    );
  }
});

test("server login uses the exact authenticated account route and keeps public writes closed", async (t) => {
  let received;
  const { origin, requests } = await fixture(t, { handler: async (req, res) => {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    received = JSON.parse(Buffer.concat(chunks).toString());
    res.writeHead(200, { "content-type": "application/json", "cache-control": "no-store" });
    res.end(JSON.stringify({ code: 0, data: { serverLoginToken: "opaque-test-reference" } }));
  } });
  const path = "/x/idc-core/api/v1/account/catalog/products/server/purchase-configuration/server-login";
  const response = await fetch(origin + path, {
    method: "POST", headers: { authorization: "Bearer test-session", "content-type": "application/json", origin },
    body: JSON.stringify({ configurationRevision: "revision", hostname: "server-one", password: "TestPassword7!" }),
  });
  assert.equal(response.status, 200);
  assert.equal((await response.json()).data.serverLoginToken, "opaque-test-reference");
  assert.equal(requests.at(-1).headers.authorization, "Bearer test-session");
  assert.equal(received.hostname, "server-one");
  assert.equal(received.password, "TestPassword7!");
  const count = requests.length;
  assert.equal((await fetch(origin + path.replace("/account/", "/public/"), { method: "POST", body: "{}" })).status, 405);
  assert.equal((await fetch(origin + path, { method: "POST", headers: { origin: "https://untrusted.test" }, body: "{}" })).status, 403);
  assert.equal(requests.length, count);
});
