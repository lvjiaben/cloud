<script setup lang="ts">
import type {
  CustomerTaxInvoiceItem,
  CustomerTaxInvoiceProfile,
  CustomerTaxInvoiceSource,
  CustomerTaxInvoiceStatus,
} from '#/api/idc/customer-tax-invoices';

import { computed, onMounted, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Alert,
  Button,
  Card,
  Checkbox,
  Empty,
  Input,
  message,
  Select,
  Spin,
  Tag,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  createCustomerTaxInvoice,
  getCustomerTaxInvoiceOptions,
  listCustomerTaxInvoices,
} from '#/api/idc/customer-tax-invoices';
import { $t } from '#/locales';

const router = useRouter();
const optionsLoading = ref(false);
const optionsFailed = ref(false);
const historyLoading = ref(false);
const historyLoadingMore = ref(false);
const historyFailed = ref(false);
const submitting = ref(false);
const profiles = ref<CustomerTaxInvoiceProfile[]>([]);
const invoices = ref<CustomerTaxInvoiceSource[]>([]);
const history = ref<CustomerTaxInvoiceItem[]>([]);
const nextCursor = ref('');
const profileNo = ref<string>();
const selected = reactive<Record<string, boolean>>({});
const amountInputs = reactive<Record<string, string>>({});

function currencyFractionDigits(currencyCode: string) {
  try {
    return (
      new Intl.NumberFormat('en', {
        currency: currencyCode,
        style: 'currency',
      }).resolvedOptions().maximumFractionDigits ?? 2
    );
  } catch {
    return 2;
  }
}

function formatMinor(amountMinor: string, currencyCode: string) {
  const sign = amountMinor.startsWith('-') ? '-' : '';
  const digits = amountMinor.replace(/^-/, '').replace(/^0+(?=\d)/, '');
  const scale = currencyFractionDigits(currencyCode);
  if (scale === 0) return `${currencyCode} ${sign}${digits || '0'}`;
  const padded = (digits || '0').padStart(scale + 1, '0');
  return `${currencyCode} ${sign}${padded.slice(0, -scale)}.${padded.slice(-scale)}`;
}

function parseMajorToMinor(value: string, currencyCode: string) {
  const normalized = value.trim();
  const scale = currencyFractionDigits(currencyCode);
  const pattern =
    scale === 0 ? /^\d+$/u : new RegExp(`^\\d+(?:\\.\\d{0,${scale}})?$`, 'u');
  if (!pattern.test(normalized)) return null;
  const [whole = '', fraction = ''] = normalized.split('.');
  const digits = `${whole}${fraction.padEnd(scale, '0')}`.replace(
    /^0+(?=\d)/,
    '',
  );
  try {
    const amount = BigInt(digits || '0');
    if (amount <= 0n || amount > 9_223_372_036_854_775_807n) return null;
    return amount;
  } catch {
    return null;
  }
}

function formatTime(value?: number) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '—';
}

function randomKey() {
  const value =
    typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : Array.from(crypto.getRandomValues(new Uint8Array(24)), (item) =>
          item.toString(16).padStart(2, '0'),
        ).join('');
  return `tax-invoice-${value}`;
}

const profileOptions = computed(() =>
  profiles.value.map((profile) => ({
    label: `${profile.invoiceTitle} · ${$t(`taxInvoices.profile.${profile.titleTypeCode}`)}${
      profile.isDefault ? ` · ${$t('taxInvoices.profile.default')}` : ''
    }`,
    value: profile.profileNo,
  })),
);

const selectedInvoices = computed(() =>
  invoices.value.filter((invoice) => selected[invoice.billingInvoiceNo]),
);

const selectedCurrency = computed(
  () => selectedInvoices.value[0]?.currencyCode || '',
);

function amountProblem(invoice: CustomerTaxInvoiceSource) {
  if (!selected[invoice.billingInvoiceNo]) return '';
  const input = amountInputs[invoice.billingInvoiceNo] || '';
  if (!input.trim()) return 'required';
  const amount = parseMajorToMinor(input, invoice.currencyCode);
  if (amount === null) return 'invalid';
  if (amount > BigInt(invoice.remainingGrossMinor)) return 'tooLarge';
  return '';
}

