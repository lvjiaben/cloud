const apiOrigin = process.env.NUXT_API_ORIGIN?.trim() || 'http://localhost:9999';

export default defineNuxtConfig({
  compatibilityDate: '2026-08-01',
  devServer: {
    host: '0.0.0.0',
    port: 6681,
  },
  devtools: { enabled: false },
  i18n: {
    baseUrl:
      process.env.NUXT_PUBLIC_SITE_URL?.trim() || 'http://localhost:6681',
    defaultLocale: 'zh-CN',
    detectBrowserLanguage: false,
    langDir: '../i18n/locales',
    locales: [
      { code: 'zh-CN', file: 'zh-CN.json', language: 'zh-CN', name: '简体中文' },
      { code: 'en-US', file: 'en-US.json', language: 'en-US', name: 'English' },
    ],
    strategy: 'prefix_except_default',
  },
  modules: ['@nuxtjs/i18n'],
  nitro: {
    devProxy: {
      '/x': {
        changeOrigin: true,
        target: `${apiOrigin}/x`,
      },
    },
    preset: 'node-server',
  },
  routeRules: {
    '/': { swr: 60 },
    '/en-US': { swr: 60 },
    '/en-US/products/**': { swr: 60 },
    '/products/**': { swr: 60 },
  },
  runtimeConfig: {
    apiOrigin,
    public: {
      siteUrl:
        process.env.NUXT_PUBLIC_SITE_URL?.trim() || 'http://localhost:6681',
    },
  },
  typescript: {
    strict: true,
    typeCheck: true,
  },
});
