// Standalone template packager implementing the documented public WASM sections.
// No repository discovery, host source import, shell interpolation or remote writes.
import {
  readFile,
  writeFile,
  mkdir,
  readdir,
  stat,
  access,
} from "node:fs/promises";
import { resolve, relative, extname } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { parse } from "yaml";
const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const source = parse(await readFile(resolve(root, "plugin.yaml"), "utf8"));
if (source.type !== "dynamic" || !source.id || !source.version)
  throw new Error("A versioned dynamic manifest is required");
const output = resolve(root, "dist");
await mkdir(output, { recursive: true });
const runtime = resolve(output, "runtime.wasm");
const build = spawnSync(
  "go",
  ["build", "-buildmode=c-shared", "-o", runtime, "."],
  {
    cwd: root,
    env: { ...process.env, GOWORK: "off", GOOS: "wasip1", GOARCH: "wasm" },
    stdio: "inherit",
  },
);
if (build.status !== 0) process.exit(build.status || 1);
const camel = (key) =>
  key === "en-US"
    ? "enUS"
    : key === "zh-CN"
      ? "zhCN"
      : key.replace(/_([a-z])/gu, (_, c) => c.toUpperCase());
function convert(value) {
  if (Array.isArray(value)) return value.map(convert);
  if (value && typeof value === "object")
    return Object.fromEntries(
      Object.entries(value).map(([key, next]) => [
        camel(key),
        key === "values" ? next : convert(next),
      ]),
    );
  return value;
}
const manifest = {};
for (const key of [
  "id",
  "name",
  "version",
  "type",
  "distribution",
  "scope_nature",
  "supports_multi_tenant",
  "default_install_mode",
  "description",
  "dependencies",
])
  if (source[key] !== undefined) manifest[camel(key)] = source[key];
for (const key of [
  "business_extensions",
  "admin_surfaces",
  "customer_surfaces",
  "menus",
  "public_assets",
])
  if (source[key])
    manifest[key] =
      key === "menus" || key === "public_assets"
        ? source[key]
        : convert(source[key]);
const mime = {
  ".html": "text/html; charset=utf-8",
  ".js": "application/javascript",
  ".css": "text/css",
  ".json": "application/json",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".ico": "image/x-icon",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".webp": "image/webp",
  ".jpg": "image/jpeg",
};
const assets = [];
async function walk(dir) {
  for (const name of (await readdir(dir)).sort()) {
    const path = resolve(dir, name);
    const info = await stat(path);
    if (info.isDirectory()) await walk(path);
    else
      assets.push({
        path: relative(root, path).replaceAll("\\", "/"),
        contentBase64: (await readFile(path)).toString("base64"),
        contentType: mime[extname(path)] || "application/octet-stream",
      });
  }
}
for (const item of source.public_assets || [])
  await walk(resolve(root, item.source));
for (const extension of source.business_extensions || []) {
  const values = extension.provider_metadata?.values;
  if (!values?.entry) continue;
  const assetRoot = resolve(root, source.public_assets[0].source);
  await access(resolve(assetRoot, values.entry));

  if (values.spa) await access(resolve(root, "frontend/src/main.tsx"));
}
const localeAssets = [];
const manifestResources = [];
const localeRoot = resolve(root, "manifest/i18n");
function merge(target, next) {
  for (const [key, value] of Object.entries(next)) {
    if (value && typeof value === "object" && !Array.isArray(value))
      merge((target[key] ??= {}), value);
    else target[key] = value;
  }
}
async function readLocale(dir, messages) {
  for (const name of (await readdir(dir)).sort()) {
    const path = resolve(dir, name);
    const info = await stat(path);
    if (info.isDirectory()) {
      if (name !== "apidoc") await readLocale(path, messages);
    } else if (name.endsWith(".json")) {
      const content = await readFile(path);
      merge(messages, JSON.parse(content));
      manifestResources.push({
        path: relative(root, path).replaceAll("\\", "/"),
        contentBase64: content.toString("base64"),
      });
    }
  }
}
try {
  for (const locale of (await readdir(localeRoot)).sort()) {
    const dir = resolve(localeRoot, locale);
    if (!(await stat(dir)).isDirectory()) continue;
    const messages = {};
    await readLocale(dir, messages);
    localeAssets.push({ locale, content: JSON.stringify(messages) });
  }
} catch (error) {
  if (error.code !== "ENOENT") throw error;
}
let bytes = await readFile(runtime);
function varint(value) {
  const out = [];
  do {
    let b = value & 127;
    value = Math.floor(value / 128);
    if (value) b |= 128;
    out.push(b);
  } while (value);
  return Buffer.from(out);
}
function section(name, value) {
  const label = Buffer.from(name);
  const payload = Buffer.from(JSON.stringify(value));
  const content = Buffer.concat([varint(label.length), label, payload]);
  bytes = Buffer.concat([
    bytes,
    Buffer.from([0]),
    varint(content.length),
    content,
  ]);
}
section("lina.plugin.manifest", manifest);
section("lina.plugin.dynamic", {
  runtimeKind: "wasm",
  abiVersion: "v1",
  frontendAssetCount: assets.length,
  i18nAssetCount: localeAssets.length,
  manifestResourceCount: manifestResources.length,
});
section("lina.plugin.frontend.assets", assets);
if (localeAssets.length) section("lina.plugin.i18n.assets", localeAssets);
if (manifestResources.length)
  section("lina.plugin.manifest.resources", manifestResources);
section("lina.plugin.backend.bridge", {
  abiVersion: "v1",
  runtimeKind: "wasm",
  routeExecution: true,
  requestCodec: "protobuf",
  responseCodec: "protobuf",
  allocExport: "lina_dynamic_route_alloc",
  executeExport: "lina_dynamic_route_execute",
});
await writeFile(resolve(output, source.id + ".wasm"), bytes);
console.log(
  "Packaged " +
    source.id +
    " " +
    source.version +
    ": " +
    assets.length +
    " assets, " +
    bytes.length +
    " bytes",
);
