<script setup lang="ts">
import type {
  ContractTemplateField,
  ContractTemplateLanguage,
  ContractTemplateProviderComponent,
  ContractTemplateProviderValue,
  ContractTemplateProviderRecipient,
  ContractTemplateScope,
  ContractTemplateSigningOrder,
  ContractTemplateSourceMode,
  ElectronicContractTemplate,
  ElectronicContractTemplateEditor,
} from "../electronic-contract-template-client";

import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { onBeforeRouteLeave } from "vue-router";

import {
  Alert,
  Button,
  Card,
  Form,
  FormItem,
  Input,
  message,
  Modal,
  RadioButton,
  RadioGroup,
  Select,
  Space,
  Spin,
  Tag,
  Upload,
} from "ant-design-vue";

import { $t } from "#/locales";

import {
  electronicContractTemplateCreate,
  electronicContractTemplateEditorGet,
  electronicContractTemplateFieldSetSave,
  electronicContractTemplateOnlineSourceSave,
  electronicContractTemplatePreviewConfirm,
  electronicContractTemplatePreviewDownload,
  electronicContractTemplatePreviewGenerate,
  electronicContractTemplateProviderMappingSave,
  electronicContractTemplateProviderPreview,
  electronicContractTemplateUpdate,
  electronicContractTemplateUploadSource,
} from "../electronic-contract-template-client";
import ContractTemplatePdfFields from "./contract-template-pdf-fields.vue";
import ContractTemplateRichEditor from "./contract-template-rich-editor.vue";

const props = defineProps<{
  item?: ElectronicContractTemplate;
  mode: "create" | "edit" | "view";
}>();

const emit = defineEmits<{
  close: [payload: { saved: boolean }];
}>();

const currentItem = ref<ElectronicContractTemplate | undefined>(props.item);
const editorDetail = ref<ElectronicContractTemplateEditor>();
const loading = ref(false);
const saving = ref(false);
const generating = ref(false);
const confirming = ref(false);
const savingFields = ref(false);
const openingProvider = ref(false);
const savingProviderMapping = ref(false);
const sourceMode = ref<ContractTemplateSourceMode>("uploaded_file");
const onlineHTML = ref("");
const selectedFile = ref<File>();
const fields = ref<ContractTemplateField[]>([]);
const pdfBlob = ref<Blob>();
const createKey = ref(commandKey("create"));
const sourceKey = ref(commandKey("source"));
const previewKey = ref(commandKey("preview"));
const fieldKey = ref(commandKey("fields"));
const providerMappingKey = ref(commandKey("provider-mapping"));
const initialMetadata = ref("");
const initialHTML = ref("");
const initialFields = ref("");
const initialProviderMapping = ref("");
const savedDuringVisit = ref(false);

const form = reactive({
  languageCode: (props.item?.languageCode ?? "") as ContractTemplateLanguage,
  name: props.item?.name ?? "",
  scopeCode: (props.item?.scopeCode ?? "") as ContractTemplateScope,
  signingOrderCode: (props.item?.signingOrderCode ??
    "") as ContractTemplateSigningOrder,
});

