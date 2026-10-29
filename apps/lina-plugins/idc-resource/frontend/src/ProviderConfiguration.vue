<script setup lang="ts">
import { computed, createApp, h, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { mountIsolatedVbenFormDrawer } from "@linapro/vben-plugin-adapter/isolated";
import type { IsolatedTableColumn, IsolatedFormField, IsolatedFormDrawerHandle, IsolatedFormDrawerOptions } from "@linapro/vben-plugin-adapter/isolated";
import StandaloneTable from "./StandaloneTable.vue";
import ProviderInterfaceDetails from "./ProviderInterfaceDetails.vue";

import {
  interfaceCandidateLabel,
  interfaceFieldDescription,
  interfaceFieldLabel,
  interfaceTreeOptions,
  normalizeInterfaceConfigurationSchema,
  secretConfigurationComplete,
  splitInterfaceConfigurationValues,
} from "../pages/provider-interface-configuration";
import type { InterfaceConfigurationSchema } from "../pages/provider-interface-configuration";
import type { PluginHostMountContext } from "./host-context";

import { createAPI } from "./api";
import type { Channel, Pool, SecretField } from "./api";
import { RequestError } from "./request";

const props = defineProps<{ context: PluginHostMountContext }>();
const api = createAPI(() => props.context);
const tr = (key: string) =>
  props.context.t(`plugin.idc-resource.providerConfiguration.${key}`, key);
const locale = () => props.context.locale || "zh-CN";
const tab = ref<"channels" | "pools">("channels");
const channels = ref<Channel[]>([]);
const pools = ref<Pool[]>([]);
const candidates = ref<
  Awaited<ReturnType<typeof api.list>>["providerCandidates"]
>([]);
const loading = ref(false);
const showDeletedChannels = ref(false);
const channelPage = ref({ currentPage: 1, pageSize: 20, total: 0 });
const poolPage = ref({ currentPage: 1, pageSize: 20, total: 0 });
let generation = 0;
const error = ref("");
const notice = ref("");
const testingChannels = ref<string[]>([]);
const refreshingChannels = ref<string[]>([]);
const channelModal = ref(false);
const poolModal = ref(false);
const editingChannel = ref<Channel>();
const editingPool = ref<Pool>();
const drawerRoot = ref<HTMLElement>();
let formDrawer: IsolatedFormDrawerHandle | undefined;
let formRevision = 0;
let candidateRevision = 0;
let poolChannelOptions: Array<{label: string; value: string}> = [];
let poolChannelPage = 0;
let poolChannelTotal = 0;
let poolChannelKeyword = "";
let loadingPoolChannels = false;
const schema = ref<InterfaceConfigurationSchema>({ fields: [], version: "" });
const channelForm = reactive<Record<string, any>>({});
const poolForm = reactive({
  channelNo: "",
  displayName: "",
  resourceKind: "virtual_server",
  routingPriority: 100,
  routingWeight: 100,
  status: "disabled",
});
const activeChannels = computed(() =>
  channels.value.filter((item) => item.status !== "deleted"),
);

function readyMap(item?: Channel) {
  return Object.fromEntries(
    (item?.secretFields || []).map((field) => [field.key, field.ready]),
  );
}
function secretsComplete(item: Channel) {
  return secretConfigurationComplete(
    normalizeInterfaceConfigurationSchema(item.interfaceConfiguration),
    readyMap(item),
  );
}
function failure(value: unknown) {
  return value instanceof Error ? value.message : tr("operationFailed");
}
function clearAuthorizedData() {
  formRevision++;
  candidateRevision++;
  formDrawer?.destroy();
  formDrawer = undefined;
  channels.value = [];
  pools.value = [];
  candidates.value = [];
  channelPage.value = { ...channelPage.value, total: 0 };
  poolPage.value = { ...poolPage.value, total: 0 };
  channelModal.value = false;
  poolModal.value = false;
  editingChannel.value = undefined;
  editingPool.value = undefined;
  for (const key of Object.keys(channelForm)) delete channelForm[key];
  poolChannelOptions = [];
}

function showDrawer(options: IsolatedFormDrawerOptions) {
  if (!drawerRoot.value) return;
  formDrawer?.destroy();
  error.value = "";
  const revision = ++formRevision;
  formDrawer = mountIsolatedVbenFormDrawer(drawerRoot.value, { ...options, locale: locale(),
    onError: value => { if (revision === formRevision) { error.value = failure(value); formDrawer?.update({ errorText: error.value }); } },
    onClosed: () => {
      if (revision !== formRevision) return;
      formRevision++;
      candidateRevision++;
      channelModal.value = false;
      poolModal.value = false;
      editingChannel.value = undefined;
      editingPool.value = undefined;
      for (const key of Object.keys(channelForm)) delete channelForm[key];
      poolChannelOptions = [];
      formDrawer?.destroy();
      formDrawer = undefined;
    },
  });
  void formDrawer.ready.catch(value => {
    if (revision !== formRevision) return;
    error.value = failure(value);
    formDrawer?.destroy();
    formDrawer = undefined;
  });
}

function channelFields(): IsolatedFormField[] {
  const fields: IsolatedFormField[] = [];
  if (!editingChannel.value) fields.push({ name: "providerPluginId", label: tr("connector"), component: "Select", required: true, props: { options: candidates.value.map(item => ({label: `${item.displayName} · ${item.releaseVersion}`, value: item.pluginId})) } });
  fields.push({ name: "displayName", label: tr("channelName"), component: "Input", required: true }, { name: "endpoint", label: tr("apiEndpoint"), component: "Input", required: true, props: { type: "url" } });
  for (const field of schema.value.fields.filter(item => item.key !== "endpoint")) {
    let component: IsolatedFormField["component"] = "Input";
    const props: Record<string, unknown> = { placeholder: field.type === "secret" && readyMap(editingChannel.value)[field.key] ? tr("secretConfiguredHint") : interfaceFieldDescription(field, locale()) };
    if (field.type === "secret") component = "InputPassword";
    else if (field.type === "multi_line") component = "Textarea";
    else if (field.type === "boolean") component = "Switch";
    else if (field.type === "integer" || field.type === "decimal") { component = "InputNumber"; props.min = field.minimum === undefined ? undefined : Number(field.minimum); props.max = field.maximum === undefined ? undefined : Number(field.maximum); if (field.type === "integer") props.precision = 0; }
    else if (field.type === "tree_select" || field.type === "address") { component = field.type === "address" ? "Cascader" : "TreeSelect"; props[field.type === "address" ? "options" : "treeData"] = interfaceTreeOptions(field, locale()); }
    else if (field.type === "single_select" || field.type === "multi_select") { component = "Select"; props.options = (field.staticCandidates || []).map(item => ({label: interfaceCandidateLabel(item, locale()), value: item.value})); if (field.type === "multi_select") props.mode = "multiple"; }
    else if (field.type === "date" || field.type === "datetime") { component = "DatePicker"; props.valueFormat = field.type === "date" ? "YYYY-MM-DD" : "YYYY-MM-DDTHH:mm:ssZ"; props.showTime = field.type === "datetime"; }
    fields.push({ name: field.key, label: interfaceFieldLabel(field, locale()), component, props, required: field.required && !(field.type === "secret" && readyMap(editingChannel.value)[field.key]) });
  }
  if (editingChannel.value) fields.push({ name: "status", label: tr("statusLabel"), component: "Select", required: true, props: { options: ["disabled", "enabled", "maintenance"].map(value => ({ value, label: tr(`status.${value}`) })) } });
  return fields;
}

function poolFields(): IsolatedFormField[] {
  return [
    ...(!editingPool.value ? [{ name: "channelNo", label: tr("owningChannel"), component: "Select" as const, required: true, props: { options: poolChannelOptions, showSearch: true, filterOption: false, loading: loadingPoolChannels, onSearch: (keyword: string) => void loadPoolChannels(keyword), onPopupScroll: (event: Event) => { const target = event.target as HTMLElement; if (target.scrollTop + target.clientHeight >= target.scrollHeight - 24 && poolChannelPage * 100 < poolChannelTotal) void loadPoolChannels(poolChannelKeyword, true); } } }] : []),
    { name: "displayName", label: tr("poolName"), component: "Input", required: true },
    { name: "resourceKind", label: tr("serverType"), component: "Select", required: true, props: { options: ["virtual_server", "bare_metal"].map(value => ({ value, label: tr(`resourceKind.${value}`) })) } },
    { name: "routingPriority", label: tr("routingPriority"), component: "InputNumber", props: { min: 0, max: 100000, precision: 0 } },
    { name: "routingWeight", label: tr("routingWeight"), component: "InputNumber", props: { min: 0, max: 10000, precision: 0 } },
    ...(editingPool.value ? [{ name: "status", label: tr("statusLabel"), component: "Select" as const, required: true, props: { options: ["disabled", "enabled", "draining", "maintenance"].map(value => ({ value, label: tr(`status.${value}`) })) } }] : []),
  ];
}

async function loadPoolChannels(keyword = "", append = false) {
  if (append && loadingPoolChannels) return;
  const revision = ++candidateRevision;
  const page = append ? poolChannelPage + 1 : 1;
  loadingPoolChannels = true;
  formDrawer?.update({ fields: poolFields() });
  try {
    const result = await api.list({ channelPage: page, channelPageSize: 100, poolPage: 1, poolPageSize: 10, channelKeyword: keyword, availableChannelsOnly: true });
    if (revision !== candidateRevision || !poolModal.value) return;
    poolChannelOptions = [...(append ? poolChannelOptions : []), ...result.channels.map(item => ({label: item.displayName, value: item.channelNo}))];
    poolChannelPage = page;
    poolChannelTotal = result.channelTotal;
    poolChannelKeyword = keyword;
  } catch (value) { if (revision === candidateRevision) { if (value instanceof RequestError && [401,403].includes(value.httpStatus ?? 0)) clearAuthorizedData(); error.value = failure(value); } }
  finally { if (revision === candidateRevision) { loadingPoolChannels = false; formDrawer?.update({ fields: poolFields() }); } }
}

function openDetails(item: Channel) {
  const values = { displayName: item.displayName, endpoint: item.endpoint, apiProducts: item.usage ? `${item.usage.activeInstances} / ${item.usage.totalInstances}` : "—", configuredProducts: String(item.usage?.configuredProducts ?? "—"), accountStatus: tr(providerUpdateNeeded(item) ? "providerUpdateNeeded" : secretsComplete(item) ? "configured" : "incomplete"), runtimeStatus: tr(`status.${item.status}`), connectionStatus: connectionStatus(item) };
  showDrawer({ title: tr("detailTitle"), description: tr("usageHint"), readOnly: true, values: {}, fields: [],
    mountContent: (container) => {
      const app = createApp({ render: () => h(ProviderInterfaceDetails, { context: props.context, channel: item, summary: values }) });
      app.mount(container);
      return () => app.unmount();
    },
  });
}
async function load(channel?: { currentPage: number; pageSize: number }, pool?: { currentPage: number; pageSize: number }) {
  const current = ++generation;
  const nextChannel = { ...channelPage.value, ...channel };
  const nextPool = { ...poolPage.value, ...pool };
  if (nextChannel.pageSize !== channelPage.value.pageSize) nextChannel.currentPage = 1;
  if (nextPool.pageSize !== poolPage.value.pageSize) nextPool.currentPage = 1;
  loading.value = true;
  try {
    const data = await api.list({ channelPage: nextChannel.currentPage, channelPageSize: nextChannel.pageSize, poolPage: nextPool.currentPage, poolPageSize: nextPool.pageSize, availableChannelsOnly: !showDeletedChannels.value });
    if (current !== generation) return;
    if (![data.channelTotal, data.poolTotal].every((total) => Number.isSafeInteger(total) && total >= 0)) throw new Error(tr("operationFailed"));
    channels.value = data.channels || [];
    pools.value = data.pools || [];
    candidates.value = data.providerCandidates || [];
    channelPage.value = { ...nextChannel, total: data.channelTotal };
    poolPage.value = { ...nextPool, total: data.poolTotal };
  } catch (value) {
    if (current !== generation) return;
    if (value instanceof RequestError && [401, 403].includes(value.httpStatus ?? 0)) clearAuthorizedData();
    error.value = failure(value);
  } finally {
    if (current === generation) {
      loading.value = false;
      channelPage.value = { ...channelPage.value };
      poolPage.value = { ...poolPage.value };
    }
  }
}
const channelPagination = computed(() => ({ ...channelPage.value, onChange: (page: { currentPage: number; pageSize: number }) => void load(page) }));
const poolPagination = computed(() => ({ ...poolPage.value, onChange: (page: { currentPage: number; pageSize: number }) => void load(undefined, page) }));
function chooseConnector(pluginId: string) {
  schema.value = normalizeInterfaceConfigurationSchema(
    candidates.value.find((item) => item.pluginId === pluginId)
      ?.interfaceConfiguration,
  );
}
function openChannel(item?: Channel) {
  editingChannel.value = item;
  for (const key of Object.keys(channelForm)) delete channelForm[key];
  const candidate = item ? undefined : candidates.value[0];
  Object.assign(channelForm, {
    displayName: item?.displayName || "",
    endpoint: item?.endpoint || "",
    tlsCertificateSha256: item?.tlsCertificateSha256 || "",
    providerPluginId: item?.providerPluginId || candidate?.pluginId || "",
    status:
      item?.status === "deleted" ? "disabled" : item?.status || "disabled",
    ...(item?.nonSecretSettings || {}),
  });
  schema.value = normalizeInterfaceConfigurationSchema(
    item?.interfaceConfiguration || candidate?.interfaceConfiguration,
  );
  channelModal.value = true;
  showDrawer({ title: tr(item ? "editChannel" : "newChannel"), fields: channelFields(), values: { ...channelForm },
    onValuesChange: values => { const previous = channelForm.providerPluginId; Object.assign(channelForm, values); if (values.providerPluginId && values.providerPluginId !== previous) { for (const key of Object.keys(channelForm)) if (!["providerPluginId", "displayName", "endpoint", "status", "tlsCertificateSha256"].includes(key)) delete channelForm[key]; chooseConnector(String(values.providerPluginId)); formDrawer?.update({ fields: channelFields(), values: { ...channelForm } }); } },
    onSubmit: async values => { Object.assign(channelForm, values); formDrawer?.update({ errorText: "" }); const saved = await saveChannel(); if (!saved) formDrawer?.update({ errorText: error.value }); return saved; },
  });
}
async function digest(value: string) {
  const hash = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(value),
  );
  return Array.from(new Uint8Array(hash), (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
}
function base64(value: string) {
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  bytes.forEach((byte) => (binary += String.fromCharCode(byte)));
  return btoa(binary);
}
async function saveSecret(
  channelNo: string,
  key: string,
  value: string,
  status?: SecretField,
) {
  const replacing = status?.ready === true;
  const expectedUpdatedAt = replacing ? Date.parse(status.updatedAt || "") : 0;
  if (replacing && !Number.isFinite(expectedUpdatedAt))
    throw new Error(tr("operationFailed"));
  const idempotencyKey = `admin-${Date.now()}-${crypto.randomUUID()}`.slice(
    0,
    128,
  );
  const access = await api.secretAccess(channelNo, {
    operation: replacing ? "replace" : "create",
    bindingName: key,
    secretDigest: await digest(value),
    expectedUpdatedAt,
    rotateLimit: 0,
    idempotencyKey,
  });
  const body = {
    bindingName: key,
    secret: base64(value),
    expectedUpdatedAt,
    idempotencyKey,
    accessContext: access.item.token,
  };
  if (replacing) await api.replaceSecret(channelNo, key, body);
  else await api.createSecret(channelNo, body);
}
async function saveChannel() {


  const revision = formRevision;
  const values = splitInterfaceConfigurationValues(schema.value, channelForm);
  const ready = readyMap(editingChannel.value);
  if (
    schema.value.fields.some(
      (field) =>
        field.type === "secret" &&
        field.required &&
        !ready[field.key] &&
        !values.secrets[field.key],
    )
  ) {
    error.value = tr("secretRequired");
    return false;
  }
  try {
    const body = {
      displayName: String(channelForm.displayName || ""),
      endpoint: String(channelForm.endpoint || ""),
      tlsCertificateSha256: undefined,
      providerPluginId: String(channelForm.providerPluginId || ""),
      status: String(channelForm.status || "disabled"),
      nonSecretSettings: values.nonSecretSettings,
    };
    let channel = editingChannel.value;
    if (channel) channel = (await api.updateChannel(channel, body)).item;
    else {
      channel = (await api.createChannel(body)).item;
      if (revision !== formRevision) return false;
      editingChannel.value = channel;
    }
    if (revision !== formRevision) return false;
    for (const [key, value] of Object.entries(values.secrets)) {
      if (revision !== formRevision) return false;
      await saveSecret(
        channel.channelNo,
        key,
        value,
        channel.secretFields.find((field) => field.key === key),
      );
    }
    if (revision !== formRevision) return false;
    notice.value = tr("channelSaved");
    await load();
    return revision === formRevision;
  } catch (value) {
    if (revision !== formRevision) return false;
    if (value instanceof RequestError && [401, 403].includes(value.httpStatus ?? 0)) clearAuthorizedData();
    error.value = failure(value);
    return false;
  }
}
function providerUpdateNeeded(item: Channel) {
 const candidate = candidates.value.find((value) => value.pluginId === item.providerPluginId);
 return !!item.providerReleaseVersion && !!candidate && candidate.releaseVersion !== item.providerReleaseVersion;
}
async function refreshChannelProvider(item: Channel) {
 if (refreshingChannels.value.includes(item.channelNo)) return;
 refreshingChannels.value = [...refreshingChannels.value, item.channelNo];
 try {
  await api.updateChannel(item,{displayName:item.displayName,endpoint:item.endpoint,status:item.status,refreshProvider:true});
  notice.value = tr("providerUpdated"); await load();
 } catch(value) {error.value=failure(value);} finally {refreshingChannels.value=refreshingChannels.value.filter((value)=>value!==item.channelNo);}
}
async function setChannelStatus(item: Channel, status: string) {
  try {
    await api.updateChannel(item, {
      displayName: item.displayName,
      endpoint: item.endpoint,
      tlsCertificateSha256: item.tlsCertificateSha256 || "",
      nonSecretSettings: item.nonSecretSettings || {},
      status,
    });
    await load();
  } catch (value) {
    error.value = failure(value);
  }
}
async function verifyChannel(item: Channel) {
  if (testingChannels.value.includes(item.channelNo)) return;
  testingChannels.value = [...testingChannels.value, item.channelNo];
  error.value = "";
  notice.value = "";
  try {
    const result = await api.verifyChannel(item.channelNo);
    if (result.verified) notice.value = tr("connectionVerified");
    else error.value = verificationMessage(result.statusCode);
    await load();
  } catch (value) {
    error.value = failure(value);
  } finally {
    testingChannels.value = testingChannels.value.filter(
      (channelNo) => channelNo !== item.channelNo,
    );
  }
}
function verificationMessage(status: string) {
 const supported = ["configuration_incomplete", "verification_unsupported", "connection_failed", "tls_untrusted", "timeout", "dns", "network", "endpoint_not_found", "invalid_response", "auth_rejected", "local_auth_policy"];
 return tr("verification." + (supported.includes(status) ? status : "connection_failed"));
}
function connectionStatus(item: Channel) {
 if(providerUpdateNeeded(item)) return tr("providerUpdateNeeded");
  if (item.connectionStatus === "healthy") return tr("connectionHealthy");
  if (item.connectionStatus === "degraded") return tr("connectionDegraded");
  return tr("connectionUnknown");
}
function openPool(item?: Pool) {
  editingPool.value = item;
  Object.assign(poolForm, {
    channelNo: item?.channelNo || activeChannels.value[0]?.channelNo || "",
    displayName: item?.displayName || "",
    resourceKind: item?.resourceKind || "virtual_server",
    routingPriority: item?.routingPriority ?? 100,
    routingWeight: item?.routingWeight ?? 100,
    status:
      item?.status === "deleted" ? "disabled" : item?.status || "disabled",
  });
  poolModal.value = true;
  poolChannelOptions = activeChannels.value.map(channel => ({label: channel.displayName, value: channel.channelNo}));
  showDrawer({ title: tr(item ? "editPool" : "newPool"), fields: poolFields(), values: { ...poolForm }, onSubmit: async values => { Object.assign(poolForm, values); formDrawer?.update({ errorText: "" }); const saved = await savePool(); if (!saved) formDrawer?.update({ errorText: error.value }); return saved; } });
  if (!item) void loadPoolChannels();
}
async function savePool() {
  const revision = formRevision;
  try {
    if (editingPool.value)
      await api.updatePool(editingPool.value, { ...poolForm });
    else await api.createPool({ ...poolForm });
    if (revision !== formRevision) return false;
    notice.value = tr("poolSaved");
    await load();
    return revision === formRevision;
  } catch (value) {
    if (revision !== formRevision) return false;
    if (value instanceof RequestError && [401, 403].includes(value.httpStatus ?? 0)) clearAuthorizedData();
    error.value = failure(value);
    return false;
  }
}
async function setPoolStatus(item: Pool, status: string) {
  try {
    await api.updatePool(item, {
      displayName: item.displayName,
      resourceKind: item.resourceKind,
      routingPriority: item.routingPriority,
      routingWeight: item.routingWeight,
      status,
    });
    await load();
  } catch (value) {
    error.value = failure(value);
  }
}
function rowButton(label: string, action: () => unknown, disabled = false, danger = false) {
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = label;
  button.className = danger ? "button danger" : "button";
  button.disabled = disabled;
  button.onclick = () => { void action(); };
  return button;
}
const channelColumns = computed<IsolatedTableColumn<Channel>[]>(() => {
  const testing = new Set(testingChannels.value);
  return [
    { field: "displayName", title: tr("channel"), minWidth: 180 },
    { field: "endpoint", title: tr("endpoint"), minWidth: 240 },
    { field: "usage", title: tr("apiProducts"), width: 170, cell: (item) => item.usage ? `${item.usage.activeInstances} / ${item.usage.totalInstances}` : "—" },
    { field: "usage.configuredProducts", title: tr("configuredProducts"), width: 130, cell: (item) => String(item.usage?.configuredProducts ?? "—") },
    { field: "secretFields", title: tr("accountStatus"), width: 130, cell: (item) => tr(providerUpdateNeeded(item) ? "providerUpdateNeeded" : secretsComplete(item) ? "configured" : "incomplete") },
    { field: "status", title: tr("runtimeStatus"), width: 120, cell: (item) => tr(`status.${item.status}`) },
    { field: "connectionStatus", title: tr("connectionStatus"), width: 140, cell: connectionStatus },
    { field: "actions", title: tr("actions"), width: 330, fixed: "right", cell: (item) => {
      const buttons = [
        rowButton(tr("details"), () => openDetails(item)),
        rowButton(tr("edit"), () => openChannel(item)),
        rowButton(tr("testConnection"), () => verifyChannel(item), item.status === "deleted" || providerUpdateNeeded(item) || !secretsComplete(item) || testing.has(item.channelNo)),
      ];
      if(providerUpdateNeeded(item) && item.status !== "deleted") buttons.unshift(rowButton(tr("updateProvider"),()=>refreshChannelProvider(item),refreshingChannels.value.includes(item.channelNo)));
      if (item.status === "disabled") buttons.push(rowButton(tr("enable"), () => setChannelStatus(item, "enabled"), !secretsComplete(item)));
      if (item.status === "enabled") buttons.push(rowButton(tr("disable"), () => setChannelStatus(item, "disabled")));
      buttons.push(item.status !== "deleted"
        ? rowButton(tr("delete"), () => setChannelStatus(item, "deleted"), false, true)
        : rowButton(tr("restore"), () => setChannelStatus(item, "disabled")));
      return buttons;
    } },
  ];
});
const poolColumns = computed<IsolatedTableColumn<Pool>[]>(() => {
  const names = new Map(channels.value.map((item) => [item.channelNo, item.displayName]));
  return [
    { field: "displayName", title: tr("pool"), minWidth: 180 },
    { field: "channelNo", title: tr("owningChannel"), minWidth: 180, cell: (item) => item.channelName || names.get(item.channelNo) || tr("unknownChannel") },
    { field: "resourceKind", title: tr("serverType"), width: 160, cell: (item) => tr(`resourceKind.${item.resourceKind}`) },
    { field: "routingPriority", title: tr("priorityWeight"), width: 150, cell: (item) => `${item.routingPriority} / ${item.routingWeight}` },
    { field: "status", title: tr("statusLabel"), width: 120, cell: (item) => tr(`status.${item.status}`) },
    { field: "actions", title: tr("actions"), width: 250, fixed: "right", cell: (item) => {
      const buttons = [rowButton(tr("edit"), () => openPool(item))];
      if (item.status === "disabled") buttons.push(rowButton(tr("enable"), () => setPoolStatus(item, "enabled")));
      if (item.status === "enabled") buttons.push(rowButton(tr("disable"), () => setPoolStatus(item, "disabled")));
      buttons.push(item.status !== "deleted"
        ? rowButton(tr("delete"), () => setPoolStatus(item, "deleted"), false, true)
        : rowButton(tr("restore"), () => setPoolStatus(item, "disabled")));
      return buttons;
    } },
  ];
});
onMounted(() => void load());
onBeforeUnmount(() => { formRevision++; candidateRevision++; formDrawer?.destroy(); });
</script>

<template>
  <main class="provider-configuration-workspace">
    <section class="toolbar card">
      <div>
        <h2>{{ tr("title") }}</h2>
        <p>{{ tr("subtitle") }}</p>
      </div>
      <button class="button" :disabled="loading" @click="load()">
        {{ tr("refresh") }}
      </button>
    </section>
    <p v-if="error" class="notice error">{{ error }}</p>
    <p v-if="notice" class="notice success">{{ notice }}</p>
    <nav class="tabs card">
      <button :class="{ active: tab === 'channels' }" @click="tab = 'channels'">
        {{ tr("channels") }}</button
      ><button :class="{ active: tab === 'pools' }" @click="tab = 'pools'">
        {{ tr("pools") }}
      </button>
    </nav>
    <section v-if="tab === 'channels'" class="card panel">
      <header>
        <div>
          <h3>{{ tr("channels") }}</h3>
          <p>{{ tr("channelHint") }}</p>
        </div>
        <button class="button" data-testid="provider-show-deleted" @click="showDeletedChannels = !showDeletedChannels; load({ currentPage: 1, pageSize: channelPage.pageSize })">
          {{ tr(showDeletedChannels ? "hideDeleted" : "showDeleted") }}
        </button>
        <button
          class="button primary"
          :disabled="candidates.length === 0"
          @click="openChannel()"
        >
          {{ tr("newChannel") }}
        </button>
      </header>
      <StandaloneTable :columns="channelColumns" :rows="channels" :pagination="channelPagination" row-key="channelNo" :locale="locale()" :empty-text="tr('emptyChannels')" :loading="loading" @failed="error = tr('operationFailed')" />
    </section>
    <section v-else class="card panel">
      <header>
        <div>
          <h3>{{ tr("pools") }}</h3>
          <p>{{ tr("poolHint") }}</p>
        </div>
        <button
          class="button primary"
          :disabled="loading"
          @click="openPool()"
        >
          {{ tr("newPool") }}
        </button>
      </header>
      <StandaloneTable :columns="poolColumns" :rows="pools" :pagination="poolPagination" row-key="poolNo" :locale="locale()" :empty-text="tr('emptyPools')" :loading="loading" @failed="error = tr('operationFailed')" />
    </section>
    <div ref="drawerRoot" />
  </main>
</template>
