<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/channel",
  title: "Downstream Channel Management",
};
</script>
<script setup lang="ts">
import type { VbenFormSchema } from "#/adapter/form";
import type { VxeGridProps } from "#/adapter/vxe-table";
import { computed, onMounted, ref, watch } from "vue";
import { Page } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Empty,
  Tabs,
  TabPane,
} from "ant-design-vue";
import { useVbenForm } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { pluginApiPath, requestClient } from "#/api/request";
import { channelText as t } from "./channel-i18n";
import { requestPreservingHttpStatus } from "./channel-request";
import ChannelPurchaseValue from "./components/channel-purchase-value.vue";
import {
  serializePurchaseValues,
  type PurchaseValueDraft,
} from "./channel-purchase-values";

type Row = Record<string, unknown>;
type Panel =
  | "accounts"
  | "publications"
  | "tokens"
  | "finance"
  | "health"
  | "capabilities";
type GridKey =
  | "accounts"
  | "publications"
  | "tokens"
  | "invoices"
  | "statements"
  | "repayments"
  | "audit"
  | "capabilities";
interface Channel extends Row {
  channelId: string;
  displayName: string;
  customerNo: string;
}
interface Finance {
  currencies: Row[];
  invoices: Row[];
  creditStatements: Row[];
  creditRepayments: Row[];
  invoiceTotal: number;
  statementTotal: number;
  repaymentTotal: number;
}
type PageRequest = { currentPage: number; pageSize: number };
type ListPage<T = Row> = { list: T[]; total: number };
const activePanel = ref<Panel>("accounts");
const selectedChannel = ref("");
const channels = ref<Channel[]>([]);
const busy = ref(false);
const notice = ref("");
const failed = ref(false);
const secret = ref("");
const currencies = ref<Row[]>([]);
const health = ref<Row>({});
const purchaseValues = ref<Array<PurchaseValueDraft & { key: number }>>([]);
let valueSequence = 0;
let loadSequence = 0;
const request = <T,>(
  path: string,
  options: {
    method?: string;
    body?: unknown;
    params?: Record<string, unknown>;
  } = {},
) =>
  requestPreservingHttpStatus<T>(requestClient, pluginApiPath("idc-channel", path), {
    method: options.method || "GET",
    data: options.body,
    params: options.params,
  });
const money = (value: unknown, currency: unknown = "") =>
  (currency ? String(currency) + " " : "") +
  (Number(value || 0) / 100).toFixed(2);
const date = (value: unknown) =>
  value ? new Date(Number(value)).toLocaleString(preferences.app.locale) : "—";
const lines = (value: unknown) =>
  String(value || "")
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);
const stateKeys = new Set([
  "pending_approval",
  "approved",
  "paused",
  "closed",
  "published",
  "active",
  "revoked",
  "enabled",
  "enabled_core_guarded",
  "disabled_unverified",
  "generic_api_v1",
  "accepted",
  "ok",
  "succeeded",
  "failed",
  "pending",
  "paid",
  "unpaid",
  "partially_paid",
  "cancelled",
  "completed",
  "issued",
  "settled",
  "overdue",
]);
const stateText = (value: unknown) =>
  stateKeys.has(String(value))
    ? t("state." + String(value))
    : String(value || "—");
