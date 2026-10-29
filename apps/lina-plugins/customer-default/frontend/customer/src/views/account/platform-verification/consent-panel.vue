<script setup lang="ts">
import type {
  CustomerPrivacyConsent,
} from '#/api/idc/customer-privacy';

import { onMounted, ref } from 'vue';

import { Alert, Button, Card, Empty, Modal, Spin, Tag, message } from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  getCustomerPrivacyOverview,
  withdrawCustomerPrivacyConsent,
} from '#/api/idc/customer-privacy';
import { $t } from '#/locales';

const loading = ref(true);
const failed = ref(false);
const busy = ref('');
const consents = ref<CustomerPrivacyConsent[]>([]);

function randomKey(prefix: string) {
  const value =
    typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : Array.from(crypto.getRandomValues(new Uint8Array(24)), (item) =>
          item.toString(16).padStart(2, '0'),
        ).join('');
  return `${prefix}-${value}`;
}

function formatTime(value?: number) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—';
}

function label(scope: string, code: string) {
  return $t(`verificationConsent.${scope}.${code}`);
}

async function load() {
  loading.value = true;
  failed.value = false;
  try {
    const result = await getCustomerPrivacyOverview();
    consents.value = result.consents;
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

function withdraw(item: CustomerPrivacyConsent) {
  Modal.confirm({
    cancelText: $t('verificationConsent.common.cancel'),
    content: $t('verificationConsent.withdraw.description'),
    okText: $t('verificationConsent.withdraw.confirm'),
    title: $t('verificationConsent.withdraw.title'),
    async onOk() {
      busy.value = `consent:${item.consentTypeCode}`;
      try {
        await withdrawCustomerPrivacyConsent({
          consentTypeCode: item.consentTypeCode,
          idempotencyKey: randomKey('privacy-withdraw'),
        });
        message.success($t('verificationConsent.messages.withdrawn'));
        await load();
      } finally {
        busy.value = '';
      }
    },
  });
}

onMounted(load);
</script>

<template>
    <div class="privacy-shell" data-testid="verification-consent-panel">
      <Card :bordered="false" class="page-toolbar">
        <div>
          <strong>{{ $t('verificationConsent.heading') }}</strong>
          <p>{{ $t('verificationConsent.subtitle') }}</p>
        </div>
      </Card>

      <Alert
        :description="$t('verificationConsent.evidenceDescription')"
        :message="$t('verificationConsent.evidenceTitle')"
        show-icon
        type="info"
      />

      <Alert v-if="failed" :message="$t('verificationConsent.loadFailed')" show-icon type="error">
        <template #action><Button size="small" @click="load">{{ $t('verificationConsent.retry') }}</Button></template>
      </Alert>

      <Spin :spinning="loading">

        <section class="privacy-section">
          <div class="section-heading"><h2>{{ $t('verificationConsent.consentTitle') }}</h2></div>
          <Empty v-if="!consents.length && !loading" :description="$t('verificationConsent.consentEmpty')" />
          <Card v-for="item in consents" :key="item.consentNo" class="consent-row">
            <div>
              <strong>{{ label('assessment', item.consentTypeCode) }}</strong>
              <p>{{ formatTime(item.decidedAt) }}</p>
            </div>
            <div class="consent-actions">
              <Tag :color="item.actionCode === 'granted' ? 'green' : 'default'">{{ label('consent', item.actionCode) }}</Tag>
              <Button
                v-if="item.actionCode === 'granted'"
                :loading="busy === `consent:${item.consentTypeCode}`"
                size="small"
                @click="withdraw(item)"
              >{{ $t('verificationConsent.withdraw.action') }}</Button>
            </div>
          </Card>
        </section>

      </Spin>
    </div>
</template>

<style scoped>
.privacy-shell { display: grid; gap: 20px; max-width: 100%; margin: 0 auto; }
.page-toolbar :deep(.ant-card-body) { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.page-toolbar strong { display: block; margin-bottom: 6px; font-size: 18px; }
.page-toolbar p { max-width: 700px; margin: 0; color: hsl(var(--muted-foreground)); }
.privacy-section { display: grid; gap: 12px; }
.section-heading h2 { margin: 0; font-size: 19px; }
.consent-row :deep(.ant-card-body) { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.consent-row p { margin: 4px 0 0; color: #64748b; }
.consent-actions { display: flex; align-items: center; gap: 10px; }
@media (max-width: 720px) { .page-toolbar :deep(.ant-card-body) { align-items: stretch; flex-direction: column; } .consent-row :deep(.ant-card-body) { align-items: flex-start; flex-direction: column; } }
</style>
