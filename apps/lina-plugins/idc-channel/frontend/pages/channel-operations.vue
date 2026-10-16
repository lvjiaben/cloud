<script lang="ts">
export const pluginPageMeta = { routePath: "idc/channel", title: "Open API" };
</script>
<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VbenFormProps } from "@vben/common-ui";
import { computed, onMounted, ref, watch } from "vue";
import { Page, useVbenDrawer } from "@vben/common-ui";
import { useAccess } from "@vben/access";
import { preferences } from "@vben/preferences";
import { Alert, Button, Card, Descriptions, DescriptionsItem, InputSearch, Popconfirm, Space, Tag, Tabs, TabPane } from "ant-design-vue";
import { useVbenForm } from "#/adapter/form";
import { useVbenVxeGrid, type VxeGridProps } from "#/adapter/vxe-table";
import { pluginApiPath, requestClient } from "#/api/request";
import { channelText as t } from "./channel-i18n";
import OpenApiGuide from "./components/open-api-guide.vue";

type Application = { channelId: string; customerNo: string; displayName: string; statusCode: string };
type Token = { id: number; scopes: string[]; revokedAt: number | null; expiresAt: number | null; createdAt: number };
const { hasAccessByCodes } = useAccess();
const canManage = computed(() => hasAccessByCodes(["idc:admin:channel:token"]));
const selected = ref<Application | null>(null);
const tokens = ref<Token[]>([]);
const requests = ref<{ id: number; requestId: string; operation: string; status: string; createdAt: number }[]>([]);
const secret = ref<{ appId: string; token: string } | null>(null);
const error = ref(""); const busy = ref(false); const keyword = ref("");
const customerOptions = ref<{ label: string; value: string }[]>([]);
let lookupVersion = 0;
const scopeNames = ["catalog.read", "order.create", "order.read", "service.read", "service.renew", "settlement.read", "service.reconfigure", "service.reinstall", "service.console", "service.change_ip", "service.terminate", "service.output", "order.cancel", "webhook.read"];
const request = <T,>(path: string, data?: unknown) => data === undefined ? requestClient.get<T>(pluginApiPath("idc-channel", path.replace(/^\/api\/v1(?=\/|$)/, ""))) : requestClient.post<T>(pluginApiPath("idc-channel", path.replace(/^\/api\/v1(?=\/|$)/, "")), data);
function schema(): VbenFormProps["schema"] { return [
 { fieldName: "customerNo", label: t("open.customer"), component: "Select", rules: "required", componentProps: { "data-testid": "openapi-customer", disabled: busy.value || !!selected.value, showSearch: true, filterOption: false, options: customerOptions.value, onSearch: searchCustomers } },
 { fieldName: "displayName", label: t("open.name"), component: "Input", rules: "required", componentProps: { "data-testid": "openapi-name", disabled: busy.value, maxlength: 160 } },
 { fieldName: "scopes", label: t("open.scopes"), component: "CheckboxGroup", rules: "required", componentProps: { disabled: busy.value, options: scopeNames.map(value => ({ label: t(`open.scope.${value}`), value })) }, help: t("open.scopeHint") },
 { fieldName: "expiresAt", label: t("open.expires"), component: "DatePicker", componentProps: { disabled: busy.value, showTime: true, valueFormat: "x", format: "YYYY-MM-DD HH:mm" }, help: t("open.expiryHint") },
 ]; }
