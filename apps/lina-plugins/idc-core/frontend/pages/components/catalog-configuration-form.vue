<script setup lang="ts">
import type {
  CatalogConfigurationCandidate,
  CatalogConfigurationAuthoritativeFieldState,
  CatalogConfigurationDeclaration,
  CatalogConfigurationField,
  CatalogConfigurationIssue,
  CatalogConfigurationSurface,
  CatalogConfigurationValues,
  CatalogLocalizedText,
} from "../catalog-configuration-state";

import { computed, reactive, ref, watch } from "vue";
import type { FormInstance } from "ant-design-vue";
import type { UploadRequestOption } from "ant-design-vue/es/vc-upload/interface";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Cascader,
  Checkbox,
  DatePicker,
  Form,
  FormItem,
  Input,
  Button,
  message,
  Select,
  TreeSelect,
  Upload,
} from "ant-design-vue";

import { catalogConfigurationMessage } from "../catalog-configuration-i18n";
import {
  catalogConfigurationCandidates,
  catalogConfigurationFileTokensWithout,
  catalogConfigurationFieldState,
  sanitizeCatalogConfigurationValues,
  validateCatalogConfigurationDraftValue,
} from "../catalog-configuration-state";

const props = defineProps<{
  configuredSecretKeys?: string[];
  declaration: CatalogConfigurationDeclaration;
  evaluationFailed?: boolean;
  fieldStates?: Record<string, CatalogConfigurationAuthoritativeFieldState>;
  issues?: CatalogConfigurationIssue[];
  modelValue?: CatalogConfigurationValues;
  readonly?: boolean;
  resolvedCandidates?: Record<string, CatalogConfigurationCandidate[]>;
  surface: CatalogConfigurationSurface;
  uploadFile?: (
    fieldKey: string,
    file: File,
  ) => Promise<{ fileName: string; fileToken: string }>;
  revokeFile?: (fileToken: string) => Promise<unknown>;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: CatalogConfigurationValues];
}>();

const values = reactive<CatalogConfigurationValues>({});
// Each template branch is selected by the declared field type. These aliases
// retain the same reactive object; they only express that narrowing to Vue TS.
const textValues = values as Record<string, string | undefined>;
const booleanValues = values as Record<string, boolean | undefined>;
const selectionValues = values as Record<string, string | string[] | undefined>;
const pathValues = values as Record<string, string[] | undefined>;
const formRef = ref<FormInstance>();
const UploadDragger = Upload.Dragger;
const uploadedNames = reactive<Record<string, Record<string, string>>>({});
const uploadBusy = reactive<Record<string, boolean>>({});
const removingTokens = reactive<Record<string, boolean>>({});

defineExpose({
  clearValidation: () => formRef.value?.clearValidate(),
  markFilesBound: () => {
    for (const fieldKey of Object.keys(uploadedNames))
      delete uploadedNames[fieldKey];
  },
  validate: () => formRef.value?.validate(),
});

function replaceValues(
  declaration: CatalogConfigurationDeclaration,
  modelValue: CatalogConfigurationValues | undefined,
) {
  for (const key of Object.keys(values)) delete values[key];
  for (const field of declaration.fields) {
    const supplied = modelValue?.[field.key];
    if (supplied !== undefined) values[field.key] = supplied;
  }
}

function sameValues(
  left: CatalogConfigurationValues,
  right: CatalogConfigurationValues | undefined,
) {
  const leftKeys = Object.keys(left).sort();
  const rightKeys = Object.keys(right ?? {}).sort();
  if (leftKeys.length !== rightKeys.length) return false;
  return leftKeys.every((key, index) => {
    if (key !== rightKeys[index]) return false;
    const leftValue = left[key];
    const rightValue = right?.[key];
    if (Array.isArray(leftValue) || Array.isArray(rightValue)) {
      return (
        Array.isArray(leftValue) &&
        Array.isArray(rightValue) &&
        leftValue.length === rightValue.length &&
        leftValue.every((item, itemIndex) => item === rightValue[itemIndex])
      );
    }
    return leftValue === rightValue;
  });
}

