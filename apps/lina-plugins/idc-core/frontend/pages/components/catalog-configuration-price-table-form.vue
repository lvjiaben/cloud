<script setup lang="ts">
import type {
  CatalogBillingRule,
  CatalogConfigurationPriceCandidate,
  CatalogPrice,
} from "../catalog-client";
import { computed, onMounted, reactive, ref, watch } from "vue";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Button,
  Input,
  Popconfirm,
  Select,
  message,
} from "ant-design-vue";
import { useVbenForm } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import {
  catalogConfigurationPriceCandidates,
  catalogConfigurationPriceBatchSave,
  catalogPriceScheduleDelete,
} from "../catalog-client";
import {
  majorToMinorAmount,
  minorToMajorAmount,
  multiplyMajorAmount,
} from "../catalog-money";

type Row = {
  id: string;
  candidate: CatalogConfigurationPriceCandidate;
  rule: CatalogBillingRule;
  amount: string;
  setup: string;
  renewal: string;
  coefficient: string;
  priceNo: string;
  original: string;
  preview: string;
};
type PriceTableData = {
  versionNo: string;
  billingRules: CatalogBillingRule[];
  prices: CatalogPrice[];
};
const props=defineProps<{settingsIdentity:string;load:(refresh?:boolean)=>Promise<PriceTableData>}>();
const busy=ref(false), stale=ref(false), refreshPending=ref(false);
const hasChanges=computed(()=>Object.values(edits).some(row=>signature(row)!==row.original));
const rows = ref<Row[]>([]);
const edits = reactive<Record<string, Row>>({});
const fieldFilter = ref("");
const currency = ref("");
const fields = ref<Array<{ value: string; label: string }>>([]);
const currencies = ref<string[]>([]);
const automatic = ref(false);
const failure = ref("");
let versionNo = "";
let loadedSettingsIdentity="";
const priceGroups=new Map<string,CatalogPrice[]>();
let candidates: CatalogConfigurationPriceCandidate[] = [];
let rules: CatalogBillingRule[] = [];
let prices: CatalogPrice[] = [];
const draftRows = new Map<string, Row[]>();
const tr = (key: string) => $t(`plugin.idc-core.catalog.priceTable.${key}`);
const localized = (zh: string, en: string) =>
  preferences.app.locale === "zh-CN" ? zh || en : en || zh;
const fieldIdentity = (c: CatalogConfigurationPriceCandidate) =>
  c.fieldToken || JSON.stringify([c.fieldLabelZhCN, c.fieldLabelEnUS]);
const signature = (r: Row) => JSON.stringify([r.amount, r.setup, r.renewal]);
const periodLabel = (r: CatalogBillingRule) =>
  `${r.periodCount} ${$t(`plugin.idc-core.catalog.detail.code.period.${r.periodUnit}`)}`;
