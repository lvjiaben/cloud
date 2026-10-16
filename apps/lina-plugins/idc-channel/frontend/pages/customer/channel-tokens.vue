<script lang="ts">
export const pluginPageMeta = {
  routePath: "/account/plugins/idc-channel/open-platform",
  title: "plugin.idc-channel.workspace.platform.title",
};
</script>
<script setup lang="ts">
import {
  computed,
  onActivated,
  onDeactivated,
  onBeforeUnmount,
  onMounted,
  nextTick,
  ref,
} from "vue";
import { Page, useVbenModal } from "@vben/common-ui";
import {
  Alert,
  Button,
  Card,
  Descriptions,
  DescriptionsItem,
  Spin,
  Tabs,
  TabPane,
  Tag,
  message,
} from "ant-design-vue";
import { useVbenVxeGrid, type VxeGridProps } from "#/adapter/vxe-table";
import { pluginApiPath, requestClient } from "#/api/request";
import { channelText as t } from "../channel-i18n";
import OpenApiGuide from "../components/open-api-guide.vue";

type Overview = {
  authorized: boolean;
  appId: string;
  displayName: string;
  status: string;
  tokenId: number;
  canReset: boolean;
  scopes: string[];
  sourceCidrs: string[];
  expiresAt?: number;
};
type Call = {
  id: number;
  method: string;
  operation: string;
  succeeded: boolean;
  durationMillis: number;
  createdAt: number;
};
const overview = ref<Overview>();
const loading = ref(false);
const failed = ref(false);
const busy = ref(false);
const secret = ref("");
const tab = ref("info");
const api = () => pluginApiPath("idc-channel", "account/open-platform");
const address = computed(() =>
  new URL(
    pluginApiPath("idc-channel", ""),
    window.location.origin,
  ).href.replace(/\/$/, ""),
);
const formatTime = (value?: number) =>
  value ? new Date(value).toLocaleString() : t("platform.unlimited");
