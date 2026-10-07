<script setup lang="ts">
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { VbenFormSchema } from "#/adapter/form";

import type {
  ContractTemplateScope,
  ContractTemplateStatus,
  ElectronicContractTemplate,
} from "../electronic-contract-template-client";

import { computed, onMounted, onBeforeUnmount, ref, watch } from "vue";
import { createNumberedPage } from "../numbered-page";
import { preferences } from "@vben/preferences";

import {
  Button,
  Card,
  message,
  Modal,
  Space,
  Tag,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";

import { $t } from "#/locales";

import {
  electronicContractTemplateCopy,
  electronicContractTemplateDisable,
  electronicContractTemplateEnable,
  electronicContractTemplateList,
  electronicContractTemplateTencentSync,
} from "../electronic-contract-template-client";

const props = defineProps<{
  canManage: boolean;
  refreshToken: number;
}>();

const emit = defineEmits<{
  openEditor: [
    payload: {
      item?: ElectronicContractTemplate;
      mode: "create" | "edit" | "view";
    },
  ];
}>();

const loading = ref(false);
const syncing = ref(false);
const items = ref<ElectronicContractTemplate[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(10);
let loadSequence = 0;
const pager = createNumberedPage<ElectronicContractTemplate>((state) => {
  items.value = state.items;
  total.value = state.total;
  page.value = state.currentPage;
  pageSize.value = state.pageSize;
  gridApi.setGridOptions({ pagerConfig: { enabled: true, currentPage: state.currentPage, pageSize: state.pageSize, total: state.total } });
});
onBeforeUnmount(() => { loadSequence++; pager.invalidate(); });
const name = ref("");
const scopeCode = ref<ContractTemplateScope | "">("");
const statusCode = ref<ContractTemplateStatus | "">("");

function columns(): VxeGridProps<ElectronicContractTemplate>["columns"] {
  return [
    {
      field: "name",
      minWidth: 260,
      slots: { default: "name" },
      title: $t("plugin.idc-compliance.contract.template.columns.name"),
    },
    {
      field: "scopeCode",
      slots: { default: "scopeCode" },
      title: $t("plugin.idc-compliance.contract.template.columns.scope"),
      width: 140,
    },
    {
      field: "signingOrderCode",
      slots: { default: "signingOrderCode" },
      title: $t("plugin.idc-compliance.contract.template.columns.order"),
      width: 150,
    },
    {
      field: "languageCode",
      slots: { default: "languageCode" },
      title: $t("plugin.idc-compliance.contract.template.columns.language"),
      width: 130,
    },
    {
      field: "statusCode",
      slots: { default: "statusCode" },
      title: $t("plugin.idc-compliance.contract.template.columns.status"),
      width: 150,
    },
    {
      field: "updatedAt",
      slots: { default: "updatedAt" },
      title: $t("plugin.idc-compliance.contract.template.columns.updatedAt"),
      width: 190,
    },
    {
      field: "actions",
      fixed: "right",
      slots: { default: "actions" },
      title: $t("plugin.idc-compliance.contract.template.columns.actions"),
      width: 300,
    },
  ];
}
const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    commonConfig: { componentProps: { allowClear: true }, labelWidth: 90 },
    schema: [],
    handleSubmit: search,
    handleReset: reset,
    submitButtonOptions: { 'data-testid': 'contract-template-search' },
    wrapperClass: "grid-cols-1 md:grid-cols-2 xl:grid-cols-3",
  },
  gridEvents: { pageChange: ({ currentPage, pageSize }: { currentPage: number; pageSize: number }) => load(currentPage, pageSize) },
  gridOptions: {
    columns: columns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0, pageSizes: [10,20,50,100] },
    rowConfig: { keyField: "templateNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-electronic-contract-templates",
  },
});
watch(items, (rows) => gridApi.setGridOptions({ data: rows }), { deep: true });
watch(
  () => preferences.app.locale,
  () => {
    gridApi.setGridOptions({ columns: columns() });
    gridApi.formApi.setState({ schema: searchSchema() });
  },
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
    label: $t("plugin.idc-compliance.contract.template.scopes.all_customers"),
    value: "all_customers",
  },
]);

const statusOptions = computed(() =>
  [
    "draft",
    "needs_configuration",
    "submission_unavailable",
    "submitted",
    "enabled",
    "disabled",
    "provider_disabled",
    "provider_offline",
  ].map((value) => ({
    label: statusText(value as ContractTemplateStatus),
    value,
  })),
);

function searchSchema(): VbenFormSchema[] {
  return [
    {
      component: "Input",
      componentProps: {
        allowClear: true,
        "data-testid": "contract-template-name-filter",
        placeholder: $t("plugin.idc-compliance.contract.template.filters.name"),
      },
      fieldName: "name",
      label: $t("plugin.idc-compliance.contract.template.columns.name"),
    },
    {
      component: "Select",
      componentProps: {
        allowClear: true,
        options: scopeOptions.value,
        placeholder: $t(
          "plugin.idc-compliance.contract.template.filters.scope",
        ),
      },
      fieldName: "scopeCode",
      label: $t("plugin.idc-compliance.contract.template.columns.scope"),
    },
    {
      component: "Select",
      componentProps: {
        allowClear: true,
        options: statusOptions.value,
        placeholder: $t(
          "plugin.idc-compliance.contract.template.filters.status",
        ),
      },
      fieldName: "statusCode",
      label: $t("plugin.idc-compliance.contract.template.columns.status"),
    },
  ];
}

