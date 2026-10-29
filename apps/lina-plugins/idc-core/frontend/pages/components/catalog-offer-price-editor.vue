<script setup lang="ts">
// Option prices are a local part of the merchant offer draft. No per-row reads,
// price writes or temporary catalog publications happen in this component.
import type {
  MerchantOptionPricingContext,
  MerchantPriceDraft,
} from "../catalog-merchant-prices";
import { computed, ref, watch } from "vue";
import {
  Alert,
  Button,
  Checkbox,
  Input,
  InputNumber,
  Switch,
} from "ant-design-vue";
import { preferences } from "@vben/preferences";
import { useVbenModal } from "@vben/common-ui";
import { useVbenForm } from "#/adapter/form";
import { $t } from "#/locales";
import VbenDataGrid from "./vben-data-grid.vue";
import {
  initialMerchantPriceDraft,
  merchantPriceDraftKey,
} from "../catalog-merchant-prices";
import { majorToMinorAmount, multiplyMajorAmount } from "../catalog-money";

const props = defineProps<{
  context: MerchantOptionPricingContext;
  drafts: Record<string, MerchantPriceDraft>;
  disabled?: boolean;
}>();
const emit = defineEmits<{
  change: [key: string, draft: MerchantPriceDraft];
}>();
const tr = (key: string) => $t(`plugin.idc-core.catalog.offerPrices.${key}`);
const currency = ref("");
const ruleNo = ref("");
const automatic = ref(false);
const batchText = (key:string) => $t(`plugin.idc-core.catalog.optionBatchPrice.${key}`);
const batchError = ref("");
const [BatchForm,batchFormApi] = useVbenForm({schema:[],showDefaultActions:false,commonConfig:{labelWidth:105}});
const [BatchModal,batchModal] = useVbenModal({onConfirm:applyBatchPrices});

const coefficients = ref<Record<string, string>>({});
const mode = ref<"per_unit" | "band_total" | "graduated">("per_unit");
const [Filters, filtersApi] = useVbenForm({
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1 md:grid-cols-2",
  commonConfig: { labelWidth: 90 },
});
const localized = (value: { zhCN: string; enUS: string }) =>
  preferences.app.locale === "zh-CN" ? value.zhCN : value.enUS;
const ruleLabel = (
  rule: MerchantOptionPricingContext["billingRules"][number],
) =>
  `${rule.periodCount} ${$t(`plugin.idc-core.catalog.detail.code.period.${rule.periodUnit}`)}`;
