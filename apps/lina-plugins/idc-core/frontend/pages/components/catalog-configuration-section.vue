<script setup lang="ts">
import type {
  CatalogConfigurationCandidate,
  CatalogConfigurationAuthoritativeFieldState,
  CatalogConfigurationDeclaration,
  CatalogConfigurationEvaluation,
  CatalogConfigurationField,
  CatalogConfigurationIssue,
  CatalogConfigurationSurface,
  CatalogConfigurationValues,
  CatalogLocalizedText,
} from "../catalog-configuration-state";

import { computed, onBeforeUnmount, ref, watch } from "vue";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Button,
  Popconfirm,
  Card,
  Collapse,
  CollapsePanel,
  Descriptions,
  Empty,
  message,
  Tag,
} from "ant-design-vue";

import { $t } from "#/locales";

import { catalogConfigurationMessage } from "../catalog-configuration-i18n";
import { catalogConfigurationPresentation } from "../catalog-configuration-state";
import { catalogConfigurationRowsToValues } from "../catalog-configuration-state";
import CatalogConfigurationForm from "./catalog-configuration-form.vue";
import VbenDrawerCompat from "./vben-drawer-compat.vue";
import VbenModalCompat from "./vben-modal-compat.vue";

const props = defineProps<{
  inline?: boolean;
  externalSave?: boolean;
  expandOptional?: boolean;
  upstreamOptionsStatus?: string;
  bindingNo?: string;
  canEdit?: boolean;
  canPrice?: boolean;
  declaration: CatalogConfigurationDeclaration;
  evaluate?: (
    values: CatalogConfigurationValues,
  ) => Promise<CatalogConfigurationEvaluation>;
  resolvedCandidates?: Record<string, CatalogConfigurationCandidate[]>;
  save?: (values: CatalogConfigurationValues) => Promise<unknown>;
  uploadFile?: (
    fieldKey: string,
    file: File,
  ) => Promise<{ fileName: string; fileToken: string }>;
  revokeFile?: (fileToken: string) => Promise<unknown>;
  surface: CatalogConfigurationSurface;
  title: string;
  values?: CatalogConfigurationValues;
}>();
const emit = defineEmits<{
  saved: [];
  price: [field: CatalogConfigurationField];
}>();

const open = ref(false);
const editingInline = computed(
  () => props.inline && props.canEdit && props.surface === "product",
);
const stale = ref(false);
let baseline = "";
const workspaceText = (key: string) =>
  $t(`plugin.idc-core.catalog.workspace.${key}`);
function settingsIdentity(
  values: CatalogConfigurationValues = props.values ?? {},
) {
  return JSON.stringify([props.bindingNo || "", props.declaration, values]);
}
function initializeDraft() {
  initializingDraft = true;
  draft.value = { ...(props.values ?? {}) };
  initializingDraft = false;
  baseline = settingsIdentity();
  stale.value = false;
  authoritativeCandidates.value = props.resolvedCandidates ?? {};
  if (props.evaluate) void evaluateNow();
}

const saving = ref(false);
let submitting = false;
const draft = ref<CatalogConfigurationValues>({});
const formRef = ref<InstanceType<typeof CatalogConfigurationForm>>();
const authoritativeCandidates = ref<
  Record<string, CatalogConfigurationCandidate[]>
>({});
const fieldStates = ref<
  Record<string, CatalogConfigurationAuthoritativeFieldState>
>({});
const issues = ref<CatalogConfigurationIssue[]>([]);
const evaluating = ref(false);
const evaluationFailed = ref(false);
let evaluationSequence = 0;
let initializingDraft = false;
let evaluationTimer: ReturnType<typeof globalThis.setTimeout> | undefined;
const summaryFields = computed(() =>
  props.declaration.fields.filter((field) => {
    if (field.type === "secret" && props.surface !== "interface") return false;
    if (props.surface !== "product" || field.required) return true;
    const value = props.values?.[field.key];
    return (
      value !== undefined &&
      value !== null &&
      value !== "" &&
      (!Array.isArray(value) || value.length > 0)
    );
  }),
);
const presentation = computed(() =>
  catalogConfigurationPresentation(props.declaration),
);

