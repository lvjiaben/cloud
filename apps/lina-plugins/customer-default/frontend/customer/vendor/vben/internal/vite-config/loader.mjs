import { createJiti } from "jiti";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const packageRoot = __dirname;
const sourceEntry = resolve(packageRoot, "src/index.ts");

const jiti = createJiti(import.meta.url, {
  "interopDefault": true,
  "alias": {
    "@vben/vite-config": packageRoot
  },
  "transformOptions": {
    "babel": {
      "plugins": []
    }
  }
})

/** @type {import("./src/index.js")} */
const _module = await jiti.import(sourceEntry);

export const loadAndConvertEnv = _module.loadAndConvertEnv;
export const defineConfig = _module.defineConfig;
export const defineApplicationConfig = _module.defineApplicationConfig;
export const defineLibraryConfig = _module.defineLibraryConfig;
export const defaultImportmapOptions = _module.defaultImportmapOptions;
export const getDefaultPwaOptions = _module.getDefaultPwaOptions;
export const loadApplicationPlugins = _module.loadApplicationPlugins;
export const loadLibraryPlugins = _module.loadLibraryPlugins;
export const viteArchiverPlugin = _module.viteArchiverPlugin;
export const viteCompressPlugin = _module.viteCompressPlugin;
export const viteDtsPlugin = _module.viteDtsPlugin;
export const viteHtmlPlugin = _module.viteHtmlPlugin;
export const viteVisualizerPlugin = _module.viteVisualizerPlugin;
export const viteVxeTableImportsPlugin = _module.viteVxeTableImportsPlugin;