const hasExistingMode = computed(() =>
  props.context.targets.some(
    (target) =>
      target.fieldKey === props.context.fieldKey &&
      target.quantity &&
      target.pricingMode,
  ),
);
const schedules = computed(() => {
  const rules = props.context.billingRules.filter(
    (rule) => !ruleNo.value || rule.billingRuleNo === ruleNo.value,
  );
  const choices = props.context.quantity
    ? [{ value: "", label: { zhCN: tr("quantity"), enUS: tr("quantity") } }]
    : props.context.options;
  if (!currency.value) return [];
  return choices.flatMap((choice) =>
    rules.map((rule) => {
      const initial = initialMerchantPriceDraft(
        props.context,
        choice.value,
        rule,
        currency.value,
      );
      const key = merchantPriceDraftKey(initial);
      const draft = props.drafts[key] || initial;
      if (!hasExistingMode.value && !props.drafts[key])
        draft.pricingMode = props.context.quantity ? mode.value : "fixed";
      return {
        key,
        draft,
        rule,
        label: localized(choice.label),
        period: ruleLabel(rule),
      };
    }),
  );
});
const rows = computed(() =>
  schedules.value.flatMap((schedule) =>
    schedule.draft.tiers.map((tier, index) => ({
      ...schedule,
      tier,
      index,
      id: schedule.key + ":" + index,
    })),
  ),
);
const columns = computed(() => [
  ...(props.context.quantity
    ? [{ key: "range", title: tr("range"), minWidth: 150 }]
    : [{ key: "label", title: tr("option"), minWidth: 120 }]),
  { key: "amount", title: tr("amount"), width: 110 },
  { key: "renewal", title: tr("renewal"), width: 110 },
  { key: "setup", title: tr("setup"), width: 110 },
  ...(automatic.value
    ? [{ key: "coefficient", title: tr("coefficient"), width: 130 }]
    : []),
  { key: "actions", title: tr("actions"), width: 128, fixed: "right" as const },
]);
type PriceRow = (typeof rows.value)[number];
function cloneDraft(draft: MerchantPriceDraft): MerchantPriceDraft {
  return { ...draft, tiers: draft.tiers.map((tier) => ({ ...tier })) };
}
function update(
  row: PriceRow,
  field: "amount" | "setup" | "renewal" | "maximumQuantity",
  value: string | number | null,
) {
  const draft = cloneDraft(row.draft);
  const tier = draft.tiers[row.index]!;
  if (field === "maximumQuantity") {
    tier.maximumQuantity =
      value === null || value === "" ? null : Number(value);
    if (draft.tiers[row.index + 1])
      draft.tiers[row.index + 1]!.minimumQuantity =
        tier.maximumQuantity ?? tier.minimumQuantity;
  } else tier[field] = String(value ?? "");
  emit("change", row.key, draft);
}
function included(row: PriceRow) {
  const draft = cloneDraft(row.draft);
  Object.assign(draft.tiers[row.index]!, {
    amount: "0",
    renewal: "0",
    setup: "0",
  });
  emit("change", row.key, draft);
}
function active(row: PriceRow, value: boolean) {
  emit("change", row.key, { ...cloneDraft(row.draft), active: value });
}
function addBand(row: PriceRow) {
  const draft = cloneDraft(row.draft),
    last = draft.tiers.at(-1)!;
  if (
    last.maximumQuantity === null ||
    last.maximumQuantity <= last.minimumQuantity ||
    draft.tiers.length >= 256
  )
    return;
  draft.tiers.push({
    minimumQuantity: last.maximumQuantity,
    maximumQuantity: null,
    amount: "",
    setup: "",
    renewal: "",
  });
  emit("change", row.key, draft);
}
function removeBand(row: PriceRow) {
  const draft = cloneDraft(row.draft);
  if (draft.tiers.length <= 1) return;
  draft.tiers.splice(row.index, 1);
  if (row.index === 0) draft.tiers[0]!.minimumQuantity = 0;
  else
    draft.tiers[row.index - 1]!.maximumQuantity =
      draft.tiers[row.index]?.minimumQuantity ?? null;
  emit("change", row.key, draft);
}
function coefficientPreview(row: PriceRow) {
  try {
    return multiplyMajorAmount(
      row.tier.amount,
      coefficients.value[row.id] || "",
      row.draft.currencyCode,
    );
  } catch {
    return "";
  }
}
function applyCoefficient(row: PriceRow) {
  const preview = coefficientPreview(row);
  if (!preview) return;
  const draft = cloneDraft(row.draft);
  // Only the purchase price is previewed here; never change hidden renewal
  // or setup fees when applying this explicitly scoped calculation.
  draft.tiers[row.index]!.amount = preview;
  emit("change", row.key, draft);
}
async function openBatchPrices() {
  if (props.disabled || props.context.quantity || schedules.value.length < 2) return;
  batchError.value = "";
  batchModal.open();
  await batchFormApi.setState({schema:[
    {fieldName:"targets",label:batchText("targets"),component:"Select",rules:"required",componentProps:{mode:"multiple",virtual:false,class:"w-full",maxTagCount:"responsive",options:schedules.value.map(row=>({value:row.key,label:`${row.label} · ${row.period}`})),"data-testid":"catalog-option-price-batch-targets"}},
    ...(["amount","renewal","setup"] as const).map(field=>({fieldName:field,label:tr(field),component:"Input" as const,componentProps:{placeholder:batchText("keep"),inputmode:"decimal","data-testid":`catalog-option-price-batch-${field}`}})),
  ]});
  await batchFormApi.setValues({targets:[],amount:"",renewal:"",setup:""},false);
}
async function applyBatchPrices() {
  if (props.disabled || !(await batchFormApi.validate()).valid) return;
  const values = await batchFormApi.getValues();
  const targets = Array.isArray(values.targets) ? values.targets as string[] : [];
  const selected = schedules.value.filter(row=>targets.includes(row.key));
  if (!selected.length || selected.length !== targets.length || selected.length > 256) {batchError.value=batchText("invalid");return;}
  const edits = (["amount","renewal","setup"] as const).filter(key=>String(values[key]||"").trim()!=="");
  try {
    if (!edits.length) throw new Error("empty");
    for (const key of edits) majorToMinorAmount(String(values[key]),currency.value);
  } catch {batchError.value=batchText("invalid");return;}
  const changes = selected.map(row=>{
    const draft=cloneDraft(row.draft);
    for(const tier of draft.tiers) for(const key of edits) {
      if(key==='renewal' && !['both','renewal'].includes(row.rule.pricePhase)) continue;
      if((key==='amount'||key==='setup') && row.rule.pricePhase==='renewal') continue;
      tier[key]=String(values[key]).trim();
    }
    return {key:row.key,draft};
  });
  for(const change of changes) emit("change",change.key,change.draft);
  batchModal.close();
}
async function renderFilters() {
  const context = props.context;
  if (!context.currencies.includes(currency.value))
    currency.value = context.currencies[0] || "";
  if (!context.billingRules.some((rule) => rule.billingRuleNo === ruleNo.value))
    ruleNo.value = context.billingRules[0]?.billingRuleNo || "";
  const existing = context.targets.find(
    (target) => target.fieldKey === context.fieldKey && target.quantity,
  )?.pricingMode;
  mode.value =
    existing === "band_total" || existing === "graduated"
      ? existing
      : "per_unit";
  await filtersApi.setState({
    schema: [
      {
        fieldName: "currency",
        component: "Select",
        label: tr("currency"),
        componentProps: {
          options: context.currencies.map((value) => ({ value, label: value })),
          onChange: (value: string) => {
            currency.value = value;
          },
        },
      },
      {
        fieldName: "rule",
        component: "Select",
        label: tr("period"),
        componentProps: {
          options: context.billingRules.map((rule) => ({
            value: rule.billingRuleNo,
            label: ruleLabel(rule),
          })),
          onChange: (value: string) => {
            ruleNo.value = value;
          },
        },
      },
      ...(context.quantity && !hasExistingMode.value
        ? [
            {
              fieldName: "mode",
              component: "Select" as const,
              label: tr("mode"),
              componentProps: {
                options: ["per_unit", "band_total", "graduated"].map(
                  (value) => ({
                    value,
                    label: tr(value),
                  }),
                ),
                onChange: (value: "per_unit" | "band_total" | "graduated") => {
                  mode.value = value;
                  for (const schedule of schedules.value)
                    emit("change", schedule.key, {
                      ...cloneDraft(schedule.draft),
                      pricingMode: value,
                    });
                },
              },
            },
          ]
        : []),
    ],
  });
  await filtersApi.setValues(
    { currency: currency.value, rule: ruleNo.value, mode: mode.value },
    false,
  );
}
watch(
  () =>
    JSON.stringify([
      props.context.fieldKey,
      props.context.currencies,
      props.context.billingRules.map((rule) => rule.billingRuleNo),
      props.context.targets,
    ]),
  () => void renderFilters(),
  { immediate: true },
);
</script>

