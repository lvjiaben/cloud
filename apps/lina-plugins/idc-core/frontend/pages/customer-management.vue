<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/customers",
  title: "Customer Management",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VbenFormProps } from "@vben/common-ui";
import type { VbenFormSchema } from "#/adapter/form";

import type { VxeGridProps } from "#/adapter/vxe-table";

import type {
  CustomerListParams,
  CustomerSegment,
  CustomerStatus,
  CustomerSummary,
  CustomerType,
  VerificationStatus,
} from "./customer-client";

import {
  computed,
  h,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";

import { useAccess } from "./plugin-access";
import { useRoute } from "vue-router";
import OperationsDetailPanel from "./components/operations-detail-panel.vue";
import { minorToMajorAmount } from "./catalog-money";
import { createNumberedPage } from "./numbered-page";
import { Page, useVbenDrawer, useVbenModal } from "@vben/common-ui";
import { $te } from "@vben/locales";
import { preferences } from "@vben/preferences";

import {
  Badge,
  Button as AButton,
  Modal,
  Space,
  Tag,
  message,
} from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";

import enPluginMessages from "../../manifest/i18n/en-US/plugin.json";
import zhPluginMessages from "../../manifest/i18n/zh-CN/plugin.json";
import AssistedQuoteCreateDrawer from "./components/assisted-quote-create-drawer.vue";
import CustomerOverviewDrawer from "./components/customer-overview-drawer.vue";
import CustomerTagBatchModal from "./components/customer-tag-batch-modal.vue";
import {
  customerDirectCreate,
  customerList,
  customerPasswordReset,
  customerSegments,
} from "./customer-client";


const accessCodes = {
  create: "idc:admin:customer:create",
  query: "idc:admin:customer:query",
  tagBatch: "idc:admin:customer:tag:batch",
  update: "idc:admin:customer:update",
};

const { hasAccessByCodes } = useAccess();
const pagination = createNumberedPage<CustomerSummary>(
  ({ currentPage, pageSize, total }) => gridApi.setGridOptions({ pagerConfig: { currentPage, pageSize, total } }),
  () => { customerOverviewDrawerApi.close(); customerTagBatchModalApi.close(); void gridApi.grid?.clearCheckboxRow(); },
);
let activeFilterSignature = "";
const groupSegmentParams = reactive({ namePrefix: "" });
const tagSegmentParams = reactive({ namePrefix: "" });
const segmentSearchTimers: Partial<
  Record<"group" | "tag", ReturnType<typeof setTimeout>>
> = {};

const canAssist = computed(() => hasAccessByCodes(["idc:admin:commerce:assisted-quote:create"]));
const canViewOverview = computed(() => hasAccessByCodes([accessCodes.query]));
const canBatchTags = computed(() => hasAccessByCodes([accessCodes.tagBatch]));
const canCreateCustomer = computed(() =>
  hasAccessByCodes([accessCodes.create]),
);
const canResetPassword = computed(() => hasAccessByCodes([accessCodes.update]));
const directSaving = ref(false);
const selectedCustomerCount = ref(0);
const directRequestKey = ref("");
const initialPassword = ref("");

function randomPassword() {
  const bytes = crypto.getRandomValues(new Uint8Array(20));
  const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789";
  return "Aa7!" + Array.from(bytes, value => alphabet[value % alphabet.length]).join("");
}
function showPassword(password: string) {
  Modal.success({ title: directText("password"), okText: $t("plugin.idc-core.customer.password.done"), content: () => h("div", { class: "space-y-3" }, [h("code", { "data-testid": "customer-generated-password", class: "block break-all text-base" }, password), h(AButton, { "data-testid": "customer-password-copy", onClick: async () => { try { await navigator.clipboard.writeText(password); message.success($t("plugin.idc-core.customer.overview.copied")); } catch { message.warning($t("plugin.idc-core.customer.overview.copyFailed")); } } }, () => $t("plugin.idc-core.customer.password.copy"))]), maskClosable: false });
}
function directFormSchema(): VbenFormSchema[] {
  return [
    { component: "Input", componentProps: { "data-testid": "customer-direct-email", autocomplete: "off", maxlength: 320 }, fieldName: "email", label: $t("plugin.idc-core.customer.activation.email"), rules: z.string().trim().email({ message: $t("plugin.idc-core.customer.validation.emailInvalid") }).optional().or(z.literal("")) },
    { component: "Input", componentProps: { "data-testid": "customer-direct-phone", autocomplete: "off", maxlength: 32 }, fieldName: "phone", label: directText("phone") },
  ];
}
const [DirectCustomerForm, directFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 80 }, schema: [], showDefaultActions: false,
});
const [DirectModal, directModalApi] = useVbenModal({
  class: "w-[min(96vw,480px)]", onConfirm: createDirectCustomer,
});
function openPasswordReset(row: CustomerSummary) {
  const password = randomPassword();
  const key = `customer-password-reset-${crypto.randomUUID()}`;
  Modal.confirm({
    title: $t("plugin.idc-core.customer.password.title"),
    content: $t("plugin.idc-core.customer.password.confirm", { account: row.primaryPhone || row.primaryEmail || row.displayName }),
    async onOk() {
      await customerPasswordReset(row.customerNo, { idempotencyKey: key, newPassword: password, reason: "Administrator confirmed customer password reset." });
      showPassword(password);
    },
  });
}

