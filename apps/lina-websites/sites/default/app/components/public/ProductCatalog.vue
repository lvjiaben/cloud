<script setup lang="ts">
// This page renders the anonymous catalog without presenting summaries as quotes.
import { computed, onMounted, onServerPrefetch, ref } from "vue";
import {
  Alert as AAlert,
  Button as AButton,
  Card as ACard,
  Empty as AEmpty,
  InputSearch as AInputSearch,
  Select as ASelect,
  Skeleton as ASkeleton,
  Tag as ATag,
} from "ant-design-vue";

import {
  getCatalogProduct,
  listPublicCatalogCampaigns,
  listCatalogProducts,
  recordPublicCatalogCampaignExposure,
  type CatalogCategory,
  type CatalogPage,
  type CatalogProduct,
  type PublicCatalogCampaign,
} from "./catalog-client";
import type { Locale } from "./public-i18n";

const route = useRoute();
const localePath = useLocalePath();
const { locale } = useI18n();
const publicLocale = computed<Locale>(() =>
  locale.value === "en-US" ? "en-US" : "zh-CN",
);

const content = {
  "zh-CN": {
    allCategories: "全部分类",
    allCurrencies: "全部币种",
    available: "库存可用",
    back: "返回商品列表",
    browse: "浏览全部商品",
    currency: "可用币种",
    empty: "暂无符合条件的商品",
    error: "商品暂时加载失败，请稍后重试。",
    filterCategory: "选择分类",
    filterCurrency: "选择币种",
    manual: "下单前确认库存",
    campaignCurrency: "活动币种",
    campaignEmpty: "当前币种暂无适用于此商品的公开活动",
    campaignError: "活动暂时加载失败，不影响浏览商品。",
    campaignPeriod: "活动时间",
    campaigns: "当前活动",
    next: "下一页",
    previous: "上一页",
    pricingNote: "商品购买能力将在对应插件提供通用配置声明后开放。",
    retry: "重新加载",
    search: "搜索商品名称",
    unavailable: "暂不可售",
    viewDetails: "查看详情",
  },
  "en-US": {
    allCategories: "All categories",
    allCurrencies: "All currencies",
    available: "Available",
    back: "Back to products",
    browse: "Browse all products",
    currency: "Available currencies",
    empty: "No products match these filters",
    error: "Products are temporarily unavailable. Please try again.",
    filterCategory: "Choose category",
    filterCurrency: "Choose currency",
    manual: "Inventory confirmation required",
    campaignCurrency: "Campaign currency",
    campaignEmpty:
      "No public campaign currently applies to this product and currency",
    campaignError:
      "Campaigns are temporarily unavailable. Product browsing still works.",
    campaignPeriod: "Campaign period",
    campaigns: "Current campaigns",
    next: "Next",
    previous: "Previous",
    pricingNote:
      "Purchasing will open after the relevant plugin provides its general configuration declaration.",
    retry: "Try again",
    search: "Search products",
    unavailable: "Unavailable",
    viewDetails: "View details",
  },
} as const;

const text = computed(() => content[publicLocale.value]);
const loading = ref(true);
const failed = ref(false);
const page = ref<CatalogPage>({
  availableCurrencies: [],
  categories: [],
  nextCursor: "",
  products: [],
});
const detail = ref<CatalogProduct>();
const namePrefix = ref("");
const categoryNo = ref<string>();
const currency = ref<string>();
const campaignCurrency = ref("");
const campaigns = ref<PublicCatalogCampaign[]>([]);
const campaignsFailed = ref(false);
const campaignsLoading = ref(false);
const cursors = ref<string[]>([]);

const detailSlug = computed(() => {
  const path = route.path.replace(/^\/en-US(?=\/|$)/u, "") || "/";
  const match = path.match(/^\/products\/([^/]+)$/);
  return match?.[1] ? decodeURIComponent(match[1]) : "";
});

const categories = computed(() => page.value.categories);

function localized(product: CatalogProduct, key: "name" | "summary") {
  if (key === "name")
    return publicLocale.value === "zh-CN" ? product.nameZhCN : product.nameEnUS;
  return publicLocale.value === "zh-CN" ? product.summaryZhCN : product.summaryEnUS;
}

function localizedCategory(category: CatalogCategory) {
  return publicLocale.value === "zh-CN" ? category.nameZhCN : category.nameEnUS;
}

function availabilityLabel(value: CatalogProduct["availability"]) {
  if (value === "available") return text.value.available;
  if (value === "manual_confirmation") return text.value.manual;
  return text.value.unavailable;
}

