<script setup lang="ts">
import type { CatalogBillingRule, CatalogPrice } from "../catalog-client";
import { computed, onMounted, reactive, ref, watch } from "vue";
import { preferences } from "@vben/preferences";
import { useVbenModal } from "@vben/common-ui";
import { useVbenForm } from "#/adapter/form";
import {
  Alert,
  Button,
  Empty,
  Input,
  Popconfirm,
  Select,
  Switch,
  message,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { catalogConfigurationPriceBatchSave } from "../catalog-client";
import { currencyOptions } from "../currency-client";
import { majorToMinorAmount, minorToMajorAmount } from "../catalog-money";

interface Row {
  id: string;
  rule: CatalogBillingRule;
  currency: string;
  priceNo: string;
  amount: string;
  setup: string;
  renewal: string;
  enabled: boolean;
  minimum: number;
  advanced: boolean;
  conflict: boolean;
  originalEdit: string;
  storedSignature: string;
  ruleSignature: string;
}
const props = defineProps<{
  versionNo: string;
  billingRules: CatalogBillingRule[];
  prices: CatalogPrice[];
  writable: boolean;
  externalSave?: boolean;
  refresh: () => Promise<void>;
}>();
const emit = defineEmits<{ busy: [value: boolean] }>();
const edits = reactive<Record<string, Row>>({});
const currency = ref("all");
const globalCurrencies = ref<string[]>([]);
const currenciesFailed = ref(false);
const copySource = ref("");
const copyTargets = ref<string[]>([]);
const saving = ref(false);
const refreshing = ref(false);
const refreshPending = ref(false);
const failure = ref("");
let editingVersion = "";
const visitedCurrencies = new Set<string>();
const tr = (key: string, params: Record<string, string | number> = {}) =>
  $t(`plugin.idc-core.catalog.workspace.${key}`, params);
const overviewTr = (key: string, params: Record<string, string | number> = {}) =>
  $t(`plugin.idc-core.catalog.priceOverview.${key}`, params);
const eligibleRules = computed(() =>
  props.billingRules.filter(
    (rule) =>
      rule.status === "active" &&
      rule.quantityMode === "fixed" &&
      ["one_time", "recurring", "trial"].includes(rule.chargeType) &&
      ["new_purchase", "both"].includes(rule.pricePhase),
  ),
);
const availableCurrencies = computed(() => [
  ...new Set([
    ...globalCurrencies.value,
    ...props.prices
      .filter((price) => !price.configurationPriceDimensionNo)
      .map((price) => price.currencyCode),
  ]),
]);
const hasConflict = computed(() =>
  Object.values(edits).some((row) => row.conflict),
);
function editSignature(row: Row) {
  const money = (value: string) => {
    try {
      return majorToMinorAmount(value, row.currency);
    } catch {
      return `invalid:${value}`;
    }
  };
  return JSON.stringify([
    money(row.amount),
    money(row.setup),
    row.rule.pricePhase === "new_purchase" ? "" : money(row.renewal),
    row.enabled,
  ]);
}
const changedRows = computed(() =>
  Object.values(edits).filter(
    (row) => !row.advanced && editSignature(row) !== row.originalEdit,
  ),
);
const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: [],
    data: [],
    height: 360,
    pagerConfig: { enabled: false },
    toolbarConfig: { enabled: false },
    rowConfig: { keyField: "id" },
  },
});
function periodLabel(rule: CatalogBillingRule) {
  if (rule.periodUnit === "once")
    return $t("plugin.idc-core.catalog.detail.code.period.once");
  const period = `${rule.periodCount} ${$t(`plugin.idc-core.catalog.detail.code.period.${rule.periodUnit}`)}`;
  return rule.chargeType === "trial"
    ? `${$t("plugin.idc-core.catalog.detail.code.charge.trial")} · ${period}`
    : period;
}
function columns() {
  return [
    { field: "currency", title: $t("plugin.idc-core.catalog.configurationPrice.currency"), width: 100 },
    {
      field: "period",
      title: tr("billingPeriodColumn"),
      minWidth: 160,
      formatter: ({ row }: { row: Row }) => periodLabel(row.rule),
    },
    ...(["amount", "renewal", "setup"] as const).map((field) => ({
      field,
      title: tr(`basePrice_${field}`),
      width: 160,
      slots: { default: field },
    })),
    {
      field: "enabled",
      title: tr("priceEnabled"),
      width: 110,
      fixed: "right" as const,
      align: "center" as const,
      slots: { default: "enabled" },
    },
  ];
}
function incomingRows() {
  // One pass over the bounded detail projection; no per-row request or scan.
  const groups = new Map<string, CatalogPrice[]>();
  for (const price of props.prices) {
    if (price.configurationPriceDimensionNo) continue;
    const key = `${price.billingRuleNo}:${price.currencyCode}`;
    const group = groups.get(key) ?? [];
    group.push(price);
    groups.set(key, group);
  }
  const result = new Map<string, Row>();
  for (const code of visitedCurrencies)
    for (const rule of eligibleRules.value) {
      const stored = (groups.get(`${rule.billingRuleNo}:${code}`) ?? [])
        .slice()
        .sort((a, b) => a.minimumQuantity - b.minimumQuantity);
      if ((!props.writable || rule.chargeType === "trial") && !stored.length)
        continue;
      const first = stored[0];
      const ruleSignature = JSON.stringify([
        rule.ruleCode,
        rule.chargeType,
        rule.periodUnit,
        rule.periodCount,
        rule.pricePhase,
        rule.quantityMode,
        rule.minimumPurchaseCycles,
        rule.maximumRenewalCycles,
        rule.trialQuantity,
        rule.status,
      ]);
      const storedSignature = JSON.stringify([
        ruleSignature,
        stored.map((price) => [
          price.amountMinor,
          price.setupFeeMinor,
          price.renewalAmountMinor,
          price.minimumQuantity,
          price.maximumQuantity,
          price.status,
        ]),
      ]);
      const advanced =
        stored.length > 1 ||
        (!!first &&
          (![0, 1].includes(first.minimumQuantity) ||
            first.maximumQuantity != null ||
            (rule.pricePhase === "new_purchase" &&
              first.renewalAmountMinor != null)));
      const row: Row = {
        id: `${rule.ruleCode}:${code}`,
        rule,
        currency: code,
        priceNo: first?.priceNo || "",
        amount: first ? minorToMajorAmount(first.amountMinor, code) : "",
        setup: first ? minorToMajorAmount(first.setupFeeMinor, code) : "0",
        renewal:
          first?.renewalAmountMinor != null
            ? minorToMajorAmount(first.renewalAmountMinor, code)
            : "",
        enabled: first ? first.status === "active" : true,
        minimum: first?.minimumQuantity ?? 0,
        advanced,
        conflict: false,
        originalEdit: "",
        storedSignature,
        ruleSignature,
      };
      row.originalEdit = editSignature(row);
      result.set(row.id, row);
    }
  return result;
}
async function synchronize(discard = false) {
  if (discard) for (const key of Object.keys(edits)) delete edits[key];
  const incoming = incomingRows();
  for (const [key, next] of incoming) {
    const previous = edits[key];
    const dirty = previous && editSignature(previous) !== previous.originalEdit;
    if (!previous || !dirty) {
      edits[key] = next;
      continue;
    }
    if (previous.storedSignature === next.storedSignature) {
      // An internal snapshot changed only row identities. Keep the pending input
      // and advance its exact write anchors to the equivalent current rows.
      previous.rule = next.rule;
      previous.priceNo = next.priceNo;
      previous.minimum = next.minimum;
      previous.conflict = false;
    } else if (
      previous.ruleSignature === next.ruleSignature &&
      editSignature(previous) === editSignature(next)
    ) {
      // A read can confirm a previously response-lost save; do not write again.
      edits[key] = next;
    } else previous.conflict = true;
  }
  for (const key of Object.keys(edits)) {
    if (incoming.has(key)) continue;
    if (editSignature(edits[key]!) !== edits[key]!.originalEdit)
      edits[key]!.conflict = true;
    else delete edits[key];
  }
  editingVersion = props.versionNo;
  const visibleRows = Object.values(edits).filter(
    (row) => currency.value === "all" || row.currency === currency.value,
  );
  await gridApi.setGridOptions({
    columns: columns(),
    data: visibleRows,
    height: Math.min(480, Math.max(120, visibleRows.length * 44 + 48)),
  });
}
async function selectCurrency(code: unknown) {
  if (typeof code !== "string") return;
  currency.value = code;
  if (code === "all") availableCurrencies.value.forEach((item) => visitedCurrencies.add(item));
  else visitedCurrencies.add(code);
  await synchronize();
}
async function loadCurrencies() {
  if (!props.writable) return;
  currenciesFailed.value = false;
  try {
    globalCurrencies.value = (await currencyOptions()).currencies;
  } catch {
    currenciesFailed.value = true;
  }
  await selectCurrency(currency.value);
}
async function refresh() {
  if (refreshing.value || saving.value) return;
  refreshing.value = true;
  try {
    await props.refresh();
    refreshPending.value = false;
    failure.value = "";
  } catch {
    failure.value = tr("basePriceSaveFailed");
  } finally {
    refreshing.value = false;
  }
}
async function save() {
  if (
    !props.writable ||
    saving.value ||
    hasConflict.value ||
    refreshPending.value
  )
    return;
  const changed = changedRows.value;
  if (!changed.length) return;
  if (changed.length > 256) {
    failure.value = tr("tooManyPriceChanges");
    return;
  }
  failure.value = "";
  const requests = [];
  for (const row of changed) {
    try {
      requests.push({
        targetKind: "base" as const,
        active: row.enabled,
        previousPriceNo: row.priceNo,
        billingRuleNo: row.rule.billingRuleNo,
        currencyCode: row.currency,
        tiers: [
          {
            minimumQuantity: row.minimum,
            maximumQuantity: null,
            amountMinor: majorToMinorAmount(row.amount, row.currency),
            setupFeeMinor: majorToMinorAmount(row.setup, row.currency),
            ...(row.rule.pricePhase === "new_purchase"
              ? {}
              : {
                  renewalAmountMinor: majorToMinorAmount(
                    row.renewal,
                    row.currency,
                  ),
                }),
          },
        ],
      });
    } catch {
      failure.value = tr("invalidBasePriceRow", {
        period: periodLabel(row.rule),
        currency: row.currency,
      });
      return;
    }
  }
  saving.value = true;
  emit("busy", true);
  try {
    await catalogConfigurationPriceBatchSave(editingVersion, requests);
    for (const row of changed) row.originalEdit = editSignature(row);
    message.success({
      content: tr("basePricesSaved"),
      key: "idc-catalog-base-prices-saved",
    });
    try {
      await props.refresh();
      refreshPending.value = false;
    } catch {
      refreshPending.value = true;
    }
  } catch {
    failure.value = tr("basePriceSaveFailed");
  } finally {
    saving.value = false;
    emit("busy", false);
  }
}
watch(
  () => [props.versionNo, props.prices, props.billingRules, props.writable],
  () => void synchronize(),
);
watch(
  () => preferences.app.locale,
  () => gridApi.setGridOptions({ columns: columns() }),
);
onMounted(async () => {
  await selectCurrency(currency.value);
  await loadCurrencies();
  await synchronize();
});
const saveDisabled = computed(()=>!changedRows.value.length || hasConflict.value || refreshPending.value || saving.value);
const dirty = computed(() => changedRows.value.length > 0);
function canCopyTarget(source: Row, target: Row) {
  return target.id !== source.id && !target.advanced && !target.conflict &&
    target.currency === source.currency && target.rule.chargeType === source.rule.chargeType && target.rule.pricePhase === source.rule.pricePhase;
}
const copyableRows = computed(() => {
  const rows = Object.values(edits).filter((row) => !row.advanced && !row.conflict && row.rule.chargeType !== "trial");
  const groupKey = (row: Row) => `${row.currency}:${row.rule.chargeType}:${row.rule.pricePhase}`;
  const groupSizes = new Map<string, number>();
  for (const row of rows) groupSizes.set(groupKey(row), (groupSizes.get(groupKey(row)) || 0) + 1);
  return rows.filter((row) => {
    if (!row.amount || groupSizes.get(groupKey(row))! < 2) return false;
    try {
      majorToMinorAmount(row.amount, row.currency);
      majorToMinorAmount(row.setup, row.currency);
      if (row.rule.pricePhase !== "new_purchase") majorToMinorAmount(row.renewal, row.currency);
      return true;
    } catch { return false; }
  });
});
const copyCandidates = computed(() => {
  const source = edits[copySource.value];
  if (!source) return [];
  return Object.values(edits).filter((row) => canCopyTarget(source, row));
});
const selectedCopyRows = computed(() => copyCandidates.value.filter((row) => copyTargets.value.includes(row.id)));
const [CopyForm, copyFormApi] = useVbenForm({
  schema: [], showDefaultActions: false,
  commonConfig: { labelWidth: 100, componentProps: { class: "w-full" } },
});
const [CopyGrid, copyGridApi] = useVbenVxeGrid({ gridOptions: {
  columns: [], data: [], height: 160, pagerConfig: { enabled: false },
  toolbarConfig: { enabled: false }, rowConfig: { keyField: "id" },
} });
function copyRowLabel(row: Row) { return `${row.currency} · ${periodLabel(row.rule)}`; }
async function updateCopyPreview() {
  const source = edits[copySource.value];
  copyModalApi.setState({ confirmDisabled: !selectedCopyRows.value.length || !copyableRows.value.some((row) => row.id === copySource.value) });
  await copyGridApi.setGridOptions({
    columns: [
      { field: "period", title: overviewTr("target"), minWidth: 130, formatter: ({row}: {row: Row}) => copyRowLabel(row) },
      ...(["amount", "renewal", "setup"] as const).map((field) => ({
        field, title: tr(`basePrice_${field}`), width: 170,
        formatter: ({row}: {row: Row}) => field === "renewal" && row.rule.pricePhase === "new_purchase" ? "—" : `${row[field] || "—"} → ${source?.[field] || "—"}`,
      })),
    ],
    data: selectedCopyRows.value,
    height: Math.min(280, Math.max(140, selectedCopyRows.value.length * 44 + 48)),
  });
}
async function buildCopyForm() {
  await copyFormApi.setState({ schema: [
    { fieldName: "source", component: "Select", label: overviewTr("source"),
      componentProps: { "data-testid": "catalog-base-price-copy-source", options: copyableRows.value.map((row) => ({ value: row.id, label: copyRowLabel(row) })),
        onChange: async (value: string) => { copySource.value = value; copyTargets.value = []; await copyFormApi.setFieldValue("targets", []); await buildCopyForm(); await updateCopyPreview(); } },
    },
    { fieldName: "targets", component: "Select", label: overviewTr("targets"),
      componentProps: { "data-testid": "catalog-base-price-copy-targets", mode: "multiple", allowClear: true,
        options: copyCandidates.value.map((row) => ({ value: row.id, label: copyRowLabel(row) })),
        onChange: async (value: string[]) => { copyTargets.value = value; await updateCopyPreview(); } },
    },
  ] });
}
const [CopyModal, copyModalApi] = useVbenModal({
  async onOpenChange(open) {
    if (!open) return;
    copySource.value = copyableRows.value.find((row) => currency.value === "all" || row.currency === currency.value)?.id || copyableRows.value[0]?.id || "";
    copyTargets.value = [];
    await copyFormApi.resetForm(); await buildCopyForm();
    await copyFormApi.setValues({ source: copySource.value, targets: [] }); await updateCopyPreview();
  },
  async onConfirm() {
    const source = edits[copySource.value];
    if (!props.writable || saving.value || hasConflict.value || refreshPending.value || !source ||
        !copyableRows.value.some((row) => row.id === source.id) || !selectedCopyRows.value.length) return false;
    for (const target of selectedCopyRows.value) {
      target.amount = source.amount; target.setup = source.setup;
      if (target.rule.pricePhase !== "new_purchase") target.renewal = source.renewal;
    }
    await synchronize(); copyModalApi.close();
    message.info(overviewTr("copiedDraft", {count: selectedCopyRows.value.length}));
  },
});
defineExpose({save,saveDisabled,dirty});
</script>

