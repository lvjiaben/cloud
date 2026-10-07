<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/server-accessories",
  title: "Server Management",
};
</script>

<script setup lang="ts">
import type {
  ServerBatch,
  ServerBatchAction,
  ServerServiceSummary,
  StaffAccessoryAction,
  StaffServerAccessory,
  StaffServerAccessoryDetail,
} from "./server-accessory-client";
import type { VxeGridProps } from "#/adapter/vxe-table";

import { computed, reactive, ref, watch } from "vue";

import { Page, useVbenDrawer } from "@vben/common-ui";
import { $te } from "@vben/locales";
import { preferences } from "@vben/preferences";

import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  DescriptionsItem,
  Empty,
  Form,
  FormItem,
  Input,
  InputNumber,
  message,
  Select,
  SelectOption,
  Space,
  Spin,
  Tag,
  Textarea,
} from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import VbenDrawerCompat from "./components/vben-drawer-compat.vue";
import VbenModalCompat from "./components/vben-modal-compat.vue";
import TrafficPackagePurchase from "./components/traffic-package-purchase.vue";

import enPluginMessages from "../../manifest/i18n/en-US/plugin.json";
import zhPluginMessages from "../../manifest/i18n/zh-CN/plugin.json";
import {
  confirmStaffServerBatch,
  createStaffServerAccessoryOperation,
  executeStaffServerBatch,
  getStaffServerAccessories,
  listStaffServerServices,
  previewStaffServerBatch,
} from "./server-accessory-client";

const serviceNo = ref("");
const loading = ref(false);
const submitting = ref(false);
const failed = ref(false);
const detail = ref<StaffServerAccessoryDetail>();
const activeAction = ref<StaffAccessoryAction>();
const dialogOpen = ref(false);
const selectedServiceNos = ref<string[]>([]);
const batchOpen = ref(false);
const batchLoading = ref(false);
const batchAction = ref<ServerBatchAction>("renewal_reminder");
const batchDetail = ref("");
const batchPreview = ref<ServerBatch>();
const batchAcknowledged = ref(false);
const form = reactive({
  confirmedServiceNo: "",
  externalPort: undefined as number | undefined,
  internalPort: undefined as number | undefined,
  name: "",
  objectNo: "",
  protocol: "tcp",
});
const [TrafficDrawer, trafficDrawerApi] = useVbenDrawer({ connectedComponent: TrafficPackagePurchase });

function serviceColumns(): VxeGridProps<ServerServiceSummary>["columns"] {
  return [
    {
      field: "selection",
      slots: { default: "selection" },
      title: "",
      width: 48,
    },
    { field: "serviceNo", minWidth: 180, title: text("field.serviceNo") },
    { field: "customerNo", minWidth: 150, title: text("field.customerNo") },
    {
      field: "product",
      minWidth: 180,
      slots: { default: "product" },
      title: text("field.product"),
    },
    {
      field: "entitlementStatus",
      width: 140,
      slots: { default: "entitlementStatus" },
      title: text("field.entitlement"),
    },
    {
      field: "runtimeStatus",
      width: 140,
      slots: { default: "runtimeStatus" },
      title: text("field.runtime"),
    },
    {
      field: "operation",
      fixed: "right",
      slots: { default: "operation" },
      title: text("field.operation"),
      width: 120,
    },
  ];
}

function previewColumns(): VxeGridProps<
  ServerBatch["items"][number]
>["columns"] {
  return [
    { field: "serviceNo", minWidth: 180, title: text("field.serviceNo") },
    {
      field: "previewStatus",
      minWidth: 150,
      slots: { default: "previewStatus" },
      title: text("batch.previewStatusLabel"),
    },
    {
      field: "resourceSummary",
      minWidth: 220,
      slots: { default: "resourceSummary" },
      title: text("batch.impact"),
    },
  ];
}

