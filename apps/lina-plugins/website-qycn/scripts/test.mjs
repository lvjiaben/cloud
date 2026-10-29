// Compile the independent guest with no repository workspace dependency.
import { spawnSync } from "node:child_process";
const result = spawnSync("go", ["test", "./...", "-count=1"], {
  cwd: new URL("..", import.meta.url),
  env: { ...process.env, GOWORK: "off" },
  stdio: "inherit",
});
if (result.status !== 0) process.exit(result.status ?? 1);
const delivery = spawnSync(
  process.execPath,
  ["--test", "scripts/delivery.test.mjs", "scripts/download.test.mjs"],
  {
    cwd: new URL("..", import.meta.url),
    stdio: "inherit",
  },
);
process.exit(delivery.status ?? 1);
