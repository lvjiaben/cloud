import { readFileSync, readdirSync } from "node:fs";
import { spawnSync } from "node:child_process";
import path from "node:path";

import { validateCiShards } from "./ci-shards.mjs";
import {
  allowlistCategoriesForFile,
  detectRiskCategories,
  e2eDir,
  exists,
  highRiskRules,
  isolationAllowlist,
  isPluginWorkspaceReady,
  isHostTcFile,
  isPluginTcFile,
  knownIsolationCategorySet,
  legacyGlobalTcFilePattern,
  listLegacyPluginE2EDirs,
  listPluginE2EFiles,
  listSourcePluginIdentifiers,
  listTcFiles,
  loadManifest,
  moduleRequiresPluginWorkspace,
  playwrightFileArg,
  pluginTestEntry,
  pluginTestRelativePath,
  repoRoot,
  resolveEntries,
  serialCategoryMap,
  serialFileSet,
  serialIsolationEntries,
  testsDir,
  toPosix,
  walk,
} from "./execution-governance.mjs";

const manifest = loadManifest();
const errors = [];
const pluginWorkspaceReady = isPluginWorkspaceReady();
const highRiskRuleByCategory = new Map(
  highRiskRules.map((rule) => [rule.category, rule]),
);
const knownCategories = knownIsolationCategorySet();

function addError(message) {
  errors.push(message);
}

function validateFrontendI18nKeys() {
  const isWindows = process.platform === "win32";
  const command = isWindows ? (process.env.ComSpec ?? "cmd.exe") : "pnpm";
  const pnpmArgs = ["-F", "@lina/web-antd", "i18n:check"];
  const args = isWindows ? ["/d", "/s", "/c", "pnpm", ...pnpmArgs] : pnpmArgs;
  const result = spawnSync(command, args, {
    cwd: path.resolve(testsDir, "../../apps/lina-vben"),
    encoding: "utf8",
  });

  if (result.status !== 0) {
    addError(
      [
        "Frontend i18n key validation failed.",
        result.error?.message ?? "",
        result.stdout?.trim() ?? "",
        result.stderr?.trim() ?? "",
      ]
        .filter(Boolean)
        .join("\n"),
    );
  }
}

validateFrontendI18nKeys();

