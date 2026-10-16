<script lang="ts">
export const pluginPageMeta = {
  routePath: "/account/plugins/idc-core/trial-conversions",
  title: "plugin.idc-core.trials.title",
};
</script>
<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { Page, useVbenModal } from "@vben/common-ui";
import { Alert, Button, message } from "ant-design-vue";
import { preferences } from "@vben/preferences";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { useVbenForm, z } from "#/adapter/form";
import { pluginApiPath, requestClient } from "#/api/request";
import { $t } from "#/locales";
import { minorToMajorAmount } from "./catalog-money";

type Period = { billingRuleNo: string; periodUnitCode: string; periodCount: number };
type Trial = { entitlementNo: string; productNameZhCN: string; productNameEnUS: string; currencyCode: string; statusCode: string; expiresAt: number; periods: Period[] };
type Quote = { quoteNo: string; currencyCode: string; netAmountMinor: string; taxAmountMinor: string; totalAmountMinor: string; expiresAt: number };
type Operation = { operationNo: string; invoiceNo: string; statusCode: string };
const selected = ref<Trial>();
const quote = ref<Quote>();
const operation = ref<Operation>();
const busy = ref(false);
const failed = ref(false);
let quoteKey = "";
let confirmKey = "";
let requestRule = "";
const api = (path: string) => pluginApiPath("idc-core", `account/${path}`);
const text = (key: string) => $t(`plugin.idc-core.trials.${key}`);
function periodName(period: Period) {
  return `${period.periodCount} ${text(`unit_${period.periodUnitCode}`)}`;
}
function formatAmount(amount: string) {
  return quote.value ? `${quote.value.currencyCode} ${minorToMajorAmount(amount, quote.value.currencyCode)}` : "";
}
const [Grid, gridApi] = useVbenVxeGrid({ gridOptions: {
  columns: [], height: 480, rowConfig: { keyField: "entitlementNo" }, pagerConfig: {},
  proxyConfig: { ajax: { query: async ({ page }: { page: { currentPage: number; pageSize: number } }) => {
    const result = await requestClient.get<{ items: Trial[]; total: number }>(api("trial-conversions"), { params: { page: page.currentPage, pageSize: page.pageSize } });
    return { items: result.items, total: result.total };
  } } },
} });
const [Form, formApi] = useVbenForm({ schema: [], showDefaultActions: false });
const [Modal, modalApi] = useVbenModal({
  onBeforeClose: () => !busy.value,
  onConfirm: async () => {
    if (busy.value || !selected.value) return;
    busy.value = true;
    failed.value = false;
    modalApi.setState({ confirmLoading: true });
    try {
      if (!quote.value) {
        if (!(await formApi.validate()).valid) return;
        const values = await formApi.getValues<{ billingRuleNo: string }>();
        if (requestRule !== values.billingRuleNo) {
          requestRule = values.billingRuleNo;
          quoteKey = crypto.randomUUID();
          confirmKey = crypto.randomUUID();
        }
        const result = await requestClient.post<{ quote: Quote }>(api(`trial-conversions/${selected.value.entitlementNo}/quotes`), { billingRuleNo: requestRule, idempotencyKey: quoteKey });
        quote.value = result.quote;
        modalApi.setState({ confirmText: text("pay") });
        return;
      }
      if (!operation.value && quote.value.expiresAt <= Date.now()) {
        quote.value = undefined;
        requestRule = "";
        modalApi.setState({ confirmText: text("quote") });
        message.warning(text("expired"));
        return;
      }
      if (!operation.value) {
        const result = await requestClient.post<{ operation: Operation }>(api(`trial-conversion-quotes/${quote.value.quoteNo}/confirmations`), { idempotencyKey: confirmKey });
        operation.value = result.operation;
      }
      if (operation.value.statusCode !== "succeeded") {
        const result = await requestClient.post<{ operation: Operation }>(api(`trial-conversion-operations/${operation.value.operationNo}/balance-payments`));
        operation.value = result.operation;
      }
      message.success(text("paid"));
      busy.value = false;
      await modalApi.close();
      await gridApi.query();
    } catch {
      failed.value = true;
    } finally {
      busy.value = false;
      modalApi.setState({ confirmLoading: false });
    }
  },
});
async function open(row: Trial) {
  selected.value = row;
  quote.value = undefined;
  operation.value = undefined;
  failed.value = false;
  requestRule = "";
  modalApi.setState({ title: text("convert"), confirmText: text("quote") });
  modalApi.open();
  await formApi.setState({ schema: [{ fieldName: "billingRuleNo", component: "Select", label: text("period"), rules: z.string().min(1), componentProps: { options: row.periods.map(period => ({ label: periodName(period), value: period.billingRuleNo })), "data-testid": "trial-period" } }] });
  await formApi.setValues({ billingRuleNo: row.periods[0]?.billingRuleNo });
}
function updateColumns() {
  gridApi.setGridOptions({ columns: [
    { field: "productNameZhCN", title: text("product"), minWidth: 220, formatter: ({ row }: { row: Trial }) => preferences.app.locale === "zh-CN" ? row.productNameZhCN : row.productNameEnUS },
    { field: "entitlementNo", title: text("service"), width: 260 },
    { field: "statusCode", title: text("status"), width: 120, formatter: ({ cellValue }: { cellValue: string }) => text(cellValue === "active" ? "active" : "suspended") },
    { field: "expiresAt", title: text("expiry"), width: 190, formatter: ({ cellValue }: { cellValue: number }) => cellValue ? new Date(cellValue).toLocaleString() : "—" },
    { field: "actions", title: text("actions"), width: 190, fixed: "right", slots: { default: "actions" } },
  ] });
}
onMounted(() => {
  updateColumns();
});
watch(() => preferences.app.locale, updateColumns);
</script>
<template>
  <Page :title="text('title')">
    <Alert class="mb-4" type="info" :message="text('hint')" show-icon />
    <Grid>
      <template #actions="{ row }">
        <ghost-button v-if="row.periods.length" data-testid="trial-convert" @click="open(row)">{{ text('convert') }}</ghost-button>
        <span v-else class="text-muted-foreground">{{ text('contact') }}</span>
      </template>
      <template #toolbar-tools><Button @click="gridApi.query()">{{ text('refresh') }}</Button></template>
    </Grid>
    <Modal class="w-[480px]">
      <Alert v-if="failed" class="mb-4" type="error" :message="text('retry')" show-icon />
      <template v-if="quote">
        <dl class="space-y-3" data-testid="trial-quote">
          <div>{{ text('net') }}：{{ formatAmount(quote.netAmountMinor) }}</div>
          <div>{{ text('tax') }}：{{ formatAmount(quote.taxAmountMinor) }}</div>
          <div class="text-lg font-semibold">{{ text('total') }}：{{ formatAmount(quote.totalAmountMinor) }}</div>
        </dl>
        <Alert class="mt-4" type="info" :message="text('paymentHint')" show-icon />
      </template>
      <Form v-else />
    </Modal>
  </Page>
</template>
