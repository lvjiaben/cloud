<script setup lang="ts">
import type { VbenFormSchema } from "#/adapter/form";
import type {
  CatalogProductVersionConfiguration,
  CatalogProductDetail,
  CatalogCustomerFieldSelection,
} from "../catalog-client";
import { computed, markRaw, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Button,
  Checkbox,
  InputNumber,
  Switch,
  Select,
  Popconfirm,
  message,
} from "ant-design-vue";
import { useVbenModal } from "@vben/common-ui";
import { useVbenForm } from "#/adapter/form";
import { $t } from "#/locales";
import { catalogProductVersionConfigurationUpdate } from "../catalog-client";
import { catalogConfigurationRowsToValues, validateCatalogConfigurationDraftValue } from "../catalog-configuration-state";
import { previewRelationshipConditions } from "../catalog-offer-preview-state";
import {
  CustomerSelectionError,
  normalizeCustomerNumberSelection,
} from "../catalog-customer-selection";

import type {
  MerchantOptionPricingContext,
  MerchantPriceDraft,
} from "../catalog-merchant-prices";
import {
  merchantPriceDraftInput,
  MerchantPriceDraftError,
} from "../catalog-merchant-prices";
import CatalogOfferPriceEditor from "./catalog-offer-price-editor.vue";
import CatalogOptionValuesEditor from "./catalog-option-values-editor.vue";
import CatalogOfferPreview from "./catalog-offer-preview.vue";
import VbenDataGrid from "./vben-data-grid.vue";

const props = defineProps<{
  versionNo: string;
  bindingNo: string;
  configuration: CatalogProductVersionConfiguration;
  detail?: CatalogProductDetail;
  writable?: boolean;
  canPrice?: boolean;
  externalPricingDirty?: boolean;
  refresh?: () => Promise<void>;
}>();
const saving = ref(false);
const awaitingSavedRefresh = ref(false);
const savedRefreshFailed = ref(false);
const retryingSavedRefresh = ref(false);
async function retrySavedRefresh() {
  if (!props.refresh || retryingSavedRefresh.value) return;
  retryingSavedRefresh.value = true;
  try {
    await props.refresh();
    await nextTick();
    savedRefreshFailed.value = awaitingSavedRefresh.value;
  } catch {
    savedRefreshFailed.value = true;
  } finally {
    retryingSavedRefresh.value = false;
  }
}
watch(awaitingSavedRefresh, (pending) => {
  if (!pending) savedRefreshFailed.value = false;
});
const optionFormMounted = ref(false);
const optionEditorOpen = ref(false);
const optionHasInput = ref(false);
const optionAdded = ref(false);
let optionDraftBaseline = "";
let rebuildingOptionForm = false;
const optionCommitted = ref(false);
const showDetails = ref(false);
const showPreview = ref(false);
async function togglePreview() {
  await persistActiveValues();
  showPreview.value = !showPreview.value;
}
async function toggleDetails() {
  await persistActiveValues();
  await buildForm();
}
const stale = ref(false);
const priceStale = ref(false);
let priceBaseline = "";
let initialized = false;
let baseline = "";
let savedIdentity = "";
let draftBaseline = "";
const workspaceText = (key: string) =>
  $t(`plugin.idc-core.catalog.workspace.${key}`);
function configurationIdentity(
  configuration: CatalogProductVersionConfiguration,
  bindingNo: string,
) {
  const shape = (fields: typeof configuration.customerConfigurableFields) =>
    fields?.map(({ staticCandidates: _choices, ...field }) => field);
  const purchaseKeys = new Set([
    ...(configuration.productDeclaration?.fields
      .filter(
        (field) =>
          field.required &&
          ["integer", "single_select", "boolean"].includes(field.type),
      )
      .map((field) => field.key) ?? []),
    ...(configuration.customerFieldSelections
      ?.map((selection) => selection.productFieldKey)
      .filter(Boolean) ?? []),
  ]);
  return JSON.stringify([
    bindingNo,
    configuration.merchantValues.filter((value) =>
      purchaseKeys.has(value.fieldKey),
    ),
    shape(configuration.productDeclaration?.fields),
    shape(configuration.customerDeclaration?.fields),
    shape(configuration.customerConfigurableFields),
    configuration.customerFieldSelections,
    configuration.pendingAutomationFields,
    configuration.importedConfiguration,
  ]);
}

const emit = defineEmits<{
  saved: [configuration: CatalogProductVersionConfiguration];
  busy: [value: boolean];
}>();
let versionNo = "";
let configuration: CatalogProductVersionConfiguration | undefined;
let draftValues: Record<string, unknown> = {};
let directorySequence = 0;
let disposed = false;
const draftRevision = ref(0);
const selectedKeys = ref<string[]>([]);
const availableFields = ref<
  NonNullable<CatalogProductVersionConfiguration["customerConfigurableFields"]>
