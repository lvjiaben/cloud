import { access, readFile, readdir } from "node:fs/promises";

const entryPath = new URL("../dist/mount.js", import.meta.url);
await access(entryPath);

const builtModule = await import(`${entryPath.href}?verify=${Date.now()}`);
if (typeof builtModule.mount !== "function") {
  throw new Error("The plugin bundle must export mount(context).");
}

const outputDirectory = new URL("../dist/", import.meta.url);
const outputFiles = await readdir(outputDirectory);
const extractedStyles = outputFiles.filter((file) => file.endsWith(".css"));
if (extractedStyles.length > 0) {
  throw new Error(
    `Plugin styles must be embedded by mount.js; host-unloaded CSS found: ${extractedStyles.join(", ")}`,
  );
}

const bundleSource = await readFile(entryPath, "utf8");
if (!bundleSource.includes(".server-batch-workspace .batch-builder")) {
  throw new Error("The server batch layout styles are missing from mount.js.");
}