function validateIdcVbenUiGovernance() {
  const pluginRoot = path.resolve(repoRoot, "apps", "lina-plugins");
  const customerViewsRoot = path.resolve(
    repoRoot,
    "apps",
    "lina-vben",
    "apps",
    "web-customer",
    "src",
    "views",
  );
  const forbidden = [
    {
      label: "native form/table/control/button markup",
      pattern: /<(?:form|table|input|select|textarea|button)\b/,
    },
    {
      label: "Ant Table instead of the official Vben VxeGrid adapter",
      pattern:
        /import\s*\{[^}]*\bTable\b[^}]*\}\s*from\s*['"]ant-design-vue['"]/s,
    },
    {
      label: "automatic grid height that can collapse the visible rows",
      pattern: /height\s*:\s*['"]auto['"]/,
    },
    {
      label:
        "imperative grid.loadData can run before a conditional VxeGrid is mounted; update reactive data through setGridOptions instead",
      pattern: /\.grid\.loadData\s*\(/,
    },
    {
      label: "large decorative hero/header block",
      pattern: /(?:marketing-header|[-_]hero\b)/,
    },
  ];
  const complexAntFormAllowlist = new Map([
    [
      [
        "apps",
        "lina-plugins",
        "idc-core",
        "frontend",
        "pages",
        "components",
        "catalog-configuration-form.vue",
      ].join("/"),
      "the host compiles a provider-declared, versioned thirteen-type field graph with bounded conditional visibility, candidate filtering, secret preservation, tree/address linkage, and governed file state at runtime",
    ],
    [
      [
        "apps",
        "lina-plugins",
        "idc-core",
        "frontend",
        "pages",
        "financial-management.vue",
      ].join("/"),
      "one guarded editor switches between four high-risk financial action contracts",
    ],
    [
      [
        "apps",
        "lina-plugins",
        "idc-compliance",
        "frontend",
        "pages",
        "components",
        "electronic-contract-template-editor.vue",
      ].join("/"),
      "rich text, file revisions, signer placement, and unsaved-change protection form one editor",
    ],
    [
      [
        "apps",
        "lina-plugins",
        "idc-resource",
        "frontend",
        "pages",
        "server-accessories.vue",
      ].join("/"),
      "runtime server actions and batch previews use different payload and dependency graphs",
    ],
    [
      [
        "apps",
        "lina-plugins",
        "content-notice",
        "frontend",
        "pages",
        "notice-modal.vue",
      ].join("/"),
      "the notice editor embeds Tiptap rich text and dynamic attachment upload state",
    ],
    [
      "apps/lina-vben/apps/web-customer/src/views/public/presales/create.vue",
      "anonymous presales creation combines human verification, OTP, attachments, and a short-lived session state machine",
    ],
    [
      "apps/lina-vben/apps/web-customer/src/views/public/presales/detail.vue",
      "anonymous ticket access combines OTP renewal, replies, attachments, rating, and short-link state",
    ],
    [
      "apps/lina-vben/apps/web-customer/src/views/account/credit/index.vue",
      "credit evidence uses a private staged-upload lifecycle whose upload numbers are part of the guarded submission",
    ],
    [
      "apps/lina-vben/apps/web-customer/src/views/account/servers/index.vue",
      "runtime server operations use action-specific resources, quantities, quotes, and confirmation payloads",
    ],
    [
      "apps/lina-vben/apps/web-customer/src/views/account/products/configure.vue",
      "the customer purchase editor renders a plugin-declared, versioned multi-type field graph with conditional reevaluation and governed staged-file state",
    ],
  ]);

  const pluginPageRoots = exists(pluginRoot)
    ? readdirSync(pluginRoot, { withFileTypes: true })
        .filter((entry) => entry.isDirectory())
        .map((entry) => path.join(pluginRoot, entry.name, "frontend", "pages"))
        .filter(exists)
    : [];
  const sourceFiles = [
    ...pluginPageRoots.flatMap((root) => walk(root)),
    ...walk(customerViewsRoot),
  ];
  for (const file of sourceFiles) {
    if (!/\.(?:ts|vue)$/.test(file)) continue;
    const source = readFileSync(file, "utf8");
    const relativeFile = toPosix(path.relative(repoRoot, file));
    for (const rule of forbidden) {
      if (rule.pattern.test(source)) {
        addError(
          `IDC Vben UI governance rejected ${toPosix(path.relative(repoRoot, file))}: ${rule.label}.`,
        );
      }
    }
    if (/<Modal\b/.test(source) && !/const\s*\[\s*Modal\s*,/.test(source)) {
      addError(
        `IDC Vben UI governance rejected ${toPosix(path.relative(repoRoot, file))}: direct modal markup must use useVbenModal.`,
      );
    }
    if (/<Drawer\b/.test(source) && !/const\s*\[\s*Drawer\s*,/.test(source)) {
      addError(
        `IDC Vben UI governance rejected ${toPosix(path.relative(repoRoot, file))}: direct drawer markup must use useVbenDrawer.`,
      );
    }
    const importsAntForm =
      /import\s*\{[^}]*\bForm(?:Item)?\b[^}]*\}\s*from\s*['"]ant-design-vue['"]/s.test(
        source,
      );
    if (importsAntForm && !complexAntFormAllowlist.has(relativeFile)) {
      addError(
        `IDC Vben UI governance rejected ${relativeFile}: Ant Form requires a documented complex-editor exception; use useVbenForm for ordinary forms.`,
      );
    }
  }
}

validateIdcVbenUiGovernance();

function readTestFile(relativePath) {
  if (relativePath.startsWith("apps/lina-plugins/")) {
    return readFileSync(path.resolve(repoRoot, relativePath), "utf8");
  }
  return readFileSync(path.resolve(testsDir, relativePath), "utf8");
}

function requireArray(value, label) {
  if (!Array.isArray(value)) {
    addError(`${label} must be an array.`);
    return [];
  }
  return value;
}

function validateCategories(categories, ownerLabel) {
  const values = requireArray(categories, `${ownerLabel}.categories`);
  if (values.length === 0) {
    addError(`${ownerLabel}.categories must contain at least one category.`);
    return [];
  }

  const seen = new Set();
  for (const category of values) {
    if (typeof category !== "string" || category.trim() === "") {
      addError(
        `${ownerLabel}.categories contains a non-string or empty category.`,
      );
      continue;
    }
    if (!knownCategories.has(category)) {
      addError(
        `${ownerLabel}.categories contains unknown category "${category}". Known categories: ${[...knownCategories].sort().join(", ")}`,
      );
    }
    if (seen.has(category)) {
      addError(
        `${ownerLabel}.categories contains duplicate category "${category}".`,
      );
    }
    seen.add(category);
  }
  return values;
}

function validateReason(reason, ownerLabel) {
  if (typeof reason !== "string" || reason.trim() === "") {
    addError(
      `${ownerLabel}.reason must explain why the isolation decision is safe.`,
    );
  }
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/gu, "\\$&");
}