>([]);
const addingField = ref<string>();
const localType = ref<Field["type"]>("single_select");
const activeKey = ref("");
const overviewRows = computed(() => {
  draftRevision.value;
  return visibleFields.value.map((field) => {
    const index = availableFields.value.findIndex(
      (item) => item.key === field.key,
    );
    const prefix = `option_${index}`;
    const choices = parseChildren(
      draftValues[
        `${prefix}_${field.type === "integer" ? "numbers" : "choices"}`
      ],
      field.type === "integer",
    );
    const interfaceChoices = !isLocal(field)
      ? props.configuration.customerResolvedCandidates[field.key] ||
        field.staticCandidates ||
        []
      : [];
    return {
      key: field.key,
      name: fieldLabel(field),
      sortOrder: Number(draftValues[`${prefix}_order`] ?? index),
      upgradeEnabled: draftValues[`${prefix}_upgrade_enabled`] !== false,
      customerVisible: draftValues[`${prefix}_customer_visible`] !== false,
      control: tr(`type_${field.type}`),
      contents: choices.length
        ? choices
            .map(
              (choice) =>
                (zh() ? choice.label.zhCN : choice.label.enUS) || choice.value,
            )
            .join("、")
        : field.type === "integer"
          ? `${draftValues[`${prefix}_minimum`] ?? field.minimum ?? ""} — ${draftValues[`${prefix}_maximum`] ?? field.maximum ?? ""}`
          : interfaceChoices.length
            ? tr("interfaceChoiceCount", {
                count: String(interfaceChoices.length),
              })
            : tr("customerEntered"),
      binding: String(
        draftValues[`${prefix}_product_field`] ||
          (isLocal(field) ? "" : field.key),
      ),
      required: isRequiredOption(field) || !!draftValues[`${prefix}_required`],
      pendingMapping: unresolvedMapping(field, index),
    };
  });
});
const overviewColumns = computed(() => [
  { key: "sortOrder", title: tr("order"), width: 95 },
  { key: "name", title: tr("configurationName"), minWidth: 180 },
  { key: "control", title: tr("configurationType"), width: 155 },
  { key: "contents", title: tr("sellableContent"), minWidth: 120 },
  { key: "binding", title: tr("parameterBinding"), minWidth: 150 },
  { key: "customerVisible", title: tr("visibility"), width: 90 },
  { key: "upgradeEnabled", title: tr("upgradeEnabled"), width: 110 },
  { key: "actions", title: tr("actions"), width: 175, fixed: "right" as const },
]);
const previewFields = computed(() => {
  draftRevision.value;
  return visibleFields.value
    .filter((field) => {
      const index = availableFields.value.findIndex(
        (item) => item.key === field.key,
      );
      return (
        isRequiredOption(field) || !!draftValues[`option_${index}_enabled`]
      );
    })
    .map((field) => {
      const index = availableFields.value.findIndex(
        (item) => item.key === field.key,
      );
      const prefix = `option_${index}`;
      const choices = parseChildren(
        draftValues[
          `${prefix}_${field.type === "integer" ? "numbers" : "choices"}`
        ],
        field.type === "integer",
      );
      return {
        ...field,
        customerVisible: draftValues[`${prefix}_customer_visible`] !== false,
        defaultValue: defaultForField(field, index, draftValues),
        productFieldKey: String(
          draftValues[`${prefix}_product_field`] ??
            props.configuration.customerProductFields?.[field.key] ??
            (isLocal(field) ? "" : field.key),
        ),
        productChoiceBindings: draftValues[`${prefix}_choice_bindings`] as
          | Record<string, string>
          | undefined,
        label: {
          zhCN: String(draftValues[`${prefix}_name_zhCN`] || field.label.zhCN),
          enUS: String(draftValues[`${prefix}_name_enUS`] || field.label.enUS),
        },
        minimum: String(
          draftValues[`${prefix}_minimum`] ?? field.minimum ?? "0",
        ),
        maximum: String(
          draftValues[`${prefix}_maximum`] ??
            field.maximum ??
            "9007199254740991",
        ),
        displayControl: String(draftValues[`${prefix}_control`] || "auto"),
        quantityStep: String(draftValues[`${prefix}_step`] || "1"),
        unit: String(draftValues[`${prefix}_unit`] || ""),
        required:
          isRequiredOption(field) || !!draftValues[`${prefix}_required`],
        group: String(
          draftValues[`${prefix}_group_${zh() ? "zhCN" : "enUS"}`] || "",
        ),
        description: {
          zhCN: String(
            draftValues[`${prefix}_description_zhCN`] ||
              field.description?.zhCN ||
              "",
          ),
          enUS: String(
            draftValues[`${prefix}_description_enUS`] ||
              field.description?.enUS ||
              "",
          ),
        },
        staticCandidates:
          choices.length || isLocal(field)
            ? choices
            : props.configuration.customerResolvedCandidates[field.key] ||
              field.staticCandidates ||
              [],
      };
    });
});
const previewConditions = computed(() =>
  previewRelationshipConditions(
    previewFields.value,
    props.configuration.customerDeclaration?.conditions,
    props.configuration.customerChoiceRules,
    props.configuration.directoryRelationships,
  ),
);
const validationError = ref("");
const switching = ref(false);
const visibleFields = computed(() => {
  draftRevision.value;
  const orders = new Map(
    availableFields.value.map((field, index) => [
      field.key,
      Number(draftValues[`option_${index}_order`] ?? index),
    ]),
  );
  return availableFields.value
    .filter(
      (field) =>
        selectedKeys.value.includes(field.key) &&
        !availableFields.value.some(
          (other, index) =>
            other.key !== field.key &&
            selectedKeys.value.includes(other.key) &&
            draftValues[`option_${index}_product_field`] === field.key,
        ),
    )
    .sort(
      (left, right) =>
        (orders.get(left.key) ?? 0) - (orders.get(right.key) ?? 0),
    );
});
const activeProductField = computed(() => {
  draftRevision.value;
  const index = availableFields.value.findIndex(
    (field) => field.key === activeKey.value,
  );
  return configuration?.productDeclaration?.fields.find(
    (field) => field.key === draftValues[`option_${index}_product_field`],
  );
});
async function importProductChoices() {
  await persistActiveValues();
  const index = availableFields.value.findIndex(
      (field) => field.key === activeKey.value,
    ),
    target = activeProductField.value || availableFields.value[index];
  if (!target) return;
  const readChoices = () =>
    (isLocal(availableFields.value[index]!)
      ? props.configuration.productResolvedCandidates[target.key]
      : props.configuration.customerResolvedCandidates[target.key]) ||
    target.staticCandidates ||
    [];
  let choices = readChoices();
  if (!choices.length && props.refresh) {
    await props.refresh();
    await nextTick();
    choices = readChoices();
    await persistActiveValues();
  }
  draftValues[`option_${index}_choices`] = choices.map((item) => ({
    ...item,
    label: { ...item.label },
  }));
  draftValues[`option_${index}_choice_bindings`] = {};
  await buildForm();
}
function productOptionSelections(data: CatalogProductVersionConfiguration) {
  const selections = [...(data.customerFieldSelections ?? [])];
  const values = catalogConfigurationRowsToValues(
    data.merchantValues,
    data.productDeclaration,
  );
  for (const field of data.productDeclaration?.fields ?? []) {
    if (
      !field.required ||
      !["integer", "single_select", "boolean"].includes(field.type) ||
      values[field.key] === undefined ||
      data.customerConfigurableFields?.some(
        (customer) => customer.required && customer.key === field.key,
      ) ||
      selections.some(
        (selection) =>
          selection.productFieldKey === field.key ||
          selection.fieldKey === field.key ||
          data.customerProductFields?.[selection.fieldKey] === field.key,
      )
    )
      continue;
    const key = `local_product_${field.key}`;
    if (
      key.length > 64 ||
      selections.some((selection) => selection.fieldKey === key)
    )
      continue;
    const value = String(values[field.key]);
    const options =
      data.productResolvedCandidates[field.key] || field.staticCandidates || [];
    const candidate = options.find((option) => option.value === value) || {
      value,
      label: { zhCN: value, enUS: value },
    };
    const { dynamicCandidates: _dynamic, ...localField } = field;
    selections.push({
      fieldKey: key,
      productFieldKey: field.key,
      displayNameZhCN: field.label.zhCN,
      displayNameEnUS: field.label.enUS,
      displayControl: field.type === "boolean" ? "auto" : "select",
      sortOrder: selections.length,
      quantityStep: field.type === "integer" ? "1" : undefined,
      ...(field.type === "integer"
        ? {
            minimum: value,
            maximum: value,
            numericChoices: [value],
            numericPricing: "quantity" as const,
          }
        : {}),
      localField: {
        ...localField,
        key,
        required: true,
        ...(field.type === "single_select"
          ? { staticCandidates: [candidate] }
          : {}),
        ...(field.type === "integer" ? { minimum: value, maximum: value } : {}),
      },
    });
  }
  return selections;
}
const merchantPriceDrafts = ref<Record<string, MerchantPriceDraft>>({});
function draftIdentity() {
  return JSON.stringify([
    selectedKeys.value,
    availableFields.value,
    draftValues,
    merchantPriceDrafts.value,
  ]);
}
const activeField = computed(() =>
  availableFields.value.find((field) => field.key === activeKey.value),
);
const priceable = computed(
  () =>
    activeField.value &&
    [
      "single_select",
      "multi_select",
      "tree_select",
      "address",
      "integer",
      "boolean",
    ].includes(activeField.value.type),
);
const activePricing = computed<MerchantOptionPricingContext>(() => {
  draftRevision.value;
  const field = activeField.value;
  const index = availableFields.value.findIndex(
    (item) => item.key === activeKey.value,
  );
  const prefix = `option_${index}`;
  const quantity =
    field?.type === "integer" &&
    draftValues[`${prefix}_numeric_pricing`] !== "per_choice";
  let options = parseChildren(
    draftValues[
      `${prefix}_${field?.type === "integer" ? "numbers" : "choices"}`
    ],
    field?.type === "integer",
  );
  if (field?.type === "boolean")
    options = [
      {
        value: "false",
        label: {
          zhCN: $t("plugin.idc-core.catalog.offerPrices.booleanNo"),
          enUS: $t("plugin.idc-core.catalog.offerPrices.booleanNo"),
        },
      },
      {
        value: "true",
        label: {
          zhCN: $t("plugin.idc-core.catalog.offerPrices.booleanYes"),
          enUS: $t("plugin.idc-core.catalog.offerPrices.booleanYes"),
        },
      },
    ];
  else if (!options.length && field && !isLocal(field))
    options =
      props.configuration.customerResolvedCandidates[field.key] ||
      field.staticCandidates ||
      [];
  return {
    fieldKey: activeKey.value,
    quantity,
    options,
    billingRules:
      props.detail?.billingRules.filter((rule) => rule.status === "active") ||
      [],
    prices: props.detail?.prices || [],
    targets: props.configuration.merchantPriceTargets || [],
    currencies: [
      ...new Set(
        props.detail?.prices
          .filter(
            (price) =>
              !price.configurationPriceDimensionNo && price.status === "active",
          )
          .map((price) => price.currencyCode) || [],
      ),
    ],
  };
});
function changePrice(key: string, draft: MerchantPriceDraft) {
  optionHasInput.value = true;
  if (props.writable === false || saving.value || awaitingSavedRefresh.value)
    return;
  if (!Object.keys(merchantPriceDrafts.value).length)
    priceBaseline = JSON.stringify(props.detail?.prices || []);
  merchantPriceDrafts.value = { ...merchantPriceDrafts.value, [key]: draft };
}
watch(
  () => props.detail?.prices,
  () => {
    if (saving.value || awaitingSavedRefresh.value) return;
    const current = JSON.stringify(props.detail?.prices || []);
    if (
      Object.keys(merchantPriceDrafts.value).length &&
      current !== priceBaseline
    )
      priceStale.value = true;
    else if (!Object.keys(merchantPriceDrafts.value).length)
      priceBaseline = current;
  },
  { deep: true },
);
const draftLabels = ref<Record<string, { zhCN: string; enUS: string }>>({});
const localTypes = [
  "single_select",
  "multi_select",
  "boolean",
  "integer",
  "decimal",
  "single_line",
  "multi_line",
  "date",
  "datetime",
  "tree_select",
  "address",
];
const zh = () => preferences.app.locale === "zh-CN";
const tr = (key: string, params: Record<string, string> = {}) =>
  $t(`plugin.idc-core.catalog.customerOptions.${key}`, params);
const [Form, formApi] = useVbenForm({
  schema: [],
  showDefaultActions: false,
  commonConfig: {
    labelWidth: 112,
    formItemClass: "catalog-customer-option-field",
  },
  handleValuesChange: (values) => {
    if (switching.value || saving.value || awaitingSavedRefresh.value || rebuildingOptionForm) return;
    optionHasInput.value = true;
    const prefix = prefixForActive();
    for (const [key, value] of Object.entries(values))
      if (key.startsWith(`${prefix}_`)) draftValues[key] = value;
    draftRevision.value++;
  },
});
const formEditingDisabled = computed(
  () => props.writable === false || saving.value || awaitingSavedRefresh.value,
);
watch(formEditingDisabled, (disabled) => {
  void formApi.setState({ commonConfig: { disabled } });
});
type Field = (typeof availableFields.value)[number];
const isLocal = (field: Field) => field.key.startsWith("local_");
const isCore = (field: Field) => field.required && !isLocal(field);
// Required SKU parameters stay in the unified editor. This does not freeze
// their sellable quantity bounds, unlike an interface-owned customer field.
function isRequiredOption(field: Field) {
  if (configuration?.pendingAutomationFields?.includes(field.key)) return false;
  if (isCore(field)) return true;
  return (
    !!field.required &&
    !!configuration?.productDeclaration?.fields.some(
      (target) =>
        target.required && field.key === `local_product_${target.key}`,
    )
  );
}

function unresolvedMapping(field: Field, index: number) {
  return (
    !!props.bindingNo &&
    !!configuration?.pendingAutomationFields?.includes(field.key) &&
    !draftValues[`option_${index}_product_field`] &&
    !draftValues[`option_${index}_information_only`]
  );
}

const discrete = (value: unknown) => value === "select" || value === "radio";
function fieldLabel(field: Field) {
  const label = draftLabels.value[field.key] || field.label;
  return (zh() ? label?.zhCN : label?.enUS) || field.key;
}
async function syncDraftLabel(field: Field, index: number) {
  const values = await formApi.getValues();
  draftLabels.value[field.key] = {
    zhCN: String(
      values[`option_${index}_name_zhCN`] || field.label?.zhCN || field.key,
    ),
    enUS: String(
      values[`option_${index}_name_enUS`] || field.label?.enUS || field.key,
    ),
  };
}

