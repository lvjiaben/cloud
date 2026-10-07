import { access } from "node:fs/promises";
const entryPath = new URL("../dist/mount.js", import.meta.url);
await access(entryPath);
const builtModule = await import(`${entryPath.href}?verify=${Date.now()}`);
if (typeof builtModule.mount !== "function") throw new Error("The plugin bundle must export mount(context).");