watch(open, (value) => {
  if (value) {
    initializeDraft();
    return;
  }
  if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
  evaluationTimer = undefined;
  evaluationSequence += 1;
});

watch(
  editingInline,
  (value) => {
    if (props.inline) open.value = Boolean(value);
  },
  { immediate: true },
);
watch(
  () => props.resolvedCandidates,
  (candidates) => {
    if (!open.value || stale.value) return;
    authoritativeCandidates.value = candidates ?? {};
    if (props.evaluate) void evaluateNow();
  },
  { deep: true },
);
watch(
  () => props.upstreamOptionsStatus,
  (state, previous) => {
    if (
      state === "ready" &&
      state !== previous &&
      open.value &&
      !stale.value &&
      evaluationFailed.value &&
      props.evaluate
    )
      void evaluateNow();
  },
);
watch(
  () => [props.values, props.declaration, props.bindingNo],
  () => {
    if (!editingInline.value || saving.value) return;
    stale.value = settingsIdentity() !== baseline;
    if (stale.value) {
      evaluationSequence++;
      evaluating.value = false;
    }
  },
);

watch(
  draft,
  () => {
    if (initializingDraft || !open.value || !props.evaluate) return;
    // Invalidate in-flight evaluations as soon as input changes, including
    // the debounce interval before the replacement request starts.
    evaluationSequence += 1;
    evaluating.value = true;
    issues.value = [];
    if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
    evaluationTimer = globalThis.setTimeout(() => void evaluateNow(), 250);
  },
  { deep: true, flush: "sync" },
);

async function evaluateNow() {
  if (!props.evaluate) return true;
  if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
  evaluationTimer = undefined;
  const sequence = ++evaluationSequence;
  evaluating.value = true;
  try {
    const evaluation = await props.evaluate(draft.value);
    if (sequence !== evaluationSequence) return false;
    authoritativeCandidates.value = evaluation.candidates ?? {};
    fieldStates.value = evaluation.fieldStates ?? {};
    issues.value = evaluation.issues ?? [];
    evaluationFailed.value = false;
    if (evaluation.valid) {
      const normalized = catalogConfigurationRowsToValues(
        evaluation.values,
        props.declaration,
      );
      if (JSON.stringify(normalized) !== JSON.stringify(draft.value)) {
        draft.value = normalized;
      }
    }
    return evaluation.valid;
  } catch {
    if (sequence === evaluationSequence) {
      issues.value = [];
      evaluationFailed.value = true;
    }
    return false;
  } finally {
    if (sequence === evaluationSequence) evaluating.value = false;
  }
}

onBeforeUnmount(() => {
  if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
  evaluationSequence += 1;
});

function localized(value?: CatalogLocalizedText) {
  if (!value) return "";
  return preferences.app.locale === "zh-CN"
    ? value.zhCN || value.enUS
    : value.enUS || value.zhCN;
}

function findOption(
  options: CatalogConfigurationCandidate[],
  token: unknown,
): CatalogConfigurationCandidate | undefined {
  for (const option of options) {
    if (option.value === token) return option;
  }
}

function customerChoices(field: CatalogConfigurationField) {
  if (props.surface !== "customer") return [];
  return props.resolvedCandidates?.[field.key] ?? field.staticCandidates ?? [];
}