function prefixForActive() {
  return `option_${availableFields.value.findIndex((field) => field.key === activeKey.value)}`;
}
function defaultDraftValue(value:unknown): string | boolean | string[] | undefined {
  if (typeof value === "string" || typeof value === "boolean") return value;
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  if (Array.isArray(value) && value.every(item => typeof item === "string")) return value;
  return undefined;
}
function defaultForField(field: Field, index:number, values:Record<string,unknown>): unknown {
  const prefix = `option_${index}`;
  if (values[`${prefix}_default`] !== undefined && values[`${prefix}_default`] !== null) return values[`${prefix}_default`];
  const choices = parseChildren(values[`${prefix}_${field.type === "integer" ? "numbers" : "choices"}`], field.type === "integer");
  const options = choices.length ? choices : configuration?.customerResolvedCandidates[field.key] || field.staticCandidates || [];
  if (field.type === "integer") return discrete(values[`${prefix}_control`]) && options.length ? Number(options[0]!.value) : Number(values[`${prefix}_minimum`] ?? field.minimum ?? 0);
  if (field.type === "boolean") return false;
  if (["single_select","tree_select"].includes(field.type)) return options[0]?.value;
  if (["multi_select","address"].includes(field.type)) return options.length ? [options[0]!.value] : [];
  return "";
}
async function buildForm() {
  if (!optionFormMounted.value) return;
  rebuildingOptionForm = true;
  try {
  const schema: VbenFormSchema[] = [];
  for (const [index, field] of availableFields.value.entries()) {
    if (field.key !== activeKey.value) continue;
    const prefix = `option_${index}`;
    const enabled = `${prefix}_enabled`;
    const visible = {
      triggerFields: [enabled],
      show: (values: Record<string, unknown>) => !!values[enabled],
    };
    const required = {
      ...visible,
      required: (values: Record<string, unknown>) => !!values[enabled],
    };
    schema.push({
      fieldName: `${prefix}_customer_visible`, label: tr("visibility"), component: "Switch",
      componentProps: { "aria-label": tr("visibility"), checkedChildren: tr("shown"), unCheckedChildren: tr("hidden"), "data-testid": `${prefix}_customer_visible` },
    });
    draftValues[`${prefix}_customer_visible`] ??= true;
    const defaultChoices = (values: Record<string, unknown>) => {
      const choices = parseChildren(values[`${prefix}_${field.type === "integer" ? "numbers" : "choices"}`], field.type === "integer");
      return choices.length ? choices : configuration?.customerResolvedCandidates[field.key] || field.staticCandidates || [];
    };
    schema.push({
      fieldName: `${prefix}_default`, label: tr("defaultValue"),
      component: ["single_select", "multi_select", "tree_select", "address"].includes(field.type) ? "Select" : field.type === "integer" ? "InputNumber" : field.type === "boolean" ? "Switch" : "Input",
      componentProps: (values) => ({
        "data-testid": `${prefix}_default`, class: "w-full", placeholder: tr("defaultPlaceholder"),
        ...(["single_select", "multi_select", "tree_select", "address"].includes(field.type) ? { virtual: false, options: defaultChoices(values).map(choice => ({ value: choice.value, label: zh() ? choice.label.zhCN : choice.label.enUS })), ...(["multi_select", "address"].includes(field.type) ? {mode:"multiple"} : {}) } : {}),
        ...(field.type === "integer" ? {precision:0,min:Number(values[`${prefix}_minimum`] ?? field.minimum ?? 0),max:Number(values[`${prefix}_maximum`] ?? field.maximum ?? 2147483647)} : {}),
      }),
      dependencies: {triggerFields:[`${prefix}_customer_visible`], show: (values:Record<string,unknown>) => values[`${prefix}_customer_visible`] === false},
    });
    draftValues[enabled] ??= true;
    for (const language of ["zhCN", "enUS"]) {
      if (language === "enUS" && !showDetails.value) continue;
      schema.push({
        fieldName: `${prefix}_name_${language}`,
        label: tr(`name_${language}`),
        component: "Input",
        componentProps: {
          maxlength: 128,
          onBlur: () => syncDraftLabel(field, index),
          "data-testid": `${prefix}_name_${language}`,
        },
        dependencies: required,
      });
      draftValues[`${prefix}_name_${language}`] ??=
        field.label?.[language as "zhCN" | "enUS"] || fieldLabel(field);
      if (!showDetails.value) continue;
      schema.push({
        fieldName: `${prefix}_group_${language}`,
        label: tr(`group_${language}`),
        component: "Input",
        componentProps: { maxlength: 128 },
        dependencies: visible,
      });
      schema.push({
        fieldName: `${prefix}_description_${language}`,
        label: tr(`description_${language}`),
        component: "Textarea",
        componentProps: { maxlength: 2048, rows: 2 },
        dependencies: visible,
      });
    }
    schema.push({
      fieldName: `${prefix}_order`,
      label: tr("order"),
      component: "InputNumber",
      componentProps: {
        min: 0,
        max: 1024,
        precision: 0,
        "data-testid": `${prefix}_order`,
      },
      dependencies: required,
    });
    draftValues[`${prefix}_order`] ??= index;
    schema.push({
      fieldName: `${prefix}_upgrade_enabled`,
      label: tr("upgradeEnabled"),
      component: "Switch",
      componentProps: { "data-testid": `${prefix}_upgrade_enabled` },
      dependencies: visible,
    });
    draftValues[`${prefix}_upgrade_enabled`] ??= true;
    const upgradeVisible = {
      triggerFields: [enabled, `${prefix}_upgrade_enabled`],
      show: (values: Record<string, unknown>) => !!values[enabled] && values[`${prefix}_upgrade_enabled`] !== false,
    };
    if (field.type === "integer") {
      for (const bound of ["minimum", "maximum"] as const) {
        schema.push({ fieldName: `${prefix}_upgrade_${bound}`, label: tr(`upgrade_${bound}`), component: "InputNumber",
          componentProps: { min: Number(field.minimum ?? 0), max: Number(field.maximum ?? 2147483647), precision: 0,
            placeholder: tr("upgradeAllRange"), "data-testid": `${prefix}_upgrade_${bound}` }, dependencies: upgradeVisible });
      }
    } else if (field.type === "single_select") {
      schema.push({ fieldName: `${prefix}_upgrade_choices`, label: tr("upgradeChoices"), component: "Select",
        componentProps: (values) => {
          const choices = parseChildren(values[`${prefix}_choices`]);
          const options = choices.length ? choices : configuration?.customerResolvedCandidates[field.key] || field.staticCandidates || [];
          return { mode: "multiple", allowClear: true, virtual: false, class: "w-full", style: { width: "100%" }, placeholder: tr("upgradeAllRange"),
            options: options.map(choice => ({ value: choice.value, label: zh() ? choice.label.zhCN : choice.label.enUS })),
            "data-testid": `${prefix}_upgrade_choices` };
        }, dependencies: upgradeVisible });
    }

    schema.push({
      fieldName: `${prefix}_unit`,
      label: tr("unit"),
      component: "Input",
      componentProps: { maxlength: 32 },
      dependencies: visible,
    });
    schema.push({
      fieldName: `${prefix}_advanced`,
      label: tr("advanced"),
      component: "Switch",
      dependencies: visible,
    });
    if (isLocal(field)) {
      if (configuration?.productDeclaration?.fields.length || props.bindingNo)
        schema.push({
          fieldName: `${prefix}_product_field`,
          label: tr("parameterBinding"),
          component: "Input",
          componentProps: {
            maxlength: 128,
            placeholder: tr("parameterPlaceholder"),
            "data-testid": `${prefix}_product_field`,
            onChange: async (event: Event) => {
              const value = (event.target as HTMLInputElement).value;
              draftValues[`${prefix}_product_field`] = value;
              draftValues[`${prefix}_information_only`] = !value;
              await formApi.setFieldValue(`${prefix}_information_only`, !value, false);
              if (value) {
                draftValues[`${prefix}_required`] = true;
                await formApi.setFieldValue(`${prefix}_required`, true, false);
              }
              draftRevision.value++;
            },
          },
          dependencies: visible,
        });
      if (draftValues[`${prefix}_product_field`]) draftValues[`${prefix}_required`] = true;

    }
    const controls =
      field.type === "integer"
        ? ["auto", "input", "slider", "select", "radio"]
        : field.type === "single_select"
          ? ["auto", "select", "radio"]
          : ["auto"];
    schema.push({
      fieldName: `${prefix}_control`,
      label: tr("control"),
      component: "Select",
      componentProps: {
        class: "w-full",
        style: { maxWidth: "100%" },
        "data-testid": `${prefix}_control`,
        options: controls.map((value) => ({
          value,
          label: tr(`control_${value}`),
        })),
      },
      dependencies: required,
    });
    draftValues[`${prefix}_control`] ??= "auto";
    if (field.type === "integer") {
      if (isCore(field)) {
        draftValues[`${prefix}_minimum`] ??= Number(field.minimum);
        draftValues[`${prefix}_maximum`] ??= Number(field.maximum);
      }
      schema.push({
        fieldName: `${prefix}_numeric_pricing`,
        label: tr("numericPricing"),
        component: "Select",
        componentProps: {
          class: "w-full",
          style: { maxWidth: "100%" },
          "data-testid": `${prefix}_numeric_pricing`,
          options: [
            { value: "quantity", label: tr("quantityPricing") },
            { value: "per_choice", label: tr("choicePricing") },
          ],
        },
        dependencies: visible,
      });
      draftValues[`${prefix}_numeric_pricing`] ??= "quantity";
      for (const bound of ["minimum", "maximum"])
        schema.push({
          fieldName: `${prefix}_${bound}`,
          label: tr(bound),
          component: "InputNumber",
          componentProps: {
            "data-testid": `${prefix}_${bound}`,
            precision: 0,
            disabled: isCore(field),
            min: Number(field.minimum),
            max: Number(field.maximum),
            class: "w-full",
          },
          dependencies: {
            triggerFields: [enabled, `${prefix}_control`],
            show: (v: Record<string, unknown>) =>
              !!v[enabled] && !discrete(v[`${prefix}_control`]),
            required: (v: Record<string, unknown>) =>
              !!v[enabled] && !discrete(v[`${prefix}_control`]),
          },
        });
      schema.push({
        fieldName: `${prefix}_step`,
        label: tr("step"),
        component: "InputNumber",
        componentProps: {
          min: 1,
          max: 9007199254740991,
          precision: 0,
          "data-testid": `${prefix}_step`,
        },
        dependencies: required,
      });
      draftValues[`${prefix}_step`] ??= 1;
      schema.push({
        fieldName: `${prefix}_numbers`,
        label: tr("numbers"),
        component: markRaw(CatalogOptionValuesEditor),
        modelPropName: "value",
        componentProps: {
          numeric: true,
          multilingual: showDetails.value,
          "data-testid": `${prefix}_numbers`,
        },
        dependencies: {
          triggerFields: [enabled, `${prefix}_control`],
          show: (v: Record<string, unknown>) =>
            !!v[enabled] && discrete(v[`${prefix}_control`]),
          required: (v: Record<string, unknown>) =>
            !!v[enabled] && discrete(v[`${prefix}_control`]),
        },
      });
    } else if (
      ["single_select", "multi_select", "tree_select", "address"].includes(
        field.type,
      )
    ) {
      if (!isLocal(field) && draftValues[`${prefix}_choices`] === undefined) {
        draftValues[`${prefix}_choices`] = (configuration?.customerResolvedCandidates[field.key] || field.staticCandidates || []).map(choice => ({ ...choice, label: { ...choice.label } }));
      }
      schema.push({
        fieldName: `${prefix}_choices`,
        label: tr("childChoices"),
        component: markRaw(CatalogOptionValuesEditor),
        modelPropName: "value",
        componentProps: (values) => ({
          multilingual: showDetails.value,
          upstream:
            isLocal(field) &&
            field.type === "single_select" &&
            values[`${prefix}_product_field`]
              ? {
                  bindings: (values[`${prefix}_choice_bindings`] ||
                    {}) as Record<string, string>,
                  choices:
                    activeProductField.value?.type === "single_select"
                      ? configuration?.productResolvedCandidates[
                          activeProductField.value.key
                        ] ||
                        activeProductField.value.staticCandidates ||
                        []
                      : undefined,
                  text:
                    !activeProductField.value ||
                    activeProductField.value.type === "single_line" &&
                    configuration?.productChoiceFields?.includes(
                      activeProductField.value.key,
                    ),
                }
              : undefined,
          "onUpdate:bindings": async (bindings: Record<string, string>) => {
            draftValues[`${prefix}_choice_bindings`] = bindings;
            // This metadata travels with the field snapshot when switching
            // rows; do not let an older form value overwrite the new mapping.
            await formApi.setFieldValue(
              `${prefix}_choice_bindings`,
              bindings,
              false,
            );
            draftRevision.value++;
          },
          "data-testid": `${prefix}_choices`,
        }),
        dependencies: isCore(field) ? visible : required,
      });
    }
  }
  const visibleSchema = showDetails.value
    ? schema
    : schema.filter(
        (field) =>
          ![
            `${prefixForActive()}_order`,
            `${prefixForActive()}_advanced`,
          ].includes(field.fieldName),
      );
  await formApi.setState({
    schema: visibleSchema,
    commonConfig: { disabled: formEditingDisabled.value },
  });
  await nextTick();
  const index = availableFields.value.findIndex(
    (field) => field.key === activeKey.value,
  );
  await formApi.setValues(
    Object.fromEntries(
      Object.entries(draftValues).filter(([key]) =>
        key.startsWith(`option_${index}_`),
      ),
    ),
    false,
  );
  // A changed field schema mounts before its draft values are restored.
  // Discard those transient required errors; Save validates the restored form.
  await nextTick();
  await formApi.resetValidate();
  } finally { rebuildingOptionForm = false; }
}
async function persistActiveValues() {
  if (!optionFormMounted.value) return;
  const index = availableFields.value.findIndex(
    (field) => field.key === activeKey.value,
  );
  if (index < 0) return;
  for (const [key, value] of Object.entries(await formApi.getValues()))
    if (key.startsWith(`option_${index}_`)) draftValues[key] = value;
  draftRevision.value++;
}
let optionSnapshot: {fields: Field[]; keys:string[]; values:Record<string,unknown>; prices:Record<string,MerchantPriceDraft>; labels:Record<string,{zhCN:string;enUS:string}>; error:string; active:string} | undefined;
const newLocal = ref(false);
let optionApplied = false;
const [OptionModal, optionModal] = useVbenModal({
  destroyOnClose:false,
  async onOpened() { optionFormMounted.value = true; optionEditorOpen.value = true; await buildForm(); optionDraftBaseline = draftIdentity(); optionHasInput.value = false; },
  onBeforeClose() { return !saving.value && !awaitingSavedRefresh.value; },
  async onClosed() {
    if (!optionApplied && optionSnapshot) {
      availableFields.value = optionSnapshot.fields;
      selectedKeys.value = optionSnapshot.keys;
      draftValues = optionSnapshot.values;
      merchantPriceDrafts.value = optionSnapshot.prices;
      draftLabels.value = optionSnapshot.labels;
      validationError.value = optionSnapshot.error;
      activeKey.value = optionSnapshot.active;
      draftRevision.value++;
      await buildForm();
    }
    optionSnapshot = undefined;
    newLocal.value = false;
    optionEditorOpen.value = false;
    optionHasInput.value = false;
    optionAdded.value = false;
  },
});
watch([saving,awaitingSavedRefresh], ([busy,pending]) => {
  if (!busy && !pending && optionCommitted.value) {
    optionApplied = true;
    optionCommitted.value = false;
    void optionModal.close();
  }
});
async function snapshotOption() {
  await persistActiveValues();
  optionApplied = false;
  optionSnapshot = JSON.parse(JSON.stringify({fields:availableFields.value, keys:selectedKeys.value,
    values:draftValues, prices:merchantPriceDrafts.value, labels:draftLabels.value, error:validationError.value, active:activeKey.value}));
}
const rowChangesPending = ref(false);
const pendingRowOrders = ref<Record<string, number | null>>({});
function commitRowOrder(key:string) {
  const value = pendingRowOrders.value[key];
  if (value !== undefined && value !== null) setRowOrder(key, value);
  delete pendingRowOrders.value[key];
}
function setRowOrder(key: string, value: unknown) {
  const index = availableFields.value.findIndex(field => field.key === key);
  if (index < 0 || value === null || value === undefined) return;
  if (Number(draftValues[`option_${index}_order`] ?? index) === Number(value)) return;
  draftValues[`option_${index}_order`] = Number(value);
  if (key === activeKey.value) void formApi.setFieldValue(`option_${index}_order`, Number(value), false);
  rowChangesPending.value = true;
  draftRevision.value++;
}
async function setRowUpgrade(key: string, enabled: boolean) {
  const index = availableFields.value.findIndex(field => field.key === key);
  if (index < 0) return;
  draftValues[`option_${index}_upgrade_enabled`] = enabled;
  if (key === activeKey.value) await formApi.setFieldValue(`option_${index}_upgrade_enabled`, enabled, false);
  rowChangesPending.value = true;
  draftRevision.value++;
}
const configurationDirty = computed(() => {
  draftRevision.value;
  const pendingOrder = Object.entries(pendingRowOrders.value).some(([key,value]) => {
    const index = availableFields.value.findIndex(field => field.key === key);
    return index >= 0 && (value === null || Number(value) !== Number(draftValues[`option_${index}_order`] ?? index));
  });
  return initialized && (pendingOrder || rowChangesPending.value || (optionEditorOpen.value && (optionAdded.value || newLocal.value || (optionHasInput.value && draftIdentity() !== optionDraftBaseline))));
});
defineExpose({ save, dirty: configurationDirty });
async function editOption(key:string) {
  await snapshotOption(); await selectField(key); optionModal.open();
}
async function copyOption(key:string) {
  if (!canWriteConfiguration() || availableFields.value.length >= 128) return;
  await snapshotOption();
  await persistActiveValues();
  const index = availableFields.value.findIndex(field => field.key === key);
  const source = availableFields.value[index];
  if (!source) return;
  const newKey = `local_${crypto.randomUUID().replaceAll("-", "")}`;
  const field = JSON.parse(JSON.stringify({...source,key:newKey})) as Field;
  delete field.dynamicCandidates;
  availableFields.value.push(field);
  const targetIndex = availableFields.value.length - 1;
  const sourcePrefix = `option_${index}_`, targetPrefix = `option_${targetIndex}_`;
  for (const [name,value] of Object.entries(draftValues)) {
    if (name.startsWith(sourcePrefix)) draftValues[targetPrefix + name.slice(sourcePrefix.length)] = JSON.parse(JSON.stringify(value ?? null));
  }
  const label = draftLabels.value[key] || source.label;
  Object.assign(draftValues, {
    [`${targetPrefix}name_zhCN`]: `${label.zhCN}${tr("copySuffix")}`,
    [`${targetPrefix}name_enUS`]: `${label.enUS}${tr("copySuffix")}`,
    [`${targetPrefix}order`]: targetIndex,
    [`${targetPrefix}product_field`]: "",
    [`${targetPrefix}choice_bindings`]: {},
    [`${targetPrefix}information_only`]: true,
    [`${targetPrefix}required`]: source.required,
    [`${targetPrefix}enabled`]: true,
  });
  if (["single_select","multi_select","tree_select","address"].includes(field.type) && !draftValues[`${targetPrefix}choices`]) {
    draftValues[`${targetPrefix}choices`] = JSON.parse(JSON.stringify(configuration?.customerResolvedCandidates[key] || source.staticCandidates || []));
  }
  draftLabels.value[newKey] = {zhCN:String(draftValues[`${targetPrefix}name_zhCN`]),enUS:String(draftValues[`${targetPrefix}name_enUS`])};
  selectedKeys.value.push(newKey);
  newLocal.value = true; localType.value = field.type;
  await selectField(newKey); optionModal.open();
}
async function addQuickOption() {
  if (!addingField.value) return;
  await snapshotOption(); await addOption(); optionAdded.value = true; optionModal.open();
}
async function addCustomOption() {
  if (availableFields.value.length >= 128) return;
  await snapshotOption(); newLocal.value = true; await addLocal(); optionModal.open();
}
async function changeLocalType(raw:unknown) {
  const value = String(raw) as Field['type'];
  if (!localTypes.includes(value)) return;
  const field = availableFields.value.find(item => item.key === activeKey.value);
  if (!newLocal.value || !field) return;
  optionHasInput.value = true;
  await persistActiveValues(); field.type = value;
  Object.assign(field, value === 'integer' ? {minimum:'0', maximum:'2147483647'} : {});
  await buildForm();
}
async function saveOption() {
  await save();
}
async function selectField(key: string) {
  if (switching.value) return;
  switching.value = true;
  try {
    await persistActiveValues();
    activeKey.value = key;
    if (!selectedKeys.value.includes(key))
      draftValues[
        `option_${availableFields.value.findIndex((field) => field.key === key)}_enabled`
      ] ??= false;
    await buildForm();
  } finally {
    switching.value = false;
  }
}
async function addOption() {
  if (addingField.value?.startsWith("product:")) {
    const target = configuration?.productDeclaration?.fields.find(
      (field) => field.key === addingField.value?.slice(8),
    );
    if (!target) return;
    const key = `local_product_${target.key}`;
    if (availableFields.value.some((field) => field.key === key)) {
      selectedKeys.value.push(key);
      addingField.value = undefined;
      await selectField(key);
      return;
    }
    const choices =
      configuration?.productResolvedCandidates[target.key] ||
      target.staticCandidates ||
      [];
    const { dynamicCandidates: _dynamic, ...field } = target;
    availableFields.value.push({
      ...field,
      ...(target.type === "single_line"
        ? { type: "single_select" as const }
        : {}),
      key,
      required: true,
      ...(["single_select", "single_line"].includes(target.type)
        ? { staticCandidates: choices }
        : {}),
    });
    const prefix = `option_${availableFields.value.length - 1}`;
    Object.assign(draftValues, {
      [`${prefix}_enabled`]: true,
      [`${prefix}_product_field`]: target.key,
      [`${prefix}_required`]: true,
      [`${prefix}_control`]: target.type === "integer" ? "select" : "auto",
      [`${prefix}_numbers`]: [
        {
          value: target.minimum || "0",
          label: { zhCN: target.minimum || "0", enUS: target.minimum || "0" },
        },
      ],
      [`${prefix}_minimum`]: target.minimum,
      [`${prefix}_maximum`]: target.minimum,
      [`${prefix}_choices`]: choices.map((choice) => ({
        ...choice,
        label: { ...choice.label },
      })),
    });
    selectedKeys.value.push(key);
    addingField.value = undefined;
    await selectField(key);
    return;
  }
  if (!addingField.value || selectedKeys.value.includes(addingField.value))
    return;
  draftValues[
    `option_${availableFields.value.findIndex((field) => field.key === addingField.value)}_enabled`
  ] = true;
  selectedKeys.value.push(addingField.value);
  const key = addingField.value;
  addingField.value = undefined;
  await selectField(key);
}
async function addLocal() {
  if (availableFields.value.length >= 128) return;
  const key = `local_${crypto.randomUUID().replaceAll("-", "")}`;
  availableFields.value.push({
    key,
    type: localType.value,
    label: { zhCN: tr("newField"), enUS: "Custom option" },
    ...(localType.value === "integer"
      ? { minimum: "0", maximum: "2147483647" }
      : {}),
    ...(["single_line", "multi_line"].includes(localType.value)
      ? { maximumLength: 4096 }
      : {}),
  });
  selectedKeys.value.push(key);
  await selectField(key);
}
async function removeActive() {
  const field = availableFields.value.find((f) => f.key === activeKey.value);
  if (!field || isRequiredOption(field)) return;
  await persistActiveValues();
  draftValues[
    `option_${availableFields.value.findIndex((item) => item.key === field.key)}_enabled`
  ] = false;
  selectedKeys.value = selectedKeys.value.filter((key) => key !== field.key);
  activeKey.value = selectedKeys.value[0] || "";
  await buildForm();
}
function parseChildren(raw: unknown, numeric = false) {
  if (Array.isArray(raw))
    return raw.map((item) => ({
      value: String(item.value ?? ""),
      label: {
        zhCN: String(item.label?.zhCN || item.value || ""),
        enUS: String(item.label?.enUS || item.label?.zhCN || item.value || ""),
      },
      ...(item.parentValue ? { parentValue: String(item.parentValue) } : {}),
    }));
  return String(raw || "")
    .split(numeric ? /[,，\n]+/u : /\r?\n/u)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [value = "", label = "", en = "", parentValue = ""] = line
        .split("|")
        .map((part) => part.trim());
      return {
        value,
        ...(parentValue ? { parentValue } : {}),
        label: { zhCN: label || value, enUS: en || label || value },
      };
    });
}
async function initialize(data: {
  versionNo: string;
  bindingNo: string;
  configuration: CatalogProductVersionConfiguration;
}) {
  switching.value = true;
  try {
    initialized = true;
    pendingRowOrders.value = {};
    rowChangesPending.value = false;
    stale.value = false;
    priceStale.value = false;
    priceBaseline = JSON.stringify(props.detail?.prices || []);
    baseline = configurationIdentity(data.configuration, data.bindingNo);
    versionNo = data.versionNo;
    configuration = {
      ...data.configuration,
      customerFieldSelections: productOptionSelections(data.configuration),
    };
    availableFields.value = [
      ...(configuration.customerConfigurableFields ?? []),
    ];
    for (const selection of configuration.customerFieldSelections ?? [])
      if (selection.localField)
        availableFields.value.push({
          ...selection.localField,
          ...(selection.localField.type === "integer"
            ? {
                minimum:
                  configuration.productDeclaration?.fields.find(
                    (field) => field.key === selection.productFieldKey,
                  )?.minimum || "0",
                maximum:
                  configuration.productDeclaration?.fields.find(
                    (field) => field.key === selection.productFieldKey,
                  )?.maximum || "2147483647",
              }
            : {}),
        });
    selectedKeys.value = [
      ...new Set([
        ...availableFields.value.filter(isRequiredOption).map((f) => f.key),
        ...(configuration.customerFieldSelections ?? []).map((s) => s.fieldKey),
      ]),
    ];
    draftValues = {};
    merchantPriceDrafts.value = {};
    draftLabels.value = {};
    validationError.value = "";
    addingField.value = undefined;
    localType.value = "single_select";
    for (const [index, field] of availableFields.value.entries()) {
      const selection = configuration.customerFieldSelections?.find(
        (item) => item.fieldKey === field.key,
      );
      const prefix = `option_${index}`;
      if (!selection) continue;
      draftLabels.value[field.key] = {
        zhCN: selection.displayNameZhCN || field.label?.zhCN || field.key,
        enUS: selection.displayNameEnUS || field.label?.enUS || field.key,
      };
      const knownChoiceLabels = new Map(
        [
          ...(field.staticCandidates || []),
          ...(data.configuration.customerResolvedCandidates[field.key] || []),
        ].map((choice) => [choice.value, choice.label]),
      );
      Object.assign(draftValues, {
        [`${prefix}_customer_visible`]: selection.customerVisible !== false,
        [`${prefix}_default`]: selection.defaultValue,
        [`${prefix}_product_field`]: selection.productFieldKey || undefined,
        [`${prefix}_choice_bindings`]: { ...selection.productChoiceBindings },
        [`${prefix}_information_only`]: false,
        [`${prefix}_numeric_pricing`]: selection.numericPricing || "quantity",
        [`${prefix}_enabled`]: true,
        [`${prefix}_control`]: selection.displayControl || "auto",
        [`${prefix}_step`]: Number(selection.quantityStep || 1),
        [`${prefix}_numbers`]: (selection.numericChoices || []).map(
          (value) => ({
            value,
            label: selection.numericLabels?.[value] || {
              zhCN: value,
              enUS: value,
            },
          }),
        ),
        [`${prefix}_name_zhCN`]: selection.displayNameZhCN || field.label?.zhCN,
        [`${prefix}_name_enUS`]: selection.displayNameEnUS || field.label?.enUS,
        [`${prefix}_order`]: selection.sortOrder ?? index,
        [`${prefix}_minimum`]:
          selection.minimum === undefined
            ? undefined
            : Number(selection.minimum),
        [`${prefix}_maximum`]:
          selection.maximum === undefined
            ? undefined
            : Number(selection.maximum),
        [`${prefix}_candidates`]: selection.candidateValues ?? [],
        [`${prefix}_choices`]: (
          selection.localField?.staticCandidates ??
          (selection.candidateValues?.length ? selection.candidateValues : (!isLocal(field) ? (configuration?.customerResolvedCandidates[field.key] || field.staticCandidates || []).map(choice => choice.value) : undefined))?.map((value) => ({
            value,
            parentValue: undefined,
            label: selection.candidateLabels?.[value] ||
              knownChoiceLabels.get(value) || {
                zhCN: value,
                enUS: value,
              },
          })) ??
          []
        ).map((c) => ({ ...c, label: { ...c.label } })),
        [`${prefix}_group_zhCN`]: selection.groupZhCN || "",
        [`${prefix}_group_enUS`]: selection.groupEnUS || "",
        [`${prefix}_description_zhCN`]: selection.descriptionZhCN || "",
        [`${prefix}_description_enUS`]: selection.descriptionEnUS || "",
        [`${prefix}_unit`]: selection.unit || "",
        [`${prefix}_advanced`]: selection.advanced || false,
        [`${prefix}_required`]: selection.localField?.required || false,
        [`${prefix}_upgrade_enabled`]: selection.upgradeEnabled !== false,
        [`${prefix}_upgrade_minimum`]: selection.upgradeMinimum === undefined ? undefined : Number(selection.upgradeMinimum),
        [`${prefix}_upgrade_maximum`]: selection.upgradeMaximum === undefined ? undefined : Number(selection.upgradeMaximum),
        [`${prefix}_upgrade_choices`]: selection.upgradeCandidateValues ?? [],
      });
    }
    activeKey.value = visibleFields.value.some(
      (field) => field.key === activeKey.value,
    )
      ? activeKey.value
      : selectedKeys.value[0] || "";
    if (optionFormMounted.value) await formApi.resetForm();
    await buildForm();
    await nextTick();
    draftBaseline = draftIdentity();
  } finally {
    switching.value = false;
    awaitingSavedRefresh.value = false;
  }
}
// The owning workspace must wait for the authoritative refreshed version,
// so another editor never captures the version this save just made immutable.
let submitting = false;
async function save() {
  if (submitting) return false;
  submitting = true;
  emit("busy", true);
  try {
    return await saveConfiguration();
  } finally {
    submitting = false;
    emit("busy", false);
  }
}
async function saveConfiguration() {
  if (
    !canWriteConfiguration() ||
    saving.value ||
    stale.value ||
    priceStale.value ||
    !(await formApi.validate()).valid ||
    !configuration
  )
    return false;
  await persistActiveValues();
  validationError.value = "";
  const selections: CatalogCustomerFieldSelection[] = [];
  for (const [index, field] of availableFields.value.entries()) {
    const prefix = `option_${index}`;
    if (!isRequiredOption(field) && !draftValues[`${prefix}_enabled`]) continue;
    if (unresolvedMapping(field, index)) {
      validationError.value = tr("mappingRequired");
      await selectField(field.key);
      return false;
    }
    const values = draftValues;
    const control = String(
      values[`${prefix}_control`] || "auto",
    ) as CatalogCustomerFieldSelection["displayControl"];
    const children = parseChildren(values[`${prefix}_numbers`], true);
    let selection: CatalogCustomerFieldSelection = {
      fieldKey: field.key,
      customerVisible: values[`${prefix}_customer_visible`] !== false,
      defaultValue: values[`${prefix}_customer_visible`] === false ? defaultForField(field, index, values) : undefined,
      informationOnly: !!values[`${prefix}_information_only`],
      upgradeEnabled: values[`${prefix}_upgrade_enabled`] !== false,
      upgradeMinimum: field.type === "integer" && values[`${prefix}_upgrade_minimum`] != null ? String(values[`${prefix}_upgrade_minimum`]) : undefined,
      upgradeMaximum: field.type === "integer" && values[`${prefix}_upgrade_maximum`] != null ? String(values[`${prefix}_upgrade_maximum`]) : undefined,
      upgradeCandidateValues: field.type === "single_select" ? values[`${prefix}_upgrade_choices`] as string[] | undefined : undefined,
      productFieldKey: isLocal(field)
        ? String(values[`${prefix}_product_field`] || "") || undefined
        : undefined,
      productChoiceBindings:
        isLocal(field) &&
        field.type === "single_select" &&
        values[`${prefix}_product_field`]
          ? (values[`${prefix}_choice_bindings`] as
              | Record<string, string>
              | undefined)
          : undefined,
      displayControl: control,
      displayNameZhCN: String(
        values[`${prefix}_name_zhCN`] ?? field.label?.zhCN ?? "",
      ),
      displayNameEnUS: String(
        values[`${prefix}_name_enUS`] ?? field.label?.enUS ?? "",
      ),
      sortOrder: Number(values[`${prefix}_order`] ?? index),
      groupZhCN: String(values[`${prefix}_group_zhCN`] || ""),
      groupEnUS: String(values[`${prefix}_group_enUS`] || ""),
      descriptionZhCN: String(values[`${prefix}_description_zhCN`] || ""),
      descriptionEnUS: String(values[`${prefix}_description_enUS`] || ""),
      unit: String(values[`${prefix}_unit`] || ""),
      advanced: !!values[`${prefix}_advanced`],
    };
    if (
      !selection.displayNameZhCN?.trim() ||
      !selection.displayNameEnUS?.trim()
    ) {
      await selectField(field.key);
      await formApi.validate();
      return false;
    }
    if (field.type === "integer") {
      Object.assign(selection, {
        numericPricing:
          values[`${prefix}_numeric_pricing`] === "per_choice"
            ? "per_choice"
            : "quantity",
        quantityStep: String(values[`${prefix}_step`] ?? 1),
        numericChoices: discrete(control) ? children.map((c) => c.value) : [],
        numericLabels: discrete(control)
          ? Object.fromEntries(children.map((c) => [c.value, c.label]))
          : {},
        minimum: String(
          values[`${prefix}_minimum`] ??
            (isCore(field) ? field.minimum : undefined),
        ),
        maximum: String(
          values[`${prefix}_maximum`] ??
            (isCore(field) ? field.maximum : undefined),
        ),
      });
      try {
        selection = normalizeCustomerNumberSelection(
          field,
          selection,
          isCore(field),
        );
      } catch (error) {
        if (!(error instanceof CustomerSelectionError)) throw error;
        validationError.value = tr(`invalid_${error.reason}`, {
          field: fieldLabel(field),
          minimum: field.minimum || "0",
          maximum: field.maximum || "",
        });
        await selectField(field.key);
        return false;
      }
      if (isCore(field)) {
        delete selection.minimum;
        delete selection.maximum;
      }
    } else if (
      !isLocal(field) &&
      ["single_select", "multi_select", "tree_select", "address"].includes(
        field.type,
      )
    ) {
      const choices = parseChildren(values[`${prefix}_choices`]);
      selection.candidateValues = choices.map((c) => c.value);
      selection.candidateLabels = Object.fromEntries(
        choices.map((c) => [c.value, c.label]),
      );
    }
    if (isLocal(field)) {
      const textTarget = configuration.productDeclaration?.fields.find(
        (target) =>
          target.key === selection.productFieldKey &&
          target.type === field.type &&
          ["single_line", "multi_line"].includes(field.type),
      );
      selection.localField = {
        ...field,
        key: field.key,
        label: {
          zhCN: selection.displayNameZhCN!,
          enUS: selection.displayNameEnUS!,
        },
        required: !!selection.productFieldKey || !!values[`${prefix}_required`],
        ...(textTarget
          ? {
              minimumLength: textTarget.minimumLength,
              maximumLength: textTarget.maximumLength,
              textFormat: textTarget.textFormat,
            }
          : {}),
        ...([
          "single_select",
          "multi_select",
          "tree_select",
          "address",
        ].includes(field.type)
          ? { staticCandidates: parseChildren(values[`${prefix}_choices`]) }
          : {}),
        ...(field.type === "integer"
          ? { minimum: selection.minimum, maximum: selection.maximum }
          : {}),
      };
      if (
        ["single_select", "multi_select", "tree_select", "address"].includes(
          field.type,
        ) &&
        (!selection.localField.staticCandidates?.length ||
          new Set(selection.localField.staticCandidates.map((c) => c.value))
            .size !== selection.localField.staticCandidates.length)
      ) {
        validationError.value = tr("invalidChildren");
        await selectField(field.key);
        return false;
      }
      if (missingProductChoiceBinding(selection)) {
        validationError.value = tr("choiceMappingRequired");
        await selectField(field.key);
        return false;
      }
    }
    if (selection.customerVisible === false) {
      const choices = selection.localField?.staticCandidates || parseChildren(values[`${prefix}_choices`]);
      const defaultField = { ...field, ...selection.localField,
        minimum: selection.minimum ?? field.minimum, maximum: selection.maximum ?? field.maximum,
        staticCandidates: choices.length ? choices : configuration.customerResolvedCandidates[field.key] || field.staticCandidates || [],
      };
      if (["secret", "file"].includes(field.type) || selection.defaultValue === undefined || validateCatalogConfigurationDraftValue({version:"v1",fields:[defaultField]}, defaultField, {[field.key]: defaultDraftValue(selection.defaultValue)})) {
        validationError.value = tr("defaultInvalid");
        await selectField(field.key);
        return false;
      }
    }
    selections.push(selection);
  }
  const priceRows = [];
  try {
    for (const draft of Object.values(merchantPriceDrafts.value)) {
      const fieldKey = draft.fieldKey;
      const selection = selections.find(item => item.fieldKey === fieldKey);
      if (!selection) continue;
      if (!props.canPrice) throw new MerchantPriceDraftError("permission");
      const field = availableFields.value.find(
        (item) => item.key === draft.fieldKey,
      );
      const quantity =
        field?.type === "integer" && selection.numericPricing !== "per_choice";
      if (draft.quantity !== quantity) continue;
      const values =
        field?.type === "integer"
          ? selection.numericChoices || []
          : selection.localField?.staticCandidates?.map((item) => item.value) ||
            selection.candidateValues ||
            [];
      if (!quantity && values.length && !values.includes(draft.candidateValue))
        continue;
      const rule = props.detail?.billingRules.find(
        (item) =>
          item.billingRuleNo === draft.billingRuleNo &&
          item.status === "active",
      );
      if (!rule) throw new MerchantPriceDraftError("amount");
      priceRows.push(merchantPriceDraftInput({...draft,fieldKey}, rule));
    }
    if (
      priceRows.length > 256 ||
      priceRows.reduce((sum, row) => sum + row.tiers.length, 0) > 256
    )
      throw new MerchantPriceDraftError("limit");
  } catch (error) {
    if (!(error instanceof MerchantPriceDraftError)) throw error;
    validationError.value = $t(
      `plugin.idc-core.catalog.offerPrices.invalid_${error.reason}`,
    );
    return false;
  }
  if (
    !canWriteConfiguration() ||
    saving.value ||
    stale.value ||
    priceStale.value
  )
    return false;
  saving.value = true;
  try {
    const saved = await catalogProductVersionConfigurationUpdate(versionNo, {
      expectedVersion: configuration.version,
      merchantValues: configuration.merchantValues,
      customerFieldSelections: selections,
      ...(priceRows.length ? { priceRows } : {}),
    });
    rowChangesPending.value = false;
    optionCommitted.value = true;
    message.success({
      content: $t("plugin.idc-core.catalog.offerPrices.saved"),
      key: "idc-catalog-purchase-options-saved",
    });
    savedIdentity = configurationIdentity(saved.configuration, props.bindingNo);
    awaitingSavedRefresh.value = true;
    if (props.refresh) {
      try {
        await props.refresh();
        await nextTick();
        savedRefreshFailed.value = awaitingSavedRefresh.value;
      } catch {
        savedRefreshFailed.value = true;
      }
    } else emit("saved", saved.configuration);
    return !savedRefreshFailed.value;
  } finally {
    saving.value = false;
  }
}

