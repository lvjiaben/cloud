<script setup lang="ts">
import type {
  CustomerPrivacyConsent,
  CustomerPrivacyNotice,
  CustomerPrivacyRequest,
} from '#/api/idc/customer-privacy';

import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Alert, Button, Card, Empty, Modal, Spin, Tag, message } from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  createCustomerPrivacyRequest,
  getCustomerPrivacyOverview,
  withdrawCustomerPrivacyConsent,
} from '#/api/idc/customer-privacy';
import { $t } from '#/locales';

const loading = ref(true);
const failed = ref(false);
const busy = ref('');
const notices = ref<CustomerPrivacyNotice[]>([]);
const consents = ref<CustomerPrivacyConsent[]>([]);
const requests = ref<CustomerPrivacyRequest[]>([]);

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
  return $t(`privacy.${scope}.${code}`);
}

async function load() {
  loading.value = true;
  failed.value = false;
  try {
    const result = await getCustomerPrivacyOverview();
    notices.value = result.notices;
    consents.value = result.consents;
    requests.value = result.requests;
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}

function withdraw(item: CustomerPrivacyConsent) {
  Modal.confirm({
    cancelText: $t('privacy.common.cancel'),
    content: $t('privacy.withdraw.description'),
    okText: $t('privacy.withdraw.confirm'),
    title: $t('privacy.withdraw.title'),
    async onOk() {
      busy.value = `consent:${item.consentTypeCode}`;
      try {
        await withdrawCustomerPrivacyConsent({
          consentTypeCode: item.consentTypeCode,
          idempotencyKey: randomKey('privacy-withdraw'),
        });
        message.success($t('privacy.messages.withdrawn'));
        await load();
      } finally {
        busy.value = '';
      }
    },
  });
}

function submitRequest(requestTypeCode: 'delete' | 'export') {
  Modal.confirm({
    cancelText: $t('privacy.common.cancel'),
    content: $t(`privacy.request.${requestTypeCode}Description`),
    okText: $t('privacy.request.submit'),
    title: $t(`privacy.request.${requestTypeCode}`),
    async onOk() {
      busy.value = `request:${requestTypeCode}`;
      try {
        await createCustomerPrivacyRequest({
          idempotencyKey: randomKey('privacy-request'),
          requestTypeCode,
        });
        message.success($t('privacy.messages.requested'));
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
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('privacy.title')">
    <div class="privacy-shell" data-testid="customer-privacy-page">
      <Card :bordered="false" class="page-toolbar">
        <div>
          <strong>{{ $t('privacy.heading') }}</strong>
          <p>{{ $t('privacy.subtitle') }}</p>
        </div>
        <div class="privacy-actions">
          <Button :loading="busy === 'request:export'" @click="submitRequest('export')">
            {{ $t('privacy.request.export') }}
          </Button>
          <Button danger :loading="busy === 'request:delete'" @click="submitRequest('delete')">
            {{ $t('privacy.request.delete') }}
          </Button>
        </div>
      </Card>

      <Alert
        :description="$t('privacy.evidenceDescription')"
        :message="$t('privacy.evidenceTitle')"
        show-icon
        type="info"
      />

      <Alert v-if="failed" :message="$t('privacy.loadFailed')" show-icon type="error">
        <template #action><Button size="small" @click="load">{{ $t('privacy.retry') }}</Button></template>
      </Alert>

      <Spin :spinning="loading">
        <section class="privacy-section">
          <div class="section-heading"><h2>{{ $t('privacy.noticeTitle') }}</h2></div>
          <Empty v-if="!notices.length && !loading" :description="$t('privacy.noticeEmpty')" />
          <div class="privacy-grid">
            <Card v-for="notice in notices" :key="notice.assessmentNo" class="privacy-card">
              <template #title>{{ label('assessment', notice.assessmentTypeCode) }}</template>
              <template #extra><Tag>v{{ notice.version }}</Tag></template>
              <dl>
                <div><dt>{{ $t('privacy.fields.purpose') }}</dt><dd>{{ notice.purpose }}</dd></div>
                <div><dt>{{ $t('privacy.fields.necessity') }}</dt><dd>{{ notice.necessity }}</dd></div>
                <div><dt>{{ $t('privacy.fields.impact') }}</dt><dd>{{ notice.impact }}</dd></div>
                <div><dt>{{ $t('privacy.fields.recipients') }}</dt><dd>{{ notice.recipients }}</dd></div>
                <div><dt>{{ $t('privacy.fields.storage') }}</dt><dd>{{ notice.storageLocation }}</dd></div>
                <div><dt>{{ $t('privacy.fields.retention') }}</dt><dd>{{ notice.retentionDescription }} · {{ notice.retentionDays }} {{ $t('privacy.days') }}</dd></div>
                <div><dt>{{ $t('privacy.fields.rights') }}</dt><dd>{{ notice.rightsEntry }}</dd></div>
              </dl>
            </Card>
          </div>
        </section>

        <section class="privacy-section">
          <div class="section-heading"><h2>{{ $t('privacy.consentTitle') }}</h2></div>
          <Empty v-if="!consents.length && !loading" :description="$t('privacy.consentEmpty')" />
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
              >{{ $t('privacy.withdraw.action') }}</Button>
            </div>
          </Card>
        </section>

        <section class="privacy-section">
          <div class="section-heading"><h2>{{ $t('privacy.requestTitle') }}</h2></div>
          <Empty v-if="!requests.length && !loading" :description="$t('privacy.requestEmpty')" />
          <Card v-for="item in requests" :key="item.requestNo" class="request-row">
            <div><strong>{{ label('requestType', item.requestTypeCode) }}</strong><p>{{ formatTime(item.submittedAt) }}</p></div>
            <div><Tag>{{ label('requestStatus', item.statusCode) }}</Tag><small v-if="item.holdReasonCode">{{ label('hold', item.holdReasonCode) }}</small></div>
          </Card>
        </section>
      </Spin>
    </div>
  </Page>
</template>

<style scoped>
.privacy-shell { display: grid; gap: 20px; max-width: 100%; margin: 0 auto; }
.page-toolbar :deep(.ant-card-body) { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.page-toolbar strong { display: block; margin-bottom: 6px; font-size: 18px; }
.page-toolbar p { max-width: 700px; margin: 0; color: hsl(var(--muted-foreground)); }
.privacy-actions { display: flex; align-items: flex-start; gap: 10px; }
.privacy-section { display: grid; gap: 12px; }
.section-heading h2 { margin: 0; font-size: 19px; }
.privacy-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
dl { display: grid; gap: 10px; margin: 0; }
dl div { display: grid; gap: 3px; }
dt { color: #64748b; font-size: 12px; }
dd { margin: 0; line-height: 1.65; }
.consent-row :deep(.ant-card-body), .request-row :deep(.ant-card-body) { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.consent-row p, .request-row p { margin: 4px 0 0; color: #64748b; }
.consent-actions { display: flex; align-items: center; gap: 10px; }
.request-row small { display: block; margin-top: 6px; color: #b45309; }
@media (max-width: 720px) { .page-toolbar :deep(.ant-card-body) { align-items: stretch; flex-direction: column; } .privacy-actions { flex-wrap: wrap; } .privacy-grid { grid-template-columns: 1fr; } .consent-row :deep(.ant-card-body), .request-row :deep(.ant-card-body) { align-items: flex-start; flex-direction: column; } }
</style>
