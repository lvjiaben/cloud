<script setup lang="ts">
import type { PublicCatalogProduct } from '#/api/idc/purchase-configuration';

import { computed, onMounted, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { Page } from '@vben/common-ui';
import { preferences } from '@vben/preferences';
import { Alert, Button, Card, Empty, Skeleton, Tag } from 'ant-design-vue';

import { listPublicCatalogProducts } from '#/api/idc/purchase-configuration';
import { $t } from '#/locales';

import {
  applyPurchaseCatalogFailure,
  applyPurchaseCatalogPage,
  createPurchaseCatalogState,
} from './purchase-catalog-state';

const router = useRouter();
const catalog = reactive(createPurchaseCatalogState());
const initialLoading = ref(true);
const moreLoading = ref(false);
const chinese = computed(() => preferences.app.locale === 'zh-CN');

function text(
  item: PublicCatalogProduct,
  key: 'categoryName' | 'name' | 'summary',
) {
  return chinese.value
    ? item[`${key}ZhCN`] || item[`${key}EnUS`]
    : item[`${key}EnUS`] || item[`${key}ZhCN`];
}

async function load(reset = true) {
  if (reset) initialLoading.value = true;
  else moreLoading.value = true;
  const cursor = reset ? '' : catalog.nextCursor;
  try {
    Object.assign(
      catalog,
      applyPurchaseCatalogPage(
        catalog,
        reset,
        await listPublicCatalogProducts(cursor),
      ),
    );
  } catch {
    Object.assign(catalog, applyPurchaseCatalogFailure(catalog, reset));
  } finally {
    if (reset) initialLoading.value = false;
    else moreLoading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('purchaseConfiguration.catalog.title')">
    <Card class="catalog-panel">
      <template #extra
        ><Button @click="load(true)">{{
          $t('purchaseConfiguration.catalog.refresh')
        }}</Button></template
      >
      <Skeleton
        v-if="initialLoading && !catalog.products.length"
        active
        :paragraph="{ rows: 8 }"
      />
      <Alert
        v-else-if="catalog.initialFailed && !catalog.products.length"
        class="mb-4"
        :message="$t('purchaseConfiguration.catalog.loadFailed')"
        show-icon
        type="error"
      >
        <template #action
          ><Button size="small" @click="load(true)">{{
            $t('purchaseConfiguration.catalog.retry')
          }}</Button></template
        >
      </Alert>
      <Empty
        v-else-if="!catalog.products.length"
        :description="$t('purchaseConfiguration.catalog.empty')"
      />
      <div
        v-else
        class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3"
        data-testid="customer-product-catalog"
      >
        <Card
          v-for="product in catalog.products"
          :key="product.publicSlug"
          class="catalog-product"
          size="small"
        >
          <Tag color="blue" class="mb-2">{{ text(product, 'categoryName') }}</Tag>
          <h2 class="mb-2 text-base font-semibold">
            {{ text(product, 'name') }}
          </h2>
          <p class="mb-4 min-h-10 text-sm text-muted-foreground">
            {{ text(product, 'summary') }}
          </p>
          <Button
            block
            :disabled="product.availability === 'unavailable'"
            type="primary"
            @click="
              router.push(
                `/account/products/${encodeURIComponent(product.publicSlug)}/configure`,
              )
            "
            >{{
              product.availability === 'unavailable'
                ? $t('purchaseConfiguration.catalog.unavailable')
                : $t('purchaseConfiguration.catalog.configure')
            }}</Button
          >
        </Card>
      </div>
      <Alert
        v-if="catalog.moreFailed"
        class="mt-4"
        :message="$t('purchaseConfiguration.catalog.loadMoreFailed')"
        show-icon
        type="error"
      >
        <template #action
          ><Button size="small" @click="load(false)">{{
            $t('purchaseConfiguration.catalog.retry')
          }}</Button></template
        >
      </Alert>
      <div v-else-if="catalog.nextCursor" class="mt-4 flex justify-center">
        <Button :loading="moreLoading" @click="load(false)">{{
          $t('purchaseConfiguration.catalog.loadMore')
        }}</Button>
      </div>
    </Card>
  </Page>
</template>

<style scoped>
.catalog-product :deep(.ant-card-body) {
  display: flex;
  min-height: 208px;
  height: 100%;
  flex-direction: column;
  align-items: flex-start;
}
.catalog-product h2 {
  margin-block: 4px 8px;
  overflow-wrap: anywhere;
}
.catalog-product p {
  flex: 1;
  line-height: 22px;
  overflow-wrap: anywhere;
}
.catalog-product :deep(.ant-btn) {
  margin-top: 8px;
}
.catalog-product:focus-within {
  outline: 1px solid hsl(var(--primary));
  outline-offset: -1px;
}
</style>