function displayValue(field: CatalogConfigurationField) {
  if (field.type === "secret") return catalogConfigurationMessage("configured");
  if (props.surface === "customer" && field.type === "integer")
    return catalogConfigurationMessage("customerRange", {
      minimum: field.minimum ?? "0",
      maximum: field.maximum ?? "∞",
    });
  const value = props.values?.[field.key];
  if (value === undefined || value === null || value === "")
    return catalogConfigurationMessage("empty");
  if (field.type === "boolean") {
    return preferences.app.locale === "zh-CN"
      ? value
        ? "是"
        : "否"
      : value
        ? "Yes"
        : "No";
  }
  if (["single_select", "tree_select"].includes(field.type)) {
    const option = findOption(
      props.resolvedCandidates?.[field.key] ?? field.staticCandidates ?? [],
      value,
    );
    return option
      ? localized(option.label)
      : catalogConfigurationMessage("empty");
  }
  if (["multi_select", "address"].includes(field.type)) {
    if (!Array.isArray(value)) return catalogConfigurationMessage("empty");
    return value
      .map((token) =>
        findOption(
          props.resolvedCandidates?.[field.key] ?? field.staticCandidates ?? [],
          token,
        ),
      )
      .filter(Boolean)
      .map((option) => localized(option?.label))
      .join(" / ");
  }
  if (field.type === "file") {
    return Array.isArray(value) && value.length
      ? preferences.app.locale === "zh-CN"
        ? `已上传 ${value.length} 个文件`
        : `${value.length} file(s) uploaded`
      : catalogConfigurationMessage("empty");
  }
  return String(value);
}

async function submit() {
  if (saving.value || submitting || stale.value) return;
  submitting = true;
  try {
    await formRef.value?.validate();
    const valid = await evaluateNow();
    await formRef.value?.validate();
    if (!valid) return;
    saving.value = true;
    if (!props.save) return;
    await props.save(draft.value);
    formRef.value?.markFilesBound();
    message.success(
      preferences.app.locale === "zh-CN" ? "配置已保存" : "Configuration saved",
    );
    if (editingInline.value) {
      baseline = settingsIdentity(draft.value);
      stale.value = false;
    } else open.value = false;
    emit("saved");
  } catch (error) {
    if (error && typeof error === "object" && "errorFields" in error) return;
    // The request layer has already shown the localized server validation error.
  } finally {
    saving.value = false;
    submitting = false;
  }
}
async function prepare() {
  if (!props.canEdit) return {valid:true};
  if (stale.value) return {valid:false};
  if (JSON.stringify(draft.value) === JSON.stringify(props.values ?? {})) return {valid:true};
  try {
    await formRef.value?.validate();
    if (!(await evaluateNow())) return {valid:false};
    await formRef.value?.validate();
    return {valid:true,values:{...draft.value}};
  } catch { return {valid:false}; }
}
defineExpose({prepare, dirty:computed(() => JSON.stringify(draft.value) !== JSON.stringify(props.values ?? {}))});
</script>

