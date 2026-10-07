import { readFileSync } from "node:fs";

const output = readFileSync(new URL("../dist/index.js", import.meta.url), "utf8");
for (const marker of ["createVbenPluginAdapter", "fixedRightActionColumn"]) {
  if (!output.includes(marker)) {
    throw new Error(`missing adapter export: ${marker}`);
  }
}

const isolated = await import(new URL('../dist/isolated/index.js', import.meta.url).href);
if (typeof isolated.mountIsolatedVbenTable !== 'function') throw new Error('missing isolated table loader');
if (typeof isolated.mountIsolatedVbenFormDrawer !== 'function') throw new Error('missing isolated form drawer loader');
const loader = readFileSync(new URL('../dist/isolated/index.js', import.meta.url), 'utf8');
const version = readFileSync(new URL('../../plugin.yaml', import.meta.url), 'utf8').match(/^version:\s*(\S+)/m)?.[1];
if (!version || !loader.includes(`/x-assets/vben-adapter/${version}/isolated/runtime.js`)) throw new Error('isolated runtime URL does not match plugin version');
readFileSync(new URL('../dist/isolated/runtime.js', import.meta.url));
readFileSync(new URL('../dist/isolated/vben-plugin-adapter.css', import.meta.url));
