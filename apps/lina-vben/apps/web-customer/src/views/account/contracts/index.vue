<script setup lang="ts">
import type { CustomerContractItem } from '#/api/idc/customer-contracts';

import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Empty,
  message,
  Space,
  Spin,
  Tag,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  createCustomerContractSigningLink,
  listCustomerContracts,
  recheckCustomerContract,
} from '#/api/idc/customer-contracts';
import { $t } from '#/locales';
import { formatMinorAmount as formatAmount } from '#/utils/format-amount';

const props = withDefaults(defineProps<{ embedded?: boolean }>(), {
  embedded: false,
});
const pageShell = computed(() => (props.embedded ? 'div' : Page));

const route = useRoute();
const loading = ref(false);
const loadingMore = ref(false);
const loadFailed = ref(false);
const items = ref<CustomerContractItem[]>([]);
const nextCursor = ref('');
const signingBusy = ref('');
const recheckBusy = ref('');

const returnedFromSigning = computed(() => route.query.signing_return === '1');

function formatTime(value?: number) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—';
}

function statusColor(status: CustomerContractItem['statusCode']) {
  if (status === 'completed') return 'success';
  if (status === 'failed') return 'error';
  return 'processing';
}

function replaceItem(item: CustomerContractItem) {
  const index = items.value.findIndex(
    (candidate) => candidate.contractNo === item.contractNo,
  );
  if (index >= 0) items.value[index] = item;
  else items.value.unshift(item);
}

async function load(reset = true) {
  const more = !reset;
  if ((more && loadingMore.value) || (!more && loading.value)) return;
  if (more) loadingMore.value = true;
  else loading.value = true;
  loadFailed.value = false;
  try {
    const result = await listCustomerContracts({
      cursor: more ? nextCursor.value : undefined,
      limit: 20,
    });
    items.value = more
      ? [...items.value, ...(result.items || [])]
      : result.items || [];
    nextCursor.value = result.nextCursor || '';
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
    loadingMore.value = false;
  }
}

function openSafeExternalWindow(url: string, target: Window) {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    target.close();
    throw new Error($t('contracts.mine.messages.invalidSigningLink'));
  }
  if (parsed.protocol !== 'https:') {
    target.close();
    throw new Error($t('contracts.mine.messages.invalidSigningLink'));
  }
  target.location.replace(parsed.toString());
}

async function openSigning(item: CustomerContractItem) {
  if (signingBusy.value) return;
  const externalWindow = window.open('about:blank', '_blank');
  if (!externalWindow) {
    message.warning($t('contracts.mine.messages.popupBlocked'));
    return;
  }
  externalWindow.opener = null;
  signingBusy.value = item.contractNo;
  try {
    const result = await createCustomerContractSigningLink(item.contractNo);
    replaceItem(result.item.contract);
    openSafeExternalWindow(result.item.url, externalWindow);
  } catch (error) {
    externalWindow.close();
    message.error(
      error instanceof Error && error.message
        ? error.message
        : $t('contracts.mine.messages.openFailed'),
    );
  } finally {
    signingBusy.value = '';
  }
}

async function recheck(item: CustomerContractItem) {
  if (recheckBusy.value) return;
  recheckBusy.value = item.contractNo;
  try {
    const result = await recheckCustomerContract(item.contractNo);
    replaceItem(result.item);
    message.success(
      result.item.statusCode === 'completed'
        ? $t('contracts.mine.messages.completed')
        : $t('contracts.mine.messages.rechecked'),
    );
  } catch {
    message.error($t('contracts.mine.messages.recheckFailed'));
  } finally {
    recheckBusy.value = '';
  }
}

onMounted(() => load());
</script>

