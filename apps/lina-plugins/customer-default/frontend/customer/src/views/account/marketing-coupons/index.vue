<script setup lang="ts">
import type { MarketingCoupon } from '#/api/idc/marketing-coupons';

import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';
import { preferences } from '@vben/preferences';

import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  Space,
  Spin,
  Tag,
} from 'ant-design-vue';

import {
  claimMarketingCoupon,
  listMyMarketingCoupons,
} from '#/api/idc/marketing-coupons';
import { $t } from '#/locales';
import { formatTimestamp } from '#/utils/time';

const code = ref('');
const items = ref<MarketingCoupon[]>([]);
const loading = ref(true);
const submitting = ref(false);
const loadFailed = ref(false);
const claimState = ref<'failed' | 'succeeded'>();

const normalizedCode = computed(() => code.value.trim().toUpperCase());
const canSubmit = computed(() =>
  /^[A-Z0-9]{8,24}$/u.test(normalizedCode.value),
);

function name(item: MarketingCoupon) {
  return preferences.app.locale === 'zh-CN'
    ? item.nameZhCn || item.nameEnUs || item.campaignNo
    : item.nameEnUs || item.nameZhCn || item.campaignNo;
}

function value(item: MarketingCoupon) {
  if (item.faceAmountMinor !== null) {
    return `${item.currencyCode} ${item.faceAmountMinor}`;
  }
  return `${(item.rateBasisPoints || 0) / 100}%`;
}

async function load() {
  loading.value = true;
  loadFailed.value = false;
  try {
    const result = await listMyMarketingCoupons();
    items.value = result.items || [];
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
}

async function submit() {
  if (!canSubmit.value || submitting.value) return;
  submitting.value = true;
  claimState.value = undefined;
  try {
    await claimMarketingCoupon(normalizedCode.value);
    code.value = '';
    claimState.value = 'succeeded';
    await load();
  } catch {
    claimState.value = 'failed';
  } finally {
    submitting.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('marketingCoupons.title')">
    <div class="coupon-page" data-testid="customer-marketing-coupons-page">
      <Card :title="$t('marketingCoupons.claim.title')">
        <p class="lead">{{ $t('marketingCoupons.claim.lead') }}</p>
        <Space.Compact block>
          <Input
            v-model:value="code"
            :aria-label="$t('marketingCoupons.claim.code')"
            :maxlength="24"
            :placeholder="$t('marketingCoupons.claim.placeholder')"
            @press-enter="submit"
          />
          <Button
            :disabled="!canSubmit"
            :loading="submitting"
            data-testid="customer-marketing-coupon-redeem"
            type="primary"
            @click="submit"
            >{{ $t('marketingCoupons.claim.button') }}</Button
          >
        </Space.Compact>
        <Alert
          v-if="claimState === 'failed'"
          class="feedback"
          :message="$t('marketingCoupons.claim.invalid')"
          show-icon
          type="error"
        />
        <Alert
          v-else-if="claimState === 'succeeded'"
          class="feedback"
          :message="$t('marketingCoupons.claim.succeeded')"
          show-icon
          type="success"
        />
      </Card>

      <Card :title="$t('marketingCoupons.mine')">
        <Spin :spinning="loading">
          <Alert
            v-if="loadFailed"
            :message="$t('marketingCoupons.loadFailed')"
            show-icon
            type="error"
          />
          <div v-else-if="items.length" class="coupon-list">
            <article
              v-for="item in items"
              :key="item.assetNo"
              class="coupon-item"
            >
              <div>
                <strong>{{ name(item) }}</strong>
                <small>{{ item.assetNo }}</small>
              </div>
              <div class="coupon-value">{{ value(item) }}</div>
              <div>
                <Tag>{{ $t(`marketingCoupons.status.${item.status}`) }}</Tag>
                <small>{{ formatTimestamp(item.validUntil) }}</small>
              </div>
            </article>
          </div>
          <Empty v-else :description="$t('marketingCoupons.empty')" />
        </Spin>
      </Card>
    </div>
  </Page>
</template>

<style scoped>
.coupon-page {
  display: grid;
  gap: 16px;
  max-width: 100%;
}
.lead {
  color: hsl(var(--muted-foreground));
  margin-bottom: 16px;
}
.feedback {
  margin-top: 16px;
}
.coupon-list {
  display: grid;
  gap: 12px;
}
.coupon-item {
  align-items: center;
  border: 1px solid hsl(var(--border));
  border-radius: 12px;
  display: grid;
  gap: 16px;
  grid-template-columns: minmax(0, 1fr) auto auto;
  padding: 16px;
}
.coupon-item > div {
  display: grid;
  gap: 4px;
}
.coupon-item small {
  color: hsl(var(--muted-foreground));
}
.coupon-value {
  color: hsl(var(--primary));
  font-size: 20px;
  font-weight: 700;
}
@media (max-width: 640px) {
  .coupon-item {
    grid-template-columns: 1fr;
  }
}
</style>