type DirectMessageKey =
  | "created"
  | "description"
  | "login"
  | "notice"
  | "open"
  | "password"
  | "phone"
  | "required"
  | "title";

function directText(key: DirectMessageKey) {
  const messageKey = `plugin.idc-core.customer.direct.${key}`;
  if ($te(messageKey)) return $t(messageKey);
  const fallbackMessages = preferences.app.locale
    ?.toLowerCase()
    .startsWith("zh")
    ? zhPluginMessages
    : enPluginMessages;
  return fallbackMessages[messageKey as keyof typeof fallbackMessages];
}

async function openDirectCustomer() {
  directRequestKey.value = `customer-direct-${crypto.randomUUID()}`;
  initialPassword.value = randomPassword();
  directModalApi.setState({ title: directText("title"), confirmText: directText("open") }).open();
  await nextTick();
  await directFormApi.setState({ schema: directFormSchema() });
  await directFormApi.resetForm();
  await directFormApi.setValues({ email: "", phone: "" });
}
async function createDirectCustomer() {
  if (directSaving.value) return false;
  const { valid } = await directFormApi.validate();
  const values = await directFormApi.getValues<{ email?: string; phone?: string }>();
  const email = values.email?.trim() || "";
  const phone = values.phone?.trim() || "";
  if (!valid || (!email && !phone)) { message.warning(directText("required")); return false; }
  directSaving.value = true;
  directModalApi.lock(true);
  try {
    await customerDirectCreate({ countryCode: "CN", customerType: "individual", displayName: phone || email,
      email: email || undefined, phone: phone || undefined, phoneCountryCode: phone ? "+86" : undefined,
      idempotencyKey: directRequestKey.value, initialPassword: initialPassword.value,
      preferredLocale: "zh-CN", reason: "Administrator created a customer login account." });
    directModalApi.close();
    showPassword(initialPassword.value);
    initialPassword.value = "";
    await gridApi.query();
  } finally { directSaving.value = false; directModalApi.lock(false); }
}