const [Form, formApi] = useVbenForm({ schema: [], showDefaultActions: false, commonConfig: { labelWidth: 110 } });
const [Drawer, drawerApi] = useVbenDrawer({ closeOnClickModal: false, onConfirm: save, onOpenChange(open) { if (!open) { secret.value = null; ++lookupVersion; } } });
const gridOptions: VxeGridProps<Application> = {
 columns: [], height: "100%", rowConfig: { keyField: "channelId" }, pagerConfig: {},
 proxyConfig: {
  ajax: {
   query: async ({ page }) => {
    const response = await request<{ list: Application[]; total: number }>(`/api/v1/admin/channel/accounts?page=${page.currentPage}&pageSize=${page.pageSize}&keyword=${encodeURIComponent(keyword.value)}`);
    return { items: response.list, total: response.total };
   },
  },
 },
};
const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });
function refreshLabels() { formApi.setState({ schema: schema() }); gridApi.setGridOptions({ columns: [
 { field: "displayName", title: t("open.name"), minWidth: 180 }, { field: "channelId", title: "AppID", minWidth: 250 },
 { field: "statusCode", title: t("open.status"), width: 110, slots: { default: "status" } },
 { title: t("open.actions"), width: 240, fixed: "right", slots: { default: "actions" } },
 ] }); }
