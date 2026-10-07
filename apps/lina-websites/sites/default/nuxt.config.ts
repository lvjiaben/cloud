const apiOrigin = process.env.NUXT_API_ORIGIN?.trim() || 'http://localhost:9999';

export default defineNuxtConfig({
  components: [
    {
      path: '~/components/public',
      pathPrefix: false,
    },
  ],
  app: {
    head: {
      link: [{ href: '/favicon.ico', rel: 'icon' }],
      meta: [
        { charset: 'utf-8' },
        { content: 'width=device-width, initial-scale=1', name: 'viewport' },
      ],
    },
  },
  compatibilityDate: '2026-08-01',
  css: ['ant-design-vue/dist/reset.css', '~/assets/styles.css'],
  devServer: {
    host: '0.0.0.0',
    port: 6680,
  },
  devtools: { enabled: false },
  i18n: {
    baseUrl: process.env.NUXT_PUBLIC_SITE_URL?.trim() || 'http://localhost:6680',
    defaultLocale: 'zh-CN',
    detectBrowserLanguage: false,
    locales: [
      { code: 'zh-CN', language: 'zh-CN', name: '简体中文' },
      { code: 'en-US', language: 'en-US', name: 'English' },
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
    '/en-US/pages/**': { swr: 300 },
    '/en-US/products/**': { swr: 60 },
    '/pages/**': { swr: 300 },
    '/products/**': { swr: 60 },
  },
  runtimeConfig: {
    apiOrigin,
    public: {
      customerUrl:
        process.env.NUXT_PUBLIC_CUSTOMER_URL?.trim() || 'http://localhost:6032',
      siteUrl: process.env.NUXT_PUBLIC_SITE_URL?.trim() || 'http://localhost:6680',
    },
  },
  typescript: {
    strict: true,
    typeCheck: true,
  },
  vite: {
    ssr: {
      noExternal: ['ant-design-vue'],
    },
  },
});