const sourcePluginIdentifiers = listSourcePluginIdentifiers();
const hostPluginFrameworkReferenceAllowlist = new Set([
  "e2e/extension/plugin/TC019-plugin-management-manage-entry.ts",
  // This validator owns the explicit plugin-path exceptions used by the UI
  // governance scan, so it necessarily contains current source plugin IDs.
  "scripts/validate-e2e.mjs",
]);
const forbiddenRootPluginIdPattern =
  sourcePluginIdentifiers.length > 0
    ? new RegExp(
        `(?:^|[^\\w-])(?:${sourcePluginIdentifiers.map(escapeRegExp).join("|")})(?=$|[^\\w-])`,
        "u",
      )
    : null;

const allFiles = [
  ...walk(e2eDir).map((item) => toPosix(path.relative(testsDir, item))),
  ...listPluginE2EFiles(),
];
const legacyPluginE2EDirs = listLegacyPluginE2EDirs();
const testFiles = [];
const tcRegistryByDirectory = new Map();
const allowedFiles = new Set(
  Object.values(manifest.moduleScopes)
    .flat()
    .flatMap((entry) => listTcFiles(entry)),
);
const rootGovernanceFiles = [
  "config/execution-manifest.json",
  "config/service-dependency-baseline.json",
  "README.md",
  "README.zh-CN.md",
  ...walk(e2eDir).map((item) => toPosix(path.relative(testsDir, item))),
  ...walk(path.resolve(testsDir, "fixtures")).map((item) =>
    toPosix(path.relative(testsDir, item)),
  ),
  ...walk(path.resolve(testsDir, "pages")).map((item) =>
    toPosix(path.relative(testsDir, item)),
  ),
  ...walk(path.resolve(testsDir, "scripts")).map((item) =>
    toPosix(path.relative(testsDir, item)),
  ),
  ...walk(path.resolve(testsDir, "support")).map((item) =>
    toPosix(path.relative(testsDir, item)),
  ),
].filter((file) => /\.(?:json|mjs|md|ts)$/u.test(file));
const pluginGovernanceFiles = sourcePluginIdentifiers
  .flatMap((pluginId) =>
    ["e2e", "pages", "support"].flatMap((segment) =>
      walk(
        path.resolve(
          repoRoot,
          "apps/lina-plugins",
          pluginId,
          "hack/tests",
          segment,
        ),
      ),
    ),
  )
  .map((item) => toPosix(path.relative(repoRoot, item)))
  .filter((file) => /\.(?:mjs|ts)$/u.test(file));

function collectStrings(value, owner, results = []) {
  if (typeof value === "string") {
    results.push({ owner, value });
    return results;
  }
  if (Array.isArray(value)) {
    value.forEach((item, index) =>
      collectStrings(item, `${owner}[${index}]`, results),
    );
    return results;
  }
  if (value && typeof value === "object") {
    Object.entries(value).forEach(([key, item]) =>
      collectStrings(item, `${owner}.${key}`, results),
    );
  }
  return results;
}