<template>
  <section class="offer-prices" data-testid="catalog-offer-field-prices">
    <header>
      <h4>{{ tr("title") }}</h4>
      <Button v-if="!context.quantity && schedules.length > 1" :disabled="disabled" size="small" data-testid="catalog-option-price-batch-open" @click="openBatchPrices">{{ batchText('title') }}</Button>
    </header>
    <Alert
      v-if="!context.currencies.length || !context.billingRules.length"
      type="info"
      :message="tr('baseFirst')"
    />
    <template v-else>
      <Filters />
      <p v-if="context.quantity" class="offer-price-note">{{ tr(mode) }}</p>
      <Checkbox v-model:checked="automatic">{{ tr("automatic") }}</Checkbox>
      <VbenDataGrid
        class="mt-3"
        :toolbar="false"
        virtual-rows
        :row-height="automatic ? 88 : 64"
        :columns="columns"
        :data="rows"
        row-key="id"
        :height="Math.min(380, rows.length * (automatic ? 88 : 64) + 55)"
      >
        <template #cell="{ column, record }">
          <template v-if="['amount', 'renewal', 'setup'].includes(column.key)">
            <span
              v-if="
                (column.key === 'renewal' &&
                  !['both', 'renewal'].includes(record.rule.pricePhase)) ||
                (['amount', 'setup'].includes(column.key) &&
                  record.rule.pricePhase === 'renewal')
              "
              >{{ tr("notApplicable") }}</span
            >
            <Input
              v-else
              :data-testid="`catalog-offer-price-${column.key}`"
              :value="record.tier[column.key]"
              :disabled="disabled"
              :placeholder="tr('enterAmount')"
              inputmode="decimal"
              @update:value="update(record, column.key, $event)"
            />
          </template>
          <div v-else-if="column.key === 'range'" class="offer-price-range">
            <span>{{ record.tier.minimumQuantity }}</span
            ><span>—</span
            ><InputNumber
              :data-testid="'catalog-offer-price-upper'"
              :value="record.tier.maximumQuantity"
              :disabled="disabled"
              :precision="0"
              :min="record.tier.minimumQuantity + 1"
              :placeholder="tr('noUpperLimit')"
              @change="update(record, 'maximumQuantity', $event)"
            />
          </div>
          <div v-else-if="column.key === 'coefficient'">
            <Input
              data-testid="catalog-offer-price-coefficient"
              v-model:value="coefficients[record.id]"
              :placeholder="tr('coefficient')"
            /><small v-if="coefficientPreview(record)">{{
              coefficientPreview(record)
            }}</small
            ><Button
              v-if="coefficientPreview(record)"
              type="link"
              size="small"
              :disabled="disabled"
              @click="applyCoefficient(record)"
              >{{ tr("applyPreview") }}</Button
            >
          </div>
          <div v-else-if="column.key === 'actions'" class="offer-price-actions">
            <Button
              type="link"
              size="small"
              :disabled="disabled"
              @click="included(record)"
              >{{ tr("included") }}</Button
            >
            <Button
              v-if="
                context.quantity &&
                record.index === record.draft.tiers.length - 1
              "
              type="link"
              size="small"
              :disabled="disabled || record.tier.maximumQuantity === null"
              @click="addBand(record)"
              >{{ tr("addBand") }}</Button
            >
            <Button
              v-if="context.quantity && record.draft.tiers.length > 1"
              type="link"
              danger
              size="small"
              :disabled="disabled"
              @click="removeBand(record)"
              >{{ tr("removeBand") }}</Button
            >
            <Switch
              v-if="record.draft.previousPriceNo || drafts[record.key]"
              :checked="record.draft.active"
              :disabled="disabled"
              :aria-label="tr('active')"
              :checked-children="tr('enabled')"
              :un-checked-children="tr('disabled')"
              size="small"
              @change="active(record, !!$event)"
            />
          </div>
          <span v-else>{{ record[column.key] }}</span>
        </template>
      </VbenDataGrid>
    </template>
    <BatchModal :title="batchText('title')" :confirm-text="batchText('apply')" :cancel-text="$t('pages.common.cancel')" class="w-[580px]">
      <section data-testid="catalog-option-price-batch">
        <p class="mb-4">{{ currency }} · {{ schedules[0]?.period }}</p>
        <Alert v-if="batchError" type="error" :message="batchError" class="mb-3" />
        <BatchForm />
      </section>
    </BatchModal>
  </section>
</template>

<style scoped>
.offer-prices {
  margin: 0;
  padding: 0;
  min-width: 0;
}
.offer-prices > header { display:flex; align-items:center; justify-content:space-between; gap:12px; margin-bottom:20px; }
.offer-prices > header h4 { margin:0; }
.offer-prices h4 {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 20px;
}
.offer-prices header p,
.offer-price-note {
  font-size: 12px;
  color: hsl(var(--muted-foreground));
  margin: 8px 0 16px;
  line-height: 1.7;
}
.offer-price-range {
  display: flex;
  align-items: center;
  gap: 7px;
}
.offer-price-range :deep(.ant-input-number) {
  min-width: 64px;
}
.offer-price-actions :deep(.ant-btn) {padding:0;height:20px;font-size:12px;line-height:20px;}
.offer-price-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}
</style>
