import { readFileSync } from 'node:fs';
const manifestSource = readFileSync(new URL('../../plugin.yaml', import.meta.url), 'utf8');
const releaseVersion = /^version: (\S+)/mu.exec(manifestSource)?.[1];
const templateID = /^id: (\S+)/mu.exec(manifestSource)?.[1];
if (!releaseVersion || !templateID) throw new Error('Template release metadata is missing');
const staticBuild = process.env.WEBSITE_STATIC_BUILD === '1';
const apiOrigin = process.env.NUXT_API_ORIGIN?.trim() || 'http://localhost:6030';

export default defineNuxtConfig({
  ssr: !staticBuild,
  components: [
    {
      path: '~/components/public',
      pathPrefix: false,
    },
  ],
  app: {
    cdnURL: staticBuild ? `/x-assets/${templateID}/${releaseVersion}/` : undefined,
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
    preset: 'node-server',
    output: staticBuild ? { dir: '.static-output' } : undefined,
    prerender: staticBuild ? { crawlLinks: false, routes: ['/'] } : undefined,
  },
  routeRules: {
    '/': { cache: false },
    '/en-US': { cache: false },
    '/en-US/pages/**': { cache: false },
    '/en-US/products/**': { cache: false },
    '/pages/**': { cache: false },
    '/products/**': { cache: false },
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
