import { fileURLToPath, URL } from 'node:url';
import vue from '@vitejs/plugin-vue';
import { defineConfig } from 'vite';

export default defineConfig({
  define: { 'process.env.NODE_ENV': JSON.stringify('production') },
  plugins: [vue()],
  build: {
    emptyOutDir: true,
    lib: { entry: fileURLToPath(new URL('./src/mount.ts', import.meta.url)), fileName: () => 'mount.js', formats: ['es'] },
    rollupOptions: { output: { inlineDynamicImports: true } },
  },
});