watch(
  () => props.declaration,
  (declaration) => replaceValues(declaration, props.modelValue),
  { deep: true, immediate: true },
);

watch(
  () => props.modelValue,
  (modelValue) => {
    // A parent ref wraps emitted objects in a reactive proxy, so reference
    // equality cannot identify our own v-model round trip. Compare the small,
    // bounded typed value set and only rebuild controls for a real change.
    if (sameValues(values, modelValue)) return;
    replaceValues(props.declaration, modelValue);
  },
  { deep: true },
);

watch(
  values,
  () => {
    const sanitized = sanitizeCatalogConfigurationValues(
      props.declaration,
      props.surface,
      values,
    );
    emit("update:modelValue", sanitized);
  },
  { deep: true },
);

const visibleFields = computed(() =>
  props.declaration.fields.filter(
    (field) =>
      !(field.type === "secret" && props.surface !== "interface") &&
      fieldState(field).visible,
  ),
);

function localized(value?: CatalogLocalizedText) {
  if (!value) return "";
  return preferences.app.locale === "zh-CN"
    ? value.zhCN || value.enUS
    : value.enUS || value.zhCN;
}

function fieldState(field: CatalogConfigurationField) {
  return (
    props.fieldStates?.[field.key] ??
    catalogConfigurationFieldState(props.declaration, field, values)
  );
}

function preservesConfiguredSecret(field: CatalogConfigurationField) {
  return (
    field.type === "secret" &&
    props.configuredSecretKeys?.includes(field.key) === true &&
    !values[field.key]
  );
}

function candidates(field: CatalogConfigurationField) {
  const resolved = props.resolvedCandidates?.[field.key];
  if (resolved !== undefined) return resolved;
  return catalogConfigurationCandidates(props.declaration, field, values);
}

function selectOptions(field: CatalogConfigurationField) {
  return candidates(field).map((candidate) => ({
    label: localized(candidate.label),
    value: candidate.value,
  }));
}

interface CandidateTreeNode {
  children: CandidateTreeNode[];
  label: string;
  value: string;
}

function treeOptions(field: CatalogConfigurationField) {
  const source = candidates(field);
  const nodes = new Map<string, CandidateTreeNode>();
  for (const candidate of source) {
    nodes.set(candidate.value, {
      children: [],
      label: localized(candidate.label),
      value: candidate.value,
    });
  }
  const roots: CandidateTreeNode[] = [];
  for (const candidate of source) {
    const node = nodes.get(candidate.value);
    if (!node) continue;
    const parent = candidate.parentValue
      ? nodes.get(candidate.parentValue)
      : undefined;
    if (parent) parent.children.push(node);
    else roots.push(node);
  }
  return roots;
}

function disabled(field: CatalogConfigurationField) {
  return (
    props.readonly ||
    (field.type === "file" && !props.uploadFile) ||
    props.evaluationFailed ||
    (Boolean(field.dynamicCandidates) &&
      props.resolvedCandidates?.[field.key] === undefined) ||
    fieldState(field).disabled
  );
}

function uploadPolicy(field: CatalogConfigurationField) {
  return props.declaration.uploadPolicies?.find(
    (policy) => policy.id === field.uploadPolicy,
  );
}

function fileTokens(field: CatalogConfigurationField) {
  const value = values[field.key];
  return Array.isArray(value) ? value : [];
}

function fileLabel(
  field: CatalogConfigurationField,
  token: string,
  index: number,
) {
  return (
    uploadedNames[field.key]?.[token] ||
    `${catalogConfigurationMessage("configuredFile")} ${index + 1}`
  );
}