function validateRootManifestPluginEntries() {
  for (const item of collectStrings(manifest, "execution-manifest")) {
    if (item.value.startsWith("apps/lina-plugins/")) {
      addError(
        `${item.owner} must not reference plugin workspace paths. Use the generic "plugins" module scope or runtime "plugin:<plugin-id>" selector instead.`,
      );
    }
    if (item.value.startsWith("plugins/")) {
      addError(
        `${item.owner} must not reference a concrete plugin entry "${item.value}". Use "plugins" in the manifest and "plugin:<plugin-id>" at runtime instead.`,
      );
    }
  }
}

function validateRootServiceDependencyBaseline() {
  const baselinePath = path.resolve(
    testsDir,
    "config/service-dependency-baseline.json",
  );
  const parsed = JSON.parse(readFileSync(baselinePath, "utf8"));
  const entries = requireArray(
    parsed.entries ?? [],
    "service-dependency-baseline.entries",
  );
  for (const [index, entry] of entries.entries()) {
    const owner = `service-dependency-baseline.entries[${index}]`;
    if (!entry || typeof entry !== "object") {
      addError(`${owner} must be an object.`);
      continue;
    }
    if (typeof entry.path !== "string" || entry.path.trim() === "") {
      addError(`${owner}.path must be a non-empty string.`);
      continue;
    }
    if (entry.path.startsWith("apps/lina-plugins/")) {
      addError(
        `${owner}.path must not point to ${entry.path}. Plugin-specific service dependency baselines belong under apps/lina-plugins/<plugin-id>/hack/tests/config/.`,
      );
    }
  }
}

