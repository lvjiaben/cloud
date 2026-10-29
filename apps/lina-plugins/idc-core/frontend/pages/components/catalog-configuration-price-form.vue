<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type {
  CatalogBillingRule,
  CatalogPrice,
  CatalogConfigurationPriceCandidate,
} from "../catalog-client";

import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import type { CatalogConfigurationPriceEditorData } from "../catalog-configuration-price-editor";
import {
  Alert,
  Button,
  Empty,
  Popconfirm,
  Skeleton,
  Tag,
  message,
} from "ant-design-vue";
import { preferences } from "@vben/preferences";

import { $t } from "#/locales";
import { useVbenForm, z } from "#/adapter/form";

import {
  catalogConfigurationPriceCandidates,
  catalogConfigurationPriceDimensionCreate,
  catalogConfigurationPriceScheduleCreate,
  catalogPriceScheduleUpdate,
} from "../catalog-client";
import { currencyOptions } from "../currency-client";
import {
  majorToMinorAmount,
  minorToMajorAmount,
  multiplyMajorAmount,
} from "../catalog-money";

// Shared editor: all amounts, modes, dimensions and writes retain the catalog
// owner's contract. The drawer and legacy dialog only supply presentation.
const props = defineProps<{
  data: CatalogConfigurationPriceEditorData;
  settingsIdentity?: string;
  idPrefix?: string;
  showSave?: boolean;
  compact?: boolean;
  refresh?: () => Promise<CatalogConfigurationPriceEditorData>;
}>();
const emit = defineEmits<{ success: [] }>();
const loading = ref(false),
  saving = ref(false),
  stale = ref(false),
  loadFailed = ref(false),
  writeFailed = ref(false),
  refreshPending = ref(false);
let initialized = false,
  initializationSequence = 0,
  loadedIdentity = "";
let formBaseline = "";
const incomingIdentity = computed(
  () => props.settingsIdentity || JSON.stringify(props.data),
);
const tr = (key: string) => $t(`plugin.idc-core.catalog.priceEditor.${key}`);
function controlID(id: string) {
  return props.idPrefix
    ? `${props.idPrefix}-${id.replace(/^catalog-(?:configuration-price|price)-/u, "")}`
    : id;
}

const versionNo = ref("");
const rules = ref<CatalogBillingRule[]>([]);
const candidates = ref<CatalogConfigurationPriceCandidate[]>([]);
const tierCount = ref(1);
const quantitySelected = ref(false);
const singleCurrency = ref("");
const editingPrices = ref<CatalogPrice[]>([]);
const availablePrices = ref<CatalogPrice[]>([]);
const directEdit = ref(false);
const showExtraFees = ref(false);
const automaticPricing = ref(false);
const generatedPreview = ref("");
let candidateOptions: Array<{ label: string; value: string }> = [];
let currencies: string[] = [];
let selectionSequence = 0;
const selectedCandidate = (values: Record<string, unknown>) =>
  candidates.value.find(
    (item) => item.candidateToken === values.candidateToken,
  );

function localized(zh: string, en: string) {
  return preferences.app.locale === "zh-CN" ? zh || en : en || zh;
}

function billingRuleLabel(rule: CatalogBillingRule) {
  const charge = $t(
    `plugin.idc-core.catalog.detail.code.charge.${rule.chargeType}`,
  );
  const period = $t(
    `plugin.idc-core.catalog.detail.code.period.${rule.periodUnit}`,
  );
  const phase = $t(
    `plugin.idc-core.catalog.detail.code.pricePhase.${rule.pricePhase}`,
  );
  return `${charge} · ${rule.periodCount} × ${period} · ${phase}`;
}

