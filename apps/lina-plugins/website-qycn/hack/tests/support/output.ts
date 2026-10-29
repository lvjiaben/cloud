// Keep test artifacts in repository temp, with a writable standalone fallback.
import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
const plugin = fileURLToPath(new URL("../../..", import.meta.url));
const candidate = resolve(plugin, "../../..");
const workspace = existsSync(resolve(candidate, "AGENTS.md"))
  ? candidate
  : plugin;
const day = new Date().toISOString().slice(0, 10).replaceAll("-", "");
export const evidenceDirectory =
  process.env.E2E_OUTPUT_DIR || resolve(workspace, "temp", day);