const editable = computed(() => props.mode !== "view");
const localDraft = computed(
  () =>
    editable.value &&
    currentItem.value?.originCode === "local" &&
    currentItem.value.statusCode === "draft",
);
const metadataEditable = computed(
  () =>
    editable.value &&
    (!currentItem.value ||
      currentItem.value.statusCode === "draft" ||
      currentItem.value.statusCode === "needs_configuration"),
);
const nameReadonly = computed(
  () =>
    !metadataEditable.value ||
    currentItem.value?.originCode === "tencent_synced",
);
const metadataValid = computed(
  () =>
    form.name.trim().length > 0 &&
    form.languageCode.length > 0 &&
    form.scopeCode.length > 0 &&
    form.signingOrderCode.length > 0,
);
const metadataDirty = computed(
  () => metadataEditable.value && metadataSnapshot() !== initialMetadata.value,
);
const sourceDirty = computed(() => {
  if (!localDraft.value) return false;
  if (sourceMode.value === "uploaded_file") return Boolean(selectedFile.value);
  return (
    onlineHTML.value.trim().length > 0 && onlineHTML.value !== initialHTML.value
  );
});
const fieldsDirty = computed(
  () =>
    Boolean(currentItem.value) &&
    JSON.stringify(normalizedFields(fields.value)) !== initialFields.value,
);
const providerMappingEditable = computed(
  () =>
    editable.value &&
    currentItem.value?.originCode === "tencent_synced" &&
    currentItem.value.statusCode === "needs_configuration" &&
    !metadataDirty.value &&
    metadataValid.value &&
    !editorDetail.value?.providerMappingSaved,
);
const providerMappingDirty = computed(
  () =>
    providerMappingEditable.value &&
    providerMappingSnapshot() !== initialProviderMapping.value,
);
const requiredProviderRecipients = computed(
  () =>
    editorDetail.value?.providerRecipients.filter(
      (recipient) => recipient.requireSign || recipient.signComponentCount > 0,
    ) ?? [],
);
const providerMappingComplete = computed(() => {
  const detail = editorDetail.value;
  if (!detail || form.scopeCode === "all_customers") return false;
  const requiredFieldsReady = detail.providerComponents.every(
    (component) => !component.required || Boolean(component.valueCode),
  );
  const serviceTables = detail.providerComponents.filter(
    (component) => component.valueCode === "service_lines",
  ).length;
  const requiredRecipientsReady = requiredProviderRecipients.value.every(
    (recipient) => Boolean(recipient.partyCode),
  );
  const parties = new Set(
    requiredProviderRecipients.value.map((recipient) => recipient.partyCode),
  );
  return (
    requiredFieldsReady &&
    serviceTables === 1 &&
    requiredRecipientsReady &&
    parties.has("customer") &&
    parties.has("merchant")
  );
});
const dirty = computed(
  () =>
    metadataDirty.value ||
    sourceDirty.value ||
    fieldsDirty.value ||
    providerMappingDirty.value,
);
const contentReady = computed(
  () => editorDetail.value?.content?.conversionStatusCode === "ready",
);
const scopeOptions = computed(() => [
  {
    label: $t("plugin.idc-compliance.contract.template.scopes.personal"),
    value: "personal",
  },
  {
    label: $t("plugin.idc-compliance.contract.template.scopes.enterprise"),
    value: "enterprise",
  },
  {
    disabled: currentItem.value?.originCode === "tencent_synced",
    label: $t("plugin.idc-compliance.contract.template.scopes.all_customers"),
    value: "all_customers",
  },
]);
const orderOptions = computed(() => [
  {
    label: $t("plugin.idc-compliance.contract.template.orders.customer_first"),
    value: "customer_first",
  },
  {
    label: $t("plugin.idc-compliance.contract.template.orders.merchant_first"),
    value: "merchant_first",
  },
  {
    label: $t("plugin.idc-compliance.contract.template.orders.parallel"),
    value: "parallel",
  },
]);
const languageOptions = computed(() => [
  {
    label: $t("plugin.idc-compliance.contract.template.languages.zh-CN"),
    value: "zh-CN",
  },
  {
    label: $t("plugin.idc-compliance.contract.template.languages.en-US"),
    value: "en-US",
  },
]);

function commandKey(scope: string) {
  return `contract-template-${scope}-${crypto.randomUUID()}`;
}

function metadataSnapshot() {
  return JSON.stringify({
    languageCode: form.languageCode,
    name: form.name.trim(),
    scopeCode: form.scopeCode,
    signingOrderCode: form.signingOrderCode,
  });
}

function normalizedFields(value: ContractTemplateField[]) {
  return value.map(
    ({ fieldNo: _fieldNo, sequenceNo: _sequenceNo, ...field }) => field,
  );
}

function providerMappingSnapshot() {
  const detail = editorDetail.value;
  if (!detail) return "";
  return JSON.stringify({
    fields: detail.providerComponents.map(({ componentRef, valueCode }) => ({
      componentRef,
      valueCode: valueCode ?? "",
    })),
    recipients: detail.providerRecipients.map(
      ({ partyCode, recipientRef }) => ({
        partyCode: partyCode ?? "",
        recipientRef,
      }),
    ),
  });
}

function providerValueOptions(component: ContractTemplateProviderComponent) {
  const codes: ContractTemplateProviderValue[] =
    component.typeCode === "DYNAMIC_TABLE"
      ? ["service_lines"]
      : component.typeCode === "SIGN_DATE"
        ? ["signing_date"]
        : [
            "contract_no",
            "customer_name",
            "merchant_name",
            "order_no",
            "total_amount",
            "currency",
          ];
  if (
    component.typeCode !== "TEXT" &&
    component.typeCode !== "MULTI_LINE_TEXT" &&
    component.typeCode !== "DYNAMIC_TABLE" &&
    component.typeCode !== "SIGN_DATE"
  ) {
    return [];
  }
  return codes.map((value) => ({
    label: $t(
      `plugin.idc-compliance.contract.template.providerValues.${value}`,
    ),
    value,
  }));
}

function providerComponentTypeLabel(typeCode: string) {
  const known = ["DYNAMIC_TABLE", "MULTI_LINE_TEXT", "SIGN_DATE", "TEXT"];
  const key = known.includes(typeCode) ? typeCode : "unsupported";
  return $t(`plugin.idc-compliance.contract.template.providerTypes.${key}`);
}