function buildSearchSchema(): VbenFormProps["schema"] {
  const allOption = {
    label: $t("plugin.idc-core.customer.search.all"),
    value: "",
  };
  return [
    {
      component: "Input",
      componentProps: {
        "data-testid": "customer-search-code",
        maxlength: 18,
        placeholder: $t("plugin.idc-core.customer.search.codePlaceholder"),
      },
      fieldName: "customerCode",
      label: $t("plugin.idc-core.customer.search.code"),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "customer-search-phone",
        maxlength: 20,
        placeholder: $t("plugin.idc-core.customer.search.phonePlaceholder"),
      },
      fieldName: "phone",
      label: $t("plugin.idc-core.customer.search.phone"),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "customer-search-name",
        maxlength: 100,
        placeholder: $t("plugin.idc-core.customer.search.namePlaceholder"),
      },
      fieldName: "namePrefix",
      label: $t("plugin.idc-core.customer.search.name"),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": "customer-search-type",
        options: [
          allOption,
          {
            label: $t("plugin.idc-core.customer.type.individual"),
            value: "individual",
          },
          {
            label: $t("plugin.idc-core.customer.type.enterprise"),
            value: "enterprise",
          },
        ],
      },
      defaultValue: "",
      fieldName: "type",
      label: $t("plugin.idc-core.customer.search.type"),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": "customer-search-status",
        options: [
          allOption,
          ...[
            "pending_activation",
            "active",
            "restricted",
            "disabled",
            "anonymized",
          ].map((value) => ({
            label: $t(`plugin.idc-core.customer.status.${value}`),
            value,
          })),
        ],
      },
      defaultValue: "",
      fieldName: "status",
      label: $t("plugin.idc-core.customer.search.status"),
    },
    {
      component: "ApiSelect",
      componentProps: {
        "data-testid": "customer-search-group",
        api: (params: { namePrefix?: string }) =>
          segmentOptions("group", params.namePrefix || ""),
        filterOption: false,
        labelField: "label",
        onSearch: (value: string) => scheduleSegmentSearch("group", value),
        params: groupSegmentParams,
        showSearch: true,
        valueField: "value",
      },
      defaultValue: "",
      fieldName: "groupNo",
      label: $t("plugin.idc-core.customer.search.group"),
    },
    {
      component: "ApiSelect",
      componentProps: {
        "data-testid": "customer-search-tag",
        api: (params: { namePrefix?: string }) =>
          segmentOptions("tag", params.namePrefix || ""),
        filterOption: false,
        labelField: "label",
        onSearch: (value: string) => scheduleSegmentSearch("tag", value),
        params: tagSegmentParams,
        showSearch: true,
        valueField: "value",
      },
      defaultValue: "",
      fieldName: "tagNo",
      label: $t("plugin.idc-core.customer.search.tag"),
    },
  ];
}

function scheduleSegmentSearch(kind: "group" | "tag", value: string) {
  const active = segmentSearchTimers[kind];
  if (active) clearTimeout(active);
  segmentSearchTimers[kind] = setTimeout(() => {
    const target = kind === "group" ? groupSegmentParams : tagSegmentParams;
    target.namePrefix = value.trim();
    delete segmentSearchTimers[kind];
  }, 250);
}

function localizedSegmentName(item: CustomerSegment) {
  return preferences.app.locale === "zh-CN"
    ? item.nameZhCn || item.nameEnUs || item.code
    : item.nameEnUs || item.nameZhCn || item.code;
}

async function segmentOptions(kind: "group" | "tag", namePrefix: string) {
  const locale = preferences.app.locale === "zh-CN" ? "zh-CN" : "en-US";
  const result = await customerSegments({
    kind,
    limit: 25,
    locale,
    namePrefix,
  });
  return [
    {
      label: $t("plugin.idc-core.customer.search.all"),
      value: "",
    },
    ...result.items.map((item) => ({
      label: localizedSegmentName(item),
      value: item.number,
    })),
  ];
}