function availabilityColor(value: CatalogProduct["availability"]) {
  if (value === "available") return "success";
  if (value === "manual_confirmation") return "warning";
  return "default";
}

function formatCampaignTime(value: number) {
  return new Intl.DateTimeFormat(publicLocale.value, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function localizedCampaign(
  campaign: PublicCatalogCampaign,
  key: "description" | "name",
) {
  if (key === "name")
    return publicLocale.value === "zh-CN" ? campaign.nameZhCN : campaign.nameEnUS;
  return publicLocale.value === "zh-CN"
    ? campaign.descriptionZhCN
    : campaign.descriptionEnUS;
}

const campaignDeviceStorageKey = "lina.public-campaign.device-id";

function campaignDeviceID() {
  if (!import.meta.client) return "";
  const existing = localStorage.getItem(campaignDeviceStorageKey);
  if (existing && existing.length >= 16 && existing.length <= 128)
    return existing;
  const value = crypto.randomUUID();
  localStorage.setItem(campaignDeviceStorageKey, value);
  return value;
}

async function loadCampaigns(recordExposures = import.meta.client) {
  if (!detailSlug.value || !campaignCurrency.value) {
    campaigns.value = [];
    return;
  }
  campaignsLoading.value = true;
  campaignsFailed.value = false;
  try {
    campaigns.value = await listPublicCatalogCampaigns(
      detailSlug.value,
      campaignCurrency.value,
    );
    if (recordExposures && campaigns.value.length > 0) {
      const deviceId = campaignDeviceID();
      await Promise.allSettled(
        campaigns.value.map((campaign) =>
          recordPublicCatalogCampaignExposure(
            detailSlug.value,
            campaign.campaignNo,
            campaignCurrency.value,
            deviceId,
          ),
        ),
      );
    }
  } catch {
    campaigns.value = [];
    campaignsFailed.value = true;
  } finally {
    campaignsLoading.value = false;
  }
}

async function load(cursor = "") {
  loading.value = true;
  failed.value = false;
  try {
    page.value = await listCatalogProducts({
      categoryNo: categoryNo.value,
      currency: currency.value,
      cursor,
      namePrefix: namePrefix.value.trim(),
    });
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

async function loadDetail() {
  loading.value = true;
  failed.value = false;
  try {
    detail.value = await getCatalogProduct(detailSlug.value);
    campaignCurrency.value =
      campaignCurrency.value || detail.value.currencies.at(0) || "";
    await loadCampaigns();
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

function resetAndLoad() {
  cursors.value = [];
  void load();
}

function nextPage() {
  if (!page.value.nextCursor) return;
  cursors.value.push(page.value.nextCursor);
  void load(page.value.nextCursor);
}

function previousPage() {
  cursors.value.pop();
  const previous = cursors.value.at(-1) ?? "";
  void load(previous);
}

onMounted(() => {
  if (detailSlug.value) void loadDetail();
  else void load();
});

onServerPrefetch(async () => {
  if (detailSlug.value) await loadDetail();
  else await load();
});
</script>

<template>
  <section class="catalog-page" data-testid="public-catalog">
    <div v-if="!detailSlug" class="catalog-heading">
      <div>
        <p class="eyebrow">{{ text.currency }}</p>
        <h1>{{ text.browse }}</h1>
        <p>{{ text.pricingNote }}</p>
      </div>
    </div>

    <AAlert
      v-if="failed"
      class="catalog-alert"
      :message="text.error"
      show-icon
      type="error"
    >
      <template #action>
        <AButton size="small" @click="detailSlug ? loadDetail() : load()">{{
          text.retry
        }}</AButton>
      </template>
    </AAlert>

    <template v-else-if="detailSlug">
      <ASkeleton v-if="loading" active :paragraph="{ rows: 6 }" />
      <div
        v-else-if="detail"
        class="catalog-detail"
        data-testid="public-catalog-detail"
      >
        <NuxtLink class="catalog-back" :to="localePath('/products')">
          ← {{ text.back }}
        </NuxtLink>
        <ACard class="catalog-detail-card" :bordered="true">
          <div class="product-card-heading">
            <div>
              <span class="product-category">{{
                publicLocale === "zh-CN"
                  ? detail.categoryNameZhCN
                  : detail.categoryNameEnUS
              }}</span>
              <h1>{{ localized(detail, "name") }}</h1>
            </div>
            <ATag :color="availabilityColor(detail.availability)">{{
              availabilityLabel(detail.availability)
            }}</ATag>
          </div>
          <p class="product-summary">{{ localized(detail, "summary") }}</p>
          <div class="currency-row">
            <strong>{{ text.currency }}</strong>
            <ATag v-for="item in detail.currencies" :key="item">{{
              item
            }}</ATag>
          </div>
        </ACard>
        <ProductPurchaseConfigurator
          :key="detail.publicSlug"
          :currencies="detail.currencies"
          :locale="publicLocale"
          :public-slug="detail.publicSlug"
        />
        <ACard
          class="catalog-detail-card"
          :bordered="true"
          data-testid="public-product-campaigns"
        >
          <div class="product-card-heading">
            <h2>{{ text.campaigns }}</h2>
            <ASelect
              v-if="detail.currencies.length"
              v-model:value="campaignCurrency"
              :aria-label="text.campaignCurrency"
              :options="
                detail.currencies.map((item) => ({ label: item, value: item }))
              "
              style="min-width: 112px"
              @change="loadCampaigns()"
            />
          </div>
          <ASkeleton v-if="campaignsLoading" active :paragraph="{ rows: 2 }" />
          <AAlert
            v-else-if="campaignsFailed"
            :message="text.campaignError"
            show-icon
            type="warning"
          />
          <AEmpty
            v-else-if="campaigns.length === 0"
            :description="text.campaignEmpty"
          />
          <div v-else class="campaign-grid">
            <ACard
              v-for="campaign in campaigns"
              :key="campaign.campaignNo"
              class="campaign-card"
              size="small"
            >
              <h3>{{ localizedCampaign(campaign, "name") }}</h3>
              <p>{{ localizedCampaign(campaign, "description") }}</p>
              <p class="product-meta">
                {{ text.campaignPeriod }}：{{
                  formatCampaignTime(campaign.startsAt)
                }}
                – {{ formatCampaignTime(campaign.endsAt) }}
              </p>
            </ACard>
          </div>
        </ACard>
      </div>
    </template>

    <template v-else>
      <div class="catalog-filters" data-testid="public-catalog-filters">
        <AInputSearch
          v-model:value="namePrefix"
          :placeholder="text.search"
          allow-clear
          @search="resetAndLoad"
        />
        <ASelect
          v-model:value="categoryNo"
          :placeholder="text.filterCategory"
          allow-clear
          :options="
            categories.map((item) => ({
              label: localizedCategory(item),
              value: item.categoryNo,
            }))
          "
          @change="resetAndLoad"
        />
        <ASelect
          v-model:value="currency"
          :placeholder="text.filterCurrency"
          allow-clear
          :options="
            page.availableCurrencies.map((item) => ({
              label: item,
              value: item,
            }))
          "
          @change="resetAndLoad"
        />
      </div>

      <div v-if="loading" class="catalog-grid">
        <ACard v-for="item in 6" :key="item" class="product-card"
          ><ASkeleton active
        /></ACard>
      </div>
      <AEmpty
        v-else-if="page.products.length === 0"
        :description="text.empty"
      />
      <div v-else class="catalog-grid" data-testid="public-catalog-products">
        <ACard
          v-for="product in page.products"
          :key="product.publicSlug"
          class="product-card"
          hoverable
        >
          <div class="product-card-heading">
            <div>
              <span class="product-category">{{
                publicLocale === "zh-CN"
                  ? product.categoryNameZhCN
                  : product.categoryNameEnUS
              }}</span>
              <h2>{{ localized(product, "name") }}</h2>
            </div>
            <ATag :color="availabilityColor(product.availability)">{{
              availabilityLabel(product.availability)
            }}</ATag>
          </div>
          <p class="product-summary">{{ localized(product, "summary") }}</p>
          <div class="currency-row">
            <ATag v-for="item in product.currencies" :key="item">{{
              item
            }}</ATag>
          </div>
          <div class="product-card-footer">
            <AButton :href="localePath(`/products/${product.publicSlug}`)">{{
              text.viewDetails
            }}</AButton>
          </div>
        </ACard>
      </div>

      <div v-if="!loading && page.products.length" class="catalog-pagination">
        <AButton :disabled="cursors.length === 0" @click="previousPage">{{
          text.previous
        }}</AButton>
        <AButton
          :disabled="!page.nextCursor"
          type="primary"
          @click="nextPage"
          >{{ text.next }}</AButton
        >
      </div>
    </template>
  </section>
</template>

<style scoped>
.campaign-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(min(280px, 100%), 1fr));
}

.campaign-card h3 {
  margin-bottom: 8px;
}

.campaign-card p:last-child {
  margin-bottom: 0;
}
</style>