const [CalculationForm, calculationApi] = useVbenForm({
  showDefaultActions: false,
  schema: [],
  commonConfig: { labelWidth: 220 },
});
const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: [],
    data: [],
    height: 120,
    pagerConfig: { enabled: false },
    toolbarConfig: { enabled: false },
    rowConfig: { keyField: "id" },
  },
});
function columns() {
  return [
    {
      field: "candidateLabel",
      title: tr("specification"),
      minWidth: 180,
      formatter: ({ row }: { row: Row }) =>
        localized(
          row.candidate.candidateLabelZhCN,
          row.candidate.candidateLabelEnUS,
        ),
    },
    {
      field: "period",
      title: tr("period"),
      width: 190,
      formatter: ({ row }: { row: Row }) => periodLabel(row.rule),
    },
    ...(["amount", "setup", "renewal"] as const).map((field) => ({
      field,
      title: tr(field),
      width: 125,
      slots: { default: field },
    })),
    {
      field: "coefficient",
      title: tr("coefficient"),
      width: 120,
      visible: automatic.value,
      slots: { default: "coefficient" },
    },
    {
      field: "preview",
      title: tr("preview"),
      width: 120,
      visible: automatic.value,
      slots: { default: "preview" },
    },
    {
      field: "actions",
      title: tr("actions"),
      width: 100,
      fixed: "right" as const,
      slots: { default: "actions" },
    },
  ];
}
async function selectRows() {
  if (!currency.value || !fieldFilter.value) {
    rows.value = [];
    failure.value = tr(
      !currency.value ? "currencyRequired" : "noSpecifications",
    );
    await gridApi.setGridOptions({ columns: columns(), data: [] });
    return;
  }
  const key = JSON.stringify([fieldFilter.value, currency.value]);
  if (!draftRows.has(key)) {
    const choices = candidates.filter(
      (c) => !c.quantity && fieldIdentity(c) === fieldFilter.value,
    );
    const selected: Row[] = [];
    if (choices.length * rules.length > 256) {
      failure.value = tr("tooMany");
      rows.value = [];
      await gridApi.setGridOptions({ columns: columns(), data: [] });
      return;
    }
    for (const candidate of choices)
      for (const rule of rules) {
        const existing=candidate.dimensionNo ? priceGroups.get(`${candidate.dimensionNo}:${currency.value}:${rule.billingRuleNo}`) ?? [] : [];
        // Keep quantity bands and unusual existing tiers in their dedicated editor.
        if(existing.length>1 || existing.some(price=>price.minimumQuantity!==0 || price.maximumQuantity!=null))continue;
        const price = existing[0];
        const row: Row = {
          id: `${candidate.candidateToken}:${rule.billingRuleNo}:${currency.value}`,
          candidate,
          rule,
          amount: price
            ? minorToMajorAmount(price.amountMinor, currency.value)
            : "",
          setup: price
            ? minorToMajorAmount(price.setupFeeMinor, currency.value)
            : "0",
          renewal:
            price?.renewalAmountMinor != null
              ? minorToMajorAmount(price.renewalAmountMinor, currency.value)
              : "",
          coefficient: "1",
          priceNo: price?.priceNo || "",
          original: "",
          preview: "",
        };
        row.original = signature(row);
        selected.push(row);
      }
    draftRows.set(key, selected);
  }
  failure.value = "";
  rows.value = draftRows.get(key)!;
  for (const row of rows.value) {
    edits[row.id] ??= row;
    edits[row.id]!.preview = "";
  }
  await gridApi.setGridOptions({ columns: columns(), data: rows.value,height:Math.min(430,Math.max(120,rows.value.length*44+48)) });
}
async function previewPrices() {
  const values = await calculationApi.getValues();
  if (!values.autoCalculate) return;
  try {
    const generated = rows.value.map((row) =>
      multiplyMajorAmount(
        String(values.base || ""),
        edits[row.id]!.coefficient,
        currency.value,
      ),
    );
    rows.value.forEach((row, index) => {
      edits[row.id]!.preview = generated[index]!;
    });
  } catch {
    message.error(tr("invalidCalculation"));
  }
}
function applyPrices() {
  if (!automatic.value) return;
  for (const row of rows.value) {
    const edit = edits[row.id]!;
    if (edit.preview) {
      edit.amount = edit.preview;
      edit.preview = "";
    }
  }
}
async function deletePrice(row: Row) {
  if (!row.priceNo || busy.value || hasChanges.value || stale.value || refreshPending.value) return;
  busy.value=true;
  try {
    await catalogPriceScheduleDelete(versionNo, row.priceNo);
    row.priceNo = "";
    row.amount = "";
    row.setup = "0";
    row.renewal = "";
    row.original = signature(row);
    await loadTable(true);
    message.success(tr("deleted"));
  } finally {
    busy.value=false;
  }
}
async function loadTable(refresh=false) {
 busy.value=true;
 try {
  const data=await props.load(refresh);
  const identity=props.settingsIdentity;
  const nextCandidates=(await catalogConfigurationPriceCandidates(data.versionNo,{silentErrorMessage:true})).candidates;
  versionNo=data.versionNo;
  rules=data.billingRules.filter(rule=>rule.status==='active' && rule.quantityMode==='fixed' && ['one_time','recurring','trial'].includes(rule.chargeType) && ['new_purchase','both'].includes(rule.pricePhase));
  prices=data.prices;candidates=nextCandidates;loadedSettingsIdentity=identity;
  priceGroups.clear();
  for(const price of prices) {
   if(!price.configurationPriceDimensionNo || price.status!=='active')continue;
   const key=`${price.configurationPriceDimensionNo}:${price.currencyCode}:${price.billingRuleNo}`;
   const group=priceGroups.get(key) ?? [];group.push(price);priceGroups.set(key,group);
  }
  currencies.value=[...new Set(prices.filter(price=>!price.configurationPriceDimensionNo && price.status==='active').map(price=>price.currencyCode))];
  fields.value=[...new Map(candidates.filter(candidate=>!candidate.quantity).map(candidate=>[fieldIdentity(candidate),{value:fieldIdentity(candidate),label:localized(candidate.fieldLabelZhCN,candidate.fieldLabelEnUS)}])).values()];
  if(!fields.value.some(field=>field.value===fieldFilter.value)) fieldFilter.value=fields.value[0]?.value || '';
  if(!currencies.value.includes(currency.value)) currency.value=currencies.value[0] || '';
  draftRows.clear();for(const key of Object.keys(edits))delete edits[key];
  automatic.value=false;failure.value='';stale.value=false;refreshPending.value=false;
  await calculationApi.setState({schema:[
   {fieldName:'autoCalculate',label:tr('autoCalculate'),component:'Switch',componentProps:{'data-testid':'catalog-price-table-auto',onChange:async(checked:boolean)=>{automatic.value=checked;for(const row of rows.value)edits[row.id]!.preview='';await gridApi.setGridOptions({columns:columns()});}}},
   {fieldName:'base',label:tr('base'),component:'Input',componentProps:{'data-testid':'catalog-price-table-base',inputmode:'decimal',onChange:()=>{for(const row of rows.value)edits[row.id]!.preview='';}},dependencies:{triggerFields:['autoCalculate'],show:values=>!!values.autoCalculate}},
  ]});
  await calculationApi.setValues({autoCalculate:false,base:'0.00'});
  await selectRows();
 } catch {failure.value=tr('loadFailure');refreshPending.value=true;}
 finally {busy.value=false;if(loadedSettingsIdentity && props.settingsIdentity!==loadedSettingsIdentity)stale.value=true;}
}
async function save() {
    if(busy.value || stale.value || refreshPending.value)return;
    const changed = [...draftRows.entries()].flatMap(([key, list]) => {
      const [, code] = JSON.parse(key) as [string, string];
      return list
        .map((row) => edits[row.id]!)
        .filter((row) => signature(row) !== row.original)
        .map((row) => ({ row, currency: code }));
    });
    if (!changed.length) return;
    if (changed.length > 256) {
      failure.value = tr("tooMany");
      return false;
    }
    // Validate every changed row before the first write. Blank means unset;
    // explicitly enter zero for a free specification.
    let requests: Array<{
      row: Row;
      currency: string;
      tiers: Array<{
        minimumQuantity: number;
        maximumQuantity: null;
        amountMinor: string;
        setupFeeMinor: string;
        renewalAmountMinor?: string;
      }>;
    }>;
    try {
      requests = changed.map(({ row, currency }) => {
        if (row.rule.pricePhase !== "new_purchase" && !row.renewal)
          throw new Error("renewal_required");
        return {
          row,
          currency,
          tiers: [
            {
              minimumQuantity: 0,
              maximumQuantity: null,
              amountMinor: majorToMinorAmount(row.amount, currency),
              setupFeeMinor: majorToMinorAmount(row.setup || "0", currency),
              ...(row.renewal
                ? {
                    renewalAmountMinor: majorToMinorAmount(
                      row.renewal,
                      currency,
                    ),
                  }
                : {}),
            },
          ],
        };
      });
    } catch {
      failure.value = tr("invalidRows");
      return false;
    }
    busy.value=true;
    try {
      await catalogConfigurationPriceBatchSave(
        versionNo,
        requests.map(({ row, currency, tiers }) => ({
          candidateToken: row.candidate.candidateToken,
          previousPriceNo: row.priceNo,
          billingRuleNo: row.rule.billingRuleNo,
          currencyCode: currency,
          tiers,
        })),
      );
      for(const {row} of requests)row.original=signature(row);
      refreshPending.value=true;
      message.success({content:tr("saved"),key:'idc-catalog-configuration-prices-saved'});
      await loadTable(true);
    } catch {
      failure.value = tr("saveFailure");
      return false;
    } finally {
      busy.value=false;
    }
}
watch(()=>props.settingsIdentity,()=>{
 if(busy.value || !loadedSettingsIdentity || props.settingsIdentity===loadedSettingsIdentity)return;
 if(hasChanges.value)stale.value=true;
 else void loadTable();
});
onMounted(()=>loadTable());
</script>
<template>
  <section data-testid="catalog-price-table-form" @keydown.esc.stop>
    <Alert v-if="stale" class="mb-3" type="warning" show-icon :message="tr('changed')"><template #action><Popconfirm :title="tr('reloadConfirm')" @confirm="loadTable()"><Button data-testid="catalog-price-table-reload">{{ tr('reload') }}</Button></Popconfirm></template></Alert>
    <Alert v-if="refreshPending" class="mb-3" type="info" show-icon :message="tr('refreshPending')"><template #action><Button :loading="busy" data-testid="catalog-price-table-refresh" @click="loadTable(true)">{{ tr('reload') }}</Button></template></Alert>
    <div :inert="busy || undefined">
    <Alert class="mb-3" show-icon type="info" :message="tr('hint')" />
    <div class="mb-4 flex flex-wrap gap-3">
      <Select
        v-model:value="fieldFilter"
        class="min-w-[240px]"
        :options="fields"
        data-testid="catalog-price-table-field"
        @change="selectRows"
      /><Select
        v-model:value="currency"
        class="min-w-[100px]"
        :options="currencies.map((value) => ({ value, label: value }))"
        data-testid="catalog-price-table-currency"
        @change="selectRows"
      />
    </div>
    <CalculationForm v-show="rows.length" />
    <div v-if="automatic" class="mb-3 flex gap-2">
      <Button
        data-testid="catalog-price-table-preview"
        @click="previewPrices"
        >{{ tr("calculate") }}</Button
      ><Button data-testid="catalog-price-table-apply" @click="applyPrices">{{
        tr("apply")
      }}</Button>
    </div>
    <Alert
      v-if="failure"
      class="mb-3"
      show-icon
      type="error"
      :message="failure"
    ><template v-if="!refreshPending" #action><Popconfirm :title="tr('reloadConfirm')" @confirm="loadTable(true)"><Button data-testid="catalog-price-table-read-latest">{{ tr('reload') }}</Button></Popconfirm></template></Alert>
    <Grid>
      <template #amount="{ row }"
        ><Input
          v-if="edits[row.id]"
          v-model:value="edits[row.id]!.amount"
          inputmode="decimal"
          :aria-label="tr('amount')"
          :data-testid="`catalog-price-table-amount-${row.id}`"
      /></template>
      <template #setup="{ row }"
        ><Input
          v-if="edits[row.id]"
          v-model:value="edits[row.id]!.setup"
          inputmode="decimal"
          :aria-label="tr('setup')"
      /></template>
      <template #renewal="{ row }"
        ><Input
          v-if="edits[row.id]"
          v-model:value="edits[row.id]!.renewal"
          :disabled="row.rule.pricePhase === 'new_purchase'"
          inputmode="decimal"
          :aria-label="tr('renewal')"
      /></template>
      <template #coefficient="{ row }"
        ><Input
          v-if="edits[row.id]"
          v-model:value="edits[row.id]!.coefficient"
          inputmode="decimal"
          :aria-label="tr('coefficient')"
          @change="edits[row.id]!.preview = ''"
      /></template>
      <template #preview="{ row }">{{ edits[row.id]?.preview }}</template>
      <template #actions="{ row }"
        ><Popconfirm
          v-if="edits[row.id]"
          :title="tr('deleteConfirm')"
          @confirm="deletePrice(edits[row.id]!)"
          ><ghost-button danger :disabled="!edits[row.id]!.priceNo || hasChanges || stale || refreshPending">{{
            tr("delete")
          }}</ghost-button></Popconfirm
        ></template
      >
    </Grid>
    </div>
    <Button class="mt-4" type="primary" data-testid="catalog-price-table-save" :loading="busy" :disabled="!hasChanges || stale || refreshPending" @click="save">{{ tr('save') }}</Button>
  </section>
</template>
