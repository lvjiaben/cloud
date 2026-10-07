<script setup lang="ts">
import { createSiteClient } from '@lina/site-sdk';

const { locale, t } = useI18n();
const apiBase = import.meta.server
  ? `${useRuntimeConfig().apiOrigin}/x/idc-core/api/v1`
  : '/x/idc-core/api/v1';
const client = createSiteClient({ apiBase, fetch: globalThis.fetch });
const { data, error } = await useAsyncData('catalog', () =>
  client.listCatalogProducts({ limit: 12 }),
);

useSeoMeta({
  description: () => t('home.description'),
  title: () => t('home.title'),
});

function productName(product: { nameEnUS: string; nameZhCN: string }) {
  return locale.value === 'en-US' ? product.nameEnUS : product.nameZhCN;
}
</script>

<template>
  <main>
    <h1>{{ $t('home.title') }}</h1>
    <p v-if="error">{{ $t('home.catalogUnavailable') }}</p>
    <ul v-else>
      <li v-for="product in data?.products" :key="product.publicSlug">
        <NuxtLink :to="`/products/${product.publicSlug}`">
          {{ productName(product) }}
        </NuxtLink>
      </li>
    </ul>
  </main>
</template>