const [ServiceGrid, serviceGridApi] = useVbenVxeGrid({
  formOptions: { collapsed: false, showCollapseButton: false, schema: [
    { fieldName: "keyword", component: "Input", label: text("search"), componentProps: { allowClear: true, maxlength: 80, placeholder: text("batch.searchPlaceholder") } },
  ] },
  gridOptions: {
    columns: serviceColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 520,
    pagerConfig: { enabled: true, pageSize: 20, pageSizes: [10, 20, 50, 100], layouts: ["Total", "Sizes", "PrevJump", "PrevPage", "Number", "NextPage", "NextJump", "FullJump"] },
    proxyConfig: { ajax: { query: loadServices } },
    rowConfig: { keyField: "serviceNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-server-accessories-services",
  },
});
const [PreviewGrid, previewGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: previewColumns(),
    data: [],
    emptyRender: { name: "Empty" },
    height: 300,
    pagerConfig: { enabled: false },
    rowConfig: { keyField: "serviceNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-server-batch-preview",
  },
});
watch(
  () => batchPreview.value?.items || [],
  (rows) => previewGridApi.setGridOptions({ data: rows }),
  { deep: true },
);
watch(
  () => preferences.app.locale,
  () => {
    serviceGridApi.setGridOptions({ columns: serviceColumns() });
    previewGridApi.setGridOptions({ columns: previewColumns() });
  },
);

function toggleServiceSelection(item: ServerServiceSummary, checked: boolean) {
  if (checked) {
    if (
      selectedServiceNos.value.length < 20 &&
      !selectedServiceNos.value.includes(item.serviceNo)
    ) {
      selectedServiceNos.value = [...selectedServiceNos.value, item.serviceNo];
    }
    return;
  }
  selectedServiceNos.value = selectedServiceNos.value.filter(
    (value) => value !== item.serviceNo,
  );
}

const batchActions: ServerBatchAction[] = [
  "renewal_reminder",
  "tag_add",
  "tag_remove",
  "note_append",
  "reboot",
  "suspend",
  "reinstall",
  "ip_change",
  "terminate",
];

async function loadServices({ page }: { page: { currentPage: number; pageSize: number } }, values: Record<string, unknown>) {
  selectedServiceNos.value = [];
  try {
    const result = await listStaffServerServices({
      page: page.currentPage,
      keyword: String(values.keyword || "").trim() || undefined,
      limit: page.pageSize,
    });
    return result;
  } catch (error) {
    const status = (error as { response?: { status?: number }; status?: number }).response?.status ?? (error as { status?: number }).status;
    if (status === 401 || status === 403) {
      detail.value = undefined;
      batchPreview.value = undefined;
      batchOpen.value = false;
      dialogOpen.value = false;
      trafficDrawerApi.close();
      serviceGridApi.setGridOptions({ data: [], pagerConfig: { total: 0 } });
    }
    throw error;
  }
}

function manageService(row: ServerServiceSummary) {
  serviceNo.value = row.serviceNo;
  void load();
}

function openBatch() {
  if (!selectedServiceNos.value.length) return;
  batchAction.value = "renewal_reminder";
  batchDetail.value = "";
  batchPreview.value = undefined;
  batchAcknowledged.value = false;
  batchOpen.value = true;
}

function batchParameters() {
  const value = batchDetail.value.trim();
  if (batchAction.value === "tag_add" || batchAction.value === "tag_remove") {
    return /^[a-z0-9][a-z0-9._-]{0,39}$/u.test(value)
      ? { tag: value }
      : undefined;
  }
  if (batchAction.value === "note_append") {
    return value && value.length <= 1000 ? { note: value } : undefined;
  }
  if (batchAction.value === "reinstall") {
    return /^\d+$/u.test(value) ? { os: Number(value) } : undefined;
  }
  return {};
}

async function previewBatch() {
  const parameters = batchParameters();
  if (!parameters) {
    message.warning(text("batch.formRequired"));
    return;
  }
  batchLoading.value = true;
  try {
    batchPreview.value = await previewStaffServerBatch({
      action: batchAction.value,
      items: selectedServiceNos.value.map((selected) => ({
        parameters,
        serviceNo: selected,
      })),
      requestNo: `server-batch-${crypto.randomUUID()}`,
    });
    batchAcknowledged.value = false;
  } finally {
    batchLoading.value = false;
  }
}

async function executeBatch() {
  if (!batchPreview.value || !batchAcknowledged.value) return;
  batchLoading.value = true;
  try {
    let proofToken = "";
    if (batchPreview.value.dangerous) {
      proofToken = (await confirmStaffServerBatch(batchPreview.value))
        .proofToken;
    }
    batchPreview.value = await executeStaffServerBatch(
      batchPreview.value,
      proofToken,
    );
    message.success(text("batch.submitted"));
  } finally {
    batchLoading.value = false;
  }
}