const selectedTotalMinor = computed(() => {
  let total = 0n;
  for (const invoice of selectedInvoices.value) {
    const amount = parseMajorToMinor(
      amountInputs[invoice.billingInvoiceNo] || '',
      invoice.currencyCode,
    );
    if (amount !== null) total += amount;
  }
  return total;
});

const canSubmit = computed(
  () =>
    Boolean(profileNo.value) &&
    selectedInvoices.value.length > 0 &&
    selectedInvoices.value.length <= 20 &&
    selectedInvoices.value.every(
      (invoice) =>
        invoice.currencyCode === selectedCurrency.value &&
        !amountProblem(invoice),
    ),
);

function toggleInvoice(invoice: CustomerTaxInvoiceSource, checked: boolean) {
  if (
    checked &&
    !selected[invoice.billingInvoiceNo] &&
    selectedInvoices.value.length >= 20
  ) {
    message.warning($t('taxInvoices.invoice.selectionLimit'));
    return;
  }
  if (
    checked &&
    selectedCurrency.value &&
    selectedCurrency.value !== invoice.currencyCode
  ) {
    return;
  }
  selected[invoice.billingInvoiceNo] = checked;
  if (!checked) amountInputs[invoice.billingInvoiceNo] = '';
}

function invoiceDisabled(invoice: CustomerTaxInvoiceSource) {
  if (selected[invoice.billingInvoiceNo]) return false;
  return (
    selectedInvoices.value.length >= 20 ||
    Boolean(
      selectedCurrency.value && selectedCurrency.value !== invoice.currencyCode,
    )
  );
}

function clearSelection() {
  for (const key of Object.keys(selected)) delete selected[key];
  for (const key of Object.keys(amountInputs)) delete amountInputs[key];
}

async function loadOptions() {
  if (optionsLoading.value) return;
  optionsLoading.value = true;
  optionsFailed.value = false;
  try {
    const result = await getCustomerTaxInvoiceOptions();
    profiles.value = result.profiles || [];
    invoices.value = result.invoices || [];
    if (
      !profileNo.value ||
      !profiles.value.some((profile) => profile.profileNo === profileNo.value)
    ) {
      profileNo.value =
        profiles.value.find((profile) => profile.isDefault)?.profileNo ||
        profiles.value[0]?.profileNo;
    }
    clearSelection();
  } catch {
    optionsFailed.value = true;
  } finally {
    optionsLoading.value = false;
  }
}

async function loadHistory(reset = true) {
  if (reset ? historyLoading.value : historyLoadingMore.value) return;
  if (reset) historyLoading.value = true;
  else historyLoadingMore.value = true;
  historyFailed.value = false;
  try {
    const result = await listCustomerTaxInvoices({
      cursor: reset ? undefined : nextCursor.value,
      limit: 20,
    });
    history.value = reset
      ? result.items || []
      : [...history.value, ...(result.items || [])];
    nextCursor.value = result.nextCursor || '';
  } catch {
    historyFailed.value = true;
  } finally {
    historyLoading.value = false;
    historyLoadingMore.value = false;
  }
}

async function refresh() {
  await Promise.all([loadOptions(), loadHistory()]);
}

async function submit() {
  if (!canSubmit.value || !profileNo.value || submitting.value) return;
  submitting.value = true;
  try {
    const result = await createCustomerTaxInvoice({
      idempotencyKey: randomKey(),
      profileNo: profileNo.value,
      selections: selectedInvoices.value.map((invoice) => ({
        billingInvoiceNo: invoice.billingInvoiceNo,
        requestedGrossMinor: String(
          parseMajorToMinor(
            amountInputs[invoice.billingInvoiceNo] || '',
            invoice.currencyCode,
          ),
        ),
      })),
    });
    message.success(
      $t(
        result.reused
          ? 'taxInvoices.messages.reused'
          : 'taxInvoices.messages.created',
      ),
    );
    await refresh();
  } catch {
    message.error($t('taxInvoices.messages.createFailed'));
  } finally {
    submitting.value = false;
  }
}

