import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import type { Page } from "@host-tests/support/playwright";

import { config } from "../../../../../../hack/tests/fixtures/config";

// Exercise the actual independent artifact without publishing a retired route
// or changing the real host's menus. Network mocks remain scoped to this page.
export async function openStandaloneWorkspace(page: Page, routePath: string) {
  const bundle = readFileSync(
    resolve(import.meta.dirname, "../../../frontend/dist/mount.js"),
    "utf8",
  );
  const stylesheet = readFileSync(
    resolve(
      import.meta.dirname,
      "../../../../vben-adapter/frontend/dist/isolated/vben-plugin-adapter.css",
    ),
    "utf8",
  );
  await page.route("**/__idc-core-test/mount.js", (route) =>
    route.fulfill({ body: bundle, contentType: "text/javascript" }),
  );
  const runtimeDirectory = resolve(
    import.meta.dirname,
    "../../../../vben-adapter/frontend/dist/isolated",
  );
  const runtimeFiles = new Map(
    readdirSync(runtimeDirectory)
      .filter((file) => file.endsWith(".js"))
      .map((file) => [
        file,
        readFileSync(resolve(runtimeDirectory, file), "utf8"),
      ]),
  );
  await page.route("**/x-assets/vben-adapter/v0.1.0/isolated/*.js*", (route) => {
    const file = new URL(route.request().url()).pathname.split("/").at(-1)!;
    const body = runtimeFiles.get(file);
    return body
      ? route.fulfill({ body, contentType: "text/javascript" })
      : route.abort("failed");
  });
  await page.route("**/__idc-core-test/styles.css", (route) =>
    route.fulfill({ body: stylesheet, contentType: "text/css" }),
  );
  await page.route("**/__idc-core-standalone-test", (route) =>
    route.fulfill({
      contentType: "text/html",
      body: `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><link rel="stylesheet" href="/__idc-core-test/styles.css"></head><body><div id="root"></div><script type="module">
      import { mount } from "/__idc-core-test/mount.js";
      mount({ container: document.getElementById("root"), routePath: ${JSON.stringify(routePath)}, route: { path: ${JSON.stringify(routePath)} }, locale: "zh-CN", accessToken: "", assetURL: "", baseURL: "", messages: {}, query: {}, title: "", t: (key, fallback) => fallback ?? key });
    </script></body></html>`,
    }),
  );
  await page.goto(new URL("/__idc-core-standalone-test", config.baseURL).href, {
    waitUntil: "domcontentloaded",
  });
}
