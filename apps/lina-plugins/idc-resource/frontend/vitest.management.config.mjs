import { fileURLToPath } from 'node:url';
import vue from '@vitejs/plugin-vue';
const path = value => fileURLToPath(new URL(value, import.meta.url));
export default {
  plugins: [vue()],
  resolve: { alias: {
    vue: path('../../../lina-vben/node_modules/vue/dist/vue.runtime.esm-bundler.js'),
    '@vben/common-ui': path('../../../lina-vben/apps/web-antd/node_modules/@vben/common-ui/src/index.ts'),
    '@vben/preferences': path('../../../lina-vben/apps/web-antd/node_modules/@vben/preferences/src/index.ts'),
    '@vben/locales': path('../../../lina-vben/apps/web-antd/node_modules/@vben/locales/src/index.ts'),
    'vue-router': path('../../../lina-vben/apps/web-antd/node_modules/vue-router'),
    'ant-design-vue': path('../../../lina-vben/apps/web-antd/node_modules/ant-design-vue'),
    '#/adapter/form': path('../../../lina-vben/apps/web-antd/src/adapter/form.ts'),
    '#/adapter/vxe-table': path('../../../lina-vben/apps/web-antd/src/adapter/vxe-table.ts'),
    '#/locales': path('../../../lina-vben/apps/web-antd/src/locales/index.ts'),
    '#/utils/time': path('../../../lina-vben/apps/web-antd/src/utils/time.ts'),
    '@vue/test-utils': path('../../../lina-vben/node_modules/@vue/test-utils/dist/vue-test-utils.esm-bundler.mjs'),
    vitest: path('../../../lina-vben/node_modules/vitest/dist/index.js'),
  } },
  test: { environment: 'happy-dom', include: ['pages/server-batch-renewal-lifecycle.test.ts', 'pages/server-accessories-lifecycle.test.ts'], maxWorkers: 1 },
};