async function uploadConfigurationFile(
  field: CatalogConfigurationField,
  option: UploadRequestOption,
) {
  const file = option.file as File;
  const policy = uploadPolicy(field);
  if (
    !props.uploadFile ||
    !policy ||
    fileTokens(field).length >= policy.maximumFiles
  ) {
    option.onError?.(new Error("upload_unavailable"));
    return;
  }
  uploadBusy[field.key] = true;
  try {
    const item = await props.uploadFile(field.key, file);
    uploadedNames[field.key] ??= {};
    uploadedNames[field.key]![item.fileToken] = item.fileName;
    values[field.key] = [...fileTokens(field), item.fileToken];
    option.onSuccess?.(item);
    message.success(catalogConfigurationMessage("fileUploaded"));
  } catch (error) {
    option.onError?.(error as Error);
    message.error(catalogConfigurationMessage("fileUploadFailed"));
  } finally {
    uploadBusy[field.key] = false;
  }
}

async function removeConfigurationFile(
  field: CatalogConfigurationField,
  token: string,
) {
  if (removingTokens[token]) return;
  const isTemporaryUpload = Boolean(uploadedNames[field.key]?.[token]);
  if (isTemporaryUpload && !props.revokeFile) {
    message.error(catalogConfigurationMessage("fileRemoveFailed"));
    return;
  }
  removingTokens[token] = true;
  try {
    if (isTemporaryUpload) await props.revokeFile!(token);
    values[field.key] = catalogConfigurationFileTokensWithout(
      values[field.key],
      token,
    );
    delete uploadedNames[field.key]?.[token];
    message.success(
      catalogConfigurationMessage(
        isTemporaryUpload ? "fileRemoved" : "fileReferenceRemoved",
      ),
    );
  } catch {
    message.error(catalogConfigurationMessage("fileRemoveFailed"));
  } finally {
    removingTokens[token] = false;
  }
}

function authoritativeIssue(field: CatalogConfigurationField) {
  return props.issues?.find((issue) => issue.fieldKey === field.key)?.issueCode;
}

function rules(field: CatalogConfigurationField) {
  return [
    {
      validator: async () => {
        if (preservesConfiguredSecret(field)) return;
        const issue =
          validateCatalogConfigurationDraftValue(
            props.declaration,
            field,
            values,
          ) || authoritativeIssue(field);
        if (issue) {
          throw new Error(
            catalogConfigurationMessage(
              (issue in catalogConfigurationMessages
                ? issue
                : "invalid") as Parameters<
                typeof catalogConfigurationMessage
              >[0],
            ),
          );
        }
      },
    },
  ];
}

const catalogConfigurationMessages = {
  decimal: true,
  excludes: true,
  integer: true,
  invalid: true,
  maximum: true,
  maximumLength: true,
  minimum: true,
  minimumLength: true,
  required: true,
  requires: true,
};

function inputType(field: CatalogConfigurationField) {
  if (field.type === "secret") return "password";
  if (field.textFormat === "email") return "email";
  if (field.textFormat === "url") return "url";
  if (field.textFormat === "phone") return "tel";
  return "text";
}

function popupContainer(trigger: HTMLElement) {
  return (
    (trigger.closest('[role="dialog"]') as HTMLElement | null) ??
    trigger.parentElement ??
    document.body
  );
}
function optionPathFilter(input: string, path: Array<{ label?: unknown }>) {
  const value = input.toLocaleLowerCase();
  return path.some((option) =>
    String(option.label ?? "")
      .toLocaleLowerCase()
      .includes(value),
  );
}

function description(field: CatalogConfigurationField) {
  if (field.type === "secret") return catalogConfigurationMessage("secretHint");
  return localized(field.description);
}
</script>