function statusColor(status: CustomerTaxInvoiceStatus) {
  if (status === 'delivered' || status === 'issued') return 'success';
  if (status === 'rejected' || status === 'canceled') return 'default';
  if (status === 'failed') return 'error';
  if (status === 'result_unknown' || status === 'red_letter_pending') {
    return 'warning';
  }
  return 'processing';
}

onMounted(() => refresh());
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" :title="$t('taxInvoices.title')">
    <div class="tax-invoice-page" data-testid="tax-invoice-page">
      <Card :bordered="false" class="page-toolbar">
        <div class="page-toolbar-content">
          <div>
            <strong>{{ $t('taxInvoices.heading') }}</strong>
            <p>{{ $t('taxInvoices.subtitle') }}</p>
          </div>
          <Button :loading="optionsLoading || historyLoading" @click="refresh">
            {{ $t('taxInvoices.refresh') }}
          </Button>
        </div>
      </Card>

      <Alert
        :description="$t('taxInvoices.noticeDescription')"
        :message="$t('taxInvoices.noticeTitle')"
        show-icon
        type="info"
      />

      <Alert
        v-if="optionsFailed"
        :message="$t('taxInvoices.loadFailed')"
        show-icon
        type="error"
      />

      <Spin :spinning="optionsLoading">
        <div class="request-grid">
          <Card class="step-card">
            <template #title>{{ $t('taxInvoices.steps.profile') }}</template>
            <template v-if="profiles.length">
              <Select
                v-model:value="profileNo"
                :options="profileOptions"
                :placeholder="$t('taxInvoices.profile.placeholder')"
                data-testid="tax-invoice-profile"
                show-search
              />
              <Button
                class="manage-profile"
                type="link"
                @click="router.push('/account/invoice-profiles')"
              >
                {{ $t('taxInvoices.profile.manage') }}
              </Button>
            </template>
            <div v-else class="empty-inline">
              <p>{{ $t('taxInvoices.profile.empty') }}</p>
              <Button
                type="primary"
                @click="router.push('/account/invoice-profiles')"
              >
                {{ $t('taxInvoices.profile.manage') }}
              </Button>
            </div>
          </Card>

          <Card class="step-card bill-step">
            <template #title>{{ $t('taxInvoices.steps.invoices') }}</template>
            <Empty
              v-if="!invoices.length"
              :description="$t('taxInvoices.invoice.empty')"
            />
            <div v-else class="bill-list" data-testid="tax-invoice-source-list">
              <div
                v-for="invoice in invoices"
                :key="invoice.billingInvoiceNo"
                class="bill-row"
                :class="{
                  'bill-row--selected': selected[invoice.billingInvoiceNo],
                  'bill-row--disabled': invoiceDisabled(invoice),
                }"
                :data-testid="`tax-invoice-source-${invoice.billingInvoiceNo}`"
              >
                <div class="bill-heading">
                  <Checkbox
                    :checked="Boolean(selected[invoice.billingInvoiceNo])"
                    :data-testid="`tax-invoice-select-${invoice.billingInvoiceNo}`"
                    :disabled="invoiceDisabled(invoice)"
                    @update:checked="toggleInvoice(invoice, $event)"
                  >
                    <strong>{{ invoice.billingInvoiceNo }}</strong>
                  </Checkbox>
                  <Tag>{{ invoice.currencyCode }}</Tag>
                </div>
                <dl class="bill-facts">
                  <div>
                    <dt>{{ $t('taxInvoices.invoice.orderNo') }}</dt>
                    <dd>{{ invoice.orderNo || '—' }}</dd>
                  </div>
                  <div>
                    <dt>{{ $t('taxInvoices.invoice.settledAt') }}</dt>
                    <dd>{{ formatTime(invoice.settledAt) }}</dd>
                  </div>
                  <div>
                    <dt>{{ $t('taxInvoices.invoice.paid') }}</dt>
                    <dd>
                      {{
                        formatMinor(
                          invoice.sourceGrossMinor,
                          invoice.currencyCode,
                        )
                      }}
                    </dd>
                  </div>
                  <div>
                    <dt>{{ $t('taxInvoices.invoice.remaining') }}</dt>
                    <dd class="remaining-amount">
                      {{
                        formatMinor(
                          invoice.remainingGrossMinor,
                          invoice.currencyCode,
                        )
                      }}
                    </dd>
                  </div>
                </dl>
                <div
                  v-if="selected[invoice.billingInvoiceNo]"
                  class="amount-field"
                >
                  <label :for="`amount-${invoice.billingInvoiceNo}`">
                    {{ $t('taxInvoices.invoice.amount') }}
                  </label>
                  <Input
                    :id="`amount-${invoice.billingInvoiceNo}`"
                    v-model:value="amountInputs[invoice.billingInvoiceNo]"
                    :data-testid="`tax-invoice-amount-${invoice.billingInvoiceNo}`"
                    :placeholder="$t('taxInvoices.invoice.amountPlaceholder')"
                    inputmode="decimal"
                  >
                    <template #addonAfter>{{ invoice.currencyCode }}</template>
                  </Input>
                  <div v-if="amountProblem(invoice)" class="field-error">
                    {{
                      $t(
                        `taxInvoices.invoice.amount${
                          amountProblem(invoice) === 'required'
                            ? 'Required'
                            : amountProblem(invoice) === 'tooLarge'
                              ? 'TooLarge'
                              : 'Invalid'
                        }`,
                      )
                    }}
                  </div>
                </div>
                <div v-else-if="invoiceDisabled(invoice)" class="currency-help">
                  {{
                    $t(
                      selectedInvoices.length >= 20
                        ? 'taxInvoices.invoice.selectionLimit'
                        : 'taxInvoices.invoice.differentCurrency',
                    )
                  }}
                </div>
              </div>
            </div>
          </Card>
        </div>
      </Spin>

      <Card class="submit-card">
        <div class="submit-content">
          <div>
            <h2>{{ $t('taxInvoices.steps.submit') }}</h2>
            <p v-if="selectedCurrency">
              {{
                $t('taxInvoices.selectionSummary', {
                  amount: formatMinor(
                    String(selectedTotalMinor),
                    selectedCurrency,
                  ),
                  count: selectedInvoices.length,
                })
              }}
            </p>
            <p class="submit-hint">{{ $t('taxInvoices.submitHint') }}</p>
          </div>
          <Button
            :disabled="!canSubmit"
            :loading="submitting"
            data-testid="tax-invoice-submit"
            size="large"
            type="primary"
            @click="submit"
          >
            {{
              submitting
                ? $t('taxInvoices.submitting')
                : $t('taxInvoices.submit')
            }}
          </Button>
        </div>
      </Card>

      <section class="history-section" data-testid="tax-invoice-history">
        <div class="section-heading">
          <div>
            <h2>{{ $t('taxInvoices.history.title') }}</h2>
            <p>{{ $t('taxInvoices.history.description') }}</p>
          </div>
        </div>
        <Alert
          v-if="historyFailed"
          :message="$t('taxInvoices.history.loadFailed')"
          show-icon
          type="error"
        />
        <Spin :spinning="historyLoading">
          <Empty
            v-if="!historyLoading && !historyFailed && history.length === 0"
            :description="$t('taxInvoices.history.empty')"
          />
          <div v-else class="history-grid">
            <Card
              v-for="item in history"
              :key="item.taxInvoiceNo"
              :data-testid="`tax-invoice-history-${item.taxInvoiceNo}`"
              class="history-card"
            >
              <div class="history-heading">
                <div>
                  <span>{{ $t('taxInvoices.history.requestNo') }}</span>
                  <strong>{{ item.taxInvoiceNo }}</strong>
                </div>
                <Tag :color="statusColor(item.statusCode)">
                  {{ $t(`taxInvoices.status.${item.statusCode}`) }}
                </Tag>
              </div>
              <dl class="history-facts">
                <div>
                  <dt>{{ $t('taxInvoices.history.amount') }}</dt>
                  <dd>
                    {{ formatMinor(item.grossAmountMinor, item.currencyCode) }}
                  </dd>
                </div>
                <div>
                  <dt>{{ $t('taxInvoices.history.submittedAt') }}</dt>
                  <dd>{{ formatTime(item.submittedAt) }}</dd>
                </div>
                <div>
                  <dt>{{ $t('taxInvoices.history.invoiceType') }}</dt>
                  <dd>
                    {{ $t(`taxInvoices.invoiceType.${item.invoiceTypeCode}`) }}
                  </dd>
                </div>
                <div v-if="item.maskedInvoiceNo">
                  <dt>{{ $t('taxInvoices.history.invoiceNo') }}</dt>
                  <dd>{{ item.maskedInvoiceNo }}</dd>
                </div>
                <div v-if="item.issuedAt">
                  <dt>{{ $t('taxInvoices.history.issuedAt') }}</dt>
                  <dd>{{ formatTime(item.issuedAt) }}</dd>
                </div>
                <div v-if="item.deliveredAt">
                  <dt>{{ $t('taxInvoices.history.deliveredAt') }}</dt>
                  <dd>{{ formatTime(item.deliveredAt) }}</dd>
                </div>
              </dl>
              <Alert
                v-if="item.decisionReasonCode"
                :description="
                  $t(`taxInvoices.reason.${item.decisionReasonCode}`)
                "
                :message="$t('taxInvoices.history.reason')"
                show-icon
                type="warning"
              />
            </Card>
          </div>
        </Spin>
        <div v-if="nextCursor" class="load-more">
          <Button :loading="historyLoadingMore" @click="loadHistory(false)">
            {{ $t('taxInvoices.history.loadMore') }}
          </Button>
        </div>
      </section>
    </div>
  </Page>
