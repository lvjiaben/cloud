import { createRequire } from 'node:module';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

import tailwindcss from '@tailwindcss/vite';
import vue from '@vitejs/plugin-vue';
import { defineConfig, transformWithEsbuild } from 'vite';

const root = dirname(fileURLToPath(import.meta.url));
const vben = resolve(root, '../../../lina-vben');
const gridRequire = createRequire(resolve(vben, 'packages/effects/plugins/package.json'));
const workspacePackages = new Map<string, { path: string; exports: Record<string, unknown> }>();
function discoverPackages(directory: string) {
  const manifestPath = resolve(directory, 'package.json');
  if (existsSync(manifestPath)) {
    const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
    workspacePackages.set(manifest.name, { path: directory, exports: manifest.exports });
    return;
  }
  for (const item of readdirSync(directory, { withFileTypes: true })) {
    if (item.isDirectory() && !item.name.startsWith('.') && item.name !== 'node_modules') discoverPackages(resolve(directory, item.name));
  }
}
discoverPackages(resolve(vben, 'packages'));
function resolveExport(value: unknown): string | undefined {
  if (typeof value === 'string') return value;
  if (value && typeof value === 'object') {
    const conditions = value as Record<string, unknown>;
    return resolveExport(conditions.development ?? conditions.import ?? conditions.default);
  }
}

export default defineConfig({
  css: { preprocessorOptions: { scss: { loadPaths: [resolve(vben, 'packages/effects/plugins/node_modules')] } } },
  define: { 'process.env.NODE_ENV': JSON.stringify('production') },
  plugins: [
    {
      name: 'vben-isolated-runtime-environment',
      enforce: 'pre',
      resolveId(source) {
        if (source === '@vben-core/shared/cache') return resolve(root, 'src/isolated/memory-storage.ts');
        // The echarts barrel only uses `export *`, which drops its default export,
        // so the isolated runtime imports the echarts instance module directly.
        if (source === '@linapro-isolated/echarts-instance') return resolve(vben, 'packages/effects/plugins/src/echarts/echarts.ts');
        if (source === 'virtual:lina-vxe-locales') return '\0vben-isolated-locales';
        if (source === 'virtual:vben-isolated-messages') return '\0vben-isolated-messages';
        if (source === 'vxe-table' || source === 'vxe-pc-ui') {
          const manifestPath = gridRequire.resolve(`${source}/package.json`);
          return resolve(dirname(manifestPath), JSON.parse(readFileSync(manifestPath, 'utf8')).module);
        }
        if (source.startsWith('vxe-')) return gridRequire.resolve(source);
        if (source.startsWith('@vben/') || source.startsWith('@vben-core/')) {
          const parts = source.split('/');
          const metadata = workspacePackages.get(parts.slice(0, 2).join('/'));
          const key = parts.length === 2 ? '.' : `./${parts.slice(2).join('/')}`;
          const entry = resolveExport(metadata?.exports[key]);
          if (metadata && entry) return resolve(metadata.path, entry);
        }
        return null;
      },
      transform(code, id) {
        if (id.replaceAll('\\', '/').includes('/dom-zindex/')) {
          const documentEffect = /function getDocument\(\) \{[\s\S]*?\n\}/;
          if (documentEffect.test(code)) return code.replace(documentEffect, 'function getDocument() { return null; }');
        }
        // VXE's stock theme setter also mutates <html> during module import.
        // Keep its real reactive theme store while containing this one DOM effect.
        if (id.replaceAll('\\', '/').includes('/@vxe-ui/core/es/src/theme.js')) {
          const documentEffect = /    if \(typeof document !== 'undefined'\) \{[\s\S]*?\n    \}/;
          if (!documentEffect.test(code)) throw new Error('VXE theme environment adapter requires review after upstream update');
          return code.replace(documentEffect, '');
        }
      },
      load(id) {
        if (id === '\0vben-isolated-messages') {
          const messages = Object.fromEntries(['zh-CN', 'en-US'].map((locale) => [locale, Object.fromEntries(['common', 'ui'].map((name) => [name, JSON.parse(readFileSync(resolve(vben, `packages/locales/src/langs/${locale}/${name}.json`), 'utf8'))]))]));
          return `export default ${JSON.stringify(messages)};`;
        }
        if (id !== '\0vben-isolated-locales') return;
        return `export const vxeLocaleLoaders = { 'zh-CN': () => import(${JSON.stringify(gridRequire.resolve('vxe-pc-ui/lib/language/zh-CN'))}), 'en-US': () => import(${JSON.stringify(gridRequire.resolve('vxe-pc-ui/lib/language/en-US'))}) };`;
      },
      async generateBundle() {
        // Keep the loader safe to import from Node and existing Vue hosts. The
        // runtime is one module so a failed import can be retried as a whole;
        // a cached failed child chunk would otherwise survive an entry retry.
        const loader = await transformWithEsbuild(readFileSync(resolve(root, 'src/isolated/loader.ts'), 'utf8'), 'loader.ts', { minify:true, target:'es2022' });
        this.emitFile({type:'asset',fileName:'index.js',source:loader.code});
      },
    },
    vue(),
    tailwindcss(),
  ],
  resolve: { conditions: ['development'], dedupe: ['vue'], alias: { vue: resolve(root, 'node_modules/vue/dist/vue.runtime.esm-bundler.js') } },
  build: {
    outDir: 'dist/isolated',
    emptyOutDir: true,
    minify: 'esbuild',
    lib: { entry: resolve(root, 'src/isolated/runtime.ts'), fileName: () => 'runtime.js', formats: ['es'] },
    rollupOptions: { output: { inlineDynamicImports:true } },
  },
});