const directoryFresh = computed(
  () =>
    !!detail.value?.accessoriesExpiresAt &&
    detail.value.accessoriesExpiresAt > Date.now(),
);

function text(key: string) {
  const messageKey = `plugin.idc-resource.serverAccessories.${key}`;
  if ($te(messageKey)) return $t(messageKey);
  const fallbackMessages = preferences.app.locale
    ?.toLowerCase()
    .startsWith("zh")
    ? zhPluginMessages
    : enPluginMessages;
  return (
    fallbackMessages[messageKey as keyof typeof fallbackMessages] ?? messageKey
  );
}

function batchSelectionText(count: number) {
  const messageKey = "plugin.idc-resource.serverAccessories.batch.selection";
  if ($te(messageKey)) return $t(messageKey, { count });
  const fallbackMessages = preferences.app.locale
    ?.toLowerCase()
    .startsWith("zh")
    ? zhPluginMessages
    : enPluginMessages;
  return String(
    fallbackMessages[messageKey as keyof typeof fallbackMessages] ?? messageKey,
  ).replace("{count}", String(count));
}

function batchPreviewStatusText(status: string) {
  const translated = text(`batch.previewStatus.${status}`);
  return translated.endsWith(`batch.previewStatus.${status}`)
    ? status
    : translated;
}

function batchResourceSummaryText(summary: string) {
  const [productType, ...remainder] = summary.split(" / ");
  const translated = text(`productType.${productType}`);
  const productLabel = translated.endsWith(`productType.${productType}`)
    ? productType
    : translated;
  return remainder.length
    ? `${productLabel} / ${remainder.join(" / ")}`
    : productLabel;
}

function actionText(action?: StaffAccessoryAction) {
  return action ? text(`action.${action}`) : "";
}

function stateText(state: string) {
  const translated = text(`state.${state}`);
  return translated.includes("serverAccessories.state.") ? state : translated;
}

function kindText(kind: string) {
  const translated = text(`kind.${kind}`);
  return translated.includes("serverAccessories.kind.") ? kind : translated;
}

function accessoryLabel(item: StaffServerAccessory) {
  const projection = item.projection || {};
  for (const key of [
    "address",
    "cidr",
    "sizeGiB",
    "externalPort",
    "protocol",
  ]) {
    const value = projection[key];
    if (typeof value === "string" || typeof value === "number") {
      return `${item.name || kindText(item.kind)} · ${value}`;
    }
  }
  return item.name || kindText(item.kind);
}

function options(kind: string, state?: string) {
  return (detail.value?.accessories || []).filter(
    (item) => item.kind === kind && (!state || item.state === state),
  );
}

async function load() {
  const target = serviceNo.value.trim();
  if (!target) {
    message.warning(text("message.serviceRequired"));
    return;
  }
  loading.value = true;
  failed.value = false;
  try {
    const result = await getStaffServerAccessories(target);
    detail.value = result.detail;
    serviceNo.value = result.detail.serviceNo;
  } catch {
    failed.value = true;
    detail.value = undefined;
  } finally {
    loading.value = false;
  }
}

function resetForm() {
  form.confirmedServiceNo = "";
  form.externalPort = undefined;
  form.internalPort = undefined;
  form.name = "";
  form.objectNo = "";
  form.protocol = "tcp";
}

function openAction(action: StaffAccessoryAction) {
  if (action === "traffic_add" && detail.value) {
    trafficDrawerApi.setData({ serviceNo: detail.value.serviceNo }).open();
    return;
  }
  resetForm();
  activeAction.value = action;
  dialogOpen.value = true;
}

