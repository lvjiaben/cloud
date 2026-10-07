<script setup lang="ts">
import type { AutomationInterfaceConfiguration } from "../catalog-automation-client";
import type {
  CatalogConfigurationAuthoritativeFieldState,
  CatalogConfigurationCandidate,
  CatalogConfigurationIssue,
  CatalogConfigurationValues,
} from "../catalog-configuration-state";

import { computed, onBeforeUnmount, ref, watch } from "vue";
import { Button, message, Spin } from "ant-design-vue";

import { $t } from "#/locales";

import {
  automationInterfaceConfigurationGet,
  automationInterfaceConfigurationEvaluate,
  automationInterfaceConfigurationUpdate,
  automationInterfaceConfigurationFileUpload,
  automationInterfaceConfigurationFileRevoke,
} from "../catalog-automation-client";
import {
  catalogConfigurationPresentation,
  catalogConfigurationRowsToValues,
  catalogConfigurationValuesToRows,
} from "../catalog-configuration-state";
import CatalogConfigurationForm from "./catalog-configuration-form.vue";
import VbenDrawerCompat from "./vben-drawer-compat.vue";
import VbenModalCompat from "./vben-modal-compat.vue";

const props = defineProps<{
  disabled?: boolean;
  interfaceInstanceNo: string;
}>();
const emit = defineEmits<{ saved: [] }>();
const open = ref(false);
const loading = ref(false);
const saving = ref(false);
const configuration = ref<AutomationInterfaceConfiguration>();
const draft = ref<CatalogConfigurationValues>({});
const formRef = ref<InstanceType<typeof CatalogConfigurationForm>>();
const resolvedCandidates = ref<Record<string, CatalogConfigurationCandidate[]>>(
  {},
);
const fieldStates = ref<
  Record<string, CatalogConfigurationAuthoritativeFieldState>
>({});
const issues = ref<CatalogConfigurationIssue[]>([]);
const evaluating = ref(false);
const evaluationFailed = ref(false);
let evaluationSequence = 0;
let evaluationTimer: ReturnType<typeof globalThis.setTimeout> | undefined;
const presentation = computed(() =>
  configuration.value?.declaration
    ? catalogConfigurationPresentation(configuration.value.declaration)
    : "modal",
);

async function show() {
  if (props.disabled || loading.value) return;
  loading.value = true;
  open.value = true;
  try {
    const result = await automationInterfaceConfigurationGet(
      props.interfaceInstanceNo,
    );
    configuration.value = result.configuration;
    formRef.value?.markFilesBound();
    if (!result.configuration.declaration) {
      open.value = false;
      message.info(
        $t("plugin.idc-core.catalogAutomation.configuration.noFields"),
      );
      return;
    }
    draft.value = catalogConfigurationRowsToValues(
      result.configuration.values,
      result.configuration.declaration,
    );
    resolvedCandidates.value = result.configuration.resolvedCandidates ?? {};
    await evaluateNow();
  } catch {
    open.value = false;
  } finally {
    loading.value = false;
  }
}

function scheduleEvaluation() {
  if (!open.value || !configuration.value?.declaration) return;
  issues.value = [];
  if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
  evaluationTimer = globalThis.setTimeout(() => void evaluateNow(), 250);
}

async function evaluateNow() {
  if (!configuration.value?.declaration) return false;
  if (evaluationTimer) {
    globalThis.clearTimeout(evaluationTimer);
    evaluationTimer = undefined;
  }
  const sequence = ++evaluationSequence;
  evaluating.value = true;
  try {
    const result = await automationInterfaceConfigurationEvaluate(
      props.interfaceInstanceNo,
      {
        expectedRevision: configuration.value.revision,
        values: catalogConfigurationValuesToRows(
          configuration.value.declaration,
          "interface",
          draft.value,
        ),
      },
    );
    if (sequence !== evaluationSequence) return false;
    resolvedCandidates.value = result.evaluation.candidates ?? {};
    fieldStates.value = result.evaluation.fieldStates ?? {};
    issues.value = result.evaluation.issues ?? [];
    evaluationFailed.value = false;
    if (result.evaluation.valid) {
      const normalized = catalogConfigurationRowsToValues(
        result.evaluation.values,
        configuration.value.declaration,
      );
      for (const field of configuration.value.declaration.fields) {
        if (field.type === "secret" && draft.value[field.key] !== undefined) {
          normalized[field.key] = draft.value[field.key];
        }
      }
      if (JSON.stringify(normalized) !== JSON.stringify(draft.value)) {
        draft.value = normalized;
      }
    }
    return result.evaluation.valid;
  } catch {
    if (sequence === evaluationSequence) {
      resolvedCandidates.value = {};
      fieldStates.value = {};
      issues.value = [];
      evaluationFailed.value = true;
    }
    return false;
  } finally {
    if (sequence === evaluationSequence) evaluating.value = false;
  }
}