function buildColumns(): VxeGridProps["columns"] {
  return [
    { type: "checkbox", width: 52 },
    {
      field: "customerCode",
      width: 96,
      title: $t("plugin.idc-core.customer.table.customerNo"),
    },
    {
      field: "primaryPhone",
      slots: { default: "account" },
      minWidth: 200,
      title: $t("plugin.idc-core.customer.table.phone"),
    },
    {
      field: "displayName",
      minWidth: 160,
      title: $t("plugin.idc-core.customer.table.customer"),
    },
    { field:"balances",slots:{default:"wallet"},title:$t("plugin.idc-core.customer.workspace.wallet"),minWidth:170 },
    {
      field: "type",
      slots: { default: "type" },
      title: $t("plugin.idc-core.customer.table.type"),
      width: 90,
    },
    {
      field: "status",
      slots: { default: "status" },
      title: $t("plugin.idc-core.customer.table.status"),
      width: 110,
    },
    {
      field: "verification",
      slots: { default: "verification" },
      title: $t("plugin.idc-core.customer.table.verification"),
      width: 130,
    },
    {
      field: "level",
      slots: { default: "level" },
      title: $t("plugin.idc-core.customer.table.level"),
      width: 150,
    },
    {
      field: "createdAt",
      formatter: ({ cellValue }) => formatTimestamp(cellValue),
      minWidth: 180,
      title: $t("plugin.idc-core.customer.table.createdAt"),
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: $t("plugin.idc-core.customer.table.actions"),
      width: 300,
    }),
  ];
}

function normalizeFilters(values: Record<string, unknown>): CustomerListParams {
  return {
    customerCode: String(values.customerCode || "").trim(),
    namePrefix: String(values.namePrefix || "").trim(),
    phone: String(values.phone || "").trim(),
    groupNo: String(values.groupNo || ""),
    status: String(values.status || "") as CustomerStatus | "",
    tagNo: String(values.tagNo || ""),
    type: String(values.type || "") as CustomerType | "",
  };
}

function filterSignature(filters: CustomerListParams) {
  return JSON.stringify([
    filters.customerCode || "",
    filters.namePrefix || "",
    filters.phone || "",
    filters.type || "",
    filters.status || "",
    filters.groupNo || "",
    filters.tagNo || "",
  ]);
}

const [Grid, gridApi] = useVbenVxeGrid({
  gridEvents: {
    checkboxChange: () => { selectedCustomerCount.value = gridApi.grid.getCheckboxRecords().length; },
    checkboxAll: () => { selectedCustomerCount.value = gridApi.grid.getCheckboxRecords().length; },
  },
  formOptions: {
    commonConfig: {
      componentProps: { allowClear: true },
      labelWidth: 90,
    },
    schema: [],
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-3",
  },
  gridOptions: {
    checkboxConfig: { highlight: true, reserve: true },
    columns: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: async ({ page }: { page: { currentPage: number; pageSize: number } }, formValues = {}) => {
          const filters = normalizeFilters(formValues);
          const signature = filterSignature(filters);
          if (signature !== activeFilterSignature) {
            activeFilterSignature = signature;
            page.currentPage = 1;
          }

          return pagination.query(page, async (currentPage, pageSize) => {
          const result = await customerList({
            ...filters,
            page: currentPage,
            limit: pageSize,
          });
          return { items: result.customers, total: result.total };
          });
        },
      },
    },
    rowConfig: { keyField: "customerNo" },
    toolbarConfig: { custom: true, refresh: true, zoom: true },
    id: "idc-customer-management",
  },
});

const [AssistedOrderDrawer, assistedOrderDrawerApi] = useVbenDrawer({ connectedComponent: AssistedQuoteCreateDrawer });

const [CustomerOverviewDrawerRef, customerOverviewDrawerApi] = useVbenDrawer({
  connectedComponent: CustomerOverviewDrawer,
});

const [CustomerTagBatchModalRef, customerTagBatchModalApi] = useVbenModal({
  connectedComponent: CustomerTagBatchModal,
});

function localizedLevelName(row: CustomerSummary) {
  if (!row.level) return "—";
  return preferences.app.locale === "zh-CN"
    ? row.level.nameZhCn || row.level.nameEnUs || row.level.code
    : row.level.nameEnUs || row.level.nameZhCn || row.level.code;
}

function statusBadge(status: CustomerStatus) {
  if (status === "active") return "success";
  if (status === "pending_activation") return "processing";
  if (status === "restricted") return "warning";
  return "default";
}

