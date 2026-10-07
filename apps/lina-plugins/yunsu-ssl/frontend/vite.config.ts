import { resolve } from 'node:path';
import { defineConfig } from 'vite';

export default defineConfig({
  base: './',
  build: {
    outDir: 'dist',
    rollupOptions: {
      input: {
        admin: resolve(import.meta.dirname, 'admin/certificates/index.html'),
        customer: resolve(import.meta.dirname, 'customer/certificates/index.html'),
      },
    },
  },
});
