<script setup lang="ts">
import type {
  ClosureFinding,
  CustomerProfile,
  DataExportStatus,
} from '#/api/idc/customer-account';

import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  List,
  message,
  Modal,
  Space,
  Spin,
  Tag,
  Typography,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  createCustomerIdempotencyKey,
  createDataExportProof,
  createDataExportTicket,
  dataExportDownloadURL,
  getCustomerProfile,
  getDataExportStatus,
  requestCustomerClosure,
  requestDataExport,
} from '#/api/idc/customer-account';
import { $t } from '#/locales';

const loading = ref(false);
const exportBusy = ref(false);
const closureBusy = ref(false);
const profile = ref<CustomerProfile>();
const exportStatus = ref<DataExportStatus>();
const closureFindings = ref<ClosureFinding[]>([]);
const closureStatus = ref('');

const exportReady = computed(() => exportStatus.value?.status === 'ready');
const hardFindings = computed(() =>
  closureFindings.value.filter(
    (item) => item.code !== 'legal_retention' && item.count > 0,
  ),
);

function formatTime(value?: number) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—';
}

async function loadProfile() {
  loading.value = true;
  try {
    profile.value = (await getCustomerProfile()).item;
  } finally {
    loading.value = false;
  }
}

async function refreshExport() {
  if (!exportStatus.value?.requestNo) return;
  exportStatus.value = await getDataExportStatus(exportStatus.value.requestNo);
}

async function startExport() {
  if (!profile.value || exportBusy.value) return;
  exportBusy.value = true;
  try {
    const proof = await createDataExportProof({
      confirmed: true,
      expectedVersion: profile.value.version,
      idempotencyKey: createCustomerIdempotencyKey('customer-export-proof'),
    });
    const requested = await requestDataExport({
      confirmed: true,
      expectedVersion: profile.value.version,
      idempotencyKey: createCustomerIdempotencyKey('customer-export'),
      proofToken: proof.proofToken,
    });
    exportStatus.value = {
      expiresAt: 0,
      requestNo: requested.requestNo,
      requestedAt: requested.requestedAt,
      status: 'pending',
    };
    message.success($t('customerAccount.data.exportRequested'));
  } catch {
    message.error($t('customerAccount.data.exportFailed'));
  } finally {
    exportBusy.value = false;
  }
}

async function downloadExport() {
  if (!exportStatus.value?.requestNo || !exportReady.value) return;
  exportBusy.value = true;
  try {
    const ticket = await createDataExportTicket(exportStatus.value.requestNo);
    window.location.assign(dataExportDownloadURL(ticket.token));
  } finally {
    exportBusy.value = false;
  }
}

function askClosure() {
  if (!profile.value || closureBusy.value) return;
  Modal.confirm({
    cancelText: $t('customerAccount.data.closureCancel'),
    content: $t('customerAccount.data.closureConfirmDescription'),
    okButtonProps: { danger: true },
    okText: $t('customerAccount.data.closureConfirm'),
    title: $t('customerAccount.data.closureConfirmTitle'),
    onOk: submitClosure,
  });
}

async function submitClosure() {
  if (!profile.value) return;
  closureBusy.value = true;
  try {
    const result = await requestCustomerClosure({
      confirmed: true,
      expectedVersion: profile.value.version,
      idempotencyKey: createCustomerIdempotencyKey('customer-closure'),
    });
    closureFindings.value = result.item.findings || [];
    closureStatus.value = result.item.status;
    message.success(
      result.item.status === 'pending_review'
        ? $t('customerAccount.data.closureSubmitted')
        : $t('customerAccount.data.closureBlocked'),
    );
  } finally {
    closureBusy.value = false;
  }
}