function validateDirectAuthLoginClientType(files) {
  const directAuthLoginPattern =
    /(?:\bfetch\s*\(\s*`[^`]*auth\/login|\bfetch\s*\(\s*['"][^'"]*auth\/login|\.post\s*\(\s*['"](?:\/api\/v1\/)?auth\/login['"])/gu;
  for (const file of files) {
    const absolutePath = file.startsWith("apps/lina-plugins/")
      ? path.resolve(repoRoot, file)
      : path.resolve(testsDir, file);
    const source = readFileSync(absolutePath, "utf8");
    let match;
    while ((match = directAuthLoginPattern.exec(source)) !== null) {
      const after = source.slice(match.index, match.index + 600);
      if (!after.includes("clientType")) {
        addError(
          `Direct auth/login request must include clientType in ${file}:${source.slice(0, match.index).split("\n").length}.`,
        );
      }
    }
  }
}

function entryExistsOrResolves(entry) {
  if (listTcFiles(entry).length > 0) {
    return true;
  }
  if (entry === pluginTestEntry) {
    return true;
  }
  if (entry.startsWith("plugins/") || entry.startsWith("apps/lina-plugins/")) {
    return false;
  }
  return exists(path.resolve(testsDir, entry));
}

function isPluginEntry(entry) {
  return (
    entry === pluginTestEntry ||
    entry.startsWith("plugins/") ||
    entry.startsWith("apps/lina-plugins/")
  );
}

function isPluginScope(scope, entries) {
  return (
    scope === pluginTestEntry || entries.some((entry) => isPluginEntry(entry))
  );
}

validateRootManifestPluginEntries();
validateRootServiceDependencyBaseline();
validateDirectAuthLoginClientType([
  ...rootGovernanceFiles,
  ...pluginGovernanceFiles,
]);

for (const directory of legacyPluginE2EDirs) {
  const relativePath = pluginTestRelativePath(directory);
  addError(
    `Legacy plugin E2E directory found: ${relativePath}. Use apps/lina-plugins/<plugin-id>/hack/tests/{e2e,pages,support}/ instead.`,
  );
}

for (const file of allFiles) {
  if (!file.endsWith(".ts")) {
    addError(`Non-TypeScript file found under e2e: ${file}`);
    continue;
  }

  if (legacyGlobalTcFilePattern.test(file)) {
    addError(
      `Legacy global four-digit TC filename found: ${file}. Use module-local TC001 style numbering.`,
    );
    continue;
  }

  if (!isHostTcFile(file) && !isPluginTcFile(file)) {
    addError(`Non-test file found under e2e: ${file}`);
    continue;
  }

  testFiles.push(file);
  const tcId = file.match(/TC(\d{3})/u)?.[1];
  if (!tcId) {
    addError(`Unable to parse TC ID from ${file}`);
    continue;
  }
  const directory = path.posix.dirname(file);
  const items = tcRegistryByDirectory.get(directory) ?? [];
  items.push(file);
  tcRegistryByDirectory.set(directory, items);

  if (!allowedFiles.has(file)) {
    addError(`File is not under an allowed module scope: ${file}`);
  }

  const playwrightArg = playwrightFileArg(file);
  if (path.isAbsolute(playwrightArg)) {
    addError(
      `Playwright file argument must be repository-relative for ${file}: ${playwrightArg}`,
    );
  } else if (!exists(path.resolve(repoRoot, playwrightArg))) {
    addError(
      `Playwright file argument does not exist for ${file}: ${playwrightArg}`,
    );
  }
}

for (const [directory, files] of tcRegistryByDirectory.entries()) {
  const seen = new Map();
  const parsed = files
    .map((file) => {
      const tcId = file.match(/TC(\d{3})/u)?.[1] ?? "";
      return {
        file,
        number: Number.parseInt(tcId, 10),
      };
    })
    .sort(
      (left, right) =>
        left.number - right.number || left.file.localeCompare(right.file),
    );

  for (const item of parsed) {
    const duplicates = seen.get(item.number) ?? [];
    duplicates.push(item.file);
    seen.set(item.number, duplicates);
  }

  for (const [number, duplicates] of seen.entries()) {
    if (duplicates.length > 1) {
      addError(
        `Duplicate TC${String(number).padStart(3, "0")} in ${directory}: ${duplicates.join(", ")}`,
      );
    }
  }

  for (let index = 0; index < parsed.length; index += 1) {
    const expected = index + 1;
    if (parsed[index].number !== expected) {
      addError(
        `Module-local TC numbering must be continuous in ${directory}: expected TC${String(expected).padStart(3, "0")} but found ${parsed[index].file}.`,
      );
      break;
    }
  }
}

for (const [scope, entries] of Object.entries(manifest.moduleScopes)) {
  if (
    scope.startsWith("host:") &&
    moduleRequiresPluginWorkspace(scope, manifest)
  ) {
    addError(
      `Host-prefixed module scope must not require apps/lina-plugins: ${scope}`,
    );
  }
  if (!pluginWorkspaceReady && isPluginScope(scope, entries)) {
    continue;
  }
  const files = entries.flatMap((entry) => listTcFiles(entry));
  if (files.length === 0 && scope !== pluginTestEntry) {
    addError(`Module scope has no matching test files: ${scope}`);
  }
}

for (const file of rootGovernanceFiles) {
  const source = readFileSync(path.resolve(testsDir, file), "utf8");
  if (
    forbiddenRootPluginIdPattern?.test(source) &&
    !hostPluginFrameworkReferenceAllowlist.has(file)
  ) {
    addError(
      `Root E2E asset references an official source plugin ID: ${file}. Move plugin-specific coverage to apps/lina-plugins/<plugin-id>/hack/tests/.`,
    );
  }
}

for (const entry of manifest.smoke ?? []) {
  if (!entryExistsOrResolves(entry)) {
    addError(`Smoke entry does not exist: ${entry}`);
  }
}

const serialEntries = requireArray(manifest.serial ?? [], "serial");
for (const entry of serialEntries) {
  if (!entryExistsOrResolves(entry)) {
    addError(`Serial entry does not exist: ${entry}`);
  }
}

const isolationEntries = requireArray(
  serialIsolationEntries(manifest),
  "serialIsolation",
);
const serialEntrySet = new Set(serialEntries);
const isolationEntrySet = new Set();

for (const [index, item] of isolationEntries.entries()) {
  const owner = `serialIsolation[${index}]`;
  if (!item || typeof item !== "object") {
    addError(`${owner} must be an object.`);
    continue;
  }
  if (typeof item.entry !== "string" || item.entry.trim() === "") {
    addError(`${owner}.entry must be a non-empty string.`);
    continue;
  }
  if (!entryExistsOrResolves(item.entry)) {
    addError(`${owner}.entry does not exist: ${item.entry}`);
  }
  if (!serialEntrySet.has(item.entry)) {
    addError(`${owner}.entry is not listed in serial: ${item.entry}`);
  }
  if (isolationEntrySet.has(item.entry)) {
    addError(`${owner}.entry is duplicated: ${item.entry}`);
  }
  isolationEntrySet.add(item.entry);
  validateCategories(item.categories, owner);
  validateReason(item.reason, owner);

  const resolvedFiles = listTcFiles(item.entry);
  if (resolvedFiles.length === 0 && item.entry !== pluginTestEntry) {
    addError(`${owner}.entry does not resolve to any TC file: ${item.entry}`);
  }
}

for (const entry of serialEntries) {
  if (!isolationEntrySet.has(entry)) {
    addError(`Serial entry is missing serialIsolation metadata: ${entry}`);
  }
}

const serialFiles = serialFileSet(manifest);
const categoryMap = serialCategoryMap(manifest);
for (const file of serialFiles) {
  if (!categoryMap.has(file) || categoryMap.get(file).size === 0) {
    addError(`Serial file has no resolved isolation category: ${file}`);
  }
}

const allowlistEntries = requireArray(
  isolationAllowlist(manifest),
  "parallelIsolationAllowlist",
);
for (const [index, item] of allowlistEntries.entries()) {
  const owner = `parallelIsolationAllowlist[${index}]`;
  if (!item || typeof item !== "object") {
    addError(`${owner} must be an object.`);
    continue;
  }
  if (typeof item.file !== "string" || item.file.trim() === "") {
    addError(`${owner}.file must be a non-empty string.`);
    continue;
  }
  if (listTcFiles(item.file).length !== 1) {
    addError(`${owner}.file must reference one existing TC file: ${item.file}`);
  }
  if (serialFiles.has(item.file)) {
    addError(
      `${owner}.file is already serial and does not need a parallel allowlist: ${item.file}`,
    );
  }
  validateCategories(item.categories, owner);
  validateReason(item.reason, owner);
}

for (const file of testFiles) {
  const detectedCategories = detectRiskCategories(readTestFile(file));
  if (detectedCategories.size === 0) {
    continue;
  }

  const declaredSerialCategories = categoryMap.get(file) ?? new Set();
  const allowedParallelCategories = allowlistCategoriesForFile(file, manifest);
  for (const category of detectedCategories) {
    const rule = highRiskRuleByCategory.get(category);
    const label = rule?.label ?? category;
    if (serialFiles.has(file)) {
      if (!declaredSerialCategories.has(category)) {
        addError(
          `High-risk ${label} detected in serial file ${file}, but serialIsolation does not declare "${category}".`,
        );
      }
      continue;
    }

    if (!allowedParallelCategories.has(category)) {
      addError(
        `High-risk ${label} detected in parallel file ${file}. Add the file to serial with "${category}" isolation or add a documented parallelIsolationAllowlist entry.`,
      );
    }
  }
}

const resolvedSmoke = resolveEntries(manifest.smoke ?? []);
const unresolvedSmokeEntries = (manifest.smoke ?? []).filter(
  (entry) => listTcFiles(entry).length === 0,
);
for (const entry of unresolvedSmokeEntries) {
  addError(`Smoke entry does not resolve to any TC file: ${entry}`);
}

if (!manifest.ciShards || typeof manifest.ciShards !== "object") {
  addError(
    "execution-manifest.json must declare ciShards for CI E2E shard governance.",
  );
} else {
  for (const error of validateCiShards(manifest, {
    requireWorkspace: pluginWorkspaceReady,
  })) {
    addError(error);
  }
}

if (errors.length > 0) {
  console.error("E2E suite validation failed:");
  for (const error of errors) {
    console.error(`- ${error}`);
  }
  process.exit(1);
}

console.log(
  [
    `Validated ${testFiles.length} E2E test files`,
    `across ${Object.keys(manifest.moduleScopes).length} scopes.`,
    `Smoke files: ${resolvedSmoke.length}.`,
    `Serial files: ${serialFiles.size}.`,
    "CI shards: ok.",
  ].join(" "),
);
