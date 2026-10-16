<script setup lang="ts">
import type { UploadFile, UploadProps } from 'ant-design-vue';
import type {
  CreditAccount,
  CreditApplication,
  CreditStatement,
} from '#/api/idc/customer-credit';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { computed, onMounted, reactive, ref, watch } from 'vue';
import { Page } from '@vben/common-ui';
import {
  Alert,
  Button,
  Card,
  DatePicker,
  Empty,
  Form,
  FormItem,
  Input,
  message,
  Space,
  Statistic,
  Tabs,
  TabPane,
  Tag,
  Upload,
} from 'ant-design-vue';
import dayjs from 'dayjs';

import {
  getCreditOverview,
  issueBalanceRepaymentProof,
  listCreditApplications,
  listCreditStatements,
  repayStatementFromBalance,
  reserveCreditEvidence,
  submitCreditApplication,
  uploadCreditEvidence,
} from '#/api/idc/customer-credit';
import { $t } from '#/locales';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { preferences } from '@vben/preferences';
import VbenModalCompat from '#/components/vben-modal-compat.vue';

const UploadDragger = Upload.Dragger;
const loading = ref(true);
const failed = ref(false);
const accounts = ref<CreditAccount[]>([]);
const applications = ref<CreditApplication[]>([]);
const statements = ref<CreditStatement[]>([]);
type CreditDirectory = 'applications' | 'statements';
const pageState = reactive({
  applications: { currentPage: 1, pageSize: 10, total: 0 },
  statements: { currentPage: 1, pageSize: 10, total: 0 },
});
const requestedPages = {
  applications: { currentPage: 1, pageSize: 10 },
  statements: { currentPage: 1, pageSize: 10 },
};
const generations = { applications: 0, statements: 0 };
let overviewGeneration = 0;
const actionGenerations = { application: 0, repayment: 0 };
const pageLoading = reactive({ applications: false, statements: false });
const pageFailed = { applications: false, statements: false };
let overviewFailed = false;
const activeTab = ref('accounts');
const counts = reactive({ pending: 0, open: 0, overdue: 0 });
const applicationOpen = ref(false);
const submitting = ref(false);
const files = ref<UploadFile[]>([]);
const rawFiles = new Map<string, File>();
const form = reactive({
  currencyCode: 'CNY',
  limitMajor: '',
  validFrom: dayjs(),
  validUntil: dayjs().add(1, 'year'),
});
const repayment = reactive({
  open: false,
  statement: undefined as CreditStatement | undefined,
  amountMajor: '',
});

const totalAvailable = computed(() =>
  accounts.value.reduce(
    (sum, item) => sum + Number(item.availableLimitMinor || 0),
    0,
  ),
);
function applicationColumns(): VxeGridProps<CreditApplication>['columns'] {
  return [
    { field: 'applicationNo', minWidth: 180, title: $t('credit.table.number') },
    { field: 'currencyCode', width: 110, title: $t('credit.table.currency') },
    { field: 'requested', minWidth: 150, slots: { default: 'requested' }, title: $t('credit.table.requested') },
    { field: 'progress', width: 120, slots: { default: 'progress' }, title: $t('credit.table.progress') },
    { field: 'status', width: 130, slots: { default: 'status' }, title: $t('credit.table.status') },
  ];
}

function statementColumns(): VxeGridProps<CreditStatement>['columns'] {
  return [
    { field: 'statementNo', minWidth: 180, title: $t('credit.table.number') },
    { field: 'currencyCode', width: 110, title: $t('credit.table.currency') },
    { field: 'total', minWidth: 140, slots: { default: 'total' }, title: $t('credit.table.total') },
    { field: 'outstanding', minWidth: 140, slots: { default: 'outstanding' }, title: $t('credit.table.outstanding') },
    { field: 'due', width: 130, slots: { default: 'due' }, title: $t('credit.table.due') },
    { field: 'status', width: 130, slots: { default: 'status' }, title: $t('credit.table.status') },
    { field: 'action', fixed: 'right', slots: { default: 'action' }, title: $t('credit.table.action'), width: 120 },
  ];
}

const [ApplicationGrid, applicationGridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange({ currentPage, pageSize }: { currentPage: number; pageSize: number }) {
      void loadPage('applications', currentPage, pageSize);
    },
  },
  gridOptions: {
    columns: applicationColumns(),
    data: [],
    emptyRender: { name: 'Empty' },
    height: 420,
    pagerConfig: { enabled: true, pageSize: 10, pageSizes: [10, 20, 50, 100] },
    rowConfig: { keyField: 'applicationNo' },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: 'customer-credit-applications',
  },
});

