import { existsSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const appRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const webAntdRoot = join(appRoot, '..', 'web-antd');

const requiredPaths = [
  '.env',
  '.env.development',
  '.env.production',
  'index.html',
  'package.json',
  'tsconfig.json',
  'tsconfig.node.json',
  'vite.config.mts',
  'src/adapter/component/index.ts',
  'src/adapter/form.ts',
  'src/app.vue',
  'src/api/request.ts',
  'src/bootstrap.ts',
  'src/components/global/index.ts',
  'src/layouts/auth.vue',
  'src/layouts/basic.vue',
  'src/layouts/index.ts',
  'src/locales/index.ts',
  'src/main.ts',
  'src/preferences.ts',
  'src/router/guard.ts',
  'src/router/index.ts',
  'src/router/routes/core.ts',
  'src/router/routes/index.ts',
  'src/store/auth.ts',
  'src/store/index.ts',
  'src/views/_core/authentication/login.vue',
  'src/views/_core/fallback/not-found.vue',
];

const forbiddenPaths = ['.nuxt', '.output', 'app', 'nuxt.config.ts', 'pages'];

function readText(relativePath) {
  return readFileSync(join(appRoot, relativePath), 'utf8');
}

function fail(message) {
  throw new Error(`[web-customer alignment] ${message}`);
}

for (const relativePath of requiredPaths) {
  if (!existsSync(join(webAntdRoot, relativePath))) {
    fail(`web-antd mother template is missing ${relativePath}`);
  }
  if (!existsSync(join(appRoot, relativePath))) {
    fail(`web-customer is missing Vben structure ${relativePath}`);
  }
}

for (const relativePath of forbiddenPaths) {
  if (existsSync(join(appRoot, relativePath))) {
    fail(`Nuxt structure is forbidden in web-customer: ${relativePath}`);
  }
}

const customerPackage = JSON.parse(readText('package.json'));
const webAntdPackage = JSON.parse(
  readFileSync(join(webAntdRoot, 'package.json'), 'utf8'),
);

if (customerPackage.name !== '@lina/web-customer') {
  fail('package name must be @lina/web-customer');
}

for (const dependencyName of Object.keys(webAntdPackage.dependencies ?? {})) {
  if (
    customerPackage.dependencies?.[dependencyName] !==
    webAntdPackage.dependencies[dependencyName]
  ) {
    fail(`Vben dependency must follow web-antd: ${dependencyName}`);
  }
}

for (const scriptName of ['build', 'dev', 'preview', 'typecheck']) {
  if (
    customerPackage.scripts?.[scriptName] !==
    webAntdPackage.scripts?.[scriptName]
  ) {
    fail(`script must follow web-antd: ${scriptName}`);
  }
}

const tsconfig = JSON.parse(readText('tsconfig.json'));
if (tsconfig.extends !== '@vben/tsconfig/web-app.json') {
  fail('tsconfig must extend @vben/tsconfig/web-app.json');
}

const packageText = JSON.stringify(customerPackage);
if (/nuxt/iu.test(packageText)) {
  fail('Nuxt dependencies or scripts are forbidden in web-customer');
}

const envText = readText('.env');
if (!envText.includes('VITE_APP_NAMESPACE=lina-web-customer')) {
  fail('customer preference namespace is not isolated');
}

const developmentEnvText = readText('.env.development');
if (!developmentEnvText.includes('VITE_PORT=6032')) {
  fail('customer development port must be 6032');
}

// The customer shell may share Vben code, but its browser state must never
// collide with the management shell.  Keep both the preference namespace and
// the store encryption key independently configured so a user can run the two
// apps in the same browser without leaking theme, menu or session state.
const webAntdEnvText = readFileSync(join(webAntdRoot, '.env'), 'utf8');
function envValue(text, name) {
  return text.match(new RegExp(`^${name}=(.*)$`, 'mu'))?.[1]?.trim() || '';
}
for (const name of ['VITE_APP_NAMESPACE', 'VITE_APP_STORE_SECURE_KEY']) {
  const customerValue = envValue(envText, name);
  const webAntdValue = envValue(webAntdEnvText, name);
  if (!customerValue || !webAntdValue || customerValue === webAntdValue) {
    fail(`${name} must be present and isolated from web-antd`);
  }
}

const sourceChecks = [
  ['src/main.ts', "from '@vben/preferences'"],
  ['src/bootstrap.ts', "from '@vben/stores'"],
  ['src/bootstrap.ts', "import '@vben/styles'"],
  ['src/api/request.ts', "from '@vben/request'"],
  ['src/preferences.ts', 'defineOverridesPreferences'],
  ['src/router/index.ts', 'createRouter'],
  ['src/layouts/basic.vue', 'BasicLayout'],
];

for (const [relativePath, marker] of sourceChecks) {
  if (!readText(relativePath).includes(marker)) {
    fail(`${relativePath} no longer follows the Vben marker ${marker}`);
  }
}

console.log('web-customer Vben alignment check passed');