const gridOptions: VxeGridProps<Call> = {
  columns: [],
  height: 440,
  rowConfig: { keyField: "id" },
  pagerConfig: {},
  proxyConfig: {
    autoLoad: true,
    ajax: {
      query: async ({ page }) => {
        const result = await requestClient.get<{ list: Call[]; total: number }>(
          `${api()}/calls`,
          { params: { page: page.currentPage, pageSize: page.pageSize } },
        );
        return { items: result.list, total: result.total };
      },
    },
  },
};
const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });
function columns() {
  return [
    {
      field: "createdAt",
      title: t("platform.time"),
      width: 190,
      formatter: ({ cellValue }: { cellValue: number }) =>
        formatTime(cellValue),
    },
    { field: "method", title: t("platform.method"), width: 110 },
    { field: "operation", title: t("platform.operation"), minWidth: 200 },
    {
      field: "succeeded",
      title: t("platform.result"),
      width: 100,
      slots: { default: "result" },
    },
    { field: "durationMillis", title: t("platform.duration"), width: 150 },
  ];
}
async function load() {
  if (loading.value) return;
  loading.value = true;
  try {
    overview.value = await requestClient.get<Overview>(api());
    failed.value = false;
  } catch {
    overview.value = undefined;
    failed.value = true;
  } finally {
    loading.value = false;
  }
}
const [ResetModal, resetApi] = useVbenModal({
  onConfirm: async () => {
    if (busy.value || !overview.value?.canReset) return;
    busy.value = true;
    resetApi.setState({ confirmLoading: true });
    try {
      const result = await requestClient.post<{ secret: string }>(
        `${api()}/secrets`,
        { tokenId: overview.value.tokenId },
      );
      secret.value = result.secret;
      resetApi.close();
      secretApi.open();
      await load();
    } catch {
      message.error(t("platform.resetFailed"));
      await load();
    } finally {
      busy.value = false;
      resetApi.setState({ confirmLoading: false });
    }
  },
});
const [SecretModal, secretApi] = useVbenModal({
  onClosed: () => {
    secret.value = "";
  },
});
async function copy(value: string) {
  try {
    await navigator.clipboard.writeText(value);
    message.success(t("platform.copied"));
  } catch {
    message.error(t("platform.copyFailed"));
  }
}
async function changeTab(value: string | number) {
  if (value === "calls") {
    gridApi.setGridOptions({ columns: columns() });
    await nextTick();
    await gridApi.query();
  }
}
onMounted(() => {
  void load();
});
onActivated(() => {
  void load();
});
onDeactivated(() => {
  secret.value = "";
  secretApi.close();
  resetApi.close();
});
onBeforeUnmount(() => {
  secret.value = "";
});
</script>
<template>
  <Page data-testid="open-platform-page">
    <div class="mb-4 flex items-center justify-between">
      <h1 class="text-xl font-semibold">{{ t("platform.title") }}</h1>
      <Button :loading="loading" @click="load">{{
        t("platform.refresh")
      }}</Button>
    </div>
    <Spin :spinning="loading">
      <Alert v-if="failed" type="error" :message="t('platform.loadFailed')" />
      <Alert
        v-else-if="overview && !overview.authorized"
        type="info"
        :message="t('platform.none')"
      />
      <Tabs
        v-else-if="overview?.authorized"
        v-model:active-key="tab"
        @change="changeTab"
      >
        <TabPane key="info" :tab="t('platform.info')">
          <Card>
            <Descriptions :column="1" bordered>
              <DescriptionsItem :label="t('platform.address')"
                ><code class="break-all">{{ address }}</code
                ><Button size="small" class="ml-3" @click="copy(address)">{{
                  t("platform.copy")
                }}</Button></DescriptionsItem
              >
              <DescriptionsItem label="AppID"
                ><code data-testid="platform-app-id">{{ overview.appId }}</code
                ><Button
                  size="small"
                  class="ml-3"
                  @click="copy(overview.appId)"
                  >{{ t("platform.copy") }}</Button
                ></DescriptionsItem
              >
              <DescriptionsItem :label="t('platform.status')"
                ><Tag :color="overview.canReset ? 'success' : 'warning'">{{
                  t(`platform.${overview.status}`)
                }}</Tag></DescriptionsItem
              >
              <DescriptionsItem :label="t('platform.scopes')"
                ><Tag v-for="scope in overview.scopes" :key="scope">{{
                  scope
                }}</Tag></DescriptionsItem
              >
              <DescriptionsItem :label="t('platform.sources')">{{
                overview.sourceCidrs.join(", ") || t("platform.unlimited")
              }}</DescriptionsItem>
              <DescriptionsItem :label="t('platform.expiry')">{{
                formatTime(overview.expiresAt)
              }}</DescriptionsItem>
              <DescriptionsItem :label="t('platform.secret')"
                ><p>••••••••••••••••</p>
                <p class="my-2 text-muted-foreground">
                  {{ t("platform.secretNote") }}
                </p>
                <Button
                  data-testid="platform-reset"
                  :disabled="!overview.canReset"
                  type="primary"
                  @click="resetApi.open()"
                  >{{ t("platform.reset") }}</Button
                ></DescriptionsItem
              >
            </Descriptions>
            <Alert
              v-if="!overview.canReset"
              class="mt-4"
              type="warning"
              :message="t('platform.disabled')"
            />
          </Card>
        </TabPane>
        <TabPane key="docs" :tab="t('platform.docs')"><OpenApiGuide /></TabPane>
        <TabPane key="calls" :tab="t('platform.calls')"
          ><Alert
            class="mb-3"
            type="info"
            :message="t('platform.historyNote')"
          /><Grid
            ><template #result="{ row }"
              ><Tag :color="row.succeeded ? 'success' : 'error'">{{
                t(row.succeeded ? "platform.success" : "platform.failure")
              }}</Tag></template
            ></Grid
          ></TabPane
        >
      </Tabs>
    </Spin>
    <ResetModal :title="t('platform.reset')"
      ><p>{{ t("platform.resetTitle") }}</p>
      <p class="mt-3">{{ t("platform.resetNote") }}</p></ResetModal
    >
    <SecretModal :title="t('platform.newSecret')" :show-confirm-button="false"
      ><Alert type="warning" :message="t('platform.secretNote')" /><code
        data-testid="platform-new-secret"
        class="my-4 block break-all rounded bg-muted p-4"
        >{{ secret }}</code
      ><Button @click="copy(secret)">{{ t("platform.copy") }}</Button
      ><Button class="ml-3" type="primary" @click="secretApi.close()">{{
        t("platform.saved")
      }}</Button></SecretModal
    >
  </Page>
</template>