function translationKey(group: string, value: string) {
  return `plugin.idc-compliance.contract.template.${group}.${value}`;
}

function scopeText(value: ContractTemplateScope) {
  return value
    ? $t(translationKey("scopes", value))
    : $t("plugin.idc-compliance.contract.template.notSet");
}

function orderText(value: ElectronicContractTemplate["signingOrderCode"]) {
  return value
    ? $t(translationKey("orders", value))
    : $t("plugin.idc-compliance.contract.template.notSet");
}

function languageText(value: ElectronicContractTemplate["languageCode"]) {
  return value
    ? $t(translationKey("languages", value))
    : $t("plugin.idc-compliance.contract.template.notSet");
}

function statusText(value: ContractTemplateStatus) {
  return $t(translationKey("statuses", value));
}

function statusColor(value: ContractTemplateStatus) {
  if (value === "enabled") return "green";
  if (value === "needs_configuration") return "gold";
  if (value === "provider_disabled" || value === "provider_offline") {
    return "red";
  }
  if (value === "draft") return "blue";
  return "default";
}

function formattedTime(value: number) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function asTemplate(record: Record<string, any>) {
  return record as ElectronicContractTemplate;
}

function canEdit(item: ElectronicContractTemplate) {
  return (
    props.canManage &&
    (item.statusCode === "draft" || item.statusCode === "needs_configuration")
  );
}

function canEnable(item: ElectronicContractTemplate) {
  return (
    props.canManage &&
    (item.statusCode === "disabled" || item.statusCode === "submitted") &&
    item.providerState !== "disabled" &&
    item.providerState !== "offline"
  );
}

function canDisable(item: ElectronicContractTemplate) {
  return props.canManage && item.statusCode === "enabled";
}

async function load(requestedPage = page.value, requestedSize = pageSize.value) {
  const sequence = ++loadSequence;
  loading.value = true;
  try {
    const filters = {
      name: name.value.trim() || undefined,
      scopeCode: scopeCode.value || undefined,
      statusCode: statusCode.value || undefined,
    };
    await pager.query({ currentPage: requestedPage, pageSize: requestedSize }, (page, pageSize) => electronicContractTemplateList({ ...filters, page, pageSize }));
  } finally {
    if (sequence === loadSequence) loading.value = false;
  }
}

async function search(values: Record<string, unknown>) {
  name.value = String(values.name || "").trim();
  scopeCode.value = (values.scopeCode || "") as ContractTemplateScope | "";
  statusCode.value = (values.statusCode || "") as ContractTemplateStatus | "";
  page.value = 1;
  await load();
}

async function reset() {
  await gridApi.formApi.resetForm();
  name.value = "";
  scopeCode.value = "";
  statusCode.value = "";
  page.value = 1;
  await load();
}

async function syncTencent() {
  if (!props.canManage || syncing.value) return;
  syncing.value = true;
  try {
    const result = await electronicContractTemplateTencentSync();
    message.success(
      $t("plugin.idc-compliance.contract.template.messages.synced", [
        result.item.providerTotal,
        result.item.newVersions,
        result.item.markedOffline,
      ]),
    );
    page.value = 1;
    await load();
  } catch {
    message.error(
      $t("plugin.idc-compliance.contract.template.messages.syncFailed"),
    );
  } finally {
    syncing.value = false;
  }
}

async function copy(item: ElectronicContractTemplate) {
  if (!props.canManage) return;
  const result = await electronicContractTemplateCopy(
    item.templateNo,
    item.versionNo,
    `contract-template-copy-${crypto.randomUUID()}`,
  );
  emit("openEditor", { item: result.item, mode: "edit" });
}

function enable(item: ElectronicContractTemplate) {
  if (!canEnable(item)) return;
  Modal.confirm({
    cancelText: $t("plugin.idc-compliance.contract.template.actions.cancel"),
    content: $t(
      "plugin.idc-compliance.contract.template.messages.enableDescription",
    ),
    okText: $t("plugin.idc-compliance.contract.template.actions.enable"),
    onOk: async () => {
      await electronicContractTemplateEnable(
        item.templateNo,
        item.versionNo,
        item.lockVersion,
      );
      message.success(
        $t("plugin.idc-compliance.contract.template.messages.enabled"),
      );
      await load();
    },
    title: $t("plugin.idc-compliance.contract.template.messages.enableTitle"),
  });
}

async function disable(item: ElectronicContractTemplate) {
  if (!canDisable(item)) return;
  await electronicContractTemplateDisable(
    item.templateNo,
    item.versionNo,
    item.lockVersion,
  );
  message.success(
    $t("plugin.idc-compliance.contract.template.messages.disabled"),
  );
  await load();
}