function missingProductChoiceBinding(selection: CatalogCustomerFieldSelection) {
  if (
    selection.localField?.type !== "single_select" ||
    !selection.productFieldKey
  )
    return false;
  const target = configuration?.productDeclaration?.fields.find(
    (field) => field.key === selection.productFieldKey,
  );
  if (target?.type !== "single_select") return false;
  const choices =
    configuration?.productResolvedCandidates[target.key] ||
    target.staticCandidates ||
    [];
  // A missing directory has its own recoverable server-side reason; this
  // check only explains an otherwise-known invalid option mapping locally.
  if (!choices.length) return false;
  const allowed = new Set(choices.map((choice) => choice.value));
  return (
    selection.localField.staticCandidates?.some(
      (choice) =>
        !allowed.has(
          selection.productChoiceBindings?.[choice.value] ?? choice.value,
        ),
    ) || false
  );
}
function canWriteConfiguration() {
  return props.writable !== false;
}
// Candidate polling changes only a reconstructible projection. Rebuild the
// active field against that projection without replacing any merchant draft.
watch(
  () => [
    props.configuration.productResolvedCandidates,
    props.configuration.customerResolvedCandidates,
  ],
  async () => {
    const sequence = ++directorySequence;
    if (
      !initialized ||
      disposed ||
      switching.value ||
      saving.value ||
      awaitingSavedRefresh.value ||
      stale.value ||
      configurationIdentity(props.configuration, props.bindingNo) !== baseline
    )
      return;
    const fieldKey = activeKey.value;
    configuration = props.configuration;
    await persistActiveValues();
    if (
      disposed ||
      sequence !== directorySequence ||
      fieldKey !== activeKey.value ||
      switching.value ||
      saving.value ||
      awaitingSavedRefresh.value
    )
      return;
    await buildForm();
  },
  { deep: true, flush: "post" },
);
onBeforeUnmount(() => {
  disposed = true;
  directorySequence++;
});
// Another tab may save and create an internal snapshot. Advance the write
// target only if the stored configuration and binding are unchanged; preserve
// the user's unsaved values. A genuine concurrent change requires review.
watch(
  () => [props.versionNo, props.configuration, props.bindingNo],
  () => {
    if (!initialized) {
      void initialize(props);
      return;
    }
    if (
      savedIdentity &&
      configurationIdentity(props.configuration, props.bindingNo) ===
        savedIdentity
    ) {
      savedIdentity = "";
      void initialize(props);
      return;
    }
    if (
      configurationIdentity(props.configuration, props.bindingNo) === baseline
    ) {
      versionNo = props.versionNo;
      configuration = props.configuration;
      stale.value = false;
    } else if (!switching.value && draftIdentity() === draftBaseline) {
      // A completed change in another section may introduce interface fields.
      // Refresh an untouched editor immediately; real local edits stay intact.
      void initialize(props);
    } else stale.value = true;
  },
  { immediate: true },
);
</script>
<template>
  <section
    class="catalog-customer-options-form"
    data-testid="catalog-customer-options-form"
  >
    <Alert
      v-if="savedRefreshFailed"
      class="mb-4"
      type="warning"
      show-icon
      :message="$t('plugin.idc-core.catalog.offerPrices.savedRefreshFailed')"
      data-testid="catalog-offer-saved-refresh-failed"
    >
      <template #action
        ><Button
          data-testid="catalog-offer-saved-refresh-retry"
          :loading="retryingSavedRefresh"
          @click="retrySavedRefresh"
          >{{ workspaceText("reloadConfiguration") }}</Button
        ></template
      >
    </Alert>
    <Alert
      v-if="stale || priceStale"
      class="mb-4"
      type="warning"
      show-icon
      :message="workspaceText('configurationChanged')"
    >
      <template #action
        ><Popconfirm
          :title="workspaceText('reloadConfigurationConfirm')"
          @confirm="initialize(props)"
          ><Button
            data-testid="catalog-customer-options-reload"
            :disabled="saving || switching"
            >{{ workspaceText("reloadConfiguration") }}</Button
          ></Popconfirm
        ></template
      >
    </Alert>
    <div :inert="saving || awaitingSavedRefresh || undefined">
      <Alert
        v-if="bindingNo && configuration?.pendingAutomationFields?.length"
        class="mb-4"
        show-icon
        type="warning"
        :message="tr('mappingHint')"
        data-testid="catalog-retained-mapping-hint"
      />
      <div v-if="props.writable !== false" class="catalog-parameter-actions">
        <Button type="primary" data-testid="catalog-local-option-add" :disabled="switching || availableFields.length >= 128" @click="addCustomOption">{{ tr('addLocal') }}</Button>
        <div class="catalog-quick-parameters">        <Select
          v-model:value="addingField"
          :virtual="false"
          show-search
          option-filter-prop="label"
          class="w-[240px] max-w-full"
          data-testid="catalog-customer-option-parameter"
          v-if="
            configuration?.productDeclaration?.fields.length ||
            availableFields.some((field) => !isLocal(field))
          "
          :placeholder="tr('parameter')"
          :options="[
            ...availableFields
              .filter(
                (field) =>
                  !selectedKeys.includes(field.key) &&
                  !isLocal(field) &&
                  !configuration?.productDeclaration?.fields.some(
                    (target) => target.key === field.key,
                  ),
              )
              .map((field) => ({ value: field.key, label: fieldLabel(field) })),
            ...(configuration?.productDeclaration?.fields ?? [])
              .filter(
                (field) =>
                  (['integer', 'single_select', 'boolean'].includes(
                    field.type,
                  ) ||
                    (field.type === 'single_line' &&
                      configuration?.productChoiceFields?.includes(
                        field.key,
                      ))) &&
                  !availableFields.some(
                    (option, index) =>
                      selectedKeys.includes(option.key) &&
                      (draftValues[`option_${index}_product_field`] ===
                        field.key ||
                        option.key === field.key ||
                        configuration?.customerProductFields?.[option.key] ===
                          field.key),
                  ),
              )
              .map((field) => ({
                value: 'product:' + field.key,
                label: `${zh() ? field.label.zhCN : field.label.enUS} · ${field.key}`,
              })),
          ]"
        /><Button
          data-testid="catalog-customer-option-add"
          v-if="
            configuration?.productDeclaration?.fields.length ||
            availableFields.some((field) => !isLocal(field))
          "
          :disabled="switching || !addingField"
          @click="addQuickOption"
          >{{ tr("quickAdd") }}</Button
        ></div>
      </div>
      <Alert
        v-if="validationError"
        class="mb-4"
        show-icon
        type="error"
        data-testid="catalog-customer-options-validation"
        :message="validationError"
      />
      <div class="catalog-options-toolbar">
        <Button
          data-testid="catalog-option-preview-toggle"
          :aria-expanded="showPreview"
          @click="togglePreview"
          >{{ tr(showPreview ? "hidePreview" : "showPreview") }}</Button
        >
      </div>
      <div
        class="catalog-options-layout"
        :class="{ 'with-preview': showPreview }"
      >
        <div class="catalog-options-main">
      <VbenDataGrid
        class="catalog-options-overview"
        data-testid="catalog-options-overview" :toolbar="false"
        :columns="overviewColumns" :data="overviewRows" row-key="key"
        :height="Math.min(330, overviewRows.length * 46 + 48)"
      >
        <template #cell="{ column, record, value }">
          <Button v-if="column.key === 'name'" type="link" class="option-name"
            :class="{ 'option-name-active': record.key === activeKey }"
            :disabled="switching" :data-testid="`catalog-option-tab-${record.key}`"
            @click="editOption(record.key)">
            {{ value }} <span class="option-edit-label">{{ tr("edit") }}</span>
          </Button>
          <InputNumber v-else-if="column.key === 'sortOrder'" :value="record.key in pendingRowOrders ? pendingRowOrders[record.key] : record.sortOrder"
            :min="0" :max="1024" :precision="0" size="small" class="w-full" :aria-label="`${record.name} ${tr('order')}`"
            :disabled="formEditingDisabled" :data-testid="`catalog-option-sort-${record.key}`"
            @change="pendingRowOrders[record.key] = $event as number | null" @blur="commitRowOrder(record.key)" />
          <Switch v-else-if="column.key === 'upgradeEnabled'" :checked="record.upgradeEnabled" :aria-label="`${record.name} ${tr('upgradeEnabled')}`"
            :disabled="formEditingDisabled" :data-testid="`catalog-option-upgrade-${record.key}`"
            @change="setRowUpgrade(record.key, Boolean($event))" />
          <div v-else-if="column.key === 'actions'" class="flex items-center gap-2">
            <ghost-button :disabled="switching" @click="editOption(record.key)">{{ tr('edit') }}</ghost-button>
            <ghost-button :disabled="formEditingDisabled || availableFields.length >= 128" :data-testid="`catalog-option-copy-${record.key}`" @click="copyOption(record.key)">{{ tr('copyDefinition') }}</ghost-button>
          </div>
          <span v-else-if="column.key === 'customerVisible'">{{ record.customerVisible ? tr("shown") : tr("hidden") }}</span>
          <span v-else-if="column.key === 'binding'" :class="{ 'text-orange-600': record.pendingMapping }">{{ record.pendingMapping ? tr('mappingRequired') : value || tr('informationField') }}</span>
          <span v-else>{{ value }}</span>
        </template>
      </VbenDataGrid>

        </div>
        <CatalogOfferPreview
          v-if="showPreview"
          :fields="previewFields"
          :product-no="detail?.product.productNo || ''" :version-no="versionNo" :currencies="activePricing.currencies" :pricing-dirty="configurationDirty || props.externalPricingDirty"
          :conditions="previewConditions"
          class="catalog-preview-panel"
        />
      </div>
    </div>
    <OptionModal :title="tr(newLocal ? 'addLocal' : 'editParameter')" class="catalog-parameter-dialog w-[1440px] max-w-[96vw]">
      <div data-testid="catalog-parameter-modal">
        <Alert v-if="savedRefreshFailed" type="warning" show-icon class="mb-4" :message="$t('plugin.idc-core.catalog.offerPrices.savedRefreshFailed')">
          <template #action><Button :loading="retryingSavedRefresh" @click="retrySavedRefresh">{{workspaceText('reloadConfiguration')}}</Button></template>
        </Alert>
        <Alert v-if="stale || priceStale" type="warning" show-icon class="mb-4" :message="workspaceText('configurationChanged')" />
        <div class="catalog-option-editor" :class="{'has-prices':canPrice && priceable}">
          <section class="catalog-parameter-definition" data-testid="catalog-parameter-definition">
          <header v-if="activeKey" class="catalog-option-editor-heading">
            <h4>
              {{ tr('parameterDefinition') }}
            </h4>
            <Button
              v-if="
                props.writable !== false &&
                !availableFields.find(
                  (field) => field.key === activeKey && isRequiredOption(field),
                )
              "
              danger
              size="small"
              data-testid="catalog-option-remove"
              @click="removeActive"
              >{{ tr("remove") }}</Button
            >
          </header>
          <Button
            v-if="
              (activeProductField?.type === 'single_select' ||
                availableFields.find(
                  (f) =>
                    f.key === activeKey &&
                    isCore(f) &&
                    f.type === 'single_select',
                )) &&
              props.writable !== false
            "
            class="mb-4"
            data-testid="catalog-option-import-choices"
            @click="importProductChoices"
            >{{ tr("importChoices") }}</Button
          >
          <div class="mb-4">
            <Checkbox v-model:checked="showDetails" @change="toggleDetails">{{
              tr("showDisplayDetails")
            }}</Checkbox>
          </div>
          <Alert v-if="validationError" class="mb-4" type="error" show-icon :message="validationError" />
          <div v-if="newLocal" class="catalog-parameter-type"><label>{{ tr('configurationType') }}</label>
          <Select v-model:value="localType" class="w-full" data-testid="catalog-local-option-type"
            :options="localTypes.map(value => ({value,label:tr(`type_${value}`)}))" @change="changeLocalType" />
          </div>
          <Form /><Alert
            v-if="!selectedKeys.length"
            type="info"
            :message="tr('empty')"
          />
          </section>
          <fieldset
            :disabled="
              props.writable === false ||
              saving ||
              awaitingSavedRefresh ||
              stale ||
              priceStale
            "
            class="catalog-option-price-fence" data-testid="catalog-parameter-pricing"
          >
            <CatalogOfferPriceEditor
              v-if="canPrice && priceable"
              :key="activeKey"
              :context="activePricing"
              :drafts="merchantPriceDrafts"
              :disabled="
                props.writable === false ||
                saving ||
                awaitingSavedRefresh ||
                stale ||
                priceStale
              "
              @change="changePrice"
            />
          </fieldset>

        </div>
      </div>
          <template #footer>
          <Button data-testid="catalog-parameter-cancel" :disabled="saving || awaitingSavedRefresh" @click="optionModal.close()">{{tr('cancelParameter')}}</Button>
          <Button
            v-if="props.writable !== false"
            type="primary"
            data-testid="catalog-customer-options-save"
            :loading="saving || awaitingSavedRefresh"
            :disabled="stale || priceStale || switching || awaitingSavedRefresh"
            @click="saveOption"
            >{{ $t("plugin.idc-core.catalog.offerPrices.save") }}</Button
          >
          </template>
    </OptionModal>
  </section>
