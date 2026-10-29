<script setup lang="ts">
import type {
  PublicContentDetail,
  PublicContentKind,
  PublicContentPage,
  PublicContentSummary,
} from './content-client';
import type { Locale } from './public-i18n';

import { computed, ref } from 'vue';
import {
  Alert as AAlert,
  Button as AButton,
  Card as ACard,
  Empty as AEmpty,
  Input as AInput,
  Spin as ASpin,
} from 'ant-design-vue';

import {
  createPublicDownloadTicket,
  getPublicContent,
  listPublicContent,
  publicDownloadURL,
} from './content-client';
import { contentCopy } from './content-i18n';
import { announcementHtml } from './announcement-html';

const props = defineProps<{
  kind: PublicContentKind | 'help';
  slug?: string;
}>();

const route = useRoute();
const localePath = useLocalePath();
const { locale } = useI18n();
const publicLocale = computed<Locale>(() =>
  locale.value === 'en-US' ? 'en-US' : 'zh-CN',
);
const text = computed(() => contentCopy[publicLocale.value]);
const apiKind = computed<PublicContentKind>(() =>
  props.kind === 'help' ? 'knowledge' : props.kind,
);
const isDetail = computed(() => Boolean(props.slug));
const searchQuery = ref('');
const appliedQuery = ref('');
const downloadLoading = ref(false);
const cursor = ref('');
const cursorHistory = ref<string[]>([]);
const pageNumber = computed(() => cursorHistory.value.length + 1);

const dataKey = computed(
  () =>
    `public-content:${apiKind.value}:${props.slug || 'list'}:${cursor.value}:${appliedQuery.value}`,
);
const { data, error, pending, refresh } = await useAsyncData<
  PublicContentDetail | PublicContentPage
>(
  dataKey,
  () =>
    props.slug
      ? getPublicContent(apiKind.value, props.slug)
      : listPublicContent(apiKind.value, {
          cursor: cursor.value,
          query: appliedQuery.value,
        }),
);

const initialError = error.value as
  | (Error & {
      cause?: { status?: number };
      reason?: string;
      status?: number;
      statusCode?: number;
    })
  | undefined;
if (
  initialError &&
  (initialError.status === 404 ||
    initialError.statusCode === 404 ||
    initialError.cause?.status === 404 ||
    initialError.message === 'content_not_found' ||
    initialError.reason === 'content_not_found')
) {
  throw createError({
    fatal: true,
    message: text.value.contentUnavailable,
    statusCode: 404,
  });
}

function isContentDetail(
  value: PublicContentDetail | PublicContentPage | undefined,
): value is PublicContentDetail {
  return Boolean(value && 'bodyText' in value);
}

function isContentPage(
  value: PublicContentDetail | PublicContentPage | undefined,
): value is PublicContentPage {
  return Boolean(value && 'items' in value);
}

const detail = computed(() =>
  props.slug && isContentDetail(data.value) ? data.value : undefined,
);
const pageData = computed(() =>
  !props.slug && isContentPage(data.value) ? data.value : undefined,
);
const groupedItems = computed(() => {
  const items = pageData.value?.items || [];
  if (props.kind === 'announcements') {
    return [{ items, key: 'announcements', label: '' }];
  }
  const groups = new Map<
    string,
    { items: PublicContentSummary[]; key: string; label: string }
  >();
  for (const item of items) {
    const key = item.category
      ? `${item.category.nameZhCN}\u0000${item.category.nameEnUS}`
      : '__uncategorized__';
    const label = item.category
      ? publicLocale.value === 'en-US'
        ? item.category.nameEnUS || item.category.nameZhCN
        : item.category.nameZhCN || item.category.nameEnUS
      : text.value.uncategorized;
    const group = groups.get(key) || { items: [], key, label };
    group.items.push(item);
    groups.set(key, group);
  }
  return [...groups.values()];
});
const headingKey = computed(() => props.kind);
const introKey = computed(() => `${props.kind}Intro` as keyof typeof text.value);
const pageTitle = computed(() =>
  detail.value?.seoTitle || detail.value?.title || text.value[headingKey.value],
);
const pageDescription = computed(
  () => detail.value?.seoDescription || detail.value?.summary || text.value[introKey.value],
);
const baseListPath = computed(() =>
  props.kind === 'help' ? '/pages/help' : `/pages/${props.kind}`,
);
const currentPath = computed(() => route.path.replace(/^\/en-US(?=\/|$)/u, '') || '/');
const config = useRuntimeConfig();
const canonicalURL = computed(() => new URL(route.path, config.public.siteUrl).toString());
const zhURL = computed(() => new URL(currentPath.value, config.public.siteUrl).toString());
const enURL = computed(() =>
  new URL(`/en-US${currentPath.value === '/' ? '' : currentPath.value}`, config.public.siteUrl).toString(),
);