function buildParameters(action: StaffAccessoryAction) {
  if (action === "snapshot_create" || action === "backup_create") {
    return form.name.trim() ? { name: form.name.trim() } : undefined;
  }
  if (["snapshot_delete", "backup_delete", "nat_delete"].includes(action)) {
    return form.objectNo ? { objectNo: form.objectNo } : undefined;
  }
  if (["snapshot_restore", "backup_restore"].includes(action)) {
    if (!form.objectNo || form.confirmedServiceNo !== detail.value?.serviceNo)
      return;
    return {
      confirmedServiceNo: form.confirmedServiceNo,
      objectNo: form.objectNo,
    };
  }
  if (action === "security_group_attach" || action === "virtual_media_mount") {
    return form.objectNo ? { objectNo: form.objectNo } : undefined;
  }
  if (action === "security_group_detach" || action === "virtual_media_unmount")
    return {};
  if (action === "nat_create") {
    if (!form.name.trim() || !form.externalPort || !form.internalPort) return;
    return {
      externalPort: form.externalPort,
      internalPort: form.internalPort,
      name: form.name.trim(),
      protocol: form.protocol,
    };
  }
  return undefined;
}

async function submit() {
  const action = activeAction.value;
  if (!action || !detail.value) return;
  const parameters = buildParameters(action);
  if (!parameters) {
    message.warning(text("message.formRequired"));
    return;
  }
  submitting.value = true;
  try {
    const result = await createStaffServerAccessoryOperation(
      detail.value.serviceNo,
      {
        action,
        confirmed: true,
        parameters,
        requestNo: `staff-accessory-${crypto.randomUUID()}`,
      },
    );
    message.success(
      result.paymentRequired
        ? `${text("message.paymentRequired")} ${result.invoiceNo}`
        : text("message.submitted"),
    );
    dialogOpen.value = false;
    await load();
  } catch {
    await load();
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <Page
    :auto-content-height="true"
    data-testid="server-accessory-management-page"
  >
    <div class="accessory-shell">
      <Card :title="text('title')">
        <template #extra>
          <Button
            type="primary"
            :disabled="!selectedServiceNos.length"
            data-testid="server-batch-open"
            @click="openBatch"
          >
            {{ text("batch.open") }}（{{ selectedServiceNos.length }}/20）
          </Button>
        </template>
        <ServiceGrid>
          <template #selection="{ row }"
            ><Checkbox
              :checked="selectedServiceNos.includes(row.serviceNo)"
              :disabled="
                selectedServiceNos.length >= 20 &&
                !selectedServiceNos.includes(row.serviceNo)
              "
              @change="toggleServiceSelection(row, $event.target.checked)"
          /></template>
          <template #product="{ row }">{{
            row.productNameZhCN || row.productNameEnUS || "—"
          }}</template>
          <template #entitlementStatus="{ row }"
            ><Tag>{{ stateText(row.entitlementStatus) }}</Tag></template
          >
          <template #runtimeStatus="{ row }"
            ><Tag>{{ stateText(row.runtimeStatus) }}</Tag></template
          >
          <template #operation="{ row }"
            ><Button
              type="link"
              size="small"
              :data-testid="`server-service-manage-${row.serviceNo}`"
              @click="manageService(row)"
              >{{ text("batch.manage") }}</Button
            ></template
          >
        </ServiceGrid>
      </Card>

      <Card v-if="detail || loading || failed" :title="text('serverTitle')">
        <div class="accessory-search-row">
          <Input
            v-model:value="serviceNo"
            data-testid="server-accessory-service-no"
            :maxlength="40"
            :placeholder="text('servicePlaceholder')"
            @press-enter="load"
          />
          <Button data-testid="server-accessory-search" @click="load">
            {{ text("refresh") }}
          </Button>
        </div>
      </Card>

      <Alert
        v-if="failed"
        type="error"
        show-icon
        :message="text('message.loadFailed')"
      >
        <template #action
          ><Button size="small" @click="load">{{
            text("refresh")
          }}</Button></template
        >
      </Alert>

      <Spin :spinning="loading">
        <template v-if="detail">
          <Card :title="text('serverTitle')">
            <Descriptions bordered :column="{ xs: 1, md: 2, xl: 4 }">
              <DescriptionsItem :label="text('field.serviceNo')">{{
                detail.serviceNo
              }}</DescriptionsItem>
              <DescriptionsItem :label="text('field.customerNo')">{{
                detail.customerNo
              }}</DescriptionsItem>
              <DescriptionsItem :label="text('field.runtime')"
                ><Tag>{{
                  stateText(detail.runtimeStatus)
                }}</Tag></DescriptionsItem
              >
              <DescriptionsItem :label="text('field.entitlement')">{{
                stateText(detail.entitlementStatus)
              }}</DescriptionsItem>
            </Descriptions>
          </Card>

          <Card :title="text('actionsTitle')">
            <Alert
              v-if="detail.activeOperationNo"
              class="mb-4"
              type="warning"
              show-icon
              :message="text('message.operationActive')"
            />
            <Space v-if="detail.availableActions.length" wrap>
              <Button
                v-for="action in detail.availableActions"
                :key="action"
                :danger="
                  action.endsWith('_delete') || action.endsWith('_restore')
                "
                :disabled="!!detail.activeOperationNo || !directoryFresh"
                :data-testid="`staff-accessory-action-${action}`"
                @click="openAction(action)"
                >{{ actionText(action) }}</Button
              >
            </Space>
            <Alert
              v-else
              type="info"
              show-icon
              :message="text('message.unsupported')"
            />
          </Card>

          <Card :title="text('directoryTitle')">
            <Alert
              v-if="!directoryFresh"
              class="mb-4"
              type="warning"
              show-icon
              :message="text('message.stale')"
            />
            <Empty
              v-if="!detail.accessories.length"
              :description="text('message.empty')"
            />
            <Descriptions v-else bordered :column="{ xs: 1, md: 2 }">
              <DescriptionsItem
                v-for="item in detail.accessories"
                :key="item.objectNo"
                :label="kindText(item.kind)"
              >
                {{ accessoryLabel(item) }} · {{ stateText(item.state) }}
              </DescriptionsItem>
            </Descriptions>
            <p class="directory-time">
              {{ text("field.observedAt") }}:
              {{ formatTimestamp(detail.accessoriesObservedAt || 0) }}
            </p>
          </Card>
        </template>
      </Spin>
    </div>

    <VbenModalCompat
      v-model:open="dialogOpen"
      data-testid="staff-accessory-operation-dialog"
      :confirm-loading="submitting"
      :title="actionText(activeAction)"
      :ok-text="text('confirm')"
      :cancel-text="text('cancel')"
      @ok="submit"
    >
      <Alert
        class="mb-4"
        type="warning"
        show-icon
        :message="activeAction ? text(`confirmMessage.${activeAction}`) : ''"
      />
      <Form layout="vertical">
        <FormItem
          v-if="
            [
              'snapshot_delete',
              'snapshot_restore',
              'backup_delete',
              'backup_restore',
            ].includes(activeAction || '')
          "
          :label="text('form.object')"
        >
          <Select
            v-model:value="form.objectNo"
            data-testid="staff-accessory-object"
          >
            <SelectOption
              v-for="item in options(
                activeAction?.startsWith('snapshot_') ? 'snapshot' : 'backup',
                'available',
              )"
              :key="item.objectNo"
              :value="item.objectNo"
              >{{ accessoryLabel(item) }}</SelectOption
            >
          </Select>
        </FormItem>
        <FormItem
          v-if="
            activeAction === 'snapshot_create' ||
            activeAction === 'backup_create'
          "
          :label="text('form.name')"
        >
          <Input
            v-model:value="form.name"
            data-testid="staff-accessory-name"
            :maxlength="128"
          />
        </FormItem>
        <FormItem
          v-if="activeAction?.endsWith('_restore')"
          :label="text('form.confirmServiceNo')"
        >
          <Input
            v-model:value="form.confirmedServiceNo"
            :placeholder="detail?.serviceNo"
            autocomplete="off"
          />
        </FormItem>
        <FormItem
          v-if="activeAction === 'security_group_attach'"
          :label="text('form.securityGroup')"
        >
          <Select
            v-model:value="form.objectNo"
            data-testid="staff-accessory-object"
          >
            <SelectOption
              v-for="item in options('security_group', 'available')"
              :key="item.objectNo"
              :value="item.objectNo"
              >{{ accessoryLabel(item) }}</SelectOption
            >
          </Select>
        </FormItem>
        <FormItem
          v-if="activeAction === 'nat_delete'"
          :label="text('form.natRule')"
        >
          <Select
            v-model:value="form.objectNo"
            data-testid="staff-accessory-object"
          >
            <SelectOption
              v-for="item in options('nat', 'active')"
              :key="item.objectNo"
              :value="item.objectNo"
              >{{ accessoryLabel(item) }}</SelectOption
            >
          </Select>
        </FormItem>
        <template v-if="activeAction === 'nat_create'">
          <FormItem :label="text('form.name')"
            ><Input v-model:value="form.name" :maxlength="128"
          /></FormItem>
          <FormItem :label="text('form.externalPort')"
            ><InputNumber
              v-model:value="form.externalPort"
              :min="1"
              :max="65535"
              class="w-full"
          /></FormItem>
          <FormItem :label="text('form.internalPort')"
            ><InputNumber
              v-model:value="form.internalPort"
              :min="1"
              :max="65535"
              class="w-full"
          /></FormItem>
          <FormItem :label="text('form.protocol')">
            <Select v-model:value="form.protocol"
              ><SelectOption value="tcp">TCP</SelectOption
              ><SelectOption value="udp">UDP</SelectOption
              ><SelectOption value="tcp_udp">TCP + UDP</SelectOption></Select
            >
          </FormItem>
        </template>
        <FormItem
          v-if="activeAction === 'virtual_media_mount'"
          :label="text('form.virtualMedia')"
        >
          <Select
            v-model:value="form.objectNo"
            data-testid="staff-accessory-object"
          >
            <SelectOption
              v-for="item in options('virtual_media', 'available')"
              :key="item.objectNo"
              :value="item.objectNo"
              >{{ accessoryLabel(item) }}</SelectOption
            >
          </Select>
        </FormItem>
      </Form>
    </VbenModalCompat>

    <TrafficDrawer @success="load" />
    <VbenDrawerCompat
      v-model:open="batchOpen"
      data-testid="server-batch-drawer"
      width="min(760px, 96vw)"
      :title="text('batch.title')"
      destroy-on-close
    >
      <Space direction="vertical" class="w-full" size="middle">
        <Alert
          type="info"
          show-icon
          :message="batchSelectionText(selectedServiceNos.length)"
        />
        <Form layout="vertical">
          <FormItem :label="text('batch.actionLabel')" required>
            <Select
              v-model:value="batchAction"
              @change="batchPreview = undefined"
            >
              <SelectOption
                v-for="item in batchActions"
                :key="item"
                :value="item"
                >{{ text(`batch.action.${item}`) }}</SelectOption
              >
            </Select>
          </FormItem>
          <FormItem
            v-if="
              ['tag_add', 'tag_remove', 'note_append', 'reinstall'].includes(
                batchAction,
              )
            "
            :label="text(`batch.detail.${batchAction}`)"
            required
          >
            <Textarea
              v-if="batchAction === 'note_append'"
              v-model:value="batchDetail"
              :maxlength="1000"
              :rows="4"
            />
            <Input v-else v-model:value="batchDetail" :maxlength="40" />
          </FormItem>
        </Form>
        <Button type="primary" :loading="batchLoading" @click="previewBatch">
          {{ text("batch.preview") }}
        </Button>
        <Card v-if="batchPreview" :title="text('batch.previewResult')">
          <PreviewGrid>
            <template #previewStatus="{ row }">{{
              batchPreviewStatusText(row.previewStatus)
            }}</template>
            <template #resourceSummary="{ row }">{{
              batchResourceSummaryText(row.resourceSummary)
            }}</template>
          </PreviewGrid>
          <Checkbox v-model:checked="batchAcknowledged" class="mt-4">
            {{ text("batch.acknowledge") }}
          </Checkbox>
          <div class="mt-4 text-right">
            <Button
              type="primary"
              :danger="batchPreview.dangerous"
              :disabled="!batchAcknowledged"
              :loading="batchLoading"
              data-testid="server-batch-execute"
              @click="executeBatch"
              >{{ text("batch.execute") }}</Button
            >
          </div>
        </Card>
      </Space>
    </VbenDrawerCompat>
  </Page>
</template>

<style scoped>
.accessory-shell {
  display: grid;
  gap: 16px;
  min-height: 100%;
}
.accessory-search-row {
  display: grid;
  grid-template-columns: minmax(240px, 1fr) auto;
  gap: 12px;
  margin-bottom: 16px;
}
.directory-time {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  margin: 12px 0 0;
}
@media (max-width: 640px) {
  .accessory-search-row {
    grid-template-columns: 1fr;
  }
}
</style>