function providerRecipientTypeLabel(
  recipient: ContractTemplateProviderRecipient,
) {
  const type = recipient.isPromoter ? "PROMOTER" : recipient.typeCode;
  return $t(
    `plugin.idc-compliance.contract.template.providerRecipientTypes.${type}`,
  );
}

function providerRoutingLabel(recipient: ContractTemplateProviderRecipient) {
  if (recipient.routingOrder === null || recipient.routingOrder === undefined) {
    return $t(
      "plugin.idc-compliance.contract.template.providerMapping.orderMissing",
    );
  }
  return $t("plugin.idc-compliance.contract.template.providerMapping.order", {
    order: recipient.routingOrder,
  });
}

const providerPartyOptions = computed(() => [
  {
    label: $t(
      "plugin.idc-compliance.contract.template.providerParties.customer",
    ),
    value: "customer",
  },
  {
    label: $t(
      "plugin.idc-compliance.contract.template.providerParties.merchant",
    ),
    value: "merchant",
  },
]);

function applyEditor(value: ElectronicContractTemplateEditor) {
  editorDetail.value = value;
  currentItem.value = value.template;
  fields.value = value.fields.map((field) => ({ ...field }));
  initialFields.value = JSON.stringify(normalizedFields(fields.value));
  if (value.content) sourceMode.value = value.content.sourceModeCode;
  onlineHTML.value = value.onlineHtml ?? "";
  initialHTML.value = onlineHTML.value;
  selectedFile.value = undefined;
  form.name = value.template.name;
  form.languageCode = value.template.languageCode;
  form.scopeCode = value.template.scopeCode;
  form.signingOrderCode = value.template.signingOrderCode;
  initialMetadata.value = metadataSnapshot();
  initialProviderMapping.value = providerMappingSnapshot();
}

async function loadEditor() {
  if (!currentItem.value) return;
  loading.value = true;
  try {
    const result = await electronicContractTemplateEditorGet(
      currentItem.value.templateNo,
      currentItem.value.versionNo,
    );
    applyEditor(result.item);
    await loadPDF();
  } finally {
    loading.value = false;
  }
}

async function loadPDF() {
  pdfBlob.value = undefined;
  const detail = editorDetail.value;
  if (!detail?.content || detail.content.conversionStatusCode !== "ready")
    return;
  pdfBlob.value = await electronicContractTemplatePreviewDownload(
    detail.template.templateNo,
    detail.template.versionNo,
    detail.content.contentRevisionNo,
  );
}

async function saveMetadata() {
  if (!metadataEditable.value || !metadataValid.value || saving.value) return;
  if (!metadataDirty.value && currentItem.value) return;
  saving.value = true;
  try {
    if (!currentItem.value) {
      const created = await electronicContractTemplateCreate({
        idempotencyKey: createKey.value,
        languageCode: form.languageCode as Exclude<
          ContractTemplateLanguage,
          ""
        >,
        name: form.name.trim(),
        scopeCode: form.scopeCode,
        signingOrderCode: form.signingOrderCode,
      });
      currentItem.value = created.item;
      createKey.value = commandKey("create");
      await loadEditor();
    } else {
      const updated = await electronicContractTemplateUpdate({
        expectedVersion: currentItem.value.lockVersion,
        languageCode: form.languageCode as Exclude<
          ContractTemplateLanguage,
          ""
        >,
        name: form.name.trim(),
        scopeCode: form.scopeCode as Exclude<ContractTemplateScope, "">,
        signingOrderCode: form.signingOrderCode as Exclude<
          ContractTemplateSigningOrder,
          ""
        >,
        templateNo: currentItem.value.templateNo,
        versionNo: currentItem.value.versionNo,
      });
      currentItem.value = updated.item;
      if (editorDetail.value) editorDetail.value.template = updated.item;
      initialMetadata.value = metadataSnapshot();
    }
    savedDuringVisit.value = true;
    message.success(
      $t("plugin.idc-compliance.contract.template.messages.metadataSaved"),
    );
  } finally {
    saving.value = false;
  }
}

function beforeUpload(file: File) {
  if (!localDraft.value) return false;
  selectedFile.value = file;
  return false;
}

function sourceMediaType(file: File) {
  const extension = file.name.toLowerCase().split(".").pop();
  if (extension === "doc") return "application/msword";
  if (extension === "docx") {
    return "application/vnd.openxmlformats-officedocument.wordprocessingml.document";
  }
  if (extension === "pdf") return "application/pdf";
  if (extension === "html" || extension === "htm") return "text/html";
  return file.type || "application/octet-stream";
}

