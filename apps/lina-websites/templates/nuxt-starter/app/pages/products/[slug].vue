<script setup lang="ts">
import { createSiteClient } from '@lina/site-sdk';

const route = useRoute();
const { locale } = useI18n();
const slug = String(route.params.slug || '');
const apiBase = import.meta.server
  ? `${useRuntimeConfig().apiOrigin}/x/idc-core/api/v1`
  : '/x/idc-core/api/v1';
const client = createSiteClient({ apiBase, fetch: globalThis.fetch });
const { data: product, error } = await useAsyncData(`product:${slug}`, () =>
  client.getCatalogProduct(slug),
);

if (error.value || !product.value) {
  throw createError({ statusCode: 404, statusMessage: 'Product not found' });
}

const name = computed(() =>
  locale.value === 'en-US'
    ? product.value?.nameEnUS
    : product.value?.nameZhCN,
);
const summary = computed(() =>
  locale.value === 'en-US'
    ? product.value?.summaryEnUS
    : product.value?.summaryZhCN,
);

useSeoMeta({ description: summary, title: name });
</script>

<template>
  <main>
    <h1>{{ name }}</h1>
    <p>{{ summary }}</p>
  </main>
</template>