<template>
  <section
    class="catalog-base-price-table"
    data-testid="catalog-base-price-table"
    @keydown.esc.stop
  >

    <div class="catalog-base-price-toolbar">
      <span>{{
        $t("plugin.idc-core.catalog.configurationPrice.currency")
      }}</span
      ><Select
        :value="currency"
        :options="[{ value: 'all', label: overviewTr('allCurrencies') }, ...availableCurrencies.map((value) => ({ value, label: value }))]"
        :disabled="saving"
        class="catalog-base-price-currency"
        data-testid="catalog-base-price-table-currency"
        @change="selectCurrency"
      />
      <span v-if="dirty" class="text-muted-foreground" data-testid="catalog-base-price-pending">{{ overviewTr("pending", {count: changedRows.length}) }}</span>
      <Button v-if="writable" class="ml-auto" data-testid="catalog-base-price-copy" :disabled="saving || hasConflict || refreshPending || !copyableRows.length" @click="copyModalApi.open()">{{ overviewTr("copy") }}</Button>
    </div>
    <Alert
      v-if="currenciesFailed"
      class="mb-3"
      type="warning"
      show-icon
      :message="tr('priceCurrenciesFailed')"
      ><template #action
        ><Button @click="loadCurrencies">{{ tr("retry") }}</Button></template
      ></Alert
    >
    <Alert
      v-if="hasConflict"
      class="mb-3"
      type="warning"
      show-icon
      :message="tr('basePricesChanged')"
      ><template #action
        ><Popconfirm
          :title="tr('reloadConfigurationConfirm')"
          @confirm="synchronize(true)"
          ><Button data-testid="catalog-base-price-reload">{{
            tr("reloadConfiguration")
          }}</Button></Popconfirm
        ></template
      ></Alert
    >
    <Alert v-if="failure" class="mb-3" type="error" show-icon :message="failure"
      ><template #action
        ><Button
          :loading="refreshing"
          data-testid="catalog-base-price-refresh"
          @click="refresh"
          >{{ tr("readLatestPrices") }}</Button
        ></template
      ></Alert
    >
    <Alert
      v-if="refreshPending"
      class="mb-3"
      type="info"
      show-icon
      :message="tr('pricesSavedRefreshPending')"
      ><template #action
        ><Button :loading="refreshing" @click="refresh">{{
          tr("readLatestPrices")
        }}</Button></template
      ></Alert
    >
    <div v-if="eligibleRules.length && currency" :inert="saving || undefined">
      <Grid>
        <template
          v-for="field in ['amount', 'renewal', 'setup'] as const"
          :key="field"
          #[field]="{ row }"
        >
          <span v-if="row.advanced">{{
            field === "amount" ? tr("advancedPriceDetails") : "—"
          }}</span>
          <span
            v-else-if="
              field === 'renewal' && row.rule.pricePhase === 'new_purchase'
            "
            >—</span
          >
          <Input
            v-else-if="writable"
            v-model:value="edits[row.id]![field]"
            :disabled="row.rule.chargeType === 'trial'"
            inputmode="decimal"
            :aria-label="`${periodLabel(row.rule)} ${tr(`basePrice_${field}`)}`"
            :data-testid="`catalog-base-price-${field}-${row.id}`"
          />
          <span v-else>{{ edits[row.id]![field] || "—" }}</span>
        </template>
        <template #enabled="{ row }"
          ><Switch
            v-if="row.priceNo || row.amount"
            v-model:checked="edits[row.id]!.enabled"
            :disabled="!writable || row.advanced"
            :checked-children="tr('priceOn')"
            :un-checked-children="tr('priceOff')"
            :aria-label="`${periodLabel(row.rule)} ${tr('priceEnabled')}`"
          /><span v-else>—</span></template
        >
      </Grid>
    </div>
    <Empty
      v-else
      :image="Empty.PRESENTED_IMAGE_SIMPLE"
      :description="tr('noNormalBillingPeriods')"
    />
    <Button
      v-if="writable && !externalSave"
      class="mt-4"
      type="primary"
      data-testid="catalog-base-price-table-save"
      :loading="saving"
      :disabled="!changedRows.length || hasConflict || refreshPending"
      @click="save"
      >{{ tr("saveBasePrices") }}</Button
    >
    <CopyModal :title="overviewTr('copy')" :confirm-text="overviewTr('fillDraft')" class="w-[760px] max-w-[96vw]">
      <div data-testid="catalog-base-price-copy-modal" class="min-w-0">
        <CopyForm />
        <p class="mb-3 text-muted-foreground">{{ overviewTr('copyHelp') }}</p>
        <CopyGrid data-testid="catalog-base-price-copy-preview" />
      </div>
    </CopyModal>
  </section>
</template>

<style scoped>
.catalog-base-price-table {
  min-width: 0;
}
.catalog-base-price-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.catalog-base-price-currency {
  min-width: 160px;
}
</style>