async function approveSourceReplacement() {
  const currentMode = editorDetail.value?.content?.sourceModeCode;
  if (!currentMode || currentMode === sourceMode.value) return true;
  return new Promise<boolean>((resolve) => {
    Modal.confirm({
      cancelText: $t("plugin.idc-compliance.contract.template.actions.cancel"),
      content: $t(
        "plugin.idc-compliance.contract.template.editor.switchSourceDescription",
      ),
      okText: $t("plugin.idc-compliance.contract.template.actions.confirm"),
      onCancel: () => resolve(false),
      onOk: () => resolve(true),
      title: $t(
        "plugin.idc-compliance.contract.template.editor.switchSourceTitle",
      ),
    });
  });
}

async function saveSource() {
  const item = currentItem.value;
  if (!item || !localDraft.value || !sourceDirty.value || saving.value) return;
  if (!(await approveSourceReplacement())) return;
  saving.value = true;
  try {
    let result: { item: ElectronicContractTemplateEditor };
    if (sourceMode.value === "uploaded_file") {
      const file = selectedFile.value;
      if (!file) return;
      result = await electronicContractTemplateUploadSource({
        expectedVersion: item.lockVersion,
        file,
        idempotencyKey: sourceKey.value,
        mediaType: sourceMediaType(file),
        templateNo: item.templateNo,
        versionNo: item.versionNo,
      });
    } else {
      result = await electronicContractTemplateOnlineSourceSave({
        expectedVersion: item.lockVersion,
        html: onlineHTML.value,
        idempotencyKey: sourceKey.value,
        templateNo: item.templateNo,
        versionNo: item.versionNo,
      });
    }
    applyEditor(result.item);
    pdfBlob.value = undefined;
    sourceKey.value = commandKey("source");
    previewKey.value = commandKey("preview");
    fieldKey.value = commandKey("fields");
    savedDuringVisit.value = true;
    message.success(
      $t("plugin.idc-compliance.contract.template.messages.sourceSaved"),
    );
  } finally {
    saving.value = false;
  }
}

async function generatePreview() {
  const detail = editorDetail.value;
  if (
    !detail?.content ||
    !localDraft.value ||
    sourceDirty.value ||
    generating.value
  )
    return;
  generating.value = true;
  try {
    const result = await electronicContractTemplatePreviewGenerate({
      contentRevisionNo: detail.content.contentRevisionNo,
      expectedVersion: detail.template.lockVersion,
      idempotencyKey: previewKey.value,
      templateNo: detail.template.templateNo,
      versionNo: detail.template.versionNo,
    });
    applyEditor(result.item);
    previewKey.value = commandKey("preview");
    fieldKey.value = commandKey("fields");
    savedDuringVisit.value = true;
    await loadPDF();
    message.success(
      $t("plugin.idc-compliance.contract.template.messages.previewGenerated"),
    );
  } finally {
    generating.value = false;
  }
}

async function confirmPreview() {
  const detail = editorDetail.value;
  if (
    !detail?.content ||
    !contentReady.value ||
    detail.previewConfirmed ||
    confirming.value
  )
    return;
  confirming.value = true;
  try {
    const result = await electronicContractTemplatePreviewConfirm({
      contentRevisionNo: detail.content.contentRevisionNo,
      expectedVersion: detail.template.lockVersion,
      templateNo: detail.template.templateNo,
      versionNo: detail.template.versionNo,
    });
    applyEditor(result.item);
    savedDuringVisit.value = true;
    message.success(
      $t("plugin.idc-compliance.contract.template.messages.previewConfirmed"),
    );
  } finally {
    confirming.value = false;
  }
}

async function saveFieldSet() {
  const detail = editorDetail.value;
  if (
    !detail?.content ||
    !localDraft.value ||
    !detail.previewConfirmed ||
    !fieldsDirty.value ||
    fields.value.length === 0 ||
    savingFields.value
  )
    return;
  savingFields.value = true;
  try {
    const result = await electronicContractTemplateFieldSetSave({
      contentRevisionNo: detail.content.contentRevisionNo,
      expectedVersion: detail.template.lockVersion,
      fields: normalizedFields(fields.value),
      idempotencyKey: fieldKey.value,
      templateNo: detail.template.templateNo,
      versionNo: detail.template.versionNo,
    });
    applyEditor(result.item);
    fieldKey.value = commandKey("fields");
    savedDuringVisit.value = true;
    message.success(
      $t("plugin.idc-compliance.contract.template.messages.fieldsSaved"),
    );
  } finally {
    savingFields.value = false;
  }
}