<template>
  <Card :bordered="true" :title="title" size="small">
    <template v-if="!editingInline && canEdit && surface !== 'customer'" #extra>
      <Button type="primary" @click="open = true">
        {{
          preferences.app.locale === "zh-CN" ? "编辑配置" : "Edit configuration"
        }}
      </Button>
    </template>
    <Descriptions
      v-if="!editingInline && declaration.fields.length"
      bordered
      :column="{ xs: 1, sm: 1, md: 2, lg: 2, xl: 2, xxl: 2 }"
      size="small"
    >
      <Descriptions.Item
        v-for="field in summaryFields"
        :key="field.key"
        :label="localized(field.label)"
      >
        <Tag v-if="field.type === 'secret'" color="success">
          {{ displayValue(field) }}
        </Tag>
        <Collapse
          v-else-if="customerChoices(field).length"
          :data-testid="`catalog-customer-choices-${field.key}`"
          ghost
        >
          <CollapsePanel
            key="choices"
            :header="
              catalogConfigurationMessage('customerChoices', {
                count: customerChoices(field).length,
              })
            "
          >
            <div class="mb-2 text-xs text-muted-foreground">
              {{ catalogConfigurationMessage("customerChoiceHint") }}
            </div>
            <div class="flex max-h-48 flex-wrap gap-1 overflow-y-auto">
              <Tag
                v-for="choice in customerChoices(field)"
                :key="choice.value"
                >{{ localized(choice.label) }}</Tag
              >
            </div>
          </CollapsePanel>
        </Collapse>
        <span v-else
          >{{ displayValue(field)
          }}<Button
            v-if="
              surface === 'customer' && field.type === 'integer' && canPrice
            "
            class="ml-2"
            size="small"
            :data-testid="`catalog-field-price-${field.key}`"
            @click="emit('price', field)"
            >{{ catalogConfigurationMessage("setPrice") }}</Button
          ></span
        >
      </Descriptions.Item>
    </Descriptions>
    <Empty v-else-if="!editingInline" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
    <section
      v-if="editingInline"
      data-testid="catalog-product-parameters-form"
      @keydown.esc.stop
    >
      <Alert
        v-if="stale"
        class="mb-4"
        type="warning"
        show-icon
        :message="workspaceText('configurationChanged')"
        ><template #action
          ><Popconfirm
            :title="workspaceText('reloadConfigurationConfirm')"
            @confirm="initializeDraft"
            ><Button data-testid="catalog-product-parameters-reload">{{
              workspaceText("reloadConfiguration")
            }}</Button></Popconfirm
          ></template
        ></Alert
      >
      <div :inert="saving || undefined">
        <CatalogConfigurationForm
          :expand-optional="expandOptional"
          ref="formRef"
          v-model="draft"
          :declaration="declaration"
          :evaluation-failed="evaluationFailed"
          :field-states="fieldStates"
          :issues="issues"
          :resolved-candidates="authoritativeCandidates"
          :revoke-file="revokeFile"
          :surface="surface"
          :upload-file="uploadFile"
          @retry="evaluateNow"
        />
      </div>
      <Button
        v-if="!externalSave"
        class="mt-4"
        type="primary"
        data-testid="catalog-product-parameters-save"
        :disabled="stale || evaluating || evaluationFailed"
        :loading="saving"
        @click="submit"
        >{{ workspaceText("saveProductParameters") }}</Button
      >
    </section>

    <VbenModalCompat
      v-if="!inline && presentation === 'modal'"
      v-model:open="open"
      :confirm-loading="saving"
      :title="title"
      @ok="submit"
    >
      <CatalogConfigurationForm
        :expand-optional="expandOptional"
        ref="formRef"
        v-model="draft"
        :declaration="declaration"
        :evaluation-failed="evaluationFailed"
        :field-states="fieldStates"
        :issues="issues"
        :resolved-candidates="authoritativeCandidates"
        @retry="evaluateNow"
        :revoke-file="revokeFile"
        :surface="surface"
        :upload-file="uploadFile"
      />
    </VbenModalCompat>
    <VbenDrawerCompat v-else-if="!inline" v-model:open="open" :title="title">
      <CatalogConfigurationForm
        :expand-optional="expandOptional"
        ref="formRef"
        v-model="draft"
        :declaration="declaration"
        :evaluation-failed="evaluationFailed"
        :field-states="fieldStates"
        :issues="issues"
        :resolved-candidates="authoritativeCandidates"
        @retry="evaluateNow"
        :revoke-file="revokeFile"
        :surface="surface"
        :upload-file="uploadFile"
      />
      <div class="mt-4 flex justify-end gap-2">
        <Button class="mr-2" @click="open = false">
          {{ preferences.app.locale === "zh-CN" ? "取消" : "Cancel" }}
        </Button>
        <Button
          :disabled="evaluating || evaluationFailed"
          :loading="saving || evaluating"
          type="primary"
          @click="submit"
        >
          {{ preferences.app.locale === "zh-CN" ? "保存" : "Save" }}
        </Button>
      </div>
    </VbenDrawerCompat>
  </Card>
</template>
