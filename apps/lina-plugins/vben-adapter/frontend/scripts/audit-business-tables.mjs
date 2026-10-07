import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { extname, join, relative, resolve } from "node:path";

const repository = fileURLToPath(new URL("../../../../../", import.meta.url));
const plugins = join(repository, "apps/lina-plugins");
const roots = [
  "apps/lina-vben/apps/web-antd/src",
  "apps/lina-vben/apps/web-customer/src",
  ...readdirSync(plugins, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => `apps/lina-plugins/${entry.name}/frontend`),
];

const excludedDirectories = new Set(["node_modules", "dist", "isolated-dist", "scripts", "tests", "__tests__"]);
const sourceExtensions = new Set([".vue", ".ts", ".js", ".html"]);
// Include string templates and DOM factories, not only Vue tags. A component
// name resembling Vben does not make an underlying native table compliant.
export function nativeTableLines(source) {
  const nativeTable = /<(?:table|a-table|aTable|data-table|el-table|n-data-table|vxe-table|vxe-grid|VxeGrid)(?=[\s>])|(?:createElement|\bel|\bh)\(\s*["'`]table["'`]/giu;
  return [...source.matchAll(nativeTable)].map((match) => source.slice(0, match.index).split("\n").length);
}

function collect(directory) {
  let entries;
  try {
    entries = readdirSync(directory, { withFileTypes: true });
  } catch (error) {
    if (error.code === "ENOENT") return [];
    throw error;
  }
  return entries.flatMap((entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      return excludedDirectories.has(entry.name) ? [] : collect(path);
    }
    if (!entry.isFile() || !sourceExtensions.has(extname(path)) || /\.(?:test|spec)\./u.test(entry.name)) return [];
    return [path];
  });
}

const findings = roots.flatMap((root) => collect(resolve(repository, root))).flatMap((path) => {
  const source = readFileSync(path, "utf8");
  const lines = nativeTableLines(source);
  return lines.length ? [{
    file: relative(repository, path).replaceAll("\\", "/"),
    lines,
  }] : [];
});

// Reporting mode supports an honest migration inventory. Strict mode must not
// pass while even a separately delivered fallback retains a native table.
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  console.log(JSON.stringify({ nativeTableSources: findings.length, findings }, null, 2));
  if (process.argv.includes("--check") && findings.length) process.exitCode = 1;
}
