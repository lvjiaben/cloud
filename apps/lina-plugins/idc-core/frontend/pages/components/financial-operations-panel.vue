<script setup lang="ts">
import type {
  FinancialDeliveryPreview,
  FinancialExportStatus,
  FinancialInternalNote,
  FinancialSavedFilter,
} from "../financial-client";
import type { VbenFormSchema } from "#/adapter/form";
import type { ButtonProps } from "ant-design-vue";

import { computed, onMounted, ref, watch } from "vue";

import { useAccess } from "../plugin-access";

import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  DescriptionsItem,
  Input,
  List,
  ListItem,
  message,
  Modal as AntModal,
  Row,
  Space,
  Tag,
} from "ant-design-vue";
import { useVbenForm } from "#/adapter/form";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import {
  appendFinancialInternalNote,
  createFinancialExport,
  financialExportDownloadPath,
  financialExportTicket,
  financialInternalNotes,
  financialInvoiceDeliveryPreview,
  financialSavedFilters,
  generateFinancialExport,
  resendFinancialInvoice,
  saveFinancialFilter,
} from "../financial-client";

const props = defineProps<{
  currentFilter: Record<string, unknown>;
  viewCode: "adjustment" | "invoice" | "receivable" | "transaction";
}>();
const emit = defineEmits<{
  applyFilter: [filter: Record<string, unknown>];
}>();

const { hasAccessByCodes } = useAccess();
const canResend = computed(() =>
  hasAccessByCodes(["idc:admin:billing:invoice:resend"]),
);
const canReadNotes = computed(() =>
  hasAccessByCodes(["idc:admin:billing:note:list"]),
);
const canCreateNotes = computed(() =>
  hasAccessByCodes(["idc:admin:billing:note:create"]),
);
const canSaveFilters = computed(() =>
  hasAccessByCodes(["idc:admin:billing:filter:personal"]),
);
const canExport = computed(() =>
  hasAccessByCodes(["idc:admin:billing:export"]),
);

const invoiceNo = ref("");
const deliveryPreview = ref<FinancialDeliveryPreview>();
const deliveryLoading = ref(false);

const notes = ref<FinancialInternalNote[]>([]);
const notesLoading = ref(false);

const filterName = ref("");
const savedFilters = ref<FinancialSavedFilter[]>([]);
const filterLoading = ref(false);

const exportStatus = ref<FinancialExportStatus>();
const exportLoading = ref(false);

function t(key: string) {
  return $t(`plugin.idc-core.finance.operations.${key}`);
}

function noteFormSchema(): VbenFormSchema[] {
  const schema: VbenFormSchema[] = [
    {
      component: "Select" as const,
      componentProps: {
        options: ["order", "invoice", "service"].map((value) => ({
          label: t(`notes.target.${value}`),
          value,
        })),
      },
      defaultValue: "invoice",
      fieldName: "targetType",
      label: t("notes.targetType"),
    },
    {
      component: "Input" as const,
      componentProps: { "data-testid": "finance-note-target" },
      fieldName: "targetNo",
      label: t("notes.targetNo"),
    },
  ];
  if (canCreateNotes.value) {
    schema.push(
      {
        component: "Textarea" as const,
        componentProps: {
          "data-testid": "finance-note-content",
          maxlength: 2000,
          rows: 3,
        },
        fieldName: "content",
        label: t("notes.content"),
      },
      {
        component: "Select" as const,
        componentProps: {
          allowClear: true,
          options: notes.value.map((item) => ({
            label: `${formatTimestamp(item.createdAt)} · ${item.content.slice(0, 48)}`,
            value: item.noteNo,
          })),
          showSearch: true,
          optionFilterProp: "label",
        },
        fieldName: "correctsNoteNo",
        label: t("notes.corrects"),
      },
    );
  }
  return schema;
}

const [NoteForm, noteFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});
onMounted(async () => {
  await noteFormApi.setState({ schema: noteFormSchema() });
});

function requestNo(prefix: string) {
  return `${prefix}-${crypto.randomUUID()}`;
}