</template>

<style scoped>
:global(.catalog-parameter-dialog) {width:min(1440px,calc(100vw - 32px))!important;max-width:calc(100vw - 32px)!important;}
.catalog-customer-options-form {
  min-width: 0;
}
.catalog-options-main { min-width: 0; }
.catalog-option-editor { min-width:0; }
.catalog-option-editor.has-prices {display:grid;grid-template-columns:minmax(360px,0.85fr) minmax(0,1.4fr);gap:0 28px;align-items:start;}
.catalog-parameter-definition {min-width:0;}
.catalog-option-editor.has-prices > .catalog-parameter-definition,
.catalog-option-editor.has-prices > .catalog-option-price-fence {max-height:min(560px,calc(100dvh - 280px));overflow-y:auto;overscroll-behavior:contain;padding-right:8px;}
.catalog-option-editor.has-prices .catalog-option-price-fence {border-left:1px solid hsl(var(--border));padding-left:28px;}
.catalog-parameter-type {display:flex;align-items:center;gap:12px;margin-bottom:20px;}
.catalog-parameter-type > label {width:100px;text-align:right;flex:none;}
@media(max-width:1000px){.catalog-option-editor.has-prices{grid-template-columns:minmax(0,1fr);gap:24px;}.catalog-option-editor.has-prices > .catalog-parameter-definition,.catalog-option-editor.has-prices > .catalog-option-price-fence{max-height:none;overflow:visible;}.catalog-option-editor.has-prices .catalog-option-price-fence{border-left:0;border-top:1px solid hsl(var(--border));padding:24px 0 0;}}
.catalog-parameter-actions { display:flex; align-items:center; justify-content:space-between; gap:20px; flex-wrap:wrap; margin-bottom:18px; }
.catalog-quick-parameters { display:flex; align-items:center; gap:8px; margin-left:auto; }
.catalog-option-price-fence {
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
}
.catalog-option-prices {
  border-top: 1px solid hsl(var(--border));
  margin-top: 24px;
  padding-top: 20px;
}
.catalog-option-prices h4 {
  font-size: 15px;
  font-weight: 600;
}
.catalog-option-prices p {
  color: hsl(var(--muted-foreground));
  font-size: 13px;
  margin: 8px 0 16px;
}
/* Let the public form item's control column shrink; keep validation messages visible. */
.catalog-customer-options-form :deep(.catalog-customer-option-field > div) {
  min-width: 0;
}
.catalog-options-layout {
  margin-top: 20px;
  min-width: 0;
}
.catalog-options-layout.with-preview {
  display: grid;
  grid-template-columns: minmax(0, 1.65fr) minmax(300px, 1fr);
  gap: 20px;
  align-items: start;
}
.catalog-options-toolbar {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
.catalog-preview-panel {
  position: sticky;
  top: 12px;
}
@media (max-width: 1050px) {
  .catalog-options-layout.with-preview {
    grid-template-columns: 1fr;
  }
  .catalog-preview-panel {
    position: static;
  }
}
.catalog-options-overview {
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
  overflow: hidden;
}
.catalog-options-overview :deep(.vxe-header--column) { font-size: 12px; font-weight: 500; color: hsl(var(--muted-foreground)); }
.catalog-options-overview :deep(.vxe-body--column) { font-size: 13px; }
.catalog-options-overview :deep(.vxe-body--row:hover) { background: hsl(var(--primary) / 4%); }
.option-edit-label { margin-left: 8px; color: hsl(var(--primary)); font-size: 12px; font-weight: 400; }
.option-name { padding: 0; color: hsl(var(--foreground)); }
.option-name-active { color: hsl(var(--primary)); font-weight: 600; }
.option-indicator { margin-right: 8px; color: hsl(var(--muted-foreground)); }
.catalog-option-editor-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
  padding-bottom: 12px;
  border-bottom: 1px solid hsl(var(--border));
}
.catalog-option-editor-heading h4 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}
</style>