const [StatementGrid, statementGridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange({ currentPage, pageSize }: { currentPage: number; pageSize: number }) {
      void loadPage('statements', currentPage, pageSize);
    },
  },
  gridOptions: {
    columns: statementColumns(),
    data: [],
    emptyRender: { name: 'Empty' },
    height: 420,
    pagerConfig: { enabled: true, pageSize: 10, pageSizes: [10, 20, 50, 100] },
    rowConfig: { keyField: 'statementNo' },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: 'customer-credit-statements',
  },
});

watch(
  () => [applications.value, pageState.applications, pageLoading.applications] as const,
  ([rows, pagerConfig, busy]) => applicationGridApi.setGridOptions({ data: rows, pagerConfig: { ...pagerConfig }, loading: busy }),
  { immediate: true },
);
watch(
  () => [statements.value, pageState.statements, pageLoading.statements] as const,
  ([rows, pagerConfig, busy]) => statementGridApi.setGridOptions({ data: rows, pagerConfig: { ...pagerConfig }, loading: busy }),
  { immediate: true },
);
watch(
  () => preferences.app.locale,
  () => {
    applicationGridApi.setGridOptions({ columns: applicationColumns() });
    statementGridApi.setGridOptions({ columns: statementColumns() });
  },
);

function money(minor: string | number, currency = 'CNY') {
  const value = String(minor || '0').padStart(3, '0');
  return `${currency} ${value.slice(0, -2)}.${value.slice(-2)}`;
}
function status(code: string) {
  const key = `credit.status.${code}`;
  const text = $t(key);
  return text === key ? code : text;
}
function statusColor(code: string) {
  if (['active', 'approved', 'paid'].includes(code)) return 'success';
  if (['overdue', 'frozen', 'rejected'].includes(code)) return 'error';
  return 'processing';
}
function majorToMinor(value: string) {
  if (!/^\d+(?:\.\d{1,2})?$/u.test(value.trim())) return '';
  const [a, b = ''] = value.trim().split('.');
  return `${a}${b.padEnd(2, '0')}`.replace(/^0+(?=\d)/u, '');
}

function clearApplication() {
  actionGenerations.application += 1;
  if (applicationOpen.value) submitting.value = false;
  applicationOpen.value = false;
  files.value = [];
  rawFiles.clear();
  form.currencyCode = 'CNY';
  form.limitMajor = '';
  form.validFrom = dayjs();
  form.validUntil = dayjs().add(1, 'year');
}

function clearRepayment() {
  actionGenerations.repayment += 1;
  if (repayment.open) submitting.value = false;
  repayment.open = false;
  repayment.statement = undefined;
  repayment.amountMajor = '';
}

function clearDirectory(kind: CreditDirectory) {
  generations[kind] += 1;
  pageLoading[kind] = false;
  pageState[kind] = { ...pageState[kind], currentPage: 1, total: 0 };
  requestedPages[kind] = { currentPage: 1, pageSize: pageState[kind].pageSize };
  if (kind === 'applications') {
    applications.value = [];
    clearApplication();
  } else {
    statements.value = [];
    clearRepayment();
  }
}

function clearOverview() {
  overviewGeneration += 1;
  accounts.value = [];
  counts.pending = 0;
  counts.open = 0;
  counts.overdue = 0;
}

function handleAccessFailure(failure: unknown, scope: CreditDirectory | 'overview') {
  const status = (failure as { response?: { status?: number } } | null)?.response?.status;
  if (status === 401) {
    clearDirectory('applications');
    clearDirectory('statements');
    clearOverview();
    submitting.value = false;
  } else if (status === 403) {
    if (scope === 'overview') clearOverview();
    else clearDirectory(scope);
  }
}

async function loadPage(kind: CreditDirectory, currentPage: number, pageSize: number) {
  if (pageSize !== pageState[kind].pageSize) currentPage = 1;
  const generation = ++generations[kind];
  requestedPages[kind] = { currentPage, pageSize };
  pageLoading[kind] = true;
  pageFailed[kind] = false;
  failed.value = overviewFailed || Object.values(pageFailed).some(Boolean);
  try {
    let total: number;
    if (kind === 'applications') {
      const result = await listCreditApplications(currentPage, pageSize);
      if (generation !== generations[kind]) return;
      applications.value = result.applications || [];
      total = result.total;
    } else {
      const result = await listCreditStatements(currentPage, pageSize);
      if (generation !== generations[kind]) return;
      statements.value = result.statements || [];
      total = result.total;
    }
    pageState[kind] = { currentPage, pageSize, total };
  } catch (failure) {
    if (generation !== generations[kind]) return;
    handleAccessFailure(failure, kind);
    pageFailed[kind] = true;
    failed.value = true;
    pageState[kind] = { ...pageState[kind] };
  } finally {
    if (generation === generations[kind]) pageLoading[kind] = false;
  }
}

