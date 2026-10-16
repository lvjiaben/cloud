// This cross-platform check guards the minimum public Nuxt SSR and SEO contract.
import { readFile } from 'node:fs/promises';

const required = new Map([
  ['../nuxt.config.ts', ['@nuxtjs/i18n', "strategy: 'prefix_except_default'", "preset: 'node-server'", "'/products/**': { cache: false }"]],
  ['../app/app.vue', ['useLocaleHead', 'htmlAttrs: head.value.htmlAttrs']],
  ['../app/components/public/PublicHomePage.vue', ['useSeoMeta', "robots: 'index, follow'"]],
  ['../app/components/public/PublicContentPage.vue', ['useSeoMeta', "robots: 'index, follow'"]],
  ['../app/components/public/PublicSiteShell.vue', ['config.public.customerUrl']],
]);
const failures = [];
for (const [relativePath, needles] of required) {
  const source = await readFile(new URL(relativePath, import.meta.url), 'utf8');
  for (const needle of needles) {
    if (!source.includes(needle)) failures.push(`${relativePath}: missing ${needle}`);
  }
}
if (failures.length) throw new Error(failures.join('\n'));
console.log('default website SSR and SEO contract check passed');
