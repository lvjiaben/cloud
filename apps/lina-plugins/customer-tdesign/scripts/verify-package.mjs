import { readFile } from "node:fs/promises";
import { WASI } from "node:wasi";
import assert from "node:assert/strict";
const bytes = await readFile(
  new URL("../dist/customer-tdesign.wasm", import.meta.url),
);
const module = await WebAssembly.compile(bytes);
const section = (name) => {
  const values = WebAssembly.Module.customSections(module, name);
  assert.equal(values.length, 1);
  return JSON.parse(Buffer.from(values[0]).toString());
};
const manifest = section("lina.plugin.manifest");
assert.equal(manifest.id, "customer-tdesign");
assert.equal(
  manifest.business_extensions[0].providerMetadata.values.renderer,
  "customer-tdesign-v1",
);
const assets = section("lina.plugin.frontend.assets");
const index = assets.find((a) => a.path === "frontend/dist/index.html");
assert.ok(index);
const html = Buffer.from(index.contentBase64, "base64").toString();
for (const [, url] of html.matchAll(/(?:src|href)="(\.\/assets\/[^"]+)"/g))
  assert.ok(
    assets.some((a) => a.path === "frontend/dist/" + url.slice(2)),
    url,
  );
const wasi = new WASI({ version: "preview1", args: [], env: {}, preopens: {} });
const instance = await WebAssembly.instantiate(module, {
  wasi_snapshot_preview1: wasi.wasiImport,
});
wasi.initialize(instance);
const varint = (n) => {
  const b = [];
  do {
    let v = n & 127;
    n >>>= 7;
    if (n) v |= 128;
    b.push(v);
  } while (n);
  return Buffer.from(b);
};
const field = (n, b) => {
  b = Buffer.from(b);
  return Buffer.concat([varint((n << 3) | 2), varint(b.length), b]);
};
const invocation = {
  version: "v1",
  ownerPluginId: "idc-portal",
  providerPluginId: "customer-tdesign",
  point: "customer-template",
  contractVersion: "v1",
  method: "capabilities.describe",
  releaseVersion: "v0.1.0",
  releaseDigest: "a".repeat(64),
  declarationDigest: "b".repeat(64),
  purpose: "new-business",
  runtimeContext: { contextId: "isolated-package-verification" },
  payload: {},
};
const call = (body) => {
  const request = Buffer.concat([
    field(1, "customer-tdesign"),
    field(2, field(7, "BusinessExtensionBridgeRequest")),
    field(3, field(13, JSON.stringify(body))),
  ]);
  const ptr = instance.exports.lina_dynamic_route_alloc(request.length);
  new Uint8Array(instance.exports.memory.buffer, ptr, request.length).set(
    request,
  );
  const result = instance.exports.lina_dynamic_route_execute(request.length);
  return Buffer.from(
    new Uint8Array(
      instance.exports.memory.buffer,
      Number(result >> 32n),
      Number(result & 0xffffffffn),
    ),
  ).toString();
};
assert.ok(call(invocation).includes('"pluginId":"customer-tdesign"'));
assert.ok(
  !call({ ...invocation, ownerPluginId: "forged" }).includes('"pluginId"'),
);
console.log(
  `Verified WASI descriptor, forged-call rejection, manifest and ${assets.length} embedded assets.`,
);