async function saveProviderMapping() {
  const detail = editorDetail.value;
  if (
    !detail ||
    !providerMappingEditable.value ||
    !providerMappingDirty.value ||
    !providerMappingComplete.value ||
    savingProviderMapping.value
  )
    return;
  savingProviderMapping.value = true;
  try {
    const result = await electronicContractTemplateProviderMappingSave({
      expectedVersion: detail.template.lockVersion,
      fieldMappings: detail.providerComponents
        .filter((component) => Boolean(component.valueCode))
        .map((component) => ({
          componentRef: component.componentRef,
          valueCode: component.valueCode as ContractTemplateProviderValue,
        })),
      idempotencyKey: providerMappingKey.value,
      recipientMappings: requiredProviderRecipients.value.map((recipient) => ({
        partyCode: recipient.partyCode as "customer" | "merchant",
        recipientRef: recipient.recipientRef,
      })),
      templateNo: detail.template.templateNo,
      versionNo: detail.template.versionNo,
    });
    applyEditor(result.item);
    providerMappingKey.value = commandKey("provider-mapping");
    savedDuringVisit.value = true;
    message.success(
      $t(
        "plugin.idc-compliance.contract.template.messages.providerMappingSaved",
      ),
    );
  } finally {
    savingProviderMapping.value = false;
  }
}

async function saveAndClose() {
  if (!editable.value) {
    emit("close", { saved: savedDuringVisit.value });
    return;
  }
  if (metadataDirty.value || !currentItem.value) await saveMetadata();
  if (sourceDirty.value) await saveSource();
  if (fieldsDirty.value && editorDetail.value?.previewConfirmed)
    await saveFieldSet();
  if (providerMappingDirty.value) await saveProviderMapping();
  if (!dirty.value) emit("close", { saved: savedDuringVisit.value });
}

async function openProviderPreview() {
  const item = currentItem.value;
  if (!item || item.originCode !== "tencent_synced" || openingProvider.value)
    return;
  const externalWindow = window.open("about:blank", "_blank");
  if (!externalWindow) {
    message.warning(
      $t("plugin.idc-compliance.contract.template.messages.popupBlocked"),
    );
    return;
  }
  externalWindow.opener = null;
  openingProvider.value = true;
  try {
    const result = await electronicContractTemplateProviderPreview(
      item.templateNo,
      item.versionNo,
    );
    const parsed = new URL(result.previewUrl);
    if (parsed.protocol !== "https:")
      throw new Error("unsafe provider preview");
    externalWindow.location.replace(parsed.href);
  } catch (error) {
    externalWindow.close();
    throw error;
  } finally {
    openingProvider.value = false;
  }
}

function warnBeforeUnload(event: BeforeUnloadEvent) {
  if (!dirty.value) return;
  event.preventDefault();
  event.returnValue = "";
}

function confirmLeave(leave: () => void) {
  if (!dirty.value) {
    leave();
    return;
  }
  Modal.confirm({
    cancelText: $t("plugin.idc-compliance.contract.template.actions.cancel"),
    content: $t(
      "plugin.idc-compliance.contract.template.editor.unsavedDescription",
    ),
    okText: $t("plugin.idc-compliance.contract.template.editor.leave"),
    onOk: leave,
    title: $t("plugin.idc-compliance.contract.template.editor.unsavedTitle"),
  });
}

function close() {
  confirmLeave(() => emit("close", { saved: savedDuringVisit.value }));
}

onBeforeRouteLeave((_to, _from, next) => {
  if (!dirty.value) next();
  else confirmLeave(() => next());
});
onMounted(() => {
  initialMetadata.value = metadataSnapshot();
  window.addEventListener("beforeunload", warnBeforeUnload);
  if (currentItem.value) loadEditor();
});
onBeforeUnmount(() =>
  window.removeEventListener("beforeunload", warnBeforeUnload),
);
</script>

