import { readFileSync, readdirSync } from "node:fs";
import type { Page } from "@playwright/test";

// Mirror the complete published artifact, including hashed imports. Fulfilling
// bytes locally avoids Chrome's opaque-sandbox-to-loopback network restriction
// without relaxing sandbox flags, bridging permissions, or changing the bundle.
export async function serveChannelVbenRuntime(page: Page) {
  const runtimeDirectory = new URL("../../../../vben-adapter/frontend/dist/isolated/", import.meta.url);
  const runtimeAssets = new Set(readdirSync(runtimeDirectory).filter((name) => /^[\w.-]+\.(js|css)$/.test(name)));
  await page.route(
    "**/x-assets/vben-adapter/v0.1.0/isolated/*",
    async (route) => {
      const name = new URL(route.request().url()).pathname.split("/").at(-1);
      if (!name || !runtimeAssets.has(name))
        return route.abort();
      await route.fulfill({
        headers: { "Access-Control-Allow-Origin": "*" },
        contentType: name?.endsWith(".css") ? "text/css" : "text/javascript",
        body: readFileSync(
          new URL(name, runtimeDirectory),
          "utf8",
        ),
      });
    },
  );
}

// Serve the delivered plugin entry unchanged, with an isolated test-only bridge.
export async function openChannelStandalone(
  page: Page,
  entry: "admin-mount" | "mount",
) {
  const messages = JSON.parse(readFileSync(new URL("../../../manifest/i18n/zh-CN/plugin.json", import.meta.url), "utf8"));
  await serveChannelVbenRuntime(page);
  await page.route("**/__channel-test/*.js", async (route) => {
    const name = new URL(route.request().url()).pathname.split("/").at(-1);
    if (
      !["admin-mount.js", "mount.js", "channel-table-runtime.js"].includes(
        name || "",
      )
    )
      return route.abort();
    await route.fulfill({
      contentType: "text/javascript",
      body: readFileSync(
        new URL("../../../frontend/pages/" + name, import.meta.url),
        "utf8",
      ),
    });
  });
  await page.route("**/__channel-standalone-test", (route) =>
    route.fulfill({
      contentType: "text/html",
      body: `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"></head><body><div id="root"></div><script type="module">
      import {mount} from "/__channel-test/${entry}.js";
      const messages=${JSON.stringify(messages)};
      mount({container:document.getElementById("root"),locale:"zh-CN",t:(key,fallback)=>messages[key]||fallback,request:async(path,options={})=>{
        const url=new URL("/x/idc-channel/api/v1/"+path,location.origin);
        Object.entries(options.params||{}).forEach(([key,value])=>url.searchParams.set(key,String(value)));
        const response=await fetch(url,{method:options.method||"GET",headers:{"Content-Type":"application/json"},body:options.body?JSON.stringify(options.body):undefined});
        const payload=await response.json();if(!response.ok)throw new Error("bridge_request_failed");return payload.data;
      }});
    </script></body></html>`,
    }),
  );
  await page.goto("/__channel-standalone-test", {
    waitUntil: "domcontentloaded",
  });
}