async function loadOverview() {
  const generation = ++overviewGeneration;
  overviewFailed = false;
  try {
    const overview = await getCreditOverview();
    if (generation !== overviewGeneration) return;
    accounts.value = overview.overview.accounts || [];
    counts.pending = overview.overview.pendingApplicationCount;
    counts.open = overview.overview.openStatementCount;
    counts.overdue = overview.overview.overdueStatementCount;
  } catch (failure) {
    if (generation !== overviewGeneration) return;
    handleAccessFailure(failure, 'overview');
    overviewFailed = true;
    failed.value = true;
  }
}

async function load() {
  loading.value = true;
  failed.value = false;
  await Promise.all([
    loadOverview(),
    ...(['applications', 'statements'] as const).map((kind) => {
      const { currentPage, pageSize } = requestedPages[kind];
      return loadPage(kind, currentPage, pageSize);
    }),
  ]);
  loading.value = false;
}

const beforeUpload: UploadProps['beforeUpload'] = (file) => {
  const allowed = ['application/pdf', 'image/jpeg', 'image/png'].includes(
    file.type,
  );
  if (!allowed || file.size > 20 * 1024 * 1024 || files.value.length >= 10) {
    message.error($t('credit.application.fileRejected'));
    return Upload.LIST_IGNORE;
  }
  const uid = String(file.uid);
  rawFiles.set(uid, file as File);
  return false;
};
function removeFile(file: UploadFile) {
  rawFiles.delete(String(file.uid));
  files.value = files.value.filter((item) => item.uid !== file.uid);
  return true;
}

async function submitApplication() {
  const requestedLimitMinor = majorToMinor(form.limitMajor);
  const total = [...rawFiles.values()].reduce(
    (sum, file) => sum + file.size,
    0,
  );
  if (
    !requestedLimitMinor ||
    files.value.length < 1 ||
    total > 200 * 1024 * 1024 ||
    !form.validUntil.isAfter(form.validFrom)
  ) {
    message.warning($t('credit.application.invalid'));
    return;
  }
  submitting.value = true;
  const generation = actionGenerations.application;
  try {
    const uploadNos: string[] = [];
    for (const file of rawFiles.values()) {
      const reserved = await reserveCreditEvidence(file);
      if (generation !== actionGenerations.application) return;
      await uploadCreditEvidence(reserved.uploadNo, file);
      if (generation !== actionGenerations.application) return;
      uploadNos.push(reserved.uploadNo);
    }
    await submitCreditApplication({
      currencyCode: form.currencyCode.trim().toUpperCase(),
      requestedLimitMinor,
      requestedValidFrom: form.validFrom.startOf('day').valueOf(),
      requestedValidUntil: form.validUntil.endOf('day').valueOf(),
      uploadNos,
      idempotencyKey: `credit-application-${crypto.randomUUID()}`,
    });
    if (generation !== actionGenerations.application) return;
    message.success($t('credit.application.success'));
    applicationOpen.value = false;
    files.value = [];
    rawFiles.clear();
    await load();
  } catch (failure) {
    if (generation !== actionGenerations.application) return;
    handleAccessFailure(failure, 'applications');
    message.error($t('credit.application.failed'));
  } finally {
    if (generation === actionGenerations.application) submitting.value = false;
  }
}