const column = (
  field: string,
  title = field,
  width?: number,
): NonNullable<VxeGridProps["columns"]>[number] => ({
  field,
  title: t(title),
  minWidth: width || 180,
  ...(width ? { width } : {}),
  align: width ? "center" : "left",
  showOverflow: "tooltip",
});
const timeColumn = (field: string, title = field) => ({
  ...column(field, title, 180),
  formatter: ({ cellValue }: { cellValue: unknown }) => date(cellValue),
});
const statusColumn = (field = "status") => ({
  ...column(field, "status", 170),
  formatter: ({ cellValue }: { cellValue: unknown }) => stateText(cellValue),
});
const amountColumn = (field: string, title: string, currency = "") => ({
  ...column(field, title, 145),
  formatter: ({ row }: { row: Row }) => money(row[field], row[currency]),
});
function columns(key: GridKey): VxeGridProps["columns"] {
  switch (key) {
    case "accounts":
      return [
        {
          ...column("displayName", "channels"),
          formatter: ({ row }: { row: Row }) =>
            String(row.displayName) + " (" + String(row.channelId) + ")",
        },
        column("customerNo", "customerNo", 190),
        statusColumn("statusCode"),
        {
          ...column("newSalesEnabled", "newSalesEnabled", 120),
          formatter: ({ cellValue }: { cellValue: unknown }) =>
            t(cellValue ? "yes" : "no"),
        },
        timeColumn("updatedAt"),
      ];
    case "publications":
      return [
        column("productVersionId"),
        column("currency", "currency", 100),
        amountColumn("priceMinor", "price", "currency"),
        column("stock", "stock", 100),
        statusColumn(),
        column("publicSlug", "publicSlug", 220),
      ];
    case "tokens":
      return [
        {
          ...column("scopes"),
          formatter: ({ row }: { row: Row }) =>
            ((row.scopes as string[]) || []).join(", "),
        },
        {
          ...column("sourceCidrs", "sourceCidrs", 220),
          formatter: ({ row }: { row: Row }) =>
            ((row.sourceCidrs as string[]) || []).join(", ") ||
            t("unrestricted"),
        },
        timeColumn("expiresAt"),
        {
          ...statusColumn(),
          formatter: ({ row }: { row: Row }) =>
            t(row.revokedAt ? "state.revoked" : "state.active"),
        },
        {
          ...column("actions", "actions", 100),
          fixed: "right",
          slots: { default: "actions" },
        },
      ];
    case "invoices":
      return [
        column("invoiceNo"),
        column("orderNo", "orderNo", 200),
        amountColumn("totalAmountMinor", "amount", "currencyCode"),
        amountColumn("paidAmountMinor", "balancePaid"),
        amountColumn("creditAmountMinor", "creditSettled"),
        amountColumn("refundedAmountMinor", "refunded"),
        statusColumn(),
        timeColumn("createdAt"),
      ];
    case "statements":
      return [
        column("statementNo"),
        amountColumn("totalMinor", "amount", "currencyCode"),
        amountColumn("outstandingMinor", "outstanding"),
        statusColumn(),
        timeColumn("paymentDueAt", "expiresAt"),
      ];
    case "repayments":
      return [
        column("repaymentNo"),
        column("statementNo", "statements", 190),
        amountColumn("amountMinor", "amount", "currencyCode"),
        statusColumn(),
        timeColumn("createdAt"),
      ];
    case "audit":
      return [
        column("requestId"),
        column("operation", "operation", 170),
        statusColumn(),
        {
          ...column("resultCode", "resultCode", 160),
          formatter: ({ cellValue }: { cellValue: unknown }) =>
            stateText(cellValue),
        },
        timeColumn("createdAt"),
        timeColumn("completedAt"),
      ];
    case "capabilities":
      return [column("type", "type", 170), column("name"), statusColumn()];
  }
}
// Only the fixed capability metadata is paged locally; business totals come from owners.
function createGrid(key: GridKey) {
  let rows: Row[] = [];
  let page = 1;
  let pageSize = 10;
  let total = 0;
  const [Grid, api] = useVbenVxeGrid({
    gridOptions: {
      columns: [],
      data: [],
      height: 390,
      border: true,
      rowConfig: { keyField: "_rowKey" },
      pagerConfig: { enabled: true, pageSize, total: 0 },
      toolbarConfig: { enabled: false },
    },
    gridEvents: {
      pageChange: ({
        currentPage,
        pageSize: size,
      }: {
        currentPage: number;
        pageSize: number;
      }) => {
        if (key === "capabilities") {
          page = size === pageSize ? currentPage : 1;
          pageSize = size;
          render();
        } else {
          render();
          void loadPanel({
            key,
            currentPage: size === pageSize ? currentPage : 1,
            pageSize: size,
          });
        }
      },
    },
  });
  function render() {
    api.setGridOptions({
      data:
        key === "capabilities"
          ? rows.slice((page - 1) * pageSize, page * pageSize)
          : rows,
      pagerConfig: {
        enabled: true,
        currentPage: page,
        pageSize,
        total,
      },
    });
  }
  function update(items: Row[], count?: number, requested?: PageRequest) {
    const confirmedTotal = key === "capabilities" ? items.length : count;
    if (
      confirmedTotal === undefined ||
      !Number.isSafeInteger(confirmedTotal) ||
      confirmedTotal < 0
    )
      throw new Error("Invalid channel page total");
    rows = items.map((row, index) => ({ ...row, _rowKey: key + "-" + index }));
    total = confirmedTotal;
    if (requested) {
      page = requested.currentPage;
      pageSize = requested.pageSize;
    }
    render();
  }
  function reset() {
    page = 1;
    update([], 0);
  }
  return {
    Grid,
    api,
    update,
    reset,
    pagination: (): PageRequest => ({ currentPage: page, pageSize }),
  };
}
const accountGrid = createGrid("accounts");
const publicationGrid = createGrid("publications");
const tokenGrid = createGrid("tokens");
const invoiceGrid = createGrid("invoices");
const statementGrid = createGrid("statements");
const repaymentGrid = createGrid("repayments");
const auditGrid = createGrid("audit");
const capabilityGrid = createGrid("capabilities");
const grids = {
  accounts: accountGrid,
  publications: publicationGrid,
  tokens: tokenGrid,
  invoices: invoiceGrid,
  statements: statementGrid,
  repayments: repaymentGrid,
  audit: auditGrid,
  capabilities: capabilityGrid,
};
const input = (
  fieldName: string,
  label = fieldName,
  component: VbenFormSchema["component"] = "Input",
  defaultValue?: unknown,
): VbenFormSchema => ({
  fieldName,
  label: t(label),
  component,
  defaultValue,
  componentProps: { class: "w-full", name: fieldName },
});
function selectorSchema(): VbenFormSchema[] {
  return [
    {
      fieldName: "channelId",
      component: "Select",
      label: t("channels"),
      componentProps: {
        placeholder: t("selectChannel"),
        showSearch: true,
        filterOption: false,
        onSearch: (keyword: string) => {
          void searchChannels(keyword).catch(() => {
            failed.value = true;
            notice.value = t("loadFailed");
          });
        },
        options: channels.value.map((item) => ({
          label: item.displayName + " · " + item.channelId,
          value: item.channelId,
        })),
        "data-testid": "channel-selector",
        class: "w-full",
      },
    },
  ];
}
function accountSchema() {
  return [
    input("channelId"),
    input("customerNo", "enterpriseCustomer"),
    input("displayName"),
  ];
}
function publicationSchema(): VbenFormSchema[] {
  return [
    input("productVersionId", "productVersionId", "InputNumber"),
    input("publicSlug"),
    input("configurationRevision"),
    input("billingTermToken"),
    input("currency", "currency", "Input", "CNY"),
    input("purchaseCycles", "purchaseCycles", "InputNumber", 1),
    input("quantity", "quantity", "InputNumber", 1),
    input("stock", "stock", "InputNumber", 0),
    {
      ...input("status", "status", "Select", "published"),
      componentProps: {
        options: ["published", "paused"].map((value) => ({
          label: t("state." + value),
          value,
        })),
      },
    },
  ];
}
function tokenSchema(): VbenFormSchema[] {
  return [
    {
      ...input(
        "scopes",
        "scopesInput",
        "Textarea",
        "catalog.read\norder.create\norder.read\nservice.read\nsettlement.read",
      ),
      componentProps: { rows: 5, name: "scopes" },
    },
    {
      ...input("sourceCidrs", "sourceInput", "Textarea"),
      componentProps: { rows: 5, name: "sourceCidrs" },
    },
    {
      ...input("expiresAt", "expiresInput"),
      componentProps: { type: "datetime-local", name: "expiresAt" },
    },
  ];
}
const formOptions = {
  showDefaultActions: false,
  layout: "vertical" as const,
  wrapperClass: "grid-cols-1 md:grid-cols-2",
};
const [SelectorForm, selectorApi] = useVbenForm({
  ...formOptions,
  wrapperClass: "grid-cols-1",
  schema: [],
  handleValuesChange: (values) => {
    selectedChannel.value = String(values.channelId || "");
  },
});
const [AccountForm, accountApi] = useVbenForm({ ...formOptions, schema: [] });
const [PublicationForm, publicationApi] = useVbenForm({
  ...formOptions,
  schema: [],
});
const [TokenForm, tokenApi] = useVbenForm({ ...formOptions, schema: [] });
function refreshLabels() {
  for (const [key, grid] of Object.entries(grids))
    grid.api.setGridOptions({ columns: columns(key as GridKey) });
  selectorApi.setState({ schema: selectorSchema() });
  accountApi.setState({ schema: accountSchema() });
  publicationApi.setState({ schema: publicationSchema() });
  tokenApi.setState({ schema: tokenSchema() });
}
let channelSearchSequence = 0;
async function searchChannels(keyword = "") {
  const sequence = ++channelSearchSequence;
  const data = await request<ListPage<Channel>>("admin/channel/accounts", {
    params: { page: 1, pageSize: 100, keyword },
  });
  if (sequence !== channelSearchSequence) return;
  const selected = channels.value.find(
    (item) => item.channelId === selectedChannel.value,
  );
  channels.value = data.list || [];
  if (
    selected &&
    !channels.value.some((item) => item.channelId === selected.channelId)
  )
    channels.value.unshift(selected);
  selectorApi.setState({ schema: selectorSchema() });
}
async function loadAccounts() {
  await loadPanel();
}
let previousContext = "";
let retryPage: (PageRequest & { key: GridKey }) | undefined;
async function loadPanel(requested = retryPage) {
  const sequence = ++loadSequence;
  const panel = activePanel.value;
  const channelId = selectedChannel.value;
  const context = panel + ":" + channelId;
  const contextChanged = context !== previousContext;
  if (contextChanged) {
    previousContext = context;
    requested = undefined;
    retryPage = undefined;
  }
  const paging = (key: GridKey) =>
    requested?.key === key ? requested : grids[key].pagination();
  const params = (key: GridKey) => ({
    channelId,
    page: paging(key).currentPage,
    pageSize: paging(key).pageSize,
  });
  secret.value = "";
  notice.value = "";
  failed.value = false;
  for (const grid of Object.values(grids)) grid.api.setLoading(false);
  // Never leave the previous channel's rows under a new selector or failed read.
  if (contextChanged && panel === "publications") publicationGrid.reset();
  if (contextChanged && panel === "tokens") tokenGrid.reset();
  if (contextChanged && panel === "finance") {
    currencies.value = [];
    invoiceGrid.reset();
    statementGrid.reset();
    repaymentGrid.reset();
  }
  if (contextChanged && panel === "health") {
    health.value = {};
    auditGrid.reset();
  }
  if (!["accounts", "capabilities"].includes(panel) && !channelId) return;
  for (const grid of Object.values(grids)) grid.api.setLoading(true);
  try {
    if (panel === "accounts") {
      const data = await request<ListPage<Channel>>("admin/channel/accounts", {
        params: params("accounts"),
      });
      if (sequence !== loadSequence) return;
      accountGrid.update(data.list || [], data.total, paging("accounts"));
      await searchChannels();
    }
    if (panel === "capabilities") {
      const data = await request<{
        operations: Record<string, string>;
        adapters: Record<string, string>;
      }>("admin/channel/capabilities");
      if (sequence !== loadSequence) return;
      capabilityGrid.update([
        ...Object.entries(data.operations || {}).map(([name, status]) => ({
          type: t("genericApi"),
          name,
          status,
        })),
        ...Object.entries(data.adapters || {}).map(([name, status]) => ({
          type: t("adapter"),
          name,
          status,
        })),
      ]);
    }
    if (panel === "publications" || panel === "tokens") {
      const data = await request<ListPage>("admin/channel/" + panel, {
        params: params(panel),
      });
      if (sequence === loadSequence)
        grids[panel].update(data.list || [], data.total, paging(panel));
    }
    if (panel === "finance") {
      const data = await request<Finance>("admin/channel/finance", {
        params: {
          channelId,
          invoicePage: paging("invoices").currentPage,
          invoicePageSize: paging("invoices").pageSize,
          statementPage: paging("statements").currentPage,
          statementPageSize: paging("statements").pageSize,
          repaymentPage: paging("repayments").currentPage,
          repaymentPageSize: paging("repayments").pageSize,
        },
      });
      if (sequence !== loadSequence) return;
      currencies.value = data.currencies || [];
      invoiceGrid.update(
        data.invoices || [],
        data.invoiceTotal,
        paging("invoices"),
      );
      statementGrid.update(
        data.creditStatements || [],
        data.statementTotal,
        paging("statements"),
      );
      repaymentGrid.update(
        data.creditRepayments || [],
        data.repaymentTotal,
        paging("repayments"),
      );
    }
    if (panel === "health") {
      const [summary, audit] = await Promise.all([
        request<Row>("admin/channel/health", { params: { channelId } }),
        request<ListPage>("admin/channel/requests", {
          params: params("audit"),
        }),
      ]);
      if (sequence !== loadSequence) return;
      health.value = summary;
      auditGrid.update(audit.list || [], audit.total, paging("audit"));
    }
    if (sequence === loadSequence) retryPage = undefined;
  } catch (error) {
    if (sequence === loadSequence) {
      clearDeniedData(error, panel);
      failed.value = true;
      retryPage = requested;
      notice.value = t("loadFailed");
    }
  } finally {
    if (sequence === loadSequence)
      for (const grid of Object.values(grids)) grid.api.setLoading(false);
  }
}
function clearDeniedData(error: unknown, panel: Panel) {
  const status = Number(
    (error as { response?: { status?: number } })?.response?.status,
  );
  if (status !== 401 && status !== 403) return;
  secret.value = "";
  if (panel === "accounts") accountGrid.reset();
  if (panel === "publications") publicationGrid.reset();
  if (panel === "tokens") tokenGrid.reset();
  if (panel === "finance") {
    currencies.value = [];
    invoiceGrid.reset();
    statementGrid.reset();
    repaymentGrid.reset();
  }
  if (panel === "health") {
    health.value = {};
    auditGrid.reset();
  }
  if (panel === "capabilities") capabilityGrid.reset();
}
async function perform(work: () => Promise<void>, success: string) {
  if (busy.value) return;
  const panel = activePanel.value;
  const channelId = selectedChannel.value;
  const stillCurrent = () =>
    panel === activePanel.value && channelId === selectedChannel.value;
  busy.value = true;
  notice.value = "";
  failed.value = false;
  try {
    await work();
    if (stillCurrent()) notice.value = t(success);
  } catch (error) {
    if (stillCurrent()) {
      clearDeniedData(error, panel);
      failed.value = true;
      notice.value = t("saveFailed");
    }
  } finally {
    busy.value = false;
  }
}
async function saveChannel() {
  const values = await accountApi.getValues();
  await perform(async () => {
    await request("admin/channel/accounts", {
      method: "POST",
      body: {
        channelId: String(values.channelId || "").trim(),
        customerNo: String(values.customerNo || "").trim(),
        displayName: String(values.displayName || "").trim(),
      },
    });
    await loadAccounts();
  }, "saved");
}
async function changeStatus(action: string) {
  if (!selectedChannel.value) {
    failed.value = true;
    notice.value = t("selectFirst");
    return;
  }
  const channelId = selectedChannel.value;
  await perform(async () => {
    await request(
      "admin/channel/accounts/" + encodeURIComponent(channelId) + "/status",
      { method: "POST", body: { action } },
    );
    await loadAccounts();
  }, "statusChanged");
}
async function savePublication() {
  if (!selectedChannel.value) return;
  const channelId = selectedChannel.value;
  const values = await publicationApi.getValues();
  await perform(async () => {
    await request("admin/channel/publications", {
      method: "POST",
      body: {
        channelId,
        productVersionId: Number(values.productVersionId),
        publicSlug: String(values.publicSlug || "").trim(),
        configurationRevision: String(
          values.configurationRevision || "",
        ).trim(),
        billingTermToken: String(values.billingTermToken || "").trim(),
        currency: String(values.currency || "")
          .trim()
          .toUpperCase(),
        purchaseCycles: Number(values.purchaseCycles),
        quantity: Number(values.quantity),
        stock: Number(values.stock),
        status: values.status,
        values: serializePurchaseValues(purchaseValues.value),
      },
    });
    if (
      selectedChannel.value === channelId &&
      activePanel.value === "publications"
    )
      await loadPanel();
  }, "publicationSaved");
}
async function createToken() {
  if (!selectedChannel.value) return;
  const channelId = selectedChannel.value;
  const values = await tokenApi.getValues();
  const stillCurrent = () =>
    selectedChannel.value === channelId && activePanel.value === "tokens";
  await perform(async () => {
    const result = await request<{ token: string }>("admin/channel/tokens", {
      method: "POST",
      body: {
        channelId,
        scopes: lines(values.scopes),
        sourceCidrs: lines(values.sourceCidrs),
        expiresAt: values.expiresAt
          ? new Date(String(values.expiresAt)).getTime()
          : undefined,
      },
    });
    if (!stillCurrent()) return;
    secret.value = result.token || "";
    const paging = tokenGrid.pagination();
    const sequence = loadSequence;
    const data = await request<ListPage>("admin/channel/tokens", {
      params: {
        channelId,
        page: paging.currentPage,
        pageSize: paging.pageSize,
      },
    });
    if (stillCurrent() && sequence === loadSequence)
      tokenGrid.update(data.list || [], data.total, paging);
  }, "tokenCreated");
}
async function revokeToken(row: Row) {
  const channelId = selectedChannel.value;
  const stillCurrent = () =>
    selectedChannel.value === channelId && activePanel.value === "tokens";
  await perform(async () => {
    await request(
      "admin/channel/tokens/" + encodeURIComponent(String(row.id)),
      { method: "DELETE" },
    );
    if (!stillCurrent()) return;
    const paging = tokenGrid.pagination();
    const sequence = loadSequence;
    const data = await request<ListPage>("admin/channel/tokens", {
      params: {
        channelId,
        page: paging.currentPage,
        pageSize: paging.pageSize,
      },
    });
    if (stillCurrent() && sequence === loadSequence)
      tokenGrid.update(data.list || [], data.total, paging);
  }, "tokenRevoked");
}
const needsChannel = computed(
  () =>
    !["accounts", "capabilities"].includes(activePanel.value) &&
    !selectedChannel.value,
);
watch([activePanel, selectedChannel], () => {
  void loadPanel();
});
watch(
  () => preferences.app.locale,
  () => {
    refreshLabels();
    void loadPanel();
  },
);
onMounted(() => {
  refreshLabels();
  void loadPanel();
});
</script>
<template>
  <Page :auto-content-height="true">
    <div class="channel-page" data-testid="idc-channel-admin-page">
      <Card :title="t('title')">
        <template #extra
          ><Button
            :loading="busy"
            data-testid="refresh-channel"
            @click="loadPanel()"
            >{{ t("refresh") }}</Button
          ></template
        >
        <div class="channel-selector"><SelectorForm /></div>
        <Alert
          v-if="notice"
          class="mb-4"
          :message="notice"
          :type="failed ? 'error' : 'success'"
          show-icon
          role="status"
        />
        <Tabs v-model:active-key="activePanel">
          <TabPane key="accounts" :tab="t('channels')">
            <div class="channel-sections">
              <section data-panel="accounts">
                <p class="channel-hint">{{ t("accountHint") }}</p>
                <component
                  :is="accountGrid.Grid"
                  :table-title="t('accounts')"
                />
              </section>
              <div class="channel-form-columns">
                <section>
                  <h3>{{ t("createChannel") }}</h3>
                  <p class="channel-hint">{{ t("createHint") }}</p>
                  <AccountForm /><Button
                    type="primary"
                    :loading="busy"
                    data-testid="save-channel"
                    @click="saveChannel"
                    >{{ t("saveChannel") }}</Button
                  >
                </section>
                <section>
                  <h3>{{ t("partnership") }}</h3>
                  <p class="channel-hint">{{ t("partnershipHint") }}</p>
                  <div class="channel-actions">
                    <Button
                      v-for="action in ['approve', 'pause', 'resume', 'close']"
                      :key="action"
                      :danger="action === 'close'"
                      :type="action === 'approve' ? 'primary' : 'default'"
                      :disabled="busy"
                      @click="changeStatus(action)"
                      >{{ t(action) }}</Button
                    >
                  </div>
                </section>
              </div>
            </div>
          </TabPane>
          <TabPane key="publications" :tab="t('publications')">
            <Alert
              v-if="needsChannel"
              :message="t('selectFirst')"
              type="info"
              show-icon
            />
            <div v-else class="channel-sections" data-panel="publications">
              <section>
                <p class="channel-hint">{{ t("publicationHint") }}</p>
                <component
                  :is="publicationGrid.Grid"
                  :table-title="t('publishedProducts')"
                />
              </section>
              <section>
                <h3>{{ t("editPublication") }}</h3>
                <p class="channel-hint">{{ t("publicationFormHint") }}</p>
                <PublicationForm />
                <h3>{{ t("values") }}</h3>
                <p class="channel-hint">{{ t("valuesHint") }}</p>
                <ChannelPurchaseValue
                  v-for="item in purchaseValues"
                  :key="item.key"
                  :model-value="item"
                  @update:model-value="Object.assign(item, $event)"
                  @remove="
                    purchaseValues = purchaseValues.filter(
                      (value) => value.key !== item.key,
                    )
                  "
                />
                <div class="channel-actions">
                  <Button
                    data-testid="add-publication-value"
                    @click="
                      purchaseValues.push({
                        key: ++valueSequence,
                        fieldToken: '',
                        fieldType: 'single_line',
                        fieldValue: '',
                      })
                    "
                    >{{ t("addValue") }}</Button
                  ><Button
                    type="primary"
                    :loading="busy"
                    data-testid="save-publication"
                    @click="savePublication"
                    >{{ t("savePublication") }}</Button
                  >
                </div>
              </section>
            </div>
          </TabPane>
          <TabPane key="tokens" :tab="t('tokens')">
            <Alert
              v-if="needsChannel"
              :message="t('selectFirst')"
              type="info"
              show-icon
            />
            <div v-else class="channel-sections" data-panel="tokens">
              <section>
                <p class="channel-hint">{{ t("tokenHint") }}</p>
                <component :is="tokenGrid.Grid" :table-title="t('tokenList')"
                  ><template #actions="{ row }"
                    ><Button
                      type="link"
                      danger
                      :disabled="busy || Boolean(row.revokedAt)"
                      @click="revokeToken(row)"
                      >{{ t("revoke") }}</Button
                    ></template
                  ></component
                >
              </section>
              <section>
                <h3>{{ t("createToken") }}</h3>
                <TokenForm /><Button
                  type="primary"
                  :loading="busy"
                  @click="createToken"
                  >{{ t("createOnce") }}</Button
                >
                <p class="channel-secret" data-testid="one-time-token">
                  {{ secret || t("secretHint") }}
                </p>
              </section>
            </div>
          </TabPane>
          <TabPane key="finance" :tab="t('finance')">
            <Alert
              v-if="needsChannel"
              :message="t('selectFirst')"
              type="info"
              show-icon
            />
            <div v-else class="channel-sections" data-panel="finance">
              <section>
                <h3>{{ t("currencyOverview") }}</h3>
                <Empty
                  v-if="!currencies.length"
                  :description="t('emptyFinance')"
                /><Descriptions
                  v-for="item in currencies"
                  :key="String(item.currencyCode)"
                  :title="String(item.currencyCode)"
                  bordered
                  size="small"
                  :column="{ xs: 1, sm: 2, lg: 3 }"
                  ><DescriptionsItem
                    v-for="[field, label] in [
                      ['availableBalanceMinor', 'balance'],
                      ['creditAvailableMinor', 'credit'],
                      ['receivableMinor', 'receivable'],
                      ['paidMinor', 'paid'],
                      ['refundedMinor', 'refunded'],
                      ['serviceCostMinor', 'cost'],
                    ]"
                    :key="field"
                    :label="t(label!)"
                    >{{
                      money(item[field!], item.currencyCode)
                    }}</DescriptionsItem
                  ></Descriptions
                >
              </section>
              <section>
                <component
                  :is="invoiceGrid.Grid"
                  :table-title="t('invoices')"
                />
              </section>
              <section>
                <component
                  :is="statementGrid.Grid"
                  :table-title="t('statements')"
                />
              </section>
              <section>
                <component
                  :is="repaymentGrid.Grid"
                  :table-title="t('repayments')"
                />
              </section>
            </div>
          </TabPane>
          <TabPane key="health" :tab="t('health')">
            <Alert
              v-if="needsChannel"
              :message="t('selectFirst')"
              type="info"
              show-icon
            />
            <div v-else class="channel-sections" data-panel="health">
              <section>
                <h3>{{ t("channelHealth") }}</h3>
                <Descriptions
                  bordered
                  size="small"
                  :column="{ xs: 1, sm: 2, lg: 3 }"
                  ><DescriptionsItem :label="t('status')">{{
                    stateText(health.statusCode)
                  }}</DescriptionsItem
                  ><DescriptionsItem :label="t('newSalesEnabled')">{{
                    t(health.newSalesEnabled ? "yes" : "no")
                  }}</DescriptionsItem
                  ><DescriptionsItem :label="t('activeTokens')">{{
                    health.activeTokenCount ?? "—"
                  }}</DescriptionsItem
                  ><DescriptionsItem :label="t('publicationCount')">{{
                    health.publicationCount ?? "—"
                  }}</DescriptionsItem
                  ><DescriptionsItem :label="t('pendingEvents')">{{
                    health.pendingWebhookCount ?? "—"
                  }}</DescriptionsItem
                  ><DescriptionsItem :label="t('lastRequest')">{{
                    date(health.lastRequestAt)
                  }}</DescriptionsItem></Descriptions
                >
              </section>
              <section>
                <component :is="auditGrid.Grid" :table-title="t('audit')" />
              </section>
            </div>
          </TabPane>
          <TabPane key="capabilities" :tab="t('capabilities')"
            ><section data-panel="capabilities">
              <p class="channel-hint">{{ t("capabilityHint") }}</p>
              <component
                :is="capabilityGrid.Grid"
                :table-title="t('verifiedCapabilities')"
              /></section
          ></TabPane>
        </Tabs>
      </Card>
    </div>
  </Page>
</template>
<style scoped>
.channel-page {
  min-width: 0;
  height: 100%;
  overflow: auto;
  background: hsl(var(--background));
}
.channel-page :deep(.ant-card-body) {
  min-width: 0;
}
.channel-selector {
  max-width: 540px;
}
.channel-sections {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 20px;
}
.channel-sections > section,
.channel-form-columns > section {
  min-width: 0;
}
.channel-form-columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
}
.channel-hint {
  margin: 0 0 16px;
  color: hsl(var(--muted-foreground));
}
.channel-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.channel-secret {
  margin: 16px 0 0;
  overflow-wrap: anywhere;
}
h3 {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 600;
}
@media (max-width: 900px) {
  .channel-form-columns {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
