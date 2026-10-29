<script setup lang="ts">
import type {
  CatalogProductDetail,
  CatalogProductVersionConfiguration,
} from "../catalog-client";
import type { CatalogConfigurationValues } from "../catalog-configuration-state";
import { computed, ref } from "vue";
import { $t } from "#/locales";
import {
  catalogProductVersionConfigurationEvaluate,
  catalogProductVersionConfigurationUpdate,
  catalogProductVersionConfigurationFileUpload,
  catalogProductVersionConfigurationFileRevoke,
} from "../catalog-client";
import {
  catalogConfigurationRowsToValues,
  catalogConfigurationValuesToRows,
} from "../catalog-configuration-state";
import CatalogConfigurationSection from "./catalog-configuration-section.vue";
const props = defineProps<{
  mode?: "login" | "deployment";
  externalSave?: boolean;
  versionNo: string;
  detail: CatalogProductDetail;
  configuration: CatalogProductVersionConfiguration;
  writable: boolean;
  refresh: () => Promise<void>;
}>();
const sectionRef = ref<InstanceType<typeof CatalogConfigurationSection>>();
const loginField = (key:string) => /^(hostname|password)_(prefix|length|characters|customer_input)$/.test(key);
const emit = defineEmits<{
  busy: [value: boolean];
}>();
const tr = (key: string) =>
  $t(`plugin.idc-core.catalog.customerOptions.${key}`);
// Customer-facing parameters belong to the option editor even before they
// are enabled. Remaining fields are private deployment rules, not SKU defaults.
const rulesDeclaration = computed(() => {
  const original = props.configuration.productDeclaration;
  if (!original) return undefined;
  const customerKeys = new Set([
    ...(props.configuration.customerConfigurableFields ?? []).map(field => field.key),
    ...Object.values(props.configuration.customerProductFields ?? {}),
    ...(props.configuration.productChoiceFields ?? []),
  ]);
  const fields = original.fields.filter(field =>
    (props.mode === "login" ? loginField(field.key) : !loginField(field.key)) &&
    !customerKeys.has(field.key) &&
    !props.configuration.customerFieldSelections?.some(selection =>
      selection.productFieldKey === field.key || selection.fieldKey === field.key) &&
    !(field.required && ["integer", "single_select", "boolean"].includes(field.type)),
  );
  return fields.length ? { version: original.version, fields, uploadPolicies: original.uploadPolicies } : undefined;
});
function productValues() {
  return catalogConfigurationRowsToValues(
    props.configuration.merchantValues,
    props.configuration.productDeclaration,
  );
}
function ruleValues() {
  const keys = new Set(
    rulesDeclaration.value?.fields.map((field) => field.key),
  );
  return Object.fromEntries(
    Object.entries(productValues()).filter(([key]) => keys.has(key)),
  );
}
function mergedRows(values: CatalogConfigurationValues) {
  const declaration = props.configuration.productDeclaration;
  if (!declaration) throw new Error("configuration_unavailable");
  return catalogConfigurationValuesToRows(
    declaration,
    "product",
    { ...productValues(), ...values },
    props.configuration.productResolvedCandidates,
  );
}
async function evaluateRules(values: CatalogConfigurationValues) {
  const result = await catalogProductVersionConfigurationEvaluate(
    props.versionNo,
    {
      expectedVersion: props.configuration.version,
      merchantValues: mergedRows(values),
    },
  );
  const keys = new Set(
    rulesDeclaration.value?.fields.map((field) => field.key),
  );
  return {
    ...result.evaluation,
    values: result.evaluation.values.filter((value) =>
      keys.has(value.fieldKey),
    ),
  };
}
async function saveRules(values: CatalogConfigurationValues) {
  emit('busy', true);
  try {
    await catalogProductVersionConfigurationUpdate(props.versionNo, {
      expectedVersion: props.configuration.version,
      merchantValues: mergedRows(values),
      customerFieldSelections: props.configuration.customerFieldSelections || [],
    });
    await props.refresh();
  } finally { emit('busy', false); }
}
function uploadRuleFile(field: string, file: File) {
  return catalogProductVersionConfigurationFileUpload(
    props.versionNo,
    props.configuration.version,
    field,
    file,
  );
}
function revokeRuleFile(token: string) {
  return catalogProductVersionConfigurationFileRevoke(
    props.versionNo,
    props.configuration.version,
    token,
  );
}
async function prepare() {
  const prepared = await sectionRef.value?.prepare();
  if (prepared?.valid === false) return {valid:false};
  if (!prepared?.values) return {valid:true};
  return {valid:true,input:{expectedVersion:props.configuration.version,
    values:mergedRows(prepared.values).filter(row=>loginField(row.fieldKey))}};
}
defineExpose({prepare, dirty:computed(() => !!sectionRef.value?.dirty)});
</script>
<template>
  <section v-if="rulesDeclaration" class="provisioning-rules" data-testid="catalog-provisioning-rules">
    <CatalogConfigurationSection
      ref="sectionRef" :external-save="externalSave"
      inline expand-optional
      :binding-no="detail.fulfillmentBinding?.interfaceInstanceNo || ''"
      :can-edit="writable" :declaration="rulesDeclaration"
      :resolved-candidates="configuration.productResolvedCandidates"
      :evaluate="evaluateRules" :save="saveRules"
      :upload-file="uploadRuleFile" :revoke-file="revokeRuleFile"
      surface="product" :title="tr('provisioningRules')" :values="ruleValues()"
    />
  </section>
</template>
<style scoped>
.provisioning-rules { margin-top: 24px; padding-top: 24px; border-top: 1px solid hsl(var(--border)); }
.provisioning-rules :deep(.ant-card-head) { display: none; }
.provisioning-rules :deep(.ant-card) { border: 0; box-shadow: none; }
.provisioning-rules :deep(.ant-card-body) { padding: 0; }
h3 { margin: 0; font-size: 16px; font-weight: 600; }
p { margin: 8px 0 20px; color: hsl(var(--muted-foreground)); font-size: 13px; line-height: 1.7; }
</style>