useSeoMeta({
  description: () => pageDescription.value || undefined,
  ogDescription: () => pageDescription.value || undefined,
  ogTitle: () => pageTitle.value,
  ogType: () => (detail.value ? 'article' : 'website'),
  ogUrl: () => canonicalURL.value,
  robots: 'index, follow',
  title: () => pageTitle.value,
});
useHead(() => ({
  link: [
    { href: canonicalURL.value, rel: 'canonical' },
    { href: zhURL.value, hreflang: 'zh-CN', rel: 'alternate' },
    { href: enURL.value, hreflang: 'en-US', rel: 'alternate' },
    { href: zhURL.value, hreflang: 'x-default', rel: 'alternate' },
  ],
  script: detail.value
    ? [
        {
          innerHTML: JSON.stringify({
            '@context': 'https://schema.org',
            '@type': props.kind === 'news' ? 'NewsArticle' : 'Article',
            dateModified: new Date(detail.value.updatedAt).toISOString(),
            datePublished: new Date(detail.value.publishedAt).toISOString(),
            description: pageDescription.value || undefined,
            headline: detail.value.title,
            mainEntityOfPage: canonicalURL.value,
          }),
          type: 'application/ld+json',
        },
      ]
    : [],
}));

function detailPath(item: PublicContentSummary) {
  return localePath(item.publicPath);
}

function formatDate(timestamp: number) {
  return new Intl.DateTimeFormat(publicLocale.value, { dateStyle: 'medium' }).format(
    new Date(timestamp),
  );
}

async function applyFilters() {
  appliedQuery.value = searchQuery.value.trim();
  cursor.value = '';
  cursorHistory.value = [];
  await refresh();
}

async function nextPage() {
  if (!pageData.value?.nextCursor) return;
  cursorHistory.value.push(cursor.value);
  cursor.value = pageData.value.nextCursor;
  await refresh();
}

async function previousPage() {
  const previous = cursorHistory.value.pop();
  cursor.value = previous || '';
  await refresh();
}

async function startDownload() {
  if (!detail.value?.hasDownload) return;
  downloadLoading.value = true;
  try {
    const ticket = await createPublicDownloadTicket(detail.value.contentNo);
    if (import.meta.client) {
      window.location.assign(publicDownloadURL(ticket.token));
    }
  } finally {
    downloadLoading.value = false;
  }
}
</script>

<template>
  <main class="content-main" data-testid="public-content-shell">
        <template v-if="isDetail">
          <NuxtLink class="content-back" :to="localePath(baseListPath)">← {{ text.back }}</NuxtLink>
          <ASpin :spinning="pending">
            <AAlert v-if="error" :message="text.loadFailed" show-icon type="error">
              <template #action><AButton size="small" @click="() => refresh()">{{ text.retry }}</AButton></template>
            </AAlert>
            <article v-else-if="detail" class="content-article" data-testid="public-content-detail">
              <h1>{{ detail.title }}</h1>
              <p v-if="detail.summary" class="content-summary">{{ detail.summary }}</p>
              <time :datetime="new Date(detail.publishedAt).toISOString()">
                {{ text.publishedAt }} · {{ formatDate(detail.publishedAt) }}
              </time>
              <p
                v-if="kind !== 'announcements' && detail.category"
                class="content-category"
                data-testid="public-content-detail-category"
              >
                {{ text.category }} ·
                {{
                  publicLocale === 'en-US'
                    ? detail.category.nameEnUS || detail.category.nameZhCN
                    : detail.category.nameZhCN || detail.category.nameEnUS
                }}
              </p>
              <div class="content-body" :class="{ 'rich-content-body': /<[a-z][^>]*>/i.test(detail.bodyText) }" v-html="announcementHtml(detail.bodyText)"></div>

              <section v-if="kind === 'downloads' && detail.hasDownload" class="download-panel">
                <AButton
                  data-testid="public-download-action"
                  :loading="downloadLoading"
                  type="primary"
                  @click="startDownload"
                >{{ text.download }}</AButton>
              </section>
            </article>
          </ASpin>
        </template>

        <template v-else>
          <section class="content-intro">
            <h1>{{ text[headingKey] }}</h1>
            <p>{{ text[introKey] }}</p>
          </section>

          <form v-if="kind === 'help' || kind === 'knowledge' || kind === 'news' || kind === 'downloads'" class="content-filters" @submit.prevent="applyFilters">
            <AInput v-model:value="searchQuery" :aria-label="text.search" :placeholder="text.searchPlaceholder" />
            <AButton html-type="submit" type="primary">{{ text.search }}</AButton>
          </form>

          <AAlert v-if="error" :message="text.loadFailed" show-icon type="error">
            <template #action><AButton size="small" @click="() => refresh()">{{ text.retry }}</AButton></template>
          </AAlert>
          <ASpin :spinning="pending">
            <AEmpty v-if="!pageData?.items.length && !pending" :description="text.empty" />
            <div v-else class="content-groups" data-testid="public-content-list">
              <section v-for="group in groupedItems" :key="group.key" class="content-group">
                <h2
                  v-if="kind !== 'announcements'"
                  class="content-group-title"
                  data-testid="public-content-category"
                >
                  {{ group.label }}
                </h2>
                <div class="content-grid">
                  <NuxtLink v-for="item in group.items" :key="item.contentNo" :to="detailPath(item)">
                    <ACard :bordered="true" hoverable>
                      <div class="content-card-meta">
                        <time>{{ formatDate(item.publishedAt) }}</time>
                      </div>
                      <h2>{{ item.title }}</h2>
                      <p v-if="item.summary">{{ item.summary }}</p>
                    </ACard>
                  </NuxtLink>
                </div>
              </section>
            </div>
            <div
              v-if="cursorHistory.length || pageData?.nextCursor"
              class="content-pagination"
            >
              <AButton :disabled="!cursorHistory.length" @click="previousPage">
                {{ text.previousPage }}
              </AButton>
              <span>{{ text.pageNumber.replace('{page}', String(pageNumber)) }}</span>
              <AButton :disabled="!pageData?.nextCursor" @click="nextPage">
                {{ text.nextPage }}
              </AButton>
            </div>
          </ASpin>
        </template>
  </main>