function verificationColor(status: VerificationStatus) {
  if (status === "verified") return "success";
  if (status === "pending") return "processing";
  if (status === "unverified") return "default";
  return "error";
}

function openOverview(row: CustomerSummary) {
  customerOverviewDrawerApi.setData({ customerNo: row.customerNo });
  customerOverviewDrawerApi.open();
}

const route = useRoute();
const operationsDetail = computed(() => route.query.operationsMetric === 'customers');
function openOperationsCustomer(customerNo: string) {
  customerOverviewDrawerApi.setData({ customerNo });
  customerOverviewDrawerApi.open();
}

function openTagBatch() {
  const rows = (gridApi.grid?.getCheckboxRecords?.() ??
    []) as CustomerSummary[];
  if (rows.length === 0) return;
  if (rows.length > 100) {
    message.warning(
      $t("plugin.idc-core.customer.tagBatch.tooMany", { count: 100 }),
    );
    return;
  }
  customerTagBatchModalApi.setData({ rows });
  customerTagBatchModalApi.open();
}

async function handleTagBatchSuccess() {
  selectedCustomerCount.value = 0;
  await gridApi.grid?.clearCheckboxRow?.();
  await gridApi.query();
}


onMounted(async () => {
  gridApi.formApi.setState({ schema: buildSearchSchema() });
  gridApi.setGridOptions({ columns: buildColumns() });
  if (!operationsDetail.value) await gridApi.query();
});
watch(operationsDetail, (active) => { if (!active) void gridApi.query(); });

onBeforeUnmount(() => {
  pagination.invalidate();
  Object.values(segmentSearchTimers).forEach((timer) => clearTimeout(timer));
});
const layoutRoot = useAdminGridLayout(gridApi);
</script>

<template>
  <Page :ref="layoutRoot" class="plugin-admin-page" :auto-content-height="true" data-testid="customer-management-page">
    <OperationsDetailPanel v-if="operationsDetail" :can-open="canViewOverview" @open="openOperationsCustomer($event.id)" />
    <div v-show="!operationsDetail" class="customer-shell">
      <div class="customer-grid">
        <Grid>
          <template #toolbar-actions>
            <div class="customer-toolbar-actions">
              <a-button
                v-if="canCreateCustomer"
                data-testid="customer-direct-open"
                type="primary"
                @click="openDirectCustomer"
              >
                {{ directText("open") }}
              </a-button>
              <a-button
                v-if="canBatchTags"
                data-testid="customer-tag-batch-open"
                :disabled="selectedCustomerCount === 0"
                @click="openTagBatch"
              >
                {{ $t("plugin.idc-core.customer.actions.batchTags") }}
              </a-button>
            </div>
          </template>

          <template #type="{ row }">
            <Tag>{{ $t(`plugin.idc-core.customer.type.${row.type}`) }}</Tag>
          </template>

          <template #status="{ row }">
            <Badge
              :status="statusBadge(row.status)"
              :text="$t(`plugin.idc-core.customer.status.${row.status}`)"
            />
          </template>

          <template #verification="{ row }">
            <Tag :color="verificationColor(row.verification)">
              {{
                $t(`plugin.idc-core.customer.verification.${row.verification}`)
              }}
            </Tag>
          </template>

          <template #level="{ row }">
            <span class="level-name">{{ localizedLevelName(row) }}</span>
          </template>

          <template #account="{ row }">
            <div class="account-lines"><span v-if="row.primaryPhone">{{ row.primaryPhone }}</span><span v-if="row.primaryEmail">{{ row.primaryEmail }}</span><span v-if="!row.primaryPhone && !row.primaryEmail">—</span></div>
          </template>

          <template #wallet="{ row }">
            <div class="wallet-lines" data-testid="customer-wallet">
              <div v-for="balance in row.balances || []" :key="`${balance.accountTypeCode}-${balance.currencyCode}`">
                <small>{{ $t(`plugin.idc-core.customer.workspace.walletTypes.${["customer_balance","customer_credit","customer_gift","customer_commission"].includes(balance.accountTypeCode)?balance.accountTypeCode:"customer_balance"}`) }}</small>
                <strong>{{ balance.currencyCode }} {{ minorToMajorAmount(balance.availableBalanceMinor, balance.currencyCode) }}</strong>
                <small v-if="balance.heldBalanceMinor !== '0'">{{ $t('plugin.idc-core.customer.workspace.held') }} {{ minorToMajorAmount(balance.heldBalanceMinor, balance.currencyCode) }}</small>
              </div>
              <span v-if="!row.balances?.length">—</span>
            </div>
          </template>

          <template #action="{ row }">
            <Space>
              <ghost-button v-if="canAssist && row.status === 'active'" :data-testid="`customer-assist-${row.customerNo}`" @click="assistedOrderDrawerApi.setData(row).open()">{{ $t("plugin.idc-core.assistedCreate.title") }}</ghost-button>
              <ghost-button
                v-if="canViewOverview"
                :data-testid="`customer-overview-${row.customerNo}`"
                @click="openOverview(row)"
              >
                {{ $t("plugin.idc-core.customer.actions.overview") }}
              </ghost-button>
              <a-button
                v-if="canResetPassword && row.status === 'active'"
                :data-testid="`customer-password-reset-${row.customerNo}`"
                size="small"
                type="link"
                @click="openPasswordReset(row)"
              >
                {{ $t("plugin.idc-core.customer.password.action") }}
              </a-button>
            </Space>
          </template>
        </Grid>
      </div>
    </div>
    <AssistedOrderDrawer />
    <CustomerOverviewDrawerRef @updated="gridApi.query()" />
    <CustomerTagBatchModalRef @success="handleTagBatchSuccess" />
    <DirectModal>
      <p class="mb-5 text-muted-foreground">{{ directText('description') }}</p>
      <DirectCustomerForm />
    </DirectModal>
  </Page>