<template>
  <section class="template-editor" data-testid="contract-template-editor">
    <Spin :spinning="loading">
      <Alert
        v-if="currentItem?.originCode === 'tencent_synced'"
        :description="
          $t(
            'plugin.idc-compliance.contract.template.editor.tencentReadonlyDescription',
          )
        "
        :message="
          $t(
            'plugin.idc-compliance.contract.template.editor.tencentReadonlyTitle',
          )
        "
        show-icon
        type="info"
      />

      <Card :bordered="false" class="editor-card">
        <template #title>
          <span class="step-title"
            ><b>1</b
            >{{
              $t("plugin.idc-compliance.contract.template.editor.metadata")
            }}</span
          >
        </template>
        <Form layout="vertical">
          <FormItem
            :label="$t('plugin.idc-compliance.contract.template.editor.name')"
            required
          >
            <Input
              v-model:value="form.name"
              data-testid="contract-template-name"
              :maxlength="200"
              :readonly="nameReadonly"
            />
          </FormItem>
          <div class="metadata-grid">
            <FormItem
              :label="
                $t('plugin.idc-compliance.contract.template.columns.scope')
              "
              required
            >
              <Select
                v-model:value="form.scopeCode"
                data-testid="contract-template-scope"
                :disabled="!metadataEditable"
                :options="scopeOptions"
              />
            </FormItem>
            <FormItem
              :label="
                $t('plugin.idc-compliance.contract.template.columns.order')
              "
              required
            >
              <Select
                v-model:value="form.signingOrderCode"
                data-testid="contract-template-order"
                :disabled="!metadataEditable"
                :options="orderOptions"
              />
            </FormItem>
            <FormItem
              :label="
                $t('plugin.idc-compliance.contract.template.columns.language')
              "
              required
            >
              <Select
                v-model:value="form.languageCode"
                data-testid="contract-template-language"
                :disabled="!metadataEditable"
                :options="languageOptions"
              />
            </FormItem>
          </div>
          <Button
            v-if="metadataEditable"
            data-testid="contract-template-save-metadata"
            :disabled="!metadataValid || (!metadataDirty && !!currentItem)"
            :loading="saving"
            type="primary"
            @click="saveMetadata"
            >{{
              $t("plugin.idc-compliance.contract.template.actions.saveMetadata")
            }}</Button
          >
        </Form>
      </Card>

      <Card
        v-if="currentItem?.originCode === 'tencent_synced'"
        :bordered="false"
        class="editor-card provider-preview-card"
      >
        <template #title>
          <span class="step-title"
            ><b>2</b
            >{{
              $t(
                "plugin.idc-compliance.contract.template.editor.providerPreview",
              )
            }}</span
          >
        </template>
        <p>
          {{
            $t(
              "plugin.idc-compliance.contract.template.editor.providerPreviewDescription",
            )
          }}
        </p>
        <Button
          :loading="openingProvider"
          type="primary"
          @click="openProviderPreview"
        >
          {{
            $t(
              "plugin.idc-compliance.contract.template.actions.openProviderPreview",
            )
          }}
        </Button>
      </Card>

      <Card
        v-if="currentItem?.originCode === 'tencent_synced'"
        :bordered="false"
        class="editor-card provider-mapping-card"
        data-testid="contract-template-provider-mapping"
      >
        <template #title>
          <span class="step-title"
            ><b>3</b
            >{{
              $t(
                "plugin.idc-compliance.contract.template.providerMapping.title",
              )
            }}</span
          >
        </template>
        <Alert
          v-if="form.scopeCode === 'all_customers'"
          :description="
            $t(
              'plugin.idc-compliance.contract.template.providerMapping.allCustomersDescription',
            )
          "
          :message="
            $t(
              'plugin.idc-compliance.contract.template.providerMapping.allCustomersTitle',
            )
          "
          show-icon
          type="warning"
        />
        <p class="mapping-description">
          {{
            $t(
              "plugin.idc-compliance.contract.template.providerMapping.description",
            )
          }}
        </p>
        <h3>
          {{
            $t("plugin.idc-compliance.contract.template.providerMapping.fields")
          }}
        </h3>
        <div class="mapping-list">
          <div
            v-for="component in editorDetail?.providerComponents"
            :key="component.componentRef"
            class="mapping-row"
            :data-testid="`contract-template-provider-component-${component.componentRef}`"
          >
            <div>
              <strong>{{ component.name }}</strong>
              <div class="mapping-meta">
                <Tag :color="component.required ? 'orange' : 'default'">
                  {{
                    component.required
                      ? $t(
                          "plugin.idc-compliance.contract.template.providerMapping.required",
                        )
                      : $t(
                          "plugin.idc-compliance.contract.template.providerMapping.optional",
                        )
                  }}
                </Tag>
                <span>{{
                  providerComponentTypeLabel(component.typeCode)
                }}</span>
              </div>
            </div>
            <Select
              v-model:value="component.valueCode"
              allow-clear
              :disabled="!providerMappingEditable"
              :options="providerValueOptions(component)"
              :placeholder="
                $t(
                  'plugin.idc-compliance.contract.template.providerMapping.chooseValue',
                )
              "
            />
          </div>
        </div>
        <h3>
          {{
            $t(
              "plugin.idc-compliance.contract.template.providerMapping.signers",
            )
          }}
        </h3>
        <div class="mapping-list">
          <div
            v-for="recipient in requiredProviderRecipients"
            :key="recipient.recipientRef"
            class="mapping-row"
            :data-testid="`contract-template-provider-recipient-${recipient.recipientRef}`"
          >
            <div>
              <strong>{{ recipient.roleName }}</strong>
              <div class="mapping-meta">
                <span>{{ providerRecipientTypeLabel(recipient) }}</span>
                <span>{{ providerRoutingLabel(recipient) }}</span>
              </div>
            </div>
            <Select
              v-model:value="recipient.partyCode"
              :disabled="!providerMappingEditable"
              :options="providerPartyOptions"
              :placeholder="
                $t(
                  'plugin.idc-compliance.contract.template.providerMapping.chooseParty',
                )
              "
            />
          </div>
        </div>
        <Alert
          :message="
            editorDetail?.enableReady
              ? $t(
                  'plugin.idc-compliance.contract.template.providerMapping.ready',
                )
              : $t(
                  `plugin.idc-compliance.contract.template.enableReasons.${editorDetail?.enableReasonCode || 'provider_mapping_incomplete'}`,
                )
          "
          show-icon
          :type="editorDetail?.enableReady ? 'success' : 'warning'"
        />
        <Button
          v-if="providerMappingEditable"
          data-testid="contract-template-save-provider-mapping"
          :disabled="!providerMappingDirty || !providerMappingComplete"
          :loading="savingProviderMapping"
          type="primary"
          @click="saveProviderMapping"
          >{{
            $t(
              "plugin.idc-compliance.contract.template.actions.saveProviderMapping",
            )
          }}</Button
        >
      </Card>

      <template v-if="currentItem?.originCode === 'local'">
        <Card :bordered="false" class="editor-card">
          <template #title>
            <span class="step-title"
              ><b>2</b
              >{{
                $t("plugin.idc-compliance.contract.template.editor.body")
              }}</span
            >
          </template>
          <RadioGroup
            v-model:value="sourceMode"
            button-style="solid"
            :disabled="!localDraft"
          >
            <RadioButton value="uploaded_file">{{
              $t("plugin.idc-compliance.contract.template.editor.uploadedFile")
            }}</RadioButton>
            <RadioButton value="online_editor">{{
              $t("plugin.idc-compliance.contract.template.editor.onlineEditor")
            }}</RadioButton>
          </RadioGroup>
          <div v-if="sourceMode === 'uploaded_file'" class="upload-source">
            <Upload
              :accept="'.doc,.docx,.pdf,.html,.htm'"
              :before-upload="beforeUpload"
              :disabled="!localDraft"
              :show-upload-list="false"
            >
              <Button :disabled="!localDraft">{{
                $t("plugin.idc-compliance.contract.template.actions.chooseFile")
              }}</Button>
            </Upload>
            <span>{{
              selectedFile?.name ||
              $t("plugin.idc-compliance.contract.template.editor.uploadHelp")
            }}</span>
          </div>
          <ContractTemplateRichEditor
            v-else
            v-model="onlineHTML"
            :disabled="!localDraft"
          />
          <div v-if="localDraft" class="section-actions">
            <Button
              data-testid="contract-template-save-source"
              :disabled="!sourceDirty"
              :loading="saving"
              type="primary"
              @click="saveSource"
            >
              {{
                $t("plugin.idc-compliance.contract.template.actions.saveSource")
              }}
            </Button>
            <span>{{
              $t(
                "plugin.idc-compliance.contract.template.editor.sourceInvalidates",
              )
            }}</span>
          </div>
        </Card>

        <Card :bordered="false" class="editor-card">
          <template #title>
            <span class="step-title"
              ><b>3</b
              >{{
                $t("plugin.idc-compliance.contract.template.editor.previewStep")
              }}</span
            >
          </template>
          <Alert
            v-if="editorDetail?.content && !contentReady"
            :message="
              $t(
                'plugin.idc-compliance.contract.template.editor.previewNotGenerated',
              )
            "
            show-icon
            type="warning"
          />
          <Space v-if="localDraft" wrap>
            <Button
              data-testid="contract-template-generate-preview"
              :disabled="!editorDetail?.content || sourceDirty"
              :loading="generating"
              type="primary"
              @click="generatePreview"
              >{{
                $t(
                  "plugin.idc-compliance.contract.template.actions.generatePreview",
                )
              }}</Button
            >
            <Button
              data-testid="contract-template-confirm-preview"
              :disabled="!contentReady || editorDetail?.previewConfirmed"
              :loading="confirming"
              @click="confirmPreview"
              >{{
                editorDetail?.previewConfirmed
                  ? $t(
                      "plugin.idc-compliance.contract.template.editor.previewConfirmed",
                    )
                  : $t(
                      "plugin.idc-compliance.contract.template.actions.confirmPreview",
                    )
              }}</Button
            >
          </Space>
          <ContractTemplatePdfFields
            v-if="contentReady"
            v-model="fields"
            :disabled="!localDraft || !editorDetail?.previewConfirmed"
            :pdf="pdfBlob"
            :scope-code="form.scopeCode"
          />
        </Card>

        <Card v-if="contentReady" :bordered="false" class="editor-card">
          <template #title>
            <span class="step-title"
              ><b>4</b
              >{{
                $t("plugin.idc-compliance.contract.template.editor.fieldsStep")
              }}</span
            >
          </template>
          <Alert
            :message="
              editorDetail?.enableReady
                ? $t(
                    'plugin.idc-compliance.contract.template.editor.enableReady',
                  )
                : $t(
                    `plugin.idc-compliance.contract.template.enableReasons.${editorDetail?.enableReasonCode || 'signing_positions_incomplete'}`,
                  )
            "
            show-icon
            :type="editorDetail?.enableReady ? 'success' : 'warning'"
          />
          <Space v-if="localDraft" wrap>
            <Button
              data-testid="contract-template-save-fields"
              :disabled="
                !editorDetail?.previewConfirmed ||
                !fieldsDirty ||
                fields.length === 0
              "
              :loading="savingFields"
              type="primary"
              @click="saveFieldSet"
              >{{
                $t("plugin.idc-compliance.contract.template.actions.saveFields")
              }}</Button
            >
            <Button data-testid="contract-template-submit-tencent" disabled>
              {{
                $t(
                  "plugin.idc-compliance.contract.template.actions.submitTencent",
                )
              }}
            </Button>
            <span class="submission-note">
              {{
                $t(
                  "plugin.idc-compliance.contract.template.editor.tencentSubmissionUnavailable",
                )
              }}
            </span>
          </Space>
        </Card>
      </template>
    </Spin>

    <footer class="editor-actions">
      <Space wrap>
        <Button
          v-if="editable"
          data-testid="contract-template-save"
          :disabled="!metadataValid"
          :loading="saving || savingFields"
          type="primary"
          @click="saveAndClose"
        >
          {{
            $t("plugin.idc-compliance.contract.template.actions.saveAndClose")
          }}
        </Button>
        <Button @click="close">{{
          $t("plugin.idc-compliance.contract.template.actions.close")
        }}</Button>
      </Space>
      <span v-if="dirty">{{
        $t("plugin.idc-compliance.contract.template.editor.unsaved")
      }}</span>
    </footer>
  </section>
