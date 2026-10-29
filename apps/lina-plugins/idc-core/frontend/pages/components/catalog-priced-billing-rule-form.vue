<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { CatalogBillingRule } from "../catalog-client";
import { onMounted, ref, watch } from "vue";
import { Alert, Button, message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { catalogPricedBillingRuleCreate } from "../catalog-client";
import { currencyOptions } from "../currency-client";
import { majorToMinorAmount } from "../catalog-money";
const props = defineProps<{
  versionNo: string;
  billingRules: CatalogBillingRule[];
  refresh: () => Promise<void>;
}>();
const emit = defineEmits<{ busy: [value: boolean]; saved: [] }>();
const tr = (key: string) => $t(`plugin.idc-core.catalog.workspace.${key}`);
const saving = ref(false),
  currenciesFailed = ref(false),
  refreshPending = ref(false),
  failed = ref(false);
let ruleCode = `plan_${crypto.randomUUID().replaceAll("-", "")}`;
const amount = () => z.string().regex(/^\d{1,19}(?:\.\d{1,4})?$/);
const [Form, formApi] = useVbenForm({
  commonConfig: { labelWidth: 150, componentProps: { class: "w-full" } },
  showDefaultActions: false,
  schema: [],
});
function schema(): NonNullable<VbenFormProps["schema"]> {
  return [
    {
      component: "Select",
      fieldName: "periodUnit",
      label: $t("plugin.idc-core.catalog.commerce.period"),
      defaultValue: "month",
      componentProps: {
        "data-testid": "catalog-priced-period",
        options: ["once", "hour", "day", "month", "year"].map((value) => ({
          value,
          label: $t(`plugin.idc-core.catalog.detail.code.period.${value}`),
        })),
      },
      rules: z.enum(["once", "hour", "day", "month", "year"]),
    },
    {
      component: "InputNumber",
      fieldName: "periodCount",
      label: $t("plugin.idc-core.catalog.commerce.periodCount"),
      defaultValue: 1,
      componentProps: {
        "data-testid": "catalog-priced-count",
        min: 1,
        max: 87600,
        precision: 0,
      },
      rules: z.number().int().min(1).max(87600),
      dependencies: {
        triggerFields: ["periodUnit"],
        if: (values) => values.periodUnit !== "once",
      },
    },
    {
      component: "Select",
      fieldName: "currencyCode",
      label: $t("plugin.idc-core.catalog.configurationPrice.currency"),
      componentProps: { "data-testid": "catalog-priced-currency", options: [] },
      rules: z.string().regex(/^[A-Z]{3}$/),
    },
    {
      component: "Input",
      fieldName: "amount",
      label: tr("basePrice_amount"),
      componentProps: {
        "data-testid": "catalog-priced-amount",
        inputmode: "decimal",
      },
      rules: amount(),
    },
    {
      component: "Input",
      fieldName: "setupFee",
      label: tr("basePrice_setup"),
      defaultValue: "0",
      componentProps: {
        "data-testid": "catalog-priced-setup",
        inputmode: "decimal",
      },
      rules: amount(),
    },
    {
      component: "Input",
      fieldName: "renewalAmount",
      label: tr("basePrice_renewal"),
      componentProps: {
        "data-testid": "catalog-priced-renewal",
        inputmode: "decimal",
      },
      rules: amount(),
      dependencies: {
        triggerFields: ["periodUnit"],
        if: (values) => values.periodUnit !== "once",
      },
    },
    {
      component: "InputNumber",
      fieldName: "minimumPurchaseCycles",
      label: $t("plugin.idc-core.catalog.commerce.minimumCycles"),
      defaultValue: 1,
      componentProps: {
        "data-testid": "catalog-priced-minimum",
        min: 1,
        max: 120,
        precision: 0,
      },
      rules: z.number().int().min(1).max(120),
      dependencies: {
        triggerFields: ["periodUnit"],
        if: (values) => values.periodUnit !== "once",
      },
    },
    {
      component: "InputNumber",
      fieldName: "maximumRenewalCycles",
      label: $t("plugin.idc-core.catalog.commerce.maximumRenewal"),
      componentProps: { min: 1, max: 120, precision: 0 },
      rules: z.number().int().min(1).max(120).optional(),
      dependencies: {
        triggerFields: ["periodUnit"],
        if: (values) => values.periodUnit !== "once",
      },
    },
  ];
}
async function loadCurrencies() {
  currenciesFailed.value = false;
  try {
    const { currencies } = await currencyOptions();
    await formApi.updateSchema([
      {
        fieldName: "currencyCode",
        componentProps: {
          options: currencies.map((value) => ({ value, label: value })),
        },
      },
    ]);
    const values = await formApi.getValues();
    if (!values.currencyCode && currencies[0])
      await formApi.setFieldValue("currencyCode", currencies[0]);
  } catch {
    currenciesFailed.value = true;
  }
}
async function acknowledge() {
  ruleCode = `plan_${crypto.randomUUID().replaceAll("-", "")}`;
  refreshPending.value = false;
  failed.value = false;
  await formApi.setValues({ amount: "", renewalAmount: "", setupFee: "0" });
  emit("saved");
  message.success({
    content: tr("pricedPeriodSaved"),
    key: "idc-catalog-priced-period-saved",
  });
}
watch(
  () => props.billingRules,
  async (rules) => {
    if (rules.some((rule) => rule.ruleCode === ruleCode)) await acknowledge();
  },
);
async function refresh() {
  if (saving.value) return;
  saving.value = true;
  try {
    await props.refresh();
  } catch {
    failed.value = true;
  } finally {
    saving.value = false;
  }
}
async function save() {
  if (saving.value || refreshPending.value) return;
  saving.value = true;
  failed.value = false;
  emit("busy", true);
  try {
    if (!(await formApi.validate()).valid) return;
    const values = await formApi.getValues<{
      periodUnit: "once" | "hour" | "day" | "month" | "year";
      periodCount: number;
      minimumPurchaseCycles: number;
      maximumRenewalCycles?: number;
      currencyCode: string;
      amount: string;
      setupFee: string;
      renewalAmount: string;
    }>();
    const once = values.periodUnit === "once";
    let amounts;
    try {
      amounts = {
        amountMinor: majorToMinorAmount(values.amount, values.currencyCode),
        setupFeeMinor: majorToMinorAmount(values.setupFee, values.currencyCode),
        ...(!once
          ? {
              renewalAmountMinor: majorToMinorAmount(
                values.renewalAmount,
                values.currencyCode,
              ),
            }
          : {}),
      };
    } catch {
      message.error(
        $t("plugin.idc-core.catalog.configurationPrice.invalidAmount"),
      );
      return;
    }
    await catalogPricedBillingRuleCreate(props.versionNo, {
      ruleCode,
      periodUnit: values.periodUnit,
      periodCount: once ? 1 : values.periodCount,
      minimumPurchaseCycles: once ? 1 : values.minimumPurchaseCycles,
      ...(!once && values.maximumRenewalCycles
        ? { maximumRenewalCycles: values.maximumRenewalCycles }
        : {}),
      currencyCode: values.currencyCode,
      ...amounts,
    });
    refreshPending.value = true;
    try {
      await props.refresh();
    } catch {
      failed.value = true;
    }
  } catch {
    failed.value = true;
  } finally {
    saving.value = false;
    emit("busy", false);
  }
}
onMounted(async () => {
  await formApi.setState({ schema: schema() });
  await formApi.setValues({
    periodUnit: "month",
    periodCount: 1,
    minimumPurchaseCycles: 1,
    maximumRenewalCycles: undefined,
    amount: "",
    setupFee: "0",
    renewalAmount: "",
  });
  await loadCurrencies();
});
</script>
<template>
  <section data-testid="catalog-priced-billing-form" @keydown.esc.stop>
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
      v-if="failed || refreshPending"
      class="mb-3"
      :type="refreshPending ? 'info' : 'error'"
      show-icon
      :message="
        tr(refreshPending ? 'pricesSavedRefreshPending' : 'pricedPeriodFailed')
      "
      ><template #action
        ><Button
          data-testid="catalog-priced-refresh"
          :loading="saving"
          @click="refresh"
          >{{ tr("readLatestPrices") }}</Button
        ></template
      ></Alert
    >
    <div :inert="saving || refreshPending || undefined"><Form /></div>
    <Button
      type="primary"
      data-testid="catalog-priced-save"
      :loading="saving"
      :disabled="refreshPending"
      @click="save"
      >{{ tr("savePricedPeriod") }}</Button
    >
  </section>
</template>