watch(draft, scheduleEvaluation, { deep: true });
watch(open, (isOpen) => {
  if (isOpen) return;
  if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
  evaluationTimer = undefined;
  evaluationSequence += 1;
  evaluating.value = false;
  evaluationFailed.value = false;
});

onBeforeUnmount(() => {
  if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
  evaluationSequence += 1;
});

async function submit() {
  if (!configuration.value?.declaration || saving.value) return;
  try {
    await formRef.value?.validate();
    const valid = await evaluateNow();
    await formRef.value?.validate();
    if (!valid) return;
    saving.value = true;
    const result = await automationInterfaceConfigurationUpdate(
      props.interfaceInstanceNo,
      {
        values: catalogConfigurationValuesToRows(
          configuration.value.declaration,
          "interface",
          draft.value,
        ),
        expectedRevision: configuration.value.revision,
      },
    );
    configuration.value = result.configuration;
    message.success(
      $t("plugin.idc-core.catalogAutomation.configuration.saved"),
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

function uploadFile(fieldKey: string, file: File) {
  if (!configuration.value) throw new Error("configuration unavailable");
  return automationInterfaceConfigurationFileUpload(
    props.interfaceInstanceNo,
    configuration.value.revision,
    fieldKey,
    file,
  );
}
function revokeFile(fileToken: string) {
  if (!configuration.value) throw new Error("configuration unavailable");
  return automationInterfaceConfigurationFileRevoke(
    props.interfaceInstanceNo,
    configuration.value.revision,
    fileToken,
  );
}
</script>

<template>
  <ghost-button
    :data-testid="`automation-interface-configuration-${interfaceInstanceNo}`"
    :disabled="disabled"
    @click="show"
  >
    {{ $t("plugin.idc-core.catalogAutomation.configuration.action") }}
  </ghost-button>
  <VbenModalCompat
    v-if="presentation === 'modal'"
    v-model:open="open"
    :confirm-loading="saving"
    :title="$t('plugin.idc-core.catalogAutomation.configuration.title')"
    @ok="submit"
  >
    <Spin :spinning="loading">
      <CatalogConfigurationForm
        v-if="configuration?.declaration"
        ref="formRef"
        v-model="draft"
        :configured-secret-keys="
          configuration.secrets
            .filter((secret) => secret.configured)
            .map((secret) => secret.fieldKey)
        "
        :declaration="configuration.declaration"
        :evaluation-failed="evaluationFailed"
        :field-states="fieldStates"
        :issues="issues"
        :resolved-candidates="resolvedCandidates"
        :revoke-file="revokeFile"
        surface="interface"
        :upload-file="uploadFile"
      />
    </Spin>
  </VbenModalCompat>
  <VbenDrawerCompat
    v-else
    v-model:open="open"
    :title="$t('plugin.idc-core.catalogAutomation.configuration.title')"
  >
    <Spin :spinning="loading">
      <CatalogConfigurationForm
        v-if="configuration?.declaration"
        ref="formRef"
        v-model="draft"
        :configured-secret-keys="
          configuration.secrets
            .filter((secret) => secret.configured)
            .map((secret) => secret.fieldKey)
        "
        :declaration="configuration.declaration"
        :evaluation-failed="evaluationFailed"
        :field-states="fieldStates"
        :issues="issues"
        :resolved-candidates="resolvedCandidates"
        :revoke-file="revokeFile"
        surface="interface"
        :upload-file="uploadFile"
      />
    </Spin>
    <div class="mt-4 flex justify-end gap-2">
      <Button class="mr-2" @click="open = false">
        {{ $t("plugin.idc-core.catalogAutomation.configuration.cancel") }}
      </Button>
      <Button
        :disabled="evaluating || evaluationFailed"
        :loading="saving || evaluating"
        type="primary"
        @click="submit"
      >
        {{ $t("plugin.idc-core.catalogAutomation.configuration.save") }}
      </Button>
    </div>
  </VbenDrawerCompat>
</template>