function priceSchema(
  candidateOptions: Array<{ label: string; value: string }>,
  currencyCodes: string[],
): VbenFormProps["schema"] {
  const schema: NonNullable<VbenFormProps["schema"]> = [
    {
      component: "Select",
      componentProps: {
        "data-testid": controlID("catalog-configuration-price-candidate"),
        disabled: directEdit.value,
        dropdownStyle: { zIndex: 2300 },
        options: candidateOptions,
        showSearch: true,
        optionFilterProp: "label",
        onChange: (token: string) =>
          syncExistingPrice({ candidateToken: token }),
      },
      fieldName: "candidateToken",
      label: $t("plugin.idc-core.catalog.configurationPrice.candidate"),
      rules: z.string().min(1),
      dependencies: {
        triggerFields: ["candidateToken"],
        if: () => !props.compact || candidateOptions.length > 1,
      },
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": controlID("catalog-configuration-price-billing-rule"),
        onChange: (value: string) =>
          syncExistingPrice({ billingRuleNo: value }),
        disabled: directEdit.value,
        options: rules.value.map((rule) => ({
          label: billingRuleLabel(rule),
          value: rule.billingRuleNo,
        })),
      },
      fieldName: "billingRuleNo",
      label: $t("plugin.idc-core.catalog.configurationPrice.billingRule"),
      rules: z.string().min(1),
      dependencies: {
        triggerFields: ["candidateToken"],
        if: () => rules.value.length > 1,
      },
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": controlID("catalog-configuration-price-currency"),
        onChange: (value: string) => syncExistingPrice({ currencyCode: value }),
        disabled: directEdit.value,
        options: currencyCodes.map((value) => ({ label: value, value })),
        showSearch: true,
      },
      fieldName: "currencyCode",
      label: $t("plugin.idc-core.catalog.configurationPrice.currency"),
      rules: z.string().regex(/^[A-Z]{3}$/),
      dependencies: {
        triggerFields: ["candidateToken"],
        if: () => currencyCodes.length > 1,
      },
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": controlID("catalog-configuration-price-mode"),
        disabled: editingPrices.value.length > 0,
        options: [
          {
            label: $t("plugin.idc-core.catalog.configurationPrice.perUnit"),
            value: "per_unit",
          },
          {
            label: $t("plugin.idc-core.catalog.configurationPrice.bandTotal"),
            value: "band_total",
          },
          {
            label: $t("plugin.idc-core.catalog.configurationPrice.graduated"),
            value: "graduated",
          },
        ],
      },
      fieldName: "pricingMode",
      label: $t("plugin.idc-core.catalog.configurationPrice.mode"),
      rules: z.string().min(1),
      dependencies: {
        triggerFields: ["candidateToken"],
        if: (values) => !!selectedCandidate(values)?.quantity,
        disabled: (values) => !!selectedCandidate(values)?.dimensionNo,
      },
    },

    {
      component: "Switch",
      fieldName: "enabled",
      label: tr("enabled"),
      componentProps: {
        "data-testid": controlID("catalog-configuration-price-enabled"),
        style: { width: "44px" },
      },
    },
    {
      component: "Switch",
      fieldName: "autoCalculate",
      label: $t("plugin.idc-core.catalog.configurationPrice.autoCalculate"),
      componentProps: {
        "data-testid": controlID("catalog-price-auto"),
        style: { width: "44px" },
        onChange: (checked: boolean) => {
          automaticPricing.value = checked;
          generatedPreview.value = "";
        },
      },
    },
    {
      component: "Input",
      fieldName: "calculationBase",
      label: $t("plugin.idc-core.catalog.configurationPrice.calculationBase"),
      componentProps: {
        "data-testid": controlID("catalog-price-calculation-base"),
        inputmode: "decimal",
        onChange: () => {
          generatedPreview.value = "";
        },
      },
      dependencies: {
        triggerFields: ["autoCalculate"],
        show: (values) => !!values.autoCalculate,
      },
    },
    {
      component: "Input",
      fieldName: "calculationCoefficient",
      label: $t(
        "plugin.idc-core.catalog.configurationPrice.calculationCoefficient",
      ),
      componentProps: {
        "data-testid": controlID("catalog-price-calculation-coefficient"),
        inputmode: "decimal",
        onChange: () => {
          generatedPreview.value = "";
        },
      },
      dependencies: {
        triggerFields: ["autoCalculate"],
        show: (values) => !!values.autoCalculate,
      },
    },
  ];
  for (let index = 0; index < tierCount.value; index++) {
    const suffix = index === 0 ? "" : `_${index}`;
    if (index > 0)
      schema.push({
        component: "InputNumber",
        componentProps: {
          min: 1,
          max: 2147483646,
          precision: 0,
          "data-testid": controlID(
            `catalog-configuration-price-minimum${suffix}`,
          ),
        },
        fieldName: `minimum${suffix}`,
        label: $t("plugin.idc-core.catalog.configurationPrice.bandStart", {
          number: index + 1,
        }),
        rules: z.number().int().min(1).max(2147483646),
        dependencies: {
          triggerFields: ["candidateToken"],
          if: (values) => !!selectedCandidate(values)?.quantity,
        },
      });
    for (const field of ["amount", "setupFee", "renewalAmount"] as const) {
      schema.push({
        component: "Input",
        componentProps: {
          inputmode: "decimal",
          maxlength: 24,
          "data-testid": controlID(
            `catalog-configuration-price-${field}${suffix}`,
          ),
        },
        fieldName: `${field}${suffix}`,
        label: $t(`plugin.idc-core.catalog.configurationPrice.${field}`),
        rules:
          field === "renewalAmount"
            ? z.union([
                z.literal(""),
                z
                  .string()
                  .regex(
                    /^\d{1,19}(?:\.\d{1,4})?$/,
                    $t(
                      "plugin.idc-core.catalog.configurationPrice.invalidAmount",
                    ),
                  ),
              ])
            : z
                .string()
                .regex(
                  /^\d{1,19}(?:\.\d{1,4})?$/,
                  $t(
                    "plugin.idc-core.catalog.configurationPrice.invalidAmount",
                  ),
                ),
        dependencies: {
          triggerFields: ["candidateToken", "billingRuleNo"],
          if: (values) => index === 0 || !!selectedCandidate(values)?.quantity,
          show: (values) =>
            (field !== "setupFee" || showExtraFees.value) &&
            (field !== "renewalAmount" ||
              rules.value.find(
                (rule) => rule.billingRuleNo === values.billingRuleNo,
              )?.pricePhase !== "new_purchase"),
        },
      });
    }
  }
  return schema;
}