</template>

<style scoped>
.customer-shell {
  display: grid;
  grid-template-rows: minmax(0, 1fr);
  height: 100%;
  max-width: 100%;
  min-height: 0;
  min-width: 0;
  padding: 16px;
}

.page-indicator {
  color: hsl(var(--muted-foreground));
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.customer-grid {
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}

.customer-grid :deep(.vxe-grid) {
  max-width: 100%;
  min-width: 0;
  width: 100% !important;
}

.activation-grid {
  display: grid;
  gap: 0 16px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.activation-help {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  margin: 8px 0 0 24px;
}

@media (width <= 640px) {
  .customer-shell {
    padding: 16px;
  }

  .activation-grid {
    grid-template-columns: 1fr;
  }
}

.level-name {
  font-weight: 600;
}

.tag-count {
  background: hsl(var(--muted));
  border-radius: 999px;
  display: inline-grid;
  font-variant-numeric: tabular-nums;
  min-width: 28px;
  padding: 2px 8px;
  place-items: center;
}
</style>

<style>
.idc-plugin-form-surface .ant-form-item-label > label {
  align-items: center !important;
  display: inline-flex !important;
  flex-wrap: nowrap !important;
  white-space: nowrap;
}

.idc-plugin-form-surface .ant-form-item-required::before {
  flex: 0 0 auto;
}
</style>

<style scoped>
.account-lines { display: grid; gap: 4px; line-height: 1.4; padding: 6px 0; }
</style>

<style scoped>
.customer-toolbar-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.customer-grid :deep(.vxe-toolbar) { display: flex; align-items: center; gap: 8px; min-height: 48px; padding-block: 8px; }
.customer-grid :deep(.vxe-buttons--wrapper), .customer-grid :deep(.vxe-tools--wrapper) { display: flex; align-items: center; padding-block: 0; }
</style>

<style scoped>
.wallet-lines { display: flex; flex-direction: column; gap: 6px; padding: 6px 0; }
.wallet-lines strong { font-variant-numeric: tabular-nums; font-weight: 500; }
.wallet-lines small { display: block; color: hsl(var(--muted-foreground)); }
</style>