</template>

<style scoped>
.template-editor {
  display: grid;
  gap: 18px;
  min-height: 100%;
}
.editor-card {
  border: 1px solid rgb(148 163 184 / 20%);
  border-radius: 0;
  margin-top: 18px;
}
.step-title {
  align-items: center;
  display: flex;
  gap: 10px;
}
.step-title b {
  align-items: center;
  background: rgb(14 165 233 / 12%);
  border-radius: 50%;
  color: rgb(2 132 199);
  display: inline-flex;
  font:
    800 11px/1 ui-monospace,
    monospace;
  height: 26px;
  justify-content: center;
  width: 26px;
}
.metadata-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}
.upload-source {
  align-items: center;
  background: hsl(var(--muted) / 50%);
  border: 1px dashed rgb(14 165 233 / 30%);
  border-radius: 0;
  display: flex;
  gap: 14px;
  margin-top: 16px;
  min-height: 92px;
  padding: 18px;
}
.upload-source span,
.section-actions span,
.provider-preview-card p {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}
.section-actions {
  align-items: center;
  display: flex;
  gap: 12px;
  margin-top: 14px;
}
.submission-note {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  max-width: 560px;
}
.provider-preview-card p {
  line-height: 1.7;
}
.mapping-description {
  color: hsl(var(--muted-foreground));
  line-height: 1.7;
}
.provider-mapping-card h3 {
  font-size: 14px;
  margin: 20px 0 10px;
}
.mapping-list {
  border: 1px solid rgb(148 163 184 / 20%);
  border-radius: 0;
  overflow: hidden;
}
.mapping-row {
  align-items: center;
  display: grid;
  gap: 18px;
  grid-template-columns: minmax(0, 1fr) minmax(260px, 38%);
  padding: 14px 16px;
}
.mapping-row + .mapping-row {
  border-top: 1px solid rgb(148 163 184 / 16%);
}
.mapping-meta {
  align-items: center;
  color: hsl(var(--muted-foreground));
  display: flex;
  flex-wrap: wrap;
  font-size: 12px;
  gap: 8px;
  margin-top: 6px;
}
.editor-card :deep(.placement-shell) {
  margin-top: 16px;
}
.editor-card :deep(.ant-alert) {
  margin-bottom: 14px;
}
.editor-actions {
  align-items: center;
  background: hsl(var(--card));
  border: 1px solid rgb(148 163 184 / 20%);
  border-radius: 0;
  bottom: 12px;
  box-shadow: none;
  display: flex;
  justify-content: space-between;
  padding: 14px 18px;
  position: sticky;
  z-index: 5;
}
.editor-actions > span {
  color: rgb(217 119 6);
  font-size: 12px;
}
@media (max-width: 900px) {
  .metadata-grid,
  .mapping-row {
    grid-template-columns: 1fr;
  }
}
</style>