watch(
  () => props.refreshToken,
  () => load(),
);
onMounted(async () => {
  gridApi.formApi.setState({ schema: searchSchema() });
  await load();
});
</script>

<template>
  <section class="template-manager" data-testid="contract-template-manager">
    <div class="template-heading">
      <div>
        <span>{{ $t("plugin.idc-compliance.contract.template.eyebrow") }}</span>
        <h2>{{ $t("plugin.idc-compliance.contract.template.title") }}</h2>
        <p>{{ $t("plugin.idc-compliance.contract.template.subtitle") }}</p>
      </div>
      <Space v-if="canManage" wrap>
        <Button
          data-testid="contract-template-sync"
          :loading="syncing"
          @click="syncTencent"
        >
          {{ $t("plugin.idc-compliance.contract.template.actions.sync") }}
        </Button>
        <Button
          data-testid="contract-template-create"
          type="primary"
          @click="emit('openEditor', { mode: 'create' })"
        >
          {{ $t("plugin.idc-compliance.contract.template.actions.create") }}
        </Button>
      </Space>
    </div>

    <Card :bordered="false" class="template-card">
      <Grid :loading="loading" class="min-h-0 min-w-0">
        <template #name="{ row: record }">
          <div class="template-name">
            <strong>{{ record.name }}</strong>
            <span>
              {{
                record.originCode === "tencent_synced"
                  ? $t(
                      "plugin.idc-compliance.contract.template.origins.tencent_synced",
                    )
                  : $t("plugin.idc-compliance.contract.template.origins.local")
              }}
              · V{{ record.versionNo }}
            </span>
          </div>
        </template>
        <template #scopeCode="{ row: record }">
          {{ scopeText(record.scopeCode) }}
        </template>
        <template #signingOrderCode="{ row: record }">
          {{ orderText(record.signingOrderCode) }}
        </template>
        <template #languageCode="{ row: record }">
          {{ languageText(record.languageCode) }}
        </template>
        <template #statusCode="{ row: record }">
          <Tag :color="statusColor(record.statusCode)">
            {{ statusText(record.statusCode) }}
          </Tag>
        </template>
        <template #updatedAt="{ row: record }">
          {{ formattedTime(record.updatedAt) }}
        </template>
        <template #actions="{ row: record }">
          <Space wrap>
            <Button
              data-testid="contract-template-view"
              size="small"
              type="link"
              @click="
                emit('openEditor', { item: asTemplate(record), mode: 'view' })
              "
            >
              {{
                $t("plugin.idc-compliance.contract.template.actions.preview")
              }}
            </Button>
            <Button
              v-if="canEdit(asTemplate(record))"
              data-testid="contract-template-edit"
              size="small"
              type="link"
              @click="
                emit('openEditor', { item: asTemplate(record), mode: 'edit' })
              "
            >
              {{ $t("plugin.idc-compliance.contract.template.actions.edit") }}
            </Button>
            <Button
              v-if="canManage"
              data-testid="contract-template-copy"
              size="small"
              type="link"
              @click="copy(asTemplate(record))"
            >
              {{ $t("plugin.idc-compliance.contract.template.actions.copy") }}
            </Button>
            <Button
              v-if="canEnable(asTemplate(record))"
              data-testid="contract-template-enable"
              :data-template-no="record.templateNo"
              size="small"
              type="link"
              @click="enable(asTemplate(record))"
            >
              {{ $t("plugin.idc-compliance.contract.template.actions.enable") }}
            </Button>
            <Button
              v-if="canDisable(asTemplate(record))"
              danger
              data-testid="contract-template-disable"
              :data-template-no="record.templateNo"
              size="small"
              type="link"
              @click="disable(asTemplate(record))"
            >
              {{
                $t("plugin.idc-compliance.contract.template.actions.disable")
              }}
            </Button>
          </Space>
        </template>
      </Grid>

    </Card>
  </section>
</template>

<style scoped>
.template-manager {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}
.template-heading {
  flex: none;
  align-items: flex-end;
  display: flex;
  gap: 24px;
  justify-content: space-between;
  padding: 4px 2px;
}
.template-heading span {
  color: rgb(14 165 233);
  font-family: ui-monospace, monospace;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.14em;
}
.template-heading h2 {
  color: hsl(var(--foreground));
  font-size: 16px;
  margin: 7px 0 5px;
}
.template-heading p {
  color: hsl(var(--muted-foreground));
  margin: 0;
}
.template-card {
  flex: 1;
  min-height: 0;
  border: 1px solid rgb(148 163 184 / 20%);
  border-radius: 0;
  box-shadow: none;
  min-width: 0;
  overflow: hidden;
  width: 100%;
}
.template-card :deep(.ant-card-body) {
  height: 100%;
  min-height: 0;
  min-width: 0;
  padding: 0;
}
.template-name {
  display: grid;
  gap: 4px;
}
.template-name strong {
  color: hsl(var(--foreground));
}
.template-name span {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
}
.template-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}
@media (max-width: 900px) {
  .template-heading {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