onMounted(loadProfile);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('customerAccount.data.title')">
    <div class="data-workspace" data-testid="customer-data-rights-page">
      <Card :bordered="false" class="page-toolbar">
        <strong>{{ $t('customerAccount.data.heading') }}</strong>
        <p>{{ $t('customerAccount.data.subtitle') }}</p>
      </Card>

      <Spin :spinning="loading">
        <div class="data-grid">
          <Card class="action-card" :title="$t('customerAccount.data.exportTitle')">
            <p>{{ $t('customerAccount.data.exportDescription') }}</p>
            <Alert
              :message="$t('customerAccount.data.exportSecurity')"
              show-icon
              type="info"
            />
            <div v-if="exportStatus" class="status-panel" data-testid="customer-export-status">
              <div>
                <Typography.Text type="secondary">{{ $t('customerAccount.data.requestNo') }}</Typography.Text>
                <strong>{{ exportStatus.requestNo }}</strong>
              </div>
              <div>
                <Typography.Text type="secondary">{{ $t('customerAccount.data.status') }}</Typography.Text>
                <Tag color="blue">{{ $t(`customerAccount.exportStatus.${exportStatus.status}`) }}</Tag>
              </div>
              <div>
                <Typography.Text type="secondary">{{ $t('customerAccount.data.requestedAt') }}</Typography.Text>
                <strong>{{ formatTime(exportStatus.requestedAt) }}</strong>
              </div>
              <div v-if="exportStatus.expiresAt">
                <Typography.Text type="secondary">{{ $t('customerAccount.data.expiresAt') }}</Typography.Text>
                <strong>{{ formatTime(exportStatus.expiresAt) }}</strong>
              </div>
            </div>
            <Space wrap>
              <Button
                v-if="!exportStatus"
                :loading="exportBusy"
                data-testid="customer-export-request"
                type="primary"
                @click="startExport"
              >
                {{ $t('customerAccount.data.exportAction') }}
              </Button>
              <Button v-else :loading="exportBusy" @click="refreshExport">
                {{ $t('customerAccount.data.refreshStatus') }}
              </Button>
              <Button
                v-if="exportReady"
                :loading="exportBusy"
                data-testid="customer-export-download"
                type="primary"
                @click="downloadExport"
              >
                {{ $t('customerAccount.data.download') }}
              </Button>
            </Space>
          </Card>

          <Card class="action-card closure-card" :title="$t('customerAccount.data.closureTitle')">
            <p>{{ $t('customerAccount.data.closureDescription') }}</p>
            <Alert
              :message="$t('customerAccount.data.closureSafety')"
              show-icon
              type="warning"
            />
            <Alert
              v-if="closureStatus === 'pending_review'"
              :message="$t('customerAccount.data.closurePending')"
              show-icon
              type="success"
            />
            <div v-if="closureFindings.length" class="finding-list">
              <h3>{{ $t('customerAccount.data.findingsTitle') }}</h3>
              <List :data-source="closureFindings" size="small">
                <template #renderItem="{ item }">
                  <List.Item>
                    <span>{{ $t(`customerAccount.closureFinding.${item.code}`) }}</span>
                    <Tag :color="item.code === 'legal_retention' ? 'blue' : 'orange'">{{ item.count }}</Tag>
                  </List.Item>
                </template>
              </List>
              <Alert
                v-if="hardFindings.length"
                :message="$t('customerAccount.data.resolveFindings')"
                show-icon
                type="error"
              />
            </div>
            <Button
              danger
              :disabled="closureStatus === 'pending_review'"
              :loading="closureBusy"
              data-testid="customer-closure-request"
              @click="askClosure"
            >
              {{ $t('customerAccount.data.closureAction') }}
            </Button>
          </Card>
        </div>
      </Spin>
    </div>
  </Page>
</template>

<style scoped>
.data-workspace {
  display: grid;
  gap: 16px;
}

.page-toolbar strong {
  display: block;
  margin-bottom: 6px;
  font-size: 18px;
}
.page-toolbar p,
.action-card > :deep(.ant-card-body) > p {
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
}

.data-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.action-card {
  border-top: 3px solid hsl(var(--primary));
}

.closure-card {
  border-top-color: hsl(var(--destructive));
}

.action-card :deep(.ant-alert),
.finding-list,
.status-panel {
  margin-bottom: 16px;
}

.status-panel {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  padding: 14px;
  border: 1px solid hsl(var(--border));
  border-radius: 10px;
  background: hsl(var(--muted) / 0.3);
}

.status-panel div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.status-panel strong {
  overflow-wrap: anywhere;
}

@media (max-width: 900px) {
  .data-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 560px) {
  .status-panel {
    grid-template-columns: 1fr;
  }
}
</style>
