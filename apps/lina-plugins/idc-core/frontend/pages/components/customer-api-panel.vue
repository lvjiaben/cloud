<script setup lang="ts">
import type { CustomerAPIOverview } from "../customer-client";
import { $te } from "@vben/locales";
import { onBeforeUnmount, ref, watch } from "vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import {
  Alert,
  Button,
  Descriptions,
  Empty,
  Modal,
  Tag,
  message,
} from "ant-design-vue";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import {
  customerAPICalls,
  customerAPIOverview,
  customerAPIReset,
} from "../customer-client";

const props = defineProps<{ customerNo: string }>();
const overview = ref<CustomerAPIOverview>();
const loading = ref(false);
const failed = ref(false);
const callsFailed = ref(false);
let callSequence = 0;
const resetting = ref(false);
const secret = ref("");
let sequence = 0;
let page = 1;
let pageSize = 10;
const text = (key: string) => $t(`plugin.idc-core.customer.workspace.${key}`);
const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: [],
    data: [],
    height: 420,
    rowConfig: { keyField: "id" },
    toolbarConfig: { enabled: false },
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 10, total: 0 },
  },
  gridEvents: {
    pageChange: ({
      currentPage,
      pageSize: size,
    }: {
      currentPage: number;
      pageSize: number;
    }) => {
      page = currentPage;
      pageSize = size;
      void loadCalls();
    },
  },
});

async function loadCalls() {
  const current = sequence;
  const call = ++callSequence;
  callsFailed.value = false;
  gridApi.setLoading(true);
  try {
    const result = await customerAPICalls(props.customerNo, page, pageSize);
    if (current !== sequence || call !== callSequence) return;
    gridApi.setGridOptions({
      data: result.list,
      columns: [
        {
          field: "createdAt",
          title: text("callTime"),
          width: 180,
          formatter: ({ cellValue }: { cellValue: string | number | null }) =>
            formatTimestamp(Number(cellValue)),
        },
        { field: "method", title: text("method"), width: 90 },
        {
          field: "operation",
          title: text("operation"),
          minWidth: 220,
          formatter: ({ cellValue }: { cellValue: string }) =>
            operationName(cellValue),
        },
        {
          field: "succeeded",
          title: text("result"),
          width: 100,
          slots: { default: "result" },
        },
        {
          field: "durationMillis",
          title: text("duration"),
          width: 100,
          formatter: ({ cellValue }: { cellValue: string | number | null }) =>
            `${cellValue} ms`,
        },
      ],
      pagerConfig: {
        enabled: true,
        currentPage: page,
        pageSize,
        total: result.total,
      },
    });
  } catch {
    if (current === sequence && call === callSequence) callsFailed.value = true;
  } finally {
    if (current === sequence && call === callSequence)
      gridApi.setLoading(false);
  }
}
async function load() {
  const current = ++sequence;
  loading.value = true;
  failed.value = false;
  secret.value = "";
  try {
    const result = await customerAPIOverview(props.customerNo);
    if (current !== sequence) return;
    overview.value = result;
    if (result.authorized) await loadCalls();
  } catch {
    if (current === sequence) failed.value = true;
  } finally {
    if (current === sequence) loading.value = false;
  }
}
function operationName(scope: string) {
  const key = `plugin.idc-core.customer.workspace.apiOperations.${scope}`;
  return $te(key) ? $t(key) : text("apiCall");
}
function resetSecret() {
  const current = sequence;
  const target = props.customerNo;
  const tokenId = overview.value?.tokenId;
  Modal.confirm({
    title: text("resetConfirm"),
    content: text("resetHint"),
    onOk: async () => {
      if (!tokenId || resetting.value || current !== sequence) return;
      resetting.value = true;
      try {
        const result = await customerAPIReset(target, tokenId);
        if (target !== props.customerNo || current !== sequence) return;
        secret.value = result.secret;
        const refreshed = await customerAPIOverview(target);
        if (current !== sequence) return;
        overview.value = refreshed;
        message.success(text("resetSuccess"));
      } finally {
        resetting.value = false;
      }
    },
  });
}
async function copy(value: string) {
  await navigator.clipboard.writeText(value);
  message.success(text("copied"));
}
watch(
  () => props.customerNo,
  () => {
    page = 1;
    pageSize = 10;
    overview.value = undefined;
    void load();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  ++sequence;
  ++callSequence;
  secret.value = "";
});
</script>

<template>
  <section data-testid="customer-api-panel" class="api-panel">
    <Alert v-if="failed" type="error" :message="text('loadFailed')"
      ><template #action
        ><Button @click="load">{{ text("retry") }}</Button></template
      ></Alert
    >
    <Empty
      v-else-if="!loading && !overview?.authorized"
      :description="text('apiNotGranted')"
    />
    <template v-else-if="overview">
      <div class="api-heading">
        <h3>{{ text("apiAuthorization") }}</h3>
        <Button
          :disabled="!overview.canReset"
          :loading="resetting"
          data-testid="customer-api-reset"
          @click="resetSecret"
          >{{ text("resetSecret") }}</Button
        >
      </div>
      <Descriptions bordered :column="2">
        <Descriptions.Item label="AppID"
          ><button class="copy-value" @click="copy(overview.appId)">
            {{ overview.appId }}
          </button></Descriptions.Item
        >
        <Descriptions.Item :label="text('status')"
          ><Tag
            :color="overview.status === 'approved' ? 'success' : 'warning'"
            >{{ text(`apiStatus.${overview.status}`) }}</Tag
          ></Descriptions.Item
        >
        <Descriptions.Item label="AppSecret">{{
          text("secretHidden")
        }}</Descriptions.Item>
        <Descriptions.Item :label="text('expiry')">{{
          overview.expiresAt
            ? formatTimestamp(overview.expiresAt)
            : text("noExpiry")
        }}</Descriptions.Item>
        <Descriptions.Item :label="text('allowedAPIs')">{{
          overview.scopes.map(operationName).join("、")
        }}</Descriptions.Item>
        <Descriptions.Item :label="text('sources')">{{
          overview.sourceCidrs.join("、") || text("anySource")
        }}</Descriptions.Item>
      </Descriptions>
      <Alert v-if="secret" type="success" :message="text('secretOnce')"
        ><template #description
          ><button
            class="copy-value secret-value"
            data-testid="customer-api-secret"
            @click="copy(secret)"
          >
            {{ secret }}
          </button></template
        ></Alert
      >
      <div class="api-heading">
        <h3>{{ text("apiCalls") }}</h3>
        <Button @click="loadCalls">{{ text("refresh") }}</Button>
      </div>
      <Alert v-if="callsFailed" type="error" :message="text('loadFailed')"
        ><template #action
          ><Button @click="loadCalls">{{ text("retry") }}</Button></template
        ></Alert
      >
      <Grid
        ><template #result="{ row }"
          ><Tag :color="row.succeeded ? 'success' : 'error'">{{
            text(row.succeeded ? "success" : "failure")
          }}</Tag></template
        ></Grid
      >
    </template>
  </section>
</template>

<style scoped>
.api-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.api-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.api-heading h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.copy-value {
  color: hsl(var(--primary));
  text-decoration: underline;
  cursor: pointer;
}
.secret-value {
  overflow-wrap: anywhere;
  text-align: left;
}
</style>