</template>

<style scoped>
.tax-invoice-page {
  display: grid;
  gap: 16px;
}

.page-toolbar {
  border: 1px solid
    color-mix(in srgb, hsl(var(--primary)) 24%, hsl(var(--border)));
}

.page-toolbar-content,
.submit-content,
.history-heading,
.section-heading,
.bill-heading {
  display: flex;
  justify-content: space-between;
  gap: 20px;
}

.page-toolbar-content,
.submit-content {
  align-items: center;
}

.page-toolbar-content strong {
  display: block;
  margin-bottom: 6px;
  font-size: 18px;
}
.page-toolbar-content p,
.section-heading p,
.submit-content p {
  margin: 0;
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
}

.request-grid {
  display: grid;
  grid-template-columns: minmax(280px, 0.72fr) minmax(0, 1.6fr);
  gap: 16px;
}

.step-card,
.submit-card,
.history-card {
  border-top: 3px solid hsl(var(--primary));
}

.step-card :deep(.ant-select) {
  width: 100%;
}

.manage-profile {
  margin-top: 10px;
  padding-inline: 0;
}

.empty-inline {
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
}

.bill-list,
.history-grid {
  display: grid;
  gap: 12px;
}

.bill-row {
  min-width: 0;
  padding: 16px;
  border: 1px solid hsl(var(--border));
  border-radius: 10px;
  transition:
    border-color 0.18s ease,
    background-color 0.18s ease;
}