<template>
  <Form
    ref="formRef"
    class="grid grid-cols-1 gap-x-4 md:grid-cols-2"
    layout="vertical"
    :model="values"
  >
    <Alert
      v-if="evaluationFailed"
      class="md:col-span-2"
      :message="catalogConfigurationMessage('evaluationFailed')"
      show-icon
      type="error"
    />
    <FormItem
      v-for="field in visibleFields"
      :key="field.key"
      :class="{
        'md:col-span-2': [
          'address',
          'file',
          'multi_line',
          'tree_select',
        ].includes(field.type),
      }"
      :extra="description(field)"
      :label="localized(field.label)"
      :name="field.key"
      :required="
        fieldState(field).required && !preservesConfiguredSecret(field)
      "
      :rules="rules(field)"
    >
      <Alert
        v-if="
          field.dynamicCandidates &&
          resolvedCandidates?.[field.key] === undefined
        "
        :message="catalogConfigurationMessage('dynamicCandidatesPending')"
        show-icon
        type="warning"
      />
      <Input.TextArea
        v-else-if="field.type === 'multi_line'"
        v-model:value="textValues[field.key]"
        :disabled="disabled(field)"
        :maxlength="field.maximumLength"
        :rows="4"
        show-count
      />
      <Input
        v-else-if="['integer', 'decimal'].includes(field.type)"
        v-model:value="textValues[field.key]"
        :disabled="disabled(field)"
        inputmode="decimal"
      />
      <Checkbox
        v-else-if="field.type === 'boolean'"
        v-model:checked="booleanValues[field.key]"
        :disabled="disabled(field)"
      >
        {{ localized(field.description) }}
      </Checkbox>
      <Select
        v-else-if="['single_select', 'multi_select'].includes(field.type)"
        v-model:value="selectionValues[field.key]"
        allow-clear
        class="w-full"
        :disabled="disabled(field)"
        :get-popup-container="popupContainer"
        :max-tag-count="'responsive'"
        :mode="field.type === 'multi_select' ? 'multiple' : undefined"
        :options="selectOptions(field)"
        option-filter-prop="label"
        show-search
      />
      <DatePicker
        v-else-if="['date', 'datetime'].includes(field.type)"
        v-model:value="textValues[field.key]"
        class="w-full"
        :disabled="disabled(field)"
        :get-popup-container="popupContainer"
        :show-time="field.type === 'datetime'"
        :value-format="
          field.type === 'datetime' ? 'YYYY-MM-DDTHH:mm:ssZ' : 'YYYY-MM-DD'
        "
      />
      <TreeSelect
        v-else-if="field.type === 'tree_select'"
        v-model:value="values[field.key]"
        allow-clear
        class="w-full"
        :disabled="disabled(field)"
        :get-popup-container="popupContainer"
        :tree-data="treeOptions(field)"
        show-search
        tree-default-expand-all
        tree-node-filter-prop="label"
      />
      <Cascader
        v-else-if="field.type === 'address'"
        v-model:value="pathValues[field.key]"
        allow-clear
        class="w-full"
        :disabled="disabled(field)"
        :get-popup-container="popupContainer"
        :options="treeOptions(field)"
        :show-search="{ filter: optionPathFilter }"
      />
      <div v-else-if="field.type === 'file'">
        <UploadDragger
          :accept="uploadPolicy(field)?.allowedExtensions.join(',')"
          :custom-request="(option) => uploadConfigurationFile(field, option)"
          :disabled="disabled(field) || uploadBusy[field.key]"
          :file-list="[]"
          :multiple="(uploadPolicy(field)?.maximumFiles ?? 1) > 1"
        >
          <p>{{ catalogConfigurationMessage("fileUpload") }}</p>
          <p class="text-muted-foreground">
            {{
              catalogConfigurationMessage("filePolicy", {
                count: uploadPolicy(field)?.maximumFiles ?? 0,
                extensions:
                  uploadPolicy(field)?.allowedExtensions.join("、") || "",
              })
            }}
          </p>
        </UploadDragger>
        <div
          v-for="(token, index) in fileTokens(field)"
          :key="token"
          class="mt-2 flex items-center justify-between rounded border px-3 py-2"
        >
          <span>{{ fileLabel(field, token, index) }}</span>
          <Button
            danger
            :loading="removingTokens[token]"
            size="small"
            type="link"
            @click="removeConfigurationFile(field, token)"
            >{{
              catalogConfigurationMessage(
                uploadedNames[field.key]?.[token] ? "fileRevoke" : "fileDetach",
              )
            }}</Button
          >
        </div>
      </div>
      <Input
        v-else
        v-model:value="textValues[field.key]"
        :disabled="disabled(field)"
        :maxlength="field.maximumLength"
        :type="inputType(field)"
      />
    </FormItem>
  </Form>
</template>
