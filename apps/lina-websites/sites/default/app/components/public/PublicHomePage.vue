<script setup lang="ts">
import { computed } from 'vue';
import { Button as AButton, Card as ACard } from 'ant-design-vue';

import type { Locale } from './public-i18n';

type BrandText = { enUS: string; zhCN: string };
type HomepageBlock = { title: BrandText; description: BrandText; href?: string };
type PublishedBrand = {
  templateRenderer?: string;
  heroTitle?: BrandText;
  heroDescription?: BrandText;
  heroImage?: string;
  seoTitle?: BrandText;
  seoDescription?: BrandText;
  termsUrl?: string;
  homepageBlocks?: HomepageBlock[];
};
type BrandResponse = { data?: { brand?: PublishedBrand } };

const localePath = useLocalePath();
const { locale } = useI18n();
const currentLocale = computed<Locale>(() => locale.value === 'en-US' ? 'en-US' : 'zh-CN');
const { data: brandResponse } = await useFetch<BrandResponse>(
  '/x/idc-portal/api/v1/public/brand',
  { default: () => ({}) },
);
const minimal = computed(() => brand.value?.templateRenderer === 'public-minimal-v1');
const brand = computed(() => brandResponse.value?.data?.brand);
const content = {
  'en-US': {
    browse: 'Browse products',
    description: 'Explore products, compare available options, and find clear help before continuing to the secure customer center.',
    discover: 'What you can find here',
    intro: 'Public product and service information',
    learnMore: 'Learn more',
    items: [
      ['Products', 'Browse merchant-defined categories and products without assuming every product is a server.', '/products'],
      ['Updates', 'Read public news and announcements from the service operator.', '/pages/announcements'],
      ['Guides', 'Find knowledge articles, downloads, and practical help.', '/pages/knowledge'],
    ],
    latest: 'Read announcements',
    operator: 'From the operator',
    terms: 'Terms',
    title: 'Choose the service that fits your needs',
  },
  'zh-CN': {
    browse: '浏览商品',
    description: '先查看商品、比较可选配置并找到清楚的帮助，再进入安全的客户中心继续购买。',
    discover: '你可以在这里找到',
    intro: '公开商品与服务信息',
    learnMore: '了解更多',
    items: [
      ['商品', '按商家自定义分类浏览商品，不再假设所有商品都是服务器。', '/products'],
      ['动态', '查看服务商公开发布的新闻和公告。', '/pages/announcements'],
      ['指南', '查找知识文章、下载资料和使用帮助。', '/pages/knowledge'],
    ],
    latest: '查看公告',
    operator: '服务商信息',
    terms: '服务条款',
    title: '找到适合你的商品和服务',
  },
} as const;
const text = computed(() => content[currentLocale.value]);

const seoTitle = computed(() => currentLocale.value === 'en-US' ? brand.value?.seoTitle?.enUS || text.value.title : brand.value?.seoTitle?.zhCN || text.value.title);
const seoDescription = computed(() => currentLocale.value === 'en-US' ? brand.value?.seoDescription?.enUS || text.value.description : brand.value?.seoDescription?.zhCN || text.value.description);
const homepageBlocks = computed(() => brand.value?.homepageBlocks ?? []);
const termsURL = computed(() => brand.value?.termsUrl || '');
useSeoMeta({ description: () => seoDescription.value, robots: 'index, follow', title: () => seoTitle.value });
</script>

<template>
  <main class="home-page" :class="{ 'home-minimal': minimal }" data-testid="public-home">
    <section class="home-hero" aria-labelledby="home-title">
      <div class="home-intro">
        <p>{{ text.intro }}</p>
        <h1 id="home-title">{{ currentLocale === 'en-US' ? brand?.heroTitle?.enUS || text.title : brand?.heroTitle?.zhCN || text.title }}</h1>
        <p>{{ currentLocale === 'en-US' ? brand?.heroDescription?.enUS || text.description : brand?.heroDescription?.zhCN || text.description }}</p>
        <div class="home-actions">
          <AButton :href="localePath('/products')" type="primary">{{ text.browse }}</AButton>
          <AButton :href="localePath('/pages/announcements')">{{ text.latest }}</AButton>
          <AButton v-if="termsURL" :href="termsURL">{{ text.terms }}</AButton>
        </div>
      </div>
      <img v-if="!minimal && brand?.heroImage" :src="brand.heroImage" alt="" class="home-template-image">
      <aside v-else-if="!minimal" class="home-service-index" :aria-label="text.discover">
        <p>{{ text.discover }}</p>
        <NuxtLink
          v-for="(item, index) in text.items"
          :key="item[0]"
          class="home-service-link"
          :to="localePath(item[2])"
        >
          <span>0{{ index + 1 }}</span>
          <div><strong>{{ item[0] }}</strong><small>{{ item[1] }}</small></div>
          <b aria-hidden="true">→</b>
        </NuxtLink>
      </aside>
    </section>
    <nav v-if="minimal" class="minimal-links" :aria-label="text.discover"><NuxtLink v-for="item in text.items" :key="item[0]" :to="localePath(item[2])"><strong>{{ item[0] }}</strong><span>{{ item[1] }}</span><b aria-hidden="true">→</b></NuxtLink></nav>
    <section v-if="homepageBlocks.length" class="home-custom-blocks" aria-labelledby="home-custom-blocks-title">
      <h2 id="home-custom-blocks-title">{{ text.operator }}</h2>
      <div class="home-grid">
        <ACard v-for="block in homepageBlocks" :key="`${block.title.enUS}-${block.title.zhCN}`" :bordered="true">
          <h3>{{ currentLocale === 'en-US' ? block.title.enUS : block.title.zhCN }}</h3>
          <p>{{ currentLocale === 'en-US' ? block.description.enUS : block.description.zhCN }}</p>
          <NuxtLink v-if="block.href" :to="localePath(block.href)">{{ text.learnMore }}</NuxtLink>
        </ACard>
      </div>
    </section>
  </main>
</template>

<style scoped>
.home-template-image { width: 100%; max-height: 380px; object-fit: cover; border-radius: 20px; }
.home-minimal .home-hero { display: block; max-width: 900px; margin: 0 auto; padding: 72px 0 40px; text-align: center; }
.home-minimal .home-actions { justify-content: center; }
.minimal-links { display: grid; gap: 16px; max-width: 900px; margin: 0 auto 48px; }
.minimal-links a { display: flex; gap: 24px; align-items: center; border-bottom: 1px solid #e2e8f0; padding: 24px 8px; }
.minimal-links span { flex: 1; color: #64748b; }
@media (max-width: 640px) { .minimal-links a { flex-wrap: wrap; } .minimal-links span { flex-basis: 100%; } }
</style>