.bill-row--selected {
  border-color: hsl(var(--primary));
  background: color-mix(in srgb, hsl(var(--primary)) 5%, transparent);
}

.bill-row--disabled {
  opacity: 0.62;
}

.bill-facts,
.history-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 14px 0 0;
}

.bill-facts div,
.history-facts div {
  min-width: 0;
}

.bill-facts dt,
.history-facts dt,
.history-heading span,
.amount-field label {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.bill-facts dd,
.history-facts dd {
  margin: 3px 0 0;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.remaining-amount {
  color: hsl(var(--primary));
}

.amount-field {
  display: grid;
  gap: 6px;
  margin-top: 14px;
}

.field-error {
  color: hsl(var(--destructive));
  font-size: 12px;
}

.currency-help,
.submit-hint {
  margin-top: 12px;
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.submit-content h2,
.section-heading h2 {
  margin: 0 0 6px;
  font-size: 19px;
}

.history-section {
  display: grid;
  gap: 14px;
}

.history-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.history-heading strong {
  display: block;
  margin-top: 3px;
  overflow-wrap: anywhere;
}

.load-more {
  display: flex;
  justify-content: center;
}

@media (max-width: 900px) {
  .request-grid,
  .history-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .page-toolbar-content,
  .submit-content {
    align-items: stretch;
    flex-direction: column;
  }

  .bill-facts,
  .history-facts {
    grid-template-columns: 1fr;
  }

  .submit-content :deep(.ant-btn) {
    width: 100%;
  }
}
</style>
