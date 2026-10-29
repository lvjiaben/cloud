import { resolve } from 'node:path';
import { defineConfig } from 'vite';

export default defineConfig({
  base: './',
  build: {
    outDir: 'dist',
    rollupOptions: {
      input: {
        customer: resolve(import.meta.dirname, 'customer/certificates/index.html'),
      },
    },
  },
});
