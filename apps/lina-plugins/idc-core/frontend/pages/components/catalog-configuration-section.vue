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
  Button,
  Card,
  Descriptions,
  Empty,
  message,
  Tag,
} from "ant-design-vue";

import { catalogConfigurationMessage } from "../catalog-configuration-i18n";
import { catalogConfigurationPresentation } from "../catalog-configuration-state";
import { catalogConfigurationRowsToValues } from "../catalog-configuration-state";
import CatalogConfigurationForm from "./catalog-configuration-form.vue";
import VbenDrawerCompat from "./vben-drawer-compat.vue";
import VbenModalCompat from "./vben-modal-compat.vue";

const props = defineProps<{
  canEdit?: boolean;
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
const emit = defineEmits<{ saved: [] }>();

const open = ref(false);
const saving = ref(false);
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
let evaluationTimer: ReturnType<typeof globalThis.setTimeout> | undefined;
const presentation = computed(() =>
  catalogConfigurationPresentation(props.declaration),
);

watch(open, (value) => {
  if (value) {
    draft.value = { ...(props.values ?? {}) };
    authoritativeCandidates.value = props.resolvedCandidates ?? {};
    if (props.evaluate) void evaluateNow();
    return;
  }
  if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
  evaluationTimer = undefined;
  evaluationSequence += 1;
});

watch(
  draft,
  () => {
    if (!open.value || !props.evaluate) return;
    issues.value = [];
    if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
    evaluationTimer = globalThis.setTimeout(() => void evaluateNow(), 250);
  },
  { deep: true },
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
      authoritativeCandidates.value = {};
      fieldStates.value = {};
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

function displayValue(field: CatalogConfigurationField) {
  if (field.type === "secret") return catalogConfigurationMessage("configured");
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
  if (saving.value) return;
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
    open.value = false;
    emit("saved");
  } catch (error) {
    if (error && typeof error === "object" && "errorFields" in error) return;
    // The request layer has already shown the localized server validation error.
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <Card :bordered="true" :title="title" size="small">
    <template v-if="canEdit && surface !== 'customer'" #extra>
      <Button type="primary" @click="open = true">
        {{
          preferences.app.locale === "zh-CN" ? "编辑配置" : "Edit configuration"
        }}
      </Button>
    </template>
    <Descriptions
      v-if="declaration.fields.length"
      bordered
      :column="2"
      size="small"
    >
      <Descriptions.Item
        v-for="field in declaration.fields.filter(
          (item) => item.type !== 'secret' || surface === 'interface',
        )"
        :key="field.key"
        :label="localized(field.label)"
      >
        <Tag v-if="field.type === 'secret'" color="success">
          {{ displayValue(field) }}
        </Tag>
        <span v-else>{{ displayValue(field) }}</span>
      </Descriptions.Item>
    </Descriptions>
    <Empty v-else :image="Empty.PRESENTED_IMAGE_SIMPLE" />

    <VbenModalCompat
      v-if="presentation === 'modal'"
      v-model:open="open"
      :confirm-loading="saving"
      :title="title"
      @ok="submit"
    >
      <CatalogConfigurationForm
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
      />
    </VbenModalCompat>
    <VbenDrawerCompat v-else v-model:open="open" :title="title">
      <CatalogConfigurationForm
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