function exportStatusLabel(value: string) {
  if (["failed", "generating", "queued", "ready"].includes(value)) {
    return t(`export.status.${value}`);
  }
  return t("export.status.unknown");
}

async function previewDelivery() {
  deliveryLoading.value = true;
  deliveryPreview.value = undefined;
  try {
    deliveryPreview.value = await financialInvoiceDeliveryPreview(
      invoiceNo.value.trim(),
    );
  } catch {
    message.error(t("resend.failed"));
  } finally {
    deliveryLoading.value = false;
  }
}

async function confirmDelivery() {
  if (!deliveryPreview.value) return;
  const okButtonProps: ButtonProps & { class: string } = {
    class: "finance-resend-submit",
  };
  AntModal.confirm({
    content: t("resend.confirmDetail").replace(
      "{email}",
      deliveryPreview.value.maskedEmail,
    ),
    okButtonProps,
    okText: t("confirm"),
    title: t("resend.confirmTitle"),
    async onOk() {
      await resendFinancialInvoice(
        deliveryPreview.value!.invoiceNo,
        requestNo("resend"),
      );
      message.success(t("resend.succeeded"));
      deliveryPreview.value = undefined;
    },
  });
}

async function loadNotes() {
  const values = await noteFormApi.getValues<{
    targetNo?: string;
    targetType?: "invoice" | "order" | "service";
  }>();
  notesLoading.value = true;
  try {
    notes.value = await financialInternalNotes(
      values.targetType || "invoice",
      values.targetNo?.trim() || "",
    );
    const currentValues = await noteFormApi.getValues();
    await noteFormApi.setState({ schema: noteFormSchema() });
    await noteFormApi.setValues({ ...currentValues, correctsNoteNo: "" });
  } catch {
    message.error(t("notes.failed"));
  } finally {
    notesLoading.value = false;
  }
}

async function appendNote() {
  const values = await noteFormApi.getValues<{
    content?: string;
    correctsNoteNo?: string;
    targetNo?: string;
    targetType?: "invoice" | "order" | "service";
  }>();
  notesLoading.value = true;
  try {
    await appendFinancialInternalNote({
      content: values.content?.trim() || "",
      correctsNoteNo: values.correctsNoteNo?.trim() || undefined,
      requestNo: requestNo("note"),
      targetNo: values.targetNo?.trim() || "",
      targetType: values.targetType || "invoice",
    });
    await noteFormApi.setValues({ content: "", correctsNoteNo: "" });
    await loadNotes();
    message.success(t("notes.succeeded"));
  } catch {
    message.error(t("notes.failed"));
  } finally {
    notesLoading.value = false;
  }
}

async function loadFilters() {
  if (!canSaveFilters.value) return;
  filterLoading.value = true;
  try {
    savedFilters.value = await financialSavedFilters(props.viewCode);
  } finally {
    filterLoading.value = false;
  }
}

async function saveFilter() {
  filterLoading.value = true;
  try {
    await saveFinancialFilter({
      displayName: filterName.value.trim(),
      filter: props.currentFilter,
      viewCode: props.viewCode,
    });
    filterName.value = "";
    await loadFilters();
    message.success(t("filters.succeeded"));
  } catch {
    message.error(t("filters.failed"));
  } finally {
    filterLoading.value = false;
  }
}

function applySavedFilter(item: FinancialSavedFilter) {
  emit("applyFilter", { ...item.filter });
  message.success(t("filters.applied"));
}

async function exportXlsx() {
  exportLoading.value = true;
  try {
    const created = await createFinancialExport({
      exportType: props.viewCode,
      filter: props.currentFilter,
      requestNo: requestNo("export"),
    });
    exportStatus.value = await generateFinancialExport(created.requestNo);
    message.success(t("export.ready"));
  } catch {
    message.error(t("export.failed"));
  } finally {
    exportLoading.value = false;
  }
}

async function downloadXlsx() {
  if (!exportStatus.value) return;
  const ticket = await financialExportTicket(exportStatus.value.requestNo);
  window.open(financialExportDownloadPath(ticket.token), "_blank", "noopener");
}

watch(
  () => props.viewCode,
  () => {
    exportStatus.value = undefined;
    void loadFilters();
  },
  { immediate: true },
);
</script>