const [Form, formApi] = useVbenForm({
  commonConfig: {
    componentProps: { class: "w-full" },
    labelWidth: 140,
    formItemClass: "catalog-price-editor-field",
  },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "idc-plugin-form-surface",
});

async function save() {
  if (
    loading.value ||
    saving.value ||
    stale.value ||
    loadFailed.value ||
    refreshPending.value ||
    !initialized
  )
    return false;
  saving.value = true;
  writeFailed.value = false;
  try {
    const { valid } = await formApi.validate();
    if (!valid || !versionNo.value) return false;
    const values = await formApi.getValues<{
      amount: string;
      billingRuleNo: string;
      candidateToken: string;
      currencyCode: string;
      renewalAmount: string;
      setupFee: string;
    }>();
    const selectedRule = rules.value.find(
      (item) => item.billingRuleNo === values.billingRuleNo,
    );
    if (
      !selectedRule ||
      (selectedRule.pricePhase !== "new_purchase" && !values.renewalAmount)
    ) {
      message.error(
        $t("plugin.idc-core.catalog.configurationPrice.renewalRequired"),
      );
      return false;
    }
    const allValues =
      await formApi.getValues<Record<string, string | number | boolean>>();
    const choice = candidates.value.find(
      (item) => item.candidateToken === values.candidateToken,
    );
    const mode = choice?.quantity
      ? choice.pricingMode || String(allValues.pricingMode)
      : "fixed";
    const status =
      allValues.enabled === false ? ("inactive" as const) : ("active" as const);
    const tiers: Array<{
      amountMinor: string;
      setupFeeMinor: string;
      renewalAmountMinor?: string;
      minimumQuantity: number;
      maximumQuantity: number | null;
    }> = [];
    try {
      for (
        let index = 0;
        index < (choice?.quantity ? tierCount.value : 1);
        index++
      ) {
        const suffix = index === 0 ? "" : `_${index}`;
        const renewal = String(allValues[`renewalAmount${suffix}`] ?? "");
        if (selectedRule.pricePhase !== "new_purchase" && !renewal)
          throw new Error("renewal_required");
        const minimum = index === 0 ? 0 : Number(allValues[`minimum${suffix}`]);
        if (
          !Number.isSafeInteger(minimum) ||
          minimum < 0 ||
          minimum > 2147483646
        )
          throw new Error("invalid_band");
        if (index > 0 && minimum <= tiers[index - 1]!.minimumQuantity)
          throw new Error("invalid_band");
        if (index > 0) tiers[index - 1]!.maximumQuantity = minimum;
        tiers.push({
          amountMinor: majorToMinorAmount(
            String(allValues[`amount${suffix}`]),
            values.currencyCode,
          ),
          setupFeeMinor: majorToMinorAmount(
            String(allValues[`setupFee${suffix}`] ?? "0"),
            values.currencyCode,
          ),
          minimumQuantity: minimum,
          maximumQuantity: null,
          ...(renewal
            ? {
                renewalAmountMinor: majorToMinorAmount(
                  renewal,
                  values.currencyCode,
                ),
              }
            : {}),
        });
      }
    } catch {
      message.error(
        $t("plugin.idc-core.catalog.configurationPrice.invalidAmount"),
      );
      return false;
    }

    if (editingPrices.value.length) {
      const existing = editingPrices.value[0]!;
      await catalogPriceScheduleUpdate(
        versionNo.value,
        existing.billingRuleNo,
        existing.priceNo,
        {
          configurationPriceDimensionNo: existing.configurationPriceDimensionNo,
          currencyCode: existing.currencyCode,
          status,
          tiers,
        },
      );
    } else {
      const latest = (
        await catalogConfigurationPriceCandidates(versionNo.value)
      ).candidates;
      const candidate = latest.find(
        (item) => item.candidateToken === values.candidateToken,
      );
      if (
        !candidate ||
        (candidate.quantity &&
          candidate.pricingMode &&
          candidate.pricingMode !== mode)
      ) {
        message.error($t("plugin.idc-core.catalog.configurationPrice.changed"));
        return false;
      }
      const dimensionNo =
        candidate.dimensionNo ||
        (
          await catalogConfigurationPriceDimensionCreate(
            versionNo.value,
            candidate.candidateToken,
            mode,
          )
        ).dimensionNo;
      const payload = {
        configurationPriceDimensionNo: dimensionNo,
        currencyCode: values.currencyCode,
        status,
        tiers,
      };
      await catalogConfigurationPriceScheduleCreate(
        versionNo.value,
        values.billingRuleNo,
        payload,
      );
    }
    message.success($t("plugin.idc-core.catalog.configurationPrice.saved"));
    if (props.refresh) {
      refreshPending.value = true;
      try {
        const current = await props.refresh();
        await nextTick();
        await initialize(current);
        refreshPending.value = false;
      } catch {
        refreshPending.value = true;
      }
    }
    emit("success");
    return true;
  } catch {
    writeFailed.value = true;
    return false;
  } finally {
    saving.value = false;
    if (
      !refreshPending.value &&
      loadedIdentity &&
      incomingIdentity.value !== loadedIdentity
    )
      stale.value = true;
  }
}