async function searchCustomers(value = "") {
 const version = ++lookupVersion;
 try { const result = await requestClient.get<{ customers: { customerNo: string; customerCode: string; displayName: string }[] }>(pluginApiPath("idc-core", "/admin/customers"), { params: { namePrefix: value.trim(), limit: 20 } });
 if (version !== lookupVersion) return;
 customerOptions.value = (result.customers || []).map(row => ({ label: `${row.displayName} · #${row.customerCode}`, value: row.customerNo })); refreshLabels();
 } catch { if (version === lookupVersion) error.value = t("open.customerFailed"); }
}
async function open(row?: Application) {
 selected.value = row || null; secret.value = null; tokens.value = []; requests.value = []; error.value = ""; busy.value = false;
 drawerApi.open(); await formApi.resetForm();
 if (row) {
 customerOptions.value = [{ label: row.displayName, value: row.customerNo }]; refreshLabels();
 try { const response = await request<{ list: Token[] }>(`/api/v1/admin/channel/tokens?channelId=${encodeURIComponent(row.channelId)}&pageSize=100`); tokens.value = response.list; const audit = await request<{ list: typeof requests.value }>(`/api/v1/admin/channel/requests?channelId=${encodeURIComponent(row.channelId)}&pageSize=10`); requests.value = audit.list; } catch { error.value = t("open.loadFailed"); }
 const active = tokens.value.find(item => !item.revokedAt);
 await formApi.setValues({ customerNo: row.customerNo, displayName: row.displayName, scopes: active?.scopes.filter(item => scopeNames.includes(item)) || [], expiresAt: active?.expiresAt ? String(active.expiresAt) : undefined });
 } else { await formApi.setValues({ scopes: ["catalog.read", "order.read", "service.read", "settlement.read"] }); refreshLabels(); await searchCustomers(); }
}
async function save() {
 if (secret.value) { drawerApi.close(); return; }
 if (busy.value || !(await formApi.validate()).valid) return;
 const values = await formApi.getValues<{ customerNo: string; displayName: string; scopes: string[]; expiresAt?: string }>();
 if (values.expiresAt && Number(values.expiresAt) <= Date.now()) { error.value = t("open.invalidExpiry"); return; }
 busy.value = true; error.value = ""; drawerApi.lock(); refreshLabels();
 try { secret.value = await request<{ appId: string; token: string }>("/api/v1/admin/channel/tokens", { ...values, expiresAt: values.expiresAt ? Number(values.expiresAt) : null, scopes: [...new Set(["capability.read", ...values.scopes])], sourceCidrs: [] }); await gridApi.query(); }
 catch { error.value = t("open.saveFailed"); }
 finally { busy.value = false; drawerApi.unlock(); refreshLabels(); }
}
async function setStatus(row: Application) {
 try { await request(`/api/v1/admin/channel/accounts/${encodeURIComponent(row.channelId)}/status`, { action: row.statusCode === "approved" ? "pause" : "resume" }); await gridApi.query(); } catch { error.value = t("open.saveFailed"); }
}
async function revoke(item: Token) {
 try { await requestClient.delete(pluginApiPath("idc-channel", `/admin/channel/tokens/${item.id}`)); item.revokedAt = Date.now(); } catch { error.value = t("open.saveFailed"); }
}
watch(() => preferences.app.locale, refreshLabels);
onMounted(refreshLabels);
const layoutRoot = useAdminGridLayout(gridApi);
</script>
<template>
 <Page class="plugin-admin-page plugin-admin-workspace" auto-content-height>
  <Tabs default-active-key="access" class="plugin-admin-tabs">
   <TabPane key="access" :tab="t('open.access')">
    <div :ref="layoutRoot" class="openapi-access">
    <Alert class="mb-4" type="info" show-icon :message="t('open.intro')" :description="t('open.balanceHint')" />
    <Alert v-if="error" class="mb-4" type="error" show-icon :message="error" />
    <Grid class="min-h-0 flex-1">
     <template #toolbar-actions><InputSearch v-model:value="keyword" data-testid="openapi-search" class="w-full sm:w-80" :placeholder="t('open.search')" allow-clear @search="gridApi.reload()" /></template>
     <template #toolbar-tools><Button v-if="canManage" type="primary" data-testid="openapi-create" @click="open()">{{ t('open.create') }}</Button></template>
     <template #status="{ row }"><Tag :color="row.statusCode === 'approved' ? 'success' : 'default'">{{ t(row.statusCode === 'approved' ? 'open.enabled' : 'open.disabled') }}</Tag></template>
     <template #actions="{ row }"><Space v-if="canManage"><Button type="link" @click="open(row)">{{ t('open.manage') }}</Button><Popconfirm :title="t('open.statusConfirm')" @confirm="setStatus(row)"><Button type="link" :danger="row.statusCode === 'approved'">{{ t(row.statusCode === 'approved' ? 'open.disable' : 'open.enable') }}</Button></Popconfirm></Space></template>
    </Grid>
    </div>
   </TabPane>
   <TabPane key="guide" :tab="t('open.guide')"><OpenApiGuide /></TabPane>
  </Tabs>
  <Drawer :title="t(selected ? 'open.manage' : 'open.create')" class="w-full max-w-[680px]" :confirm-text="t(secret ? 'open.saved' : selected ? 'open.rotate' : 'open.generate')">
   <template v-if="secret">
    <Alert type="success" show-icon :message="t('open.secretTitle')" :description="t('open.secretHint')" />
    <Descriptions class="mt-4" bordered :column="1"><DescriptionsItem label="AppID"><code>{{ secret.appId }}</code></DescriptionsItem><DescriptionsItem label="AppSecret"><code class="break-all" data-testid="openapi-secret">{{ secret.token }}</code></DescriptionsItem></Descriptions>
    <Button class="mt-4" @click="drawerApi.close()">{{ t('open.saved') }}</Button>
   </template>
   <template v-else>
    <Alert v-if="selected" class="mb-4" type="warning" show-icon :message="t('open.rotateHint')" />
    <Alert v-if="error" class="mb-4" type="error" show-icon :message="error" />
    <Form />
    <Card v-if="tokens.some(item => !item.revokedAt)" :title="t('open.currentKey')" class="mt-4">
     <template v-for="item in tokens.filter(item => !item.revokedAt)" :key="item.id"><div class="flex items-center justify-between gap-3"><span>{{ new Date(item.createdAt).toLocaleString(preferences.app.locale) }}</span><Popconfirm :title="t('open.revokeConfirm')" @confirm="revoke(item)"><Button danger>{{ t('open.revoke') }}</Button></Popconfirm></div></template>
    </Card>
    <Card v-if="selected" :title="t('open.requests')" class="mt-4"><p v-if="!requests.length">{{ t('open.noRequests') }}</p><Descriptions v-else :column="1" bordered><DescriptionsItem v-for="item in requests" :key="item.id" :label="item.requestId"><span>{{ item.operation }} · {{ item.status }}</span></DescriptionsItem></Descriptions></Card>
   </template>
  </Drawer>
 </Page>
</template>

<style scoped>
.openapi-access { display: flex; flex-direction: column; gap: 12px; height: 100%; min-height: 0; }
.openapi-access > :deep(.ant-alert) { margin: 0; }
</style>
