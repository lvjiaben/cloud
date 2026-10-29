<script setup lang="ts">
import type { CustomerPaymentStatus } from '#/api/idc/customer-payments';

import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';
import { Alert, Button, Card, Skeleton, Tag } from 'ant-design-vue';

import { getCustomerPaymentStatus } from '#/api/idc/customer-payments';
import { $t } from '#/locales';

const route = useRoute();
const router = useRouter();
const loading = ref(true);
const failed = ref(false);
const status = ref<CustomerPaymentStatus>();
const attemptNo = computed(() => String(route.params.attemptNo || ''));
const complete = computed(() => status.value?.statusCode === 'succeeded');
const paymentStatusKey = computed(() => 'finance.status.' + (['created', 'submitted', 'confirming', 'succeeded', 'failed', 'expired', 'closed', 'late_success', 'manual_review', 'disputed'].includes(status.value?.statusCode ?? '') ? status.value!.statusCode : 'unavailable'));
const orderPayment = computed(() => status.value?.purposeCode === 'invoice');
const failedPayment = computed(() => ['failed', 'expired', 'closed'].includes(status.value?.statusCode ?? ''));
const needsReview = computed(() => ['late_success', 'manual_review', 'disputed'].includes(status.value?.statusCode ?? ''));
const completeTitle = computed(() => status.value?.purposeCode === 'balance_recharge' ? 'finance.paymentReturn.complete' : status.value?.purposeCode === 'credit_statement' ? 'finance.paymentReturn.creditComplete' : 'finance.paymentReturn.orderComplete');
let refreshTimer: ReturnType<typeof setTimeout> | undefined;
let refreshCount = 0;
let disposed = false;

async function refresh(quiet = false) {
  if (refreshTimer) clearTimeout(refreshTimer);
  if (!quiet) loading.value = true;
  failed.value = false;
  try {
    const response = await getCustomerPaymentStatus(attemptNo.value);
    if (disposed) return;
    status.value = response;
    if (['created', 'submitted', 'confirming'].includes(response.statusCode) && refreshCount < 24 && !document.hidden) {
      refreshTimer = setTimeout(() => { refreshCount += 1; void refresh(true); }, 5000);
    }
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

onMounted(() => void refresh());
onBeforeUnmount(() => { disposed = true; if (refreshTimer) clearTimeout(refreshTimer); });
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('finance.paymentReturn.title')">
    <div class="payment-return" data-testid="payment-return-page">
      <Card class="payment-return__card">
        <Skeleton v-if="loading" active :paragraph="{ rows: 4 }" />
        <Alert
          v-else-if="failed"
          :message="$t('finance.paymentReturn.loadFailed')"
          show-icon
          type="error"
        ><template #action><Button @click="refresh(false)">{{ $t('finance.paymentReturn.refresh') }}</Button></template></Alert>
        <template v-else-if="status">
          <div class="payment-return__heading">
            <div>
              <span>{{ $t('finance.paymentReturn.eyebrow') }}</span>
              <h1>
                {{
                  complete
                    ? $t(completeTitle)
                    : $t(needsReview ? 'finance.paymentReturn.needsReview' : failedPayment ? 'finance.paymentReturn.unsuccessful' : 'finance.paymentReturn.confirming')
                }}
              </h1>
            </div>
            <Tag :color="complete ? 'success' : failedPayment ? 'error' : needsReview ? 'warning' : 'processing'">
              {{ $t(paymentStatusKey) }}
            </Tag>
          </div>
          <p>{{ $t(needsReview ? 'finance.paymentReturn.reviewDescription' : orderPayment && complete ? 'finance.paymentReturn.orderDescription' : 'finance.paymentReturn.description') }}</p>
          <dl>
            <div>
              <dt>{{ $t('finance.paymentReturn.attemptNo') }}</dt>
              <dd>{{ status.attemptNo }}</dd>
            </div>
            <div>
              <dt>{{ $t('finance.paymentReturn.currency') }}</dt>
              <dd>{{ status.currencyCode }}</dd>
            </div>
          </dl>
          <div class="payment-return__actions">
            <Button :loading="loading" @click="refresh(false)">
              {{ $t('finance.paymentReturn.refresh') }}
            </Button>
            <Button v-if="orderPayment && complete" type="primary" @click="router.push(status.orderNo ? '/account/orders/' + encodeURIComponent(status.orderNo) : '/account/orders')">
              {{ $t('finance.paymentReturn.viewDelivery') }}
            </Button>
            <Button :type="orderPayment && complete ? 'default' : 'primary'" @click="router.push('/account/billing')">
              {{ $t('finance.paymentReturn.back') }}
            </Button>
          </div>
        </template>
      </Card>
    </div>
  </Page>
</template>

<style scoped>
.payment-return {
  display: grid;
  min-height: 60vh;
  place-items: center;
}
.payment-return__card {
  width: min(680px, 100%);
  border-top: 4px solid hsl(var(--primary));
}
.payment-return__heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
}
.payment-return__heading span,
dt {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.payment-return__heading h1 {
  margin: 6px 0 16px;
  font-size: clamp(24px, 4vw, 38px);
}
.payment-return dl {
  display: grid;
  gap: 12px;
  margin: 28px 0;
}
.payment-return dl div {
  padding: 14px 16px;
  border: 1px solid hsl(var(--border));
  border-radius: 10px;
}
.payment-return dd {
  margin: 4px 0 0;
  overflow-wrap: anywhere;
}
.payment-return__actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