async function initialize(
  data: CatalogConfigurationPriceEditorData = props.data,
) {
  const sequence = ++initializationSequence;
  const identity = incomingIdentity.value;
  loading.value = true;
  loadFailed.value = false;
  writeFailed.value = false;
  try {
    selectionSequence++;
    automaticPricing.value = false;
    generatedPreview.value = "";
    versionNo.value = data?.versionNo || "";
    rules.value = data?.billingRules ?? [];
    directEdit.value = !!data.prices?.length;
    availablePrices.value = data.availablePrices ?? [];
    editingPrices.value = [...(data.prices ?? [])].sort(
      (a, b) => a.minimumQuantity - b.minimumQuantity,
    );
    tierCount.value = editingPrices.value.length || 1;
    const existing = editingPrices.value[0];
    const nextCandidates = existing
      ? [
          {
            candidateToken: existing.configurationPriceDimensionNo,
            dimensionNo: existing.configurationPriceDimensionNo,
            pricingMode: existing.configurationPricingMode || "fixed",
            quantity:
              existing.configurationPricingMode === "per_unit" ||
              existing.configurationPricingMode === "band_total" ||
              existing.configurationPricingMode === "graduated",
            fieldLabelZhCN: existing.configurationFieldLabelZhCn,
            fieldLabelEnUS: existing.configurationFieldLabelEnUs,
            candidateLabelZhCN: existing.configurationCandidateLabelZhCn,
            candidateLabelEnUS: existing.configurationCandidateLabelEnUs,
          },
        ]
      : (
          await catalogConfigurationPriceCandidates(versionNo.value, {
            silentErrorMessage: true,
          })
        ).candidates
          .filter((item) => !data.quantityOnly || item.quantity)
          .sort((a, b) => Number(b.quantity) - Number(a.quantity));
    if (sequence !== initializationSequence) return;
    candidates.value = data.fieldToken
      ? nextCandidates.filter((item) => item.fieldToken === data.fieldToken)
      : data.fieldLabel
        ? nextCandidates.filter(
            (item) =>
              item.fieldLabelZhCN === data.fieldLabel!.zhCN &&
              item.fieldLabelEnUS === data.fieldLabel!.enUS,
          )
        : nextCandidates;
    showExtraFees.value = editingPrices.value.some(
      (price) => !/^0+$/.test(price.setupFeeMinor),
    );
    const fieldMatches = data.fieldToken
      ? candidates.value.filter((item) => item.fieldToken === data.fieldToken)
      : data.fieldLabel
        ? candidates.value.filter(
            (item) =>
              item.fieldLabelZhCN === data.fieldLabel!.zhCN &&
              item.fieldLabelEnUS === data.fieldLabel!.enUS,
          )
        : [];
    const selected =
      data.fieldToken || data.fieldLabel
        ? fieldMatches.length === 1
          ? fieldMatches[0]
          : undefined
        : editingPrices.value.length
          ? candidates.value.find(
              (item) =>
                item.dimensionNo ===
                editingPrices.value[0]!.configurationPriceDimensionNo,
            )
          : candidates.value[0];
    quantitySelected.value = !!selected?.quantity;
    const options = candidates.value.map((candidate) => ({
      value: candidate.candidateToken,
      label: candidate.quantity
        ? localized(candidate.fieldLabelZhCN, candidate.fieldLabelEnUS)
        : `${localized(candidate.fieldLabelZhCN, candidate.fieldLabelEnUS)} · ${localized(candidate.candidateLabelZhCN, candidate.candidateLabelEnUS)}`,
    }));
    const nextCurrencies = (await currencyOptions()).currencies;
    if (sequence !== initializationSequence) return;
    currencies = nextCurrencies;
    const configured = data.currencyCodes ?? [];
    currencies.sort(
      (a, b) => Number(configured.includes(b)) - Number(configured.includes(a)),
    );
    if (sequence !== initializationSequence) return;
    singleCurrency.value = currencies.length === 1 ? currencies[0]! : "";
    candidateOptions = options;
    await formApi.setState({ schema: priceSchema(options, currencies) });
    const initialValues: Record<string, unknown> = {
      candidateToken: selected?.candidateToken,
      pricingMode: selected?.pricingMode || "per_unit",
      billingRuleNo: rules.value[0]?.billingRuleNo,
      currencyCode: currencies[0],
      enabled: existing ? existing.status === "active" : true,
      autoCalculate: false,
      calculationBase: "0.00",
      calculationCoefficient: "1",
      amount: "",
      setupFee: "0.00",
      renewalAmount: "",
    };
    if (editingPrices.value.length) {
      const first = editingPrices.value[0]!;
      Object.assign(initialValues, {
        billingRuleNo: first.billingRuleNo,
        currencyCode: first.currencyCode,
      });
      for (const [index, price] of editingPrices.value.entries()) {
        const suffix = index ? `_${index}` : "";
        initialValues[`minimum${suffix}`] = price.minimumQuantity;
        initialValues[`amount${suffix}`] = minorToMajorAmount(
          price.amountMinor,
          price.currencyCode,
        );
        initialValues[`setupFee${suffix}`] = minorToMajorAmount(
          price.setupFeeMinor,
          price.currencyCode,
        );
        initialValues[`renewalAmount${suffix}`] =
          price.renewalAmountMinor === null
            ? ""
            : minorToMajorAmount(price.renewalAmountMinor, price.currencyCode);
      }
    }
    await nextTick();
    await formApi.resetForm({ values: initialValues }, { force: true });
    if (!directEdit.value) await syncExistingPrice();
    if (sequence !== initializationSequence) return;
    await nextTick();
    await formApi.resetValidate();
    formBaseline = JSON.stringify(await formApi.getValues());
    loadedIdentity = identity;
    initialized = true;
    stale.value = false;
  } catch {
    if (sequence === initializationSequence) {
      loadFailed.value = true;
      throw new Error("price_editor_read_failed");
    }
  } finally {
    if (sequence === initializationSequence) {
      loading.value = false;
      // A configuration save may advance the editable version during this
      // read. Finish against the current identity before accepting input.
      if (!loadFailed.value && incomingIdentity.value !== identity)
        void initialize().catch(() => {});
    }
  }
}