function openRepayment(item: Record<string, unknown>) {
  repayment.statement = item as unknown as CreditStatement;
  repayment.amountMajor = '';
  repayment.open = true;
}
async function submitRepayment() {
  const item = repayment.statement;
  const amountMinor = majorToMinor(repayment.amountMajor);
  if (
    !item ||
    !amountMinor ||
    BigInt(amountMinor) > BigInt(item.outstandingAmountMinor)
  ) {
    message.warning($t('credit.repayment.invalid'));
    return;
  }
  submitting.value = true;
  const generation = actionGenerations.repayment;
  try {
    const requestNo = `credit-repay-${crypto.randomUUID()}`;
    const proof = await issueBalanceRepaymentProof(item.statementNo, {
      amountMinor,
      requestNo,
    });
    if (generation !== actionGenerations.repayment) return;
    await repayStatementFromBalance(item.statementNo, {
      amountMinor,
      requestNo,
      proofToken: proof.proof.proofToken,
    });
    if (generation !== actionGenerations.repayment) return;
    message.success($t('credit.repayment.success'));
    repayment.open = false;
    await load();
  } catch (failure) {
    if (generation !== actionGenerations.repayment) return;
    handleAccessFailure(failure, 'statements');
    message.error($t('credit.repayment.failed'));
  } finally {
    if (generation === actionGenerations.repayment) submitting.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page class="customer-inner-page" header-class="customer-inner-heading" content-class="customer-inner-content" auto-content-height
    ><div class="credit-page" data-testid="customer-credit-page">
      <Card :bordered="false" class="page-toolbar">
        <div>
          <strong>{{ $t('credit.title') }}</strong>
          <p>{{ $t('credit.description') }}</p>
        </div>
        <Button
          type="primary"
          data-testid="credit-apply-open"
          @click="applicationOpen = true"
          >{{ $t('credit.application.button') }}</Button
        >
      </Card>
      <Alert
        v-if="failed"
        type="error"
        show-icon
        :message="$t('credit.loadFailed')"
        ><template #action
          ><Button @click="load">{{ $t('credit.retry') }}</Button></template
        ></Alert
      >
      <div class="metrics">
        <Card
          ><Statistic
            :title="$t('credit.metrics.available')"
            :value="money(totalAvailable)" /></Card
        ><Card
          ><Statistic
            :title="$t('credit.metrics.pending')"
            :value="counts.pending" /></Card
        ><Card
          ><Statistic
            :title="$t('credit.metrics.open')"
            :value="counts.open" /></Card
        ><Card class="danger"
          ><Statistic
            :title="$t('credit.metrics.overdue')"
            :value="counts.overdue"
        /></Card>
      </div>
      <Card :loading="loading" class="workspace"
        ><Tabs v-model:active-key="activeTab">
          <TabPane key="accounts" :tab="$t('credit.tabs.accounts')"
            ><div v-if="accounts.length" class="account-grid">
              <article
                v-for="item in accounts"
                :key="item.creditAccountNo"
                class="account-card"
              >
                <div>
                  <Tag :color="statusColor(item.statusCode)">{{
                    status(item.statusCode)
                  }}</Tag
                  ><strong>{{ item.currencyCode }}</strong>
                </div>
                <h2>
                  {{ money(item.availableLimitMinor, item.currencyCode) }}
                </h2>
                <p>{{ $t('credit.account.available') }}</p>
                <dl>
                  <div>
                    <dt>{{ $t('credit.account.total') }}</dt>
                    <dd>
                      {{ money(item.totalLimitMinor, item.currencyCode) }}
                    </dd>
                  </div>
                  <div>
                    <dt>{{ $t('credit.account.unbilled') }}</dt>
                    <dd>
                      {{
                        money(item.unbilledPrincipalMinor, item.currencyCode)
                      }}
                    </dd>
                  </div>
                  <div>
                    <dt>{{ $t('credit.account.billed') }}</dt>
                    <dd>
                      {{ money(item.billedPrincipalMinor, item.currencyCode) }}
                    </dd>
                  </div>
                </dl>
              </article>
            </div>
            <Empty v-else :description="$t('credit.account.empty')"
          /></TabPane>
          <TabPane key="applications" :tab="$t('credit.tabs.applications')">
            <ApplicationGrid>
              <template #requested="{ row }">{{ money(row.requestedLimitMinor, row.currencyCode) }}</template>
              <template #progress="{ row }">{{ row.reviewCount }}/{{ row.requiredApprovalCount }}</template>
              <template #status="{ row }"><Tag :color="statusColor(row.statusCode)">{{ status(row.statusCode) }}</Tag></template>
            </ApplicationGrid>
          </TabPane>
          <TabPane key="statements" :tab="$t('credit.tabs.statements')">
            <StatementGrid>
              <template #total="{ row }">{{ money(row.totalAmountMinor, row.currencyCode) }}</template>
              <template #outstanding="{ row }">{{ money(row.outstandingAmountMinor, row.currencyCode) }}</template>
              <template #due="{ row }">{{ dayjs(row.paymentDueAt).format('YYYY-MM-DD') }}</template>
              <template #status="{ row }"><Tag :color="statusColor(row.statusCode)">{{ status(row.statusCode) }}</Tag></template>
              <template #action="{ row }">
                <Button v-if="row.canRepayFromBalance" type="link" data-testid="credit-repay-open" @click="openRepayment(row)">{{ $t('credit.repayment.button') }}</Button>
                <span v-else>—</span>
              </template>
            </StatementGrid>
          </TabPane>
        </Tabs></Card
      >
      <VbenModalCompat
        v-model:open="applicationOpen"
        :confirm-loading="submitting"
        :title="$t('credit.application.title')"
        @ok="submitApplication"
        ><div data-testid="credit-application-modal">
          <Alert
            :message="$t('credit.application.help')"
            show-icon
            class="modal-alert"
          /><Form layout="vertical"
            ><FormItem :label="$t('credit.application.currency')"
              ><Input
                v-model:value="form.currencyCode"
                :maxlength="3" /></FormItem
            ><FormItem :label="$t('credit.application.limit')"
              ><Input
                v-model:value="form.limitMajor"
                data-testid="credit-application-limit" /></FormItem
            ><Space
              ><FormItem :label="$t('credit.application.from')"
                ><DatePicker v-model:value="form.validFrom" /></FormItem
              ><FormItem :label="$t('credit.application.until')"
                ><DatePicker
                  v-model:value="form.validUntil" /></FormItem></Space
            ><FormItem :label="$t('credit.application.files')"
              ><UploadDragger
                v-model:file-list="files"
                accept=".pdf,.jpg,.jpeg,.png"
                :before-upload="beforeUpload"
                :multiple="true"
                :max-count="10"
                @remove="removeFile"
                ><p>{{ $t('credit.application.fileHelp') }}</p></UploadDragger
              ></FormItem
            ></Form
          >
        </div></VbenModalCompat
      >
      <VbenModalCompat
        v-model:open="repayment.open"
        :confirm-loading="submitting"
        :title="$t('credit.repayment.title')"
        @ok="submitRepayment"
        ><div data-testid="credit-repayment-modal">
          <Alert
            v-if="repayment.statement"
            :message="
              $t('credit.repayment.help', {
                amount: money(
                  repayment.statement.outstandingAmountMinor,
                  repayment.statement.currencyCode,
                ),
                balance: money(
                  repayment.statement.availableBalanceMinor,
                  repayment.statement.currencyCode,
                ),
              })
            "
            show-icon
            class="modal-alert"
          /><Input
            v-model:value="repayment.amountMajor"
            data-testid="credit-repayment-amount"
            :placeholder="$t('credit.repayment.amount')"
          /></div
      ></VbenModalCompat></div
  ></Page>
</template>

<style scoped>
.credit-page {
  display: grid;
  gap: 16px;
  color: hsl(var(--foreground));
}
.page-toolbar :deep(.ant-card-body) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}
.page-toolbar strong {
  display: block;
  margin-bottom: 6px;
  font-size: 18px;
}
.page-toolbar p {
  max-width: 700px;
  margin: 0;
  color: hsl(var(--muted-foreground));
}
.metrics {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}
.metrics .ant-card {
  border-radius: 16px;
  border: 1px solid hsl(var(--border));
}
.danger {
  border-color: #f0c6be !important;
}
.workspace {
  border-radius: 20px;
}
.account-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}
.account-card {
  border: 1px solid hsl(var(--border));
  border-radius: 18px;
  padding: 16px;
  background: hsl(var(--card));
}
.account-card > div {
  display: flex;
  justify-content: space-between;
}
.account-card h2 {
  font-size: 28px;
  margin: 24px 0 2px;
}
.account-card p {
  color: #78918a;
}
.account-card dl {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  margin-top: 24px;
}
.account-card dt {
  font-size: 12px;
  color: #78918a;
}
.account-card dd {
  margin: 5px 0 0;
  font-weight: 700;
}
.modal-alert {
  margin-bottom: 18px;
}
@media (max-width: 800px) {
  .page-toolbar :deep(.ant-card-body) {
    align-items: flex-start;
    flex-direction: column;
    gap: 22px;
  }
  .metrics {
    grid-template-columns: repeat(2, 1fr);
  }
}
@media (max-width: 520px) {
  .metrics {
    grid-template-columns: 1fr;
  }
  .account-card dl {
    grid-template-columns: 1fr;
  }
  .page-toolbar button {
    width: 100%;
  }
}
</style>