<template>
  <Card :title="t('title')" class="mb-4" data-testid="financial-operations-panel">
    <Alert :message="t('boundary')" show-icon type="info" class="mb-4" />
    <Row :gutter="16">
      <Col v-if="canResend" :lg="12" :xs="24">
        <Card :title="t('resend.title')" size="small" class="h-full">
          <Space direction="vertical" class="w-full">
            <Input
              v-model:value="invoiceNo"
              data-testid="finance-resend-invoice"
              :placeholder="t('resend.invoice')"
            />
            <Button
              data-testid="finance-resend-preview"
              :loading="deliveryLoading"
              @click="previewDelivery"
              >{{ t("resend.preview") }}</Button
            >
            <Descriptions
              v-if="deliveryPreview"
              bordered
              size="small"
              :column="1"
            >
              <DescriptionsItem :label="t('resend.destination')">{{
                deliveryPreview.maskedEmail
              }}</DescriptionsItem>
            </Descriptions>
            <Button
              v-if="deliveryPreview"
              data-testid="finance-resend-confirm"
              type="primary"
              @click="confirmDelivery"
              >{{ t("resend.confirm") }}</Button
            >
          </Space>
        </Card>
      </Col>
      <Col v-if="canReadNotes" :lg="12" :xs="24">
        <Card :title="t('notes.title')" size="small" class="h-full">
          <NoteForm />
          <Space class="mb-3"
            ><Button :loading="notesLoading" @click="loadNotes">{{
              t("notes.load")
            }}</Button></Space
          >
          <template v-if="canCreateNotes">
            <Button
              data-testid="finance-note-append"
              type="primary"
              :loading="notesLoading"
              @click="appendNote"
              >{{ t("notes.append") }}</Button
            >
          </template>
          <List :data-source="notes" size="small" class="mt-3">
            <template #renderItem="{ item }">
              <ListItem>
                <div>
                  <div>{{ item.content }}</div>
                  <small>{{ formatTimestamp(item.createdAt) }}</small>
                </div>
              </ListItem>
            </template>
          </List>
        </Card>
      </Col>
    </Row>

    <Row :gutter="16" class="mt-4">
      <Col v-if="canSaveFilters" :lg="8" :xs="24">
        <Card :title="t('filters.title')" size="small" class="h-full">
          <Space direction="vertical" class="w-full">
            <Input
              v-model:value="filterName"
              data-testid="finance-filter-name"
              :placeholder="t('filters.name')"
            />
            <Button
              data-testid="finance-filter-save"
              type="primary"
              :loading="filterLoading"
              @click="saveFilter"
              >{{ t("filters.save") }}</Button
            >
            <List :data-source="savedFilters" size="small">
              <template #renderItem="{ item }">
                <ListItem>
                  <span>
                    {{ item.displayName }}
                    <Tag>{{ t(`filters.view.${item.viewCode}`) }}</Tag>
                  </span>
                  <Button type="link" @click="applySavedFilter(item)">{{
                    t("filters.apply")
                  }}</Button>
                </ListItem>
              </template>
            </List>
          </Space>
        </Card>
      </Col>
      <Col v-if="canExport" :lg="16" :xs="24">
        <Card :title="t('export.title')" size="small" class="h-full">
          <Space direction="vertical" class="w-full">
            <Alert :message="t('export.detail')" type="warning" />
            <Button
              data-testid="finance-export-create"
              type="primary"
              :loading="exportLoading"
              @click="exportXlsx"
              >{{ t("export.create") }}</Button
            >
            <template v-if="exportStatus">
              <Tag color="success">{{
                exportStatusLabel(exportStatus.status)
              }}</Tag>
              <span>{{ t("export.rows") }}: {{ exportStatus.rowCount }}</span>
              <span v-if="exportStatus.expiresAt"
                >{{ t("export.expires") }}:
                {{ formatTimestamp(exportStatus.expiresAt) }}</span
              >
              <Button @click="downloadXlsx">{{ t("export.download") }}</Button>
            </template>
          </Space>
        </Card>
      </Col>
    </Row>

  </Card>
</template>