async function reload() {
  if (saving.value || loading.value) return;
  loading.value = true;
  try {
    const current = props.refresh ? await props.refresh() : props.data;
    await nextTick();
    await initialize(current);
    refreshPending.value = false;
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
}

// The general pricing entry edits an existing schedule when the selected field,
// currency and cycle already have prices, preserving every saved tier and fee.
async function syncExistingPrice(overrides: Record<string, string> = {}) {
  if (directEdit.value) return;
  const sequence = ++selectionSequence;
  const selectedValues = { ...(await formApi.getValues()), ...overrides };
  if (sequence !== selectionSequence) return;
  const candidate = selectedCandidate(selectedValues);
  quantitySelected.value = !!candidate?.quantity;
  editingPrices.value = availablePrices.value
    .filter(
      (price) =>
        candidate?.dimensionNo &&
        price.configurationPriceDimensionNo === candidate.dimensionNo &&
        price.billingRuleNo === selectedValues.billingRuleNo &&
        price.currencyCode === selectedValues.currencyCode,
    )
    .sort((a, b) => a.minimumQuantity - b.minimumQuantity);
  tierCount.value = editingPrices.value.length || 1;
  showExtraFees.value = editingPrices.value.some(
    (price) => !/^0+$/.test(price.setupFeeMinor),
  );
  automaticPricing.value = false;
  generatedPreview.value = "";
  const values: Record<string, unknown> = {
    ...selectedValues,
    enabled: editingPrices.value[0]
      ? editingPrices.value[0].status === "active"
      : true,
    autoCalculate: false,
    calculationBase: "0.00",
    calculationCoefficient: "1",
    pricingMode: candidate?.pricingMode || "per_unit",
    minimum: 0,
    amount: "",
    setupFee: "0.00",
    renewalAmount: "",
  };
  for (const [index, price] of editingPrices.value.entries()) {
    const suffix = index ? `_${index}` : "";
    values[`minimum${suffix}`] = price.minimumQuantity;
    values[`amount${suffix}`] = minorToMajorAmount(
      price.amountMinor,
      price.currencyCode,
    );
    values[`setupFee${suffix}`] = minorToMajorAmount(
      price.setupFeeMinor,
      price.currencyCode,
    );
    values[`renewalAmount${suffix}`] =
      price.renewalAmountMinor === null
        ? ""
        : minorToMajorAmount(price.renewalAmountMinor, price.currencyCode);
  }
  await formApi.setState({ schema: priceSchema(candidateOptions, currencies) });
  if (sequence === selectionSequence) {
    await nextTick();
    await formApi.resetForm({ values }, { force: true });
    await nextTick();
    await formApi.resetValidate();
  }
}

async function toggleExtraFees() {
  const values = await formApi.getValues();
  showExtraFees.value = !showExtraFees.value;
  await formApi.setState({ schema: priceSchema(candidateOptions, currencies) });
  await formApi.setValues(values);
}
async function addBand() {
  if (tierCount.value >= 16) return;
  const values = await formApi.getValues<Record<string, unknown>>();
  if (!selectedCandidate(values)?.quantity) return;
  const index = tierCount.value++;
  const previousMinimum =
    index === 1 ? 0 : Number(values[`minimum_${index - 1}`] ?? 0);
  await formApi.setState({ schema: priceSchema(candidateOptions, currencies) });
  await formApi.setValues({
    ...values,
    [`minimum_${index}`]: previousMinimum + 100,
    [`amount_${index}`]: "0.00",
    [`setupFee_${index}`]: "0.00",
    [`renewalAmount_${index}`]: "",
  });
}
async function removeBand() {
  if (tierCount.value <= 1) return;
  const values = await formApi.getValues<Record<string, unknown>>();
  tierCount.value--;
  await formApi.setState({ schema: priceSchema(candidateOptions, currencies) });
  await formApi.setValues(values);
}

async function previewCalculatedPrice() {
  const values = await formApi.getValues<Record<string, unknown>>();
  if (!values.autoCalculate) return;
  try {
    generatedPreview.value = multiplyMajorAmount(
      String(values.calculationBase || ""),
      String(values.calculationCoefficient || ""),
      String(values.currencyCode || ""),
    );
  } catch {
    generatedPreview.value = "";
    message.error(
      $t("plugin.idc-core.catalog.configurationPrice.invalidCalculation"),
    );
  }
}
async function applyCalculatedPrice() {
  if (!generatedPreview.value || !automaticPricing.value) return;
  // Calculation changes only the current purchase-price input. Renewal and
  // setup fees retain their independent merchant-entered amounts.
  await formApi.setFieldValue("amount", generatedPreview.value);
  generatedPreview.value = "";
}
watch(
  incomingIdentity,
  async () => {
    if (saving.value || (loading.value && initialized)) return;
    if (!initialized) {
      void initialize().catch(() => {});
      return;
    }
    if (incomingIdentity.value !== loadedIdentity) {
      loading.value = true;
      const values = JSON.stringify(await formApi.getValues());
      if (!formBaseline || values === formBaseline)
        void initialize().catch(() => {});
      else {
        stale.value = true;
        loading.value = false;
      }
    }
  },
  { immediate: true },
);
watch(
  () => preferences.app.locale,
  async () => {
    if (!initialized || loading.value) return;
    candidateOptions = candidates.value.map((candidate) => ({
      value: candidate.candidateToken,
      label: candidate.quantity
        ? localized(candidate.fieldLabelZhCN, candidate.fieldLabelEnUS)
        : `${localized(candidate.fieldLabelZhCN, candidate.fieldLabelEnUS)} · ${localized(candidate.candidateLabelZhCN, candidate.candidateLabelEnUS)}`,
    }));
    const values = await formApi.getValues();
    await formApi.setState({
      schema: priceSchema(candidateOptions, currencies),
    });
    await formApi.setValues(values);
  },
);
onBeforeUnmount(() => {
  initializationSequence++;
  selectionSequence++;
});
defineExpose({ save });
</script>

<template>
  <section
    class="catalog-price-editor"
    :data-testid="props.idPrefix || 'catalog-configuration-price-editor'"
  >
    <Skeleton v-if="loading && !initialized" active :paragraph="{ rows: 4 }" />
    <Alert
      v-if="stale"
      class="mb-3"
      type="warning"
      show-icon
      :message="tr('changed')"
      ><template #action
        ><Popconfirm :title="tr('reloadConfirm')" @confirm="reload"
          ><Button :data-testid="controlID('catalog-price-reload')">{{
            tr("reload")
          }}</Button></Popconfirm
        ></template
      ></Alert
    >
    <Alert
      v-if="refreshPending"
      class="mb-3"
      type="info"
      show-icon
      :message="tr('refreshPending')"
      ><template #action
        ><Button
          :data-testid="controlID('catalog-price-refresh')"
          @click="reload"
          >{{ tr("reload") }}</Button
        ></template
      ></Alert
    >
    <Alert
      v-else-if="loadFailed"
      class="mb-3"
      type="error"
      show-icon
      :message="tr('loadFailed')"
      ><template #action
        ><Button
          :data-testid="controlID('catalog-price-refresh')"
          @click="reload"
          >{{ tr("reload") }}</Button
        ></template
      ></Alert
    >
    <Alert
      v-if="writeFailed"
      class="mb-3"
      type="error"
      show-icon
      :message="tr('saveFailed')"
    />
    <Empty
      v-if="initialized && !loading && !candidates.length"
      :description="
        tr(props.data.quantityOnly ? 'noQuantityCandidates' : 'noCandidates')
      "
      :image="Empty.PRESENTED_IMAGE_SIMPLE"
    />
    <fieldset
      v-show="candidates.length"
      :disabled="loading || saving || stale || refreshPending"
      class="catalog-price-inputs"
    >
      <Alert
        v-if="quantitySelected"
        class="mb-4"
        type="info"
        :message="$t('plugin.idc-core.catalog.configurationPrice.bandHint')"
      />
      <div class="mb-3 flex flex-wrap gap-2">
        <Tag v-if="rules.length === 1">{{ billingRuleLabel(rules[0]!) }}</Tag
        ><Tag v-if="singleCurrency">{{ singleCurrency }}</Tag>
      </div>
      <Form />
      <div
        v-if="automaticPricing"
        class="mb-4 rounded border p-3"
        :data-testid="controlID('catalog-price-auto-panel')"
      >
        <Alert
          class="mb-3"
          type="info"
          :message="
            $t('plugin.idc-core.catalog.configurationPrice.calculationHint')
          "
        />
        <Button
          :data-testid="controlID('catalog-price-auto-preview')"
          @click="previewCalculatedPrice"
          >{{
            $t("plugin.idc-core.catalog.configurationPrice.previewCalculation")
          }}</Button
        >
        <div v-if="generatedPreview" class="mt-3 flex items-center gap-3">
          <span :data-testid="controlID('catalog-price-auto-result')">{{
            generatedPreview
          }}</span
          ><Button
            :data-testid="controlID('catalog-price-auto-apply')"
            @click="applyCalculatedPrice"
            >{{
              $t("plugin.idc-core.catalog.configurationPrice.applyCalculation")
            }}</Button
          >
        </div>
      </div>
      <Button
        class="mb-3"
        :data-testid="controlID('catalog-price-extra-fees')"
        @click="toggleExtraFees"
        >{{
          $t("plugin.idc-core.catalog.configurationPrice.extraFees")
        }}</Button
      >
      <Button
        v-if="quantitySelected"
        class="mr-2"
        :data-testid="controlID('catalog-configuration-price-add-band')"
        :disabled="tierCount >= 16"
        @click="addBand"
        >{{ $t("plugin.idc-core.catalog.configurationPrice.addBand") }}</Button
      >
      <Button
        v-if="quantitySelected && tierCount > 1"
        :data-testid="controlID('catalog-configuration-price-remove-band')"
        @click="removeBand"
        >{{
          $t("plugin.idc-core.catalog.configurationPrice.removeBand")
        }}</Button
      >
    </fieldset>
    <Button
      v-if="showSave && candidates.length"
      type="primary"
      :loading="saving || loading"
      :disabled="loading || saving || stale || refreshPending || loadFailed"
      :data-testid="controlID('catalog-price-save')"
      @click="save"
      >{{ tr("save") }}</Button
    >
  </section>
</template>

<style scoped>
.catalog-price-editor {
  min-width: 0;
  max-width: 100%;
}
.catalog-price-editor :deep(.catalog-price-editor-field > div) {
  min-width: 0;
}
.catalog-price-editor :deep(.ant-tag) {
  white-space: normal;
  max-width: 100%;
}
.catalog-price-inputs {
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
}
</style>
