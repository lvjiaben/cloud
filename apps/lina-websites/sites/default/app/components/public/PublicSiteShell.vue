<script setup lang="ts">
import { computed } from 'vue';
import PublicNavigation from './PublicNavigation.vue';
import { Button as AButton, ConfigProvider as AConfigProvider } from 'ant-design-vue';

import { publicCopy, type Locale } from './public-i18n';

type BrandText = { enUS: string; zhCN: string };
type BrandNavigation = { label: BrandText; href: string; customer?: boolean; external?: boolean; children?: BrandNavigation[] };
type PublishedBrand = {
  siteName: BrandText;
  logoUrl?: string;
  primaryColor?: string;
  navigation?: BrandNavigation[];
  footerNavigation?: BrandNavigation[];
  footer: BrandText;
};
type BrandResponse = { data?: { brand?: PublishedBrand } };

const localePath = useLocalePath();
const { locale, setLocale } = useI18n();
const config = useRuntimeConfig();
const { data: brandResponse } = await useFetch<BrandResponse>(
  '/x/idc-portal/api/v1/public/brand',
  { default: () => ({}) },
);
const currentLocale = computed<Locale>(() => locale.value === 'en-US' ? 'en-US' : 'zh-CN');
const brand = computed(() => brandResponse.value?.data?.brand);
const text = computed(() => {
  const fallback = publicCopy[currentLocale.value];
  const customName = currentLocale.value === 'en-US' ? brand.value?.siteName.enUS : brand.value?.siteName.zhCN;
  const customFooter = currentLocale.value === 'en-US' ? brand.value?.footer.enUS : brand.value?.footer.zhCN;
  return {
    ...fallback,
    brand: customName || fallback.brand,
    footer: customFooter || fallback.footer,
  };
});
const navigation = computed(() => {
  const items = brand.value?.navigation ?? [];
  return brand.value?.navigation !== undefined ? items : [
    { label: { enUS: publicCopy['en-US'].products, zhCN: publicCopy['zh-CN'].products }, href: '/products' },
    { label: { enUS: publicCopy['en-US'].news, zhCN: publicCopy['zh-CN'].news }, href: '/pages/news' },
    { label: { enUS: publicCopy['en-US'].announcements, zhCN: publicCopy['zh-CN'].announcements }, href: '/pages/announcements' },
    { label: { enUS: publicCopy['en-US'].knowledge, zhCN: publicCopy['zh-CN'].knowledge }, href: '/pages/knowledge' },
    { label: { enUS: publicCopy['en-US'].downloads, zhCN: publicCopy['zh-CN'].downloads }, href: '/pages/downloads' },
    { label: { enUS: publicCopy['en-US'].help, zhCN: publicCopy['zh-CN'].help }, href: '/pages/help' },
  ];
});
const theme = computed(() => ({
  token: {
    borderRadius: 0,
    borderRadiusLG: 0,
    borderRadiusSM: 0,
    colorPrimary: brand.value?.primaryColor || '#1677ff',
    colorText: '#182230',
    controlHeight: 36,
    fontSize: 14,
  },
}));

function customerURL(path: '/account' | '/auth/login') {
  return new URL(path, `${config.public.customerUrl.replace(/\/+$/u, '')}/`).toString();
}

async function toggleLocale() {
  await setLocale(currentLocale.value === 'zh-CN' ? 'en-US' : 'zh-CN');
}

</script>

<template>
  <AConfigProvider :theme="theme">
    <div class="public-site" data-testid="public-site-shell">
      <header v-if="brand" class="site-header">
        <div class="site-header-inner">
          <NuxtLink class="site-brand" :to="localePath('/')">
            <img v-if="brand?.logoUrl" :src="brand.logoUrl" :alt="text.brand" class="site-brand-logo">
            <span v-else aria-hidden="true">L</span><strong>{{ text.brand }}</strong>
          </NuxtLink>
          <nav class="site-navigation" :aria-label="text.navigation">
            <PublicNavigation :items="navigation" :locale="currentLocale" :customer-origin="config.public.customerUrl" />
          </nav>
          <div class="site-actions">
            <AButton data-testid="public-sign-in" :href="customerURL('/auth/login')">{{ text.signIn }}</AButton>
            <AButton data-testid="public-customer-center" :href="customerURL('/account')" type="primary">{{ text.customerCenter }}</AButton>
            <AButton data-testid="public-locale-switch" size="small" @click="toggleLocale">{{ text.language }}</AButton>
          </div>
        </div>
      </header>
      <div v-if="brand" class="site-content"><slot /></div>
      <main v-else class="website-unavailable" data-testid="website-unavailable"><h1>{{ currentLocale === 'en-US' ? 'Website is not enabled' : '官网尚未启用' }}</h1></main>
      <footer v-if="brand" class="site-footer">
        <div class="site-footer-inner">
          <strong>{{ text.brand }}</strong><PublicNavigation :items="brand.footerNavigation ?? []" :locale="currentLocale" :customer-origin="config.public.customerUrl" footer /><span>{{ text.footer }}</span>
        </div>
      </footer>
    </div>
  </AConfigProvider>
</template>

<style scoped>
.website-unavailable { min-height: 100vh; display: grid; place-items: center; padding: 32px; color: #64748b; }
.website-unavailable h1 { font-size: 22px; font-weight: 500; }
</style>