</template>

<style scoped>
.content-main { min-height: calc(100vh - 140px); padding: 42px 0 72px; }
.content-intro { max-width: 760px; margin-bottom: 30px; border-left: 3px solid var(--site-primary); padding-left: 18px; }
h1 { max-width: 900px; margin-bottom: 14px; color: #111827; font-size: clamp(28px, 3.5vw, 40px); letter-spacing: -.025em; line-height: 1.2; }
.content-intro > p:last-child, .content-summary { color: #667085; font-size: 16px; line-height: 1.75; }
.content-filters { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 12px; margin-bottom: 26px; padding: 15px; border: 1px solid var(--site-border); background: #fff; }
.content-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
.content-groups, .content-group { display: grid; gap: 18px; }
.content-group + .content-group { margin-top: 12px; }
.content-group-title { margin: 0; color: #344054; font-size: 18px; }
.content-grid :deep(.ant-card) { height: 100%; border-color: var(--site-border); border-radius: 0; box-shadow: none; }
.content-grid :deep(.ant-card:hover) { border-color: #80b9f5; box-shadow: inset 3px 0 0 var(--site-primary); }
.content-grid h2 { margin: 18px 0 10px; color: #172033; font-size: 21px; }
.content-grid p { margin-bottom: 0; color: #667085; line-height: 1.7; }
.content-card-meta { display: flex; align-items: center; justify-content: space-between; gap: 12px; color: #7a8496; font-size: 12px; }
.content-pagination { display: flex; align-items: center; justify-content: flex-end; gap: 12px; margin-top: 24px; color: #667085; font-size: 13px; }
.content-back { display: inline-block; margin-bottom: 26px; font-weight: 650; }
.content-article { padding: clamp(28px, 5vw, 60px); border: 1px solid var(--site-border); border-top: 3px solid var(--site-primary); border-radius: 0; background: #fff; box-shadow: none; }
.content-article > time { color: #7a8496; font-size: 13px; }
.content-category { margin-top: 10px; color: #667085; font-size: 13px; }
.content-body { margin-top: 38px; color: #273244; font-size: 16px; line-height: 1.85; white-space: pre-wrap; overflow-wrap: anywhere; }
.rich-content-body { white-space: normal; }
.rich-content-body :deep(img) { max-width: 100%; height: auto; margin: 12px 0; }
.rich-content-body :deep(p) { margin: 8px 0; }
.download-panel { margin-top: 40px; padding: 22px; border: 1px solid #b9d7ff; border-radius: 0; background: #f5f9ff; }
@media (max-width: 720px) {
  .content-main { padding-top: 38px; }
  .content-filters, .content-grid { grid-template-columns: 1fr; }
  .content-article { padding: 24px 20px; }
}
@media (prefers-reduced-motion: reduce) { * { scroll-behavior: auto !important; } }
</style>