<template>
  <component :is="pageShell" :title="$t('contracts.mine.title')">
    <div class="contract-page" data-testid="customer-contract-page">
      <Card :bordered="false" class="page-toolbar">
        <div class="page-toolbar-content">
          <div>
            <strong>{{ $t('contracts.mine.heading') }}</strong>
            <p>{{ $t('contracts.mine.subtitle') }}</p>
          </div>
          <Button :loading="loading" @click="load()">
            {{ $t('contracts.mine.refresh') }}
          </Button>
        </div>
      </Card>

      <Alert
        v-if="returnedFromSigning"
        data-testid="contract-signing-return-notice"
        :description="$t('contracts.mine.returnDescription')"
        :message="$t('contracts.mine.returnTitle')"
        show-icon
        type="warning"
      />
      <Alert
        :description="$t('contracts.mine.authorityDescription')"
        :message="$t('contracts.mine.authorityTitle')"
        show-icon
        type="info"
      />
      <Alert
        v-if="loadFailed"
        :message="$t('contracts.mine.loadFailed')"
        show-icon
        type="error"
      >
        <template #action>
          <Button size="small" @click="load()">{{
            $t('contracts.mine.retry')
          }}</Button>
        </template>
      </Alert>

      <Spin :spinning="loading">
        <Empty
          v-if="!loading && !loadFailed && items.length === 0"
          :description="$t('contracts.mine.empty')"
        />
        <div v-else class="contract-grid" data-testid="customer-contract-list">
          <Card
            v-for="item in items"
            :key="item.contractNo"
            :data-testid="`contract-${item.contractNo}`"
            class="contract-card"
            :class="`contract-card--${item.statusCode}`"
          >
            <div class="contract-heading">
              <div>
                <div class="contract-name">{{ item.templateName }}</div>
                <div class="contract-number">{{ item.contractNo }}</div>
              </div>
              <Tag :color="statusColor(item.statusCode)">
                {{ $t(`contracts.mine.status.${item.statusCode}`) }}
              </Tag>
            </div>

            <dl class="contract-facts">
              <div>
                <dt>{{ $t('contracts.mine.fields.order') }}</dt>
                <dd>{{ item.orderNo }}</dd>
              </div>
              <div>
                <dt>{{ $t('contracts.mine.fields.amount') }}</dt>
                <dd>{{ formatAmount(item.amountMinor, item.currencyCode) }}</dd>
              </div>
              <div>
                <dt>{{ $t('contracts.mine.fields.created') }}</dt>
                <dd>{{ formatTime(item.createdAt) }}</dd>
              </div>
              <div>
                <dt>{{ $t('contracts.mine.fields.deadline') }}</dt>
                <dd>{{ formatTime(item.signingExpiresAt) }}</dd>
              </div>
            </dl>

            <Alert
              v-if="item.statusCode === 'failed'"
              class="state-note"
              :message="$t('contracts.mine.failedDescription')"
              show-icon
              type="warning"
            />

            <Space wrap>
              <Button
                v-if="item.canOpenSigning"
                data-testid="contract-open-signing"
                :loading="signingBusy === item.contractNo"
                type="primary"
                @click="openSigning(item)"
              >
                {{
                  item.statusCode === 'failed'
                    ? $t('contracts.mine.actions.retrySigning')
                    : $t('contracts.mine.actions.openSigning')
                }}
              </Button>
              <Button
                v-if="item.canRecheck"
                data-testid="contract-recheck"
                :loading="recheckBusy === item.contractNo"
                @click="recheck(item)"
              >
                {{ $t('contracts.mine.actions.recheck') }}
              </Button>
            </Space>
          </Card>
        </div>
      </Spin>

      <div v-if="nextCursor" class="load-more">
        <Button :loading="loadingMore" @click="load(false)">
          {{ $t('contracts.mine.loadMore') }}
        </Button>
      </div>
    </div>
  </component>
</template>

<style scoped>
.contract-page {
  display: grid;
  gap: 16px;
}

.page-toolbar {
  overflow: hidden;
  border: 1px solid
    color-mix(in srgb, hsl(var(--primary)) 24%, hsl(var(--border)));
}

.page-toolbar-content {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 24px;
}
.page-toolbar-content strong {
  display: block;
  margin-bottom: 6px;
  font-size: 18px;
}
.page-toolbar-content p {
  max-width: 720px;
  margin: 0;
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
}

.contract-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.contract-card {
  border-top: 3px solid #3b82f6;
}

.contract-card--completed {
  border-top-color: #22c55e;
}

.contract-card--failed {
  border-top-color: #ef4444;
}

.contract-heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 14px;
}

.contract-name {
  font-size: 18px;
  font-weight: 700;
}

.contract-number {
  margin-top: 4px;
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  overflow-wrap: anywhere;
}

.contract-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 18px 0;
}

.contract-facts div {
  min-width: 0;
  padding: 11px;
  border: 1px solid hsl(var(--border));
  border-radius: 9px;
}

.contract-facts dt {
  margin-bottom: 4px;
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.contract-facts dd {
  margin: 0;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.state-note {
  margin-bottom: 14px;
}

.load-more {
  display: flex;
  justify-content: center;
}

@media (max-width: 900px) {
  .contract-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .page-toolbar-content {
    align-items: stretch;
    flex-direction: column;
  }

  .contract-facts {
    grid-template-columns: 1fr;
  }
}
</style>
