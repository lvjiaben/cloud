<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/provider-configuration",
  title: "Server Provider Configuration",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type { VxeGridProps } from "#/adapter/vxe-table";
import type { VbenFormProps } from "#/adapter/form";
import type {
  ProviderCandidate,
  ProviderChannel,
  ProviderPool,
} from "./provider-configuration-client";

import { computed, nextTick, onMounted, ref, shallowRef, watch } from "vue";
import { Page, useVbenDrawer, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Alert, Button, Card, Descriptions, Tabs, message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";
import { $t } from "#/locales";
import ProviderCatalog from "./components/provider-catalog.vue";
import PeerLifecycle from "./components/peer-lifecycle.vue";
import {
  createProviderChannel,
  createProviderPool,
  createProviderSecret,
  createProviderSecretAccess,
  getProviderConfigurations,
  replaceProviderSecret,
  updateProviderChannel,
  updateProviderPool,
  verifyProviderChannel,
} from "./provider-configuration-client";
import {
  interfaceCandidateLabel,
  interfaceFieldDescription,
  interfaceFieldLabel,
  interfaceTreeOptions,
  normalizeInterfaceConfigurationSchema,
  secretConfigurationComplete,
  splitInterfaceConfigurationValues,
} from "./provider-interface-configuration";
import type {
  InterfaceConfigurationField,
  InterfaceConfigurationSchema,
} from "./provider-interface-configuration";

const tr = (key: string) =>
  $t(`plugin.idc-resource.providerConfiguration.${key}`);
const channels = shallowRef<ProviderChannel[]>([]);
const pools = shallowRef<ProviderPool[]>([]);
const providerCandidates = ref<ProviderCandidate[]>([]);
const loading = ref(false);
const showDeletedChannels = ref(false);
const channelPage = ref({ currentPage: 1, pageSize: 20, total: 0 });
const poolPage = ref({ currentPage: 1, pageSize: 20, total: 0 });
const detail = ref<ProviderChannel>();
const detailTab = ref("overview");
const channelOptions = ref<ProviderChannel[]>([]);
let loadGeneration = 0;
let optionGeneration = 0;
let optionKeyword = "";
let optionPage = 1;
let optionTotal = 0;
let optionLoading = false;
let detailOpen = false;
const loadedDirectories = new Set<"channels" | "pools">();
const [DetailDrawer, detailDrawerApi] = useVbenDrawer({
  showConfirmButton: false,
  onOpenChange: (open) => { detailOpen = open; },
  onClosed: () => { if (!detailOpen) detail.value = undefined; },
});
function openDetail(item: ProviderChannel) {
  detailTab.value = "overview";
  detail.value = item;
  detailDrawerApi.open();
}
watch(() => [detail.value?.catalogAvailable, detail.value?.status, detail.value?.interfaceInstanceNo], () => {
  if (!detail.value?.catalogAvailable || detail.value.status !== "enabled" || !detail.value.interfaceInstanceNo)
    detailTab.value = "overview";
});
function denied(error: unknown) {
  const status = (error as { response?: { status?: number } })?.response?.status;
  return status === 401 || status === 403;
}
const activeTab = ref("channels");
const editingChannel = ref<ProviderChannel>();
const editingPool = ref<ProviderPool>();
const selectedInterfaceSchema = ref<InterfaceConfigurationSchema>({
  fields: [],
  version: "",
});
const testingChannels = ref<string[]>([]);
const refreshingChannels = ref<string[]>([]);
const activeChannels = computed(() => channelOptions.value);

const [ChannelForm, channelFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
});
const [PoolForm, poolFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [
    {
      component: "Select",
      componentProps: { options: [] },
      dependencies: {
        show: () => !editingPool.value,
        triggerFields: ["displayName"],
      },
      fieldName: "channelNo",
      label: tr("owningChannel"),
      rules: z.string().min(1),
    },
    {
      component: "Input",
      fieldName: "displayName",
      label: tr("poolName"),
      rules: z.string().trim().min(1),
    },
    {
      component: "Select",
      componentProps: {
        options: ["virtual_server", "bare_metal"].map((value) => ({
          value,
          label: tr(`resourceKind.${value}`),
        })),
      },
      fieldName: "resourceKind",
      label: tr("serverType"),
      rules: z.string().min(1),
    },
    {
      component: "InputNumber",
      componentProps: { max: 100000, min: 0 },
      fieldName: "routingPriority",
      label: tr("routingPriority"),
      rules: z.number().min(0).max(100000),
    },
    {
      component: "InputNumber",
      componentProps: { max: 10000, min: 0 },
      fieldName: "routingWeight",
      label: tr("routingWeight"),
      rules: z.number().min(0).max(10000),
    },
    {
      component: "Select",
      componentProps: {
        options: ["enabled", "draining", "maintenance", "disabled"].map(
          (value) => ({ value, label: tr(`status.${value}`) }),
        ),
      },
      dependencies: {
        show: () => Boolean(editingPool.value),
        triggerFields: ["displayName"],
      },
      fieldName: "status",
      label: tr("statusLabel"),
    },
  ],
  showDefaultActions: false,
});
function channelColumns(): VxeGridProps<ProviderChannel>["columns"] {
  return [
    {
      field: "displayName",
      slots: { default: "channel" },
      title: tr("channel"),
      minWidth: 180,
    },
    { field: "endpoint", title: tr("endpoint"), minWidth: 220 },
    { field: "usage", title: tr("apiProducts"), width: 165, align: "center", slots: { default: "usage" } },
    { field: "usage.configuredProducts", title: tr("configuredProducts"), width: 110, align: "center" },
    {
      field: "account",
      slots: { default: "account" },
      title: tr("accountStatus"),
      width: 130,
    },
    {
      field: "status",
      slots: { default: "status" },
      title: tr("runtimeStatus"),
      width: 130,
    },
    {
      field: "connectionStatus",
      slots: { default: "connection" },
      title: tr("connectionStatus"),
      width: 180,
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: tr("actions"),
      width: 380,
    }),
  ];
}
function poolColumns(): VxeGridProps<ProviderPool>["columns"] {
  return [
    {
      field: "displayName",
      slots: { default: "pool" },
      title: tr("pool"),
      minWidth: 180,
    },
    {
      field: "channelNo",
      slots: { default: "channel" },
      title: tr("owningChannel"),
      minWidth: 160,
    },
    {
      field: "resourceKind",
      slots: { default: "kind" },
      title: tr("serverType"),
      width: 140,
    },
    {
      field: "routing",
      slots: { default: "routing" },
      title: tr("priorityWeight"),
      width: 140,
    },
    {
      field: "status",
      slots: { default: "status" },
      title: tr("statusLabel"),
      width: 130,
    },
    fixedRightActionColumn({
      field: "action",
      slots: { default: "action" },
      title: tr("actions"),
      width: 260,
    }),
  ];
}

const [ChannelGrid, channelGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: channelColumns(),
    height: "100%",
    pagerConfig: { ...channelPage.value, pageSizes: [10, 20, 50, 100] },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: ({ page }: { page: { currentPage: number; pageSize: number } }) =>
          queryDirectory("channels", page),
      },
    },
    rowConfig: { keyField: "channelNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-provider-channels",
  },
});
const [PoolGrid, poolGridApi] = useVbenVxeGrid({
  gridOptions: {
    columns: poolColumns(),
    height: "100%",
    pagerConfig: { enabled: true, ...poolPage.value, pageSizes: [10, 20, 50, 100] },
    proxyConfig: {
      autoLoad: false,
      ajax: {
        query: ({ page }: { page: { currentPage: number; pageSize: number } }) =>
          queryDirectory("pools", page),
      },
    },
    rowConfig: { keyField: "poolNo" },
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-provider-pools",
  },
});

const [ChannelModal, channelModalApi] = useVbenModal({
  onCancel: () => channelModalApi.close(),
  onConfirm: saveChannel,
  onOpenChange: initializeChannelForm,
});
const [PoolModal, poolModalApi] = useVbenModal({
  onCancel: () => poolModalApi.close(),
  onConfirm: savePool,
  onOpenChange: initializePoolForm,
});
watch(
  () => preferences.app.locale,
  () => {
    channelGridApi.setGridOptions({ columns: channelColumns() });
    poolGridApi.setGridOptions({ columns: poolColumns() });
  },
);

function clearAuthorizedData() {
  ++loadGeneration;
  ++optionGeneration;
  channels.value = []; pools.value = []; providerCandidates.value = []; channelOptions.value = [];
  detail.value = undefined; editingChannel.value = undefined; editingPool.value = undefined;
  channelPage.value = { ...channelPage.value, total: 0 };
  poolPage.value = { ...poolPage.value, total: 0 };
  loading.value = false;
  detailDrawerApi.close(); channelModalApi.close(); poolModalApi.close();
  selectedInterfaceSchema.value = { fields: [], version: "" };
  optionTotal = 0;
  optionLoading = false;
  loadedDirectories.clear();
  void channelFormApi.resetForm();
  if (typeof channelGridApi.grid?.reloadData === "function")
    void channelGridApi.grid.reloadData([]);
  if (typeof poolGridApi.grid?.reloadData === "function")
    void poolGridApi.grid.reloadData([]);
}
async function queryDirectory(
  target: "channels" | "pools",
  requested: { currentPage: number; pageSize: number },
) {
  const generation = ++loadGeneration;
  const confirmedChannel = { ...channelPage.value };
  const confirmedPool = { ...poolPage.value };
  const channel = {
    ...confirmedChannel,
    ...(target === "channels" ? requested : {}),
  };
  const pool = {
    ...confirmedPool,
    ...(target === "pools" ? requested : {}),
  };
  if (channel.pageSize !== channelPage.value.pageSize) channel.currentPage = 1;
  if (pool.pageSize !== poolPage.value.pageSize) pool.currentPage = 1;
  loading.value = true;
  try {
    const data = await getProviderConfigurations({
      channelPage: channel.currentPage, channelPageSize: channel.pageSize,
      availableChannelsOnly: !showDeletedChannels.value,
      poolPage: pool.currentPage, poolPageSize: pool.pageSize,
    });
    if (generation !== loadGeneration) return;
    if (!Number.isSafeInteger(data.channelTotal) || data.channelTotal < 0 ||
        !Number.isSafeInteger(data.poolTotal) || data.poolTotal < 0 ||
        !Array.isArray(data.channels) || data.channels.length > channel.pageSize ||
        !Array.isArray(data.pools) || data.pools.length > pool.pageSize)
      throw new Error("invalid pagination");
    channels.value = data.channels || [];
    pools.value = data.pools || [];
    providerCandidates.value = data.providerCandidates || [];
    channelPage.value = { ...channel, total: data.channelTotal };
    poolPage.value = { ...pool, total: data.poolTotal };
    loadedDirectories.add(target);
    if (detail.value) detail.value = channels.value.find((item) => item.channelNo === detail.value?.channelNo);
    if (!detail.value) detailDrawerApi.close();
    const page = target === "channels" ? channelPage.value : poolPage.value;
    const grid = target === "channels" ? channelGridApi.grid : poolGridApi.grid;
    await grid.setPageSize(page.pageSize);
    await grid.setCurrentPage(page.currentPage);
    return {
      items: target === "channels" ? channels.value : pools.value,
      total: page.total,
    };
  } catch (error) {
    const deniedRequest = denied(error);
    if (generation !== loadGeneration)
      return {
        items: target === "channels" ? channels.value : pools.value,
        total: target === "channels" ? channelPage.value.total : poolPage.value.total,
      };
    if (deniedRequest) clearAuthorizedData();
    const page = deniedRequest
      ? { currentPage: 1, pageSize: requested.pageSize, total: 0 }
      : target === "channels" ? confirmedChannel : confirmedPool;
    const grid = target === "channels" ? channelGridApi.grid : poolGridApi.grid;
    await grid.setPageSize(page.pageSize);
    await grid.setCurrentPage(page.currentPage);
    message.error(tr("operationFailed"));
    return {
      items: deniedRequest ? [] : target === "channels" ? channels.value : pools.value,
      total: page.total,
    };
  } finally {
    if (generation === loadGeneration) loading.value = false;
  }
}
async function load() {
  if (activeTab.value === "pools") await poolGridApi.query();
  else await channelGridApi.query();
}
watch(activeTab, async (target) => {
  if (
    (target === "channels" || target === "pools") &&
    !loadedDirectories.has(target)
  ) {
    // Inactive tab panes are mounted lazily. Wait until the selected grid is
    // available before asking VXE's proxy to perform its first query.
    await nextTick();
    await load();
  }
}, { flush: "post" });
function loadMoreChannels(event: Event) {
  const element = event.target as HTMLElement | null;
  if (!element || optionLoading || channelOptions.value.length >= optionTotal) return;
  if (element.scrollTop + element.clientHeight >= element.scrollHeight - 16) void searchChannels(optionKeyword, true);
}
async function searchChannels(keyword = "", append = false) {
  const generation = ++optionGeneration;
  const page = append ? optionPage + 1 : 1;
  optionLoading = true;
  try {
    const data = await getProviderConfigurations({ channelPage: page, channelPageSize: 100,
      poolPage: 1, poolPageSize: 10, channelKeyword: keyword, availableChannelsOnly: true });
    if (generation !== optionGeneration) return;
    optionKeyword = keyword;
    optionPage = page;
    optionTotal = data.channelTotal;
    channelOptions.value = append ? [...channelOptions.value, ...data.channels] : data.channels;
    await poolFormApi.updateSchema([{ fieldName: "channelNo", componentProps: {
      options: channelOptions.value.map((item) => ({ value: item.channelNo, label: item.displayName })),
      showSearch: true, filterOption: false, onSearch: (value: string) => void searchChannels(value),
      onPopupScroll: loadMoreChannels,
      placeholder: tr("searchChannel"),
    } }]);
  } catch (error) {
    if (generation !== optionGeneration) return;
    if (denied(error)) clearAuthorizedData();
    message.error(tr("operationFailed"));
  } finally {
    if (generation === optionGeneration) optionLoading = false;
  }
}
function openChannel(item?: ProviderChannel) {
  if (item && !item.interfaceConfiguration) return;
  editingChannel.value = item;
  channelModalApi.open();
}
async function initializeChannelForm(open: boolean) {
  if (!open) return;
  const generation = loadGeneration;
  channelModalApi.setState({ loading: true });
  try {
    const item = editingChannel.value;
    const candidate = item ? undefined : providerCandidates.value[0];
    await applyChannelSchema(
      item?.interfaceConfiguration || candidate?.interfaceConfiguration,
      item,
    );
    if (generation !== loadGeneration) return;
    await channelFormApi.setValues({
      providerDisplayName: item ? connectorDisplayName(item) : "",
      providerPluginId: item?.providerPluginId || candidate?.pluginId || "",
      displayName: item?.displayName || "",
      endpoint: item?.endpoint || "",
      tlsCertificateSha256: item?.tlsCertificateSha256 || "",
      ...(item?.nonSecretSettings || {}),
      status:
        item?.status === "deleted" ? "disabled" : item?.status || "disabled",
    });
  } finally {
    channelModalApi.setState({ loading: false });
  }
}
async function saveChannel() {
  const { valid } = await channelFormApi.validate();
  if (!valid) return;
  const values = await channelFormApi.getValues<Record<string, unknown>>();
  const { nonSecretSettings, secrets } = splitInterfaceConfigurationValues(
    selectedInterfaceSchema.value,
    values,
  );
  const configured = secretReadyMap(editingChannel.value);
  const missingSecret = selectedInterfaceSchema.value.fields.some(
    (field) =>
      field.type === "secret" &&
      field.required &&
      !configured[field.key] &&
      !secrets[field.key],
  );
  if (missingSecret) {
    message.error(tr("secretRequired"));
    return;
  }
  channelModalApi.lock();
  try {
    const body = {
      displayName: String(values.displayName || ""),
      endpoint: String(values.endpoint || ""),
      tlsCertificateSha256: String(values.tlsCertificateSha256 || "").trim().toLowerCase(),
      nonSecretSettings,
      providerPluginId: String(values.providerPluginId || ""),
      status: String(values.status || "disabled"),
    };
    let channel = editingChannel.value;
    if (channel) {
      const result = await updateProviderChannel(channel, body);
      channel = result.item;
    } else {
      const result = await createProviderChannel(body);
      channel = result.item;
      editingChannel.value = channel;
    }
    for (const [bindingName, value] of Object.entries(secrets)) {
      const status = channel.secretFields?.find(
        (field) => field.key === bindingName,
      );
      await saveSecret(channel.channelNo, bindingName, value, status);
    }
    message.success(tr("channelSaved"));
    channelModalApi.close();
    await load();
  } catch {
    message.error(tr("operationFailed"));
  } finally {
    channelModalApi.unlock();
  }
}

function currentLocale() {
  return String(preferences.app.locale || "zh-CN");
}

function connectorDisplayName(item: ProviderChannel) {
  const current = providerCandidates.value.find(
    (candidate) => candidate.pluginId === item.providerPluginId,
  );
  return current?.displayName || tr("installedConnector");
}

function fieldRules(field: InterfaceConfigurationField) {
  if (!field.required) return z.any().optional();
  if (field.type === "boolean") return z.boolean();
  if (["multi_select", "file"].includes(field.type))
    return z.array(z.any()).min(1);
  if (["integer", "decimal"].includes(field.type)) return z.number();
  return z
    .any()
    .refine((value) => value !== undefined && value !== null && value !== "");
}

function interfaceFieldSchema(
  field: InterfaceConfigurationField,
): NonNullable<VbenFormProps["schema"]>[number] {
  const locale = currentLocale();
  const description = interfaceFieldDescription(field, locale);
  const candidates = (field.staticCandidates || []).map((candidate) => ({
    label: interfaceCandidateLabel(candidate, locale),
    value: candidate.value,
  }));
  const common = {
    fieldName: field.key,
    label: interfaceFieldLabel(field, locale),
    rules: fieldRules(field),
  };
  if (field.type === "secret") {
    const configured = secretReadyMap(editingChannel.value)[field.key];
    return {
      ...common,
      component: "InputPassword",
      componentProps: {
        autocomplete: "new-password",
        maxlength: field.maximumLength || 4096,
        placeholder: configured ? tr("secretConfiguredHint") : description,
      },
      rules: configured ? z.any().optional() : fieldRules(field),
    };
  }
  if (field.type === "multi_line")
    return {
      ...common,
      component: "Textarea",
      componentProps: {
        maxlength: field.maximumLength,
        placeholder: description,
        rows: 4,
      },
    };
  if (field.type === "integer" || field.type === "decimal")
    return {
      ...common,
      component: "InputNumber",
      componentProps: {
        max: field.maximum ? Number(field.maximum) : undefined,
        min: field.minimum ? Number(field.minimum) : undefined,
        precision: field.type === "integer" ? 0 : undefined,
      },
    };
  if (field.type === "boolean") return { ...common, component: "Switch" };
  if (field.type === "single_select" || field.type === "multi_select")
    return {
      ...common,
      component: "Select",
      componentProps: {
        disabled: candidates.length === 0 && Boolean(field.dynamicCandidates),
        mode: field.type === "multi_select" ? "multiple" : undefined,
        options: candidates,
        placeholder:
          candidates.length === 0 && field.dynamicCandidates
            ? tr("dynamicCandidatesUnavailable")
            : description,
        showSearch: true,
      },
    };
  if (field.type === "tree_select")
    return {
      ...common,
      component: "TreeSelect",
      componentProps: {
        treeData: interfaceTreeOptions(field, locale),
        treeDefaultExpandAll: true,
      },
    };
  if (field.type === "address")
    return {
      ...common,
      component: "Cascader",
      componentProps: { options: interfaceTreeOptions(field, locale) },
    };
  if (field.type === "date" || field.type === "datetime")
    return {
      ...common,
      component: "DatePicker",
      componentProps: {
        showTime: field.type === "datetime",
        valueFormat:
          field.type === "date" ? "YYYY-MM-DD" : "YYYY-MM-DDTHH:mm:ssZ",
      },
    };
  if (field.type === "file")
    return {
      ...common,
      component: "Upload",
      componentProps: {
        disabled: true,
        maxCount: 0,
        placeholder: tr("fileFieldUnavailable"),
      },
    };
  return {
    ...common,
    component: "Input",
    componentProps: {
      maxlength: field.maximumLength,
      placeholder: description,
    },
  };
}

function channelBaseSchema(item?: ProviderChannel): NonNullable<VbenFormProps["schema"]> {
  return [
    item
      ? {
          component: "Input",
          componentProps: { disabled: true },
          fieldName: "providerDisplayName",
          label: tr("connector"),
        }
      : {
          component: "Select",
          componentProps: {
            onChange: (pluginId: string) => {
              const candidate = providerCandidates.value.find(
                (value) => value.pluginId === pluginId,
              );
              void applyChannelSchema(candidate?.interfaceConfiguration);
            },
            options: providerCandidates.value.map((candidate) => ({
              label: `${candidate.displayName} · ${candidate.releaseVersion}`,
              value: candidate.pluginId,
            })),
          },
          fieldName: "providerPluginId",
          label: tr("connector"),
          rules: z.string().min(1),
        },
    {
      component: "Input",
      fieldName: "displayName",
      label: tr("channelName"),
      rules: z.string().trim().min(1),
    },
    {
      component: "Input",
      fieldName: "endpoint",
      label: tr("apiEndpoint"),
      rules: z.string().trim().url(),
    },
    {
      component: "Input",
      fieldName: "tlsCertificateSha256",
      label: tr("tlsCertificateSha256"),
      componentProps: { placeholder: tr("tlsCertificateHint"), maxlength: 64 },
      rules: z.string().trim().regex(/^(?:[a-fA-F0-9]{64})?$/).optional(),
    },
    ...(item
      ? [
          {
            component: "Select" as const,
            componentProps: {
              options: ["disabled", "enabled", "maintenance"].map((value) => ({
                value,
                label: tr(`status.${value}`),
              })),
            },
            fieldName: "status",
            label: tr("statusLabel"),
          },
        ]
      : []),
  ];
}

async function applyChannelSchema(
  rawSchema?: InterfaceConfigurationSchema,
  item = editingChannel.value,
) {
  const currentValues =
    await channelFormApi.getValues<Record<string, unknown>>();
  selectedInterfaceSchema.value =
    normalizeInterfaceConfigurationSchema(rawSchema);
  channelFormApi.setState({
    schema: [
      ...channelBaseSchema(item),
      ...selectedInterfaceSchema.value.fields
        .filter((field) => field.key !== "endpoint")
        .map(interfaceFieldSchema),
    ],
  });
  await channelFormApi.setValues(currentValues, false);
}
function openPool(item?: ProviderPool) {
  editingPool.value = item;
  poolModalApi.open();
}
async function initializePoolForm(open: boolean) {
  if (!open) return;
  const generation = loadGeneration;
  poolModalApi.setState({ loading: true });
  try {
    const item = editingPool.value;
    if (!item) await searchChannels();
    if (generation !== loadGeneration) return;
    await poolFormApi.updateSchema([
      {
        componentProps: {
          options: activeChannels.value.map((channel) => ({
            value: channel.channelNo,
            label: channel.displayName,
          })),
        },
        fieldName: "channelNo",
      },
    ]);
    await poolFormApi.setValues({
      channelNo: item?.channelNo || activeChannels.value[0]?.channelNo || "",
      displayName: item?.displayName || "",
      resourceKind: item?.resourceKind || "virtual_server",
      routingPriority: item?.routingPriority ?? 100,
      routingWeight: item?.routingWeight ?? 100,
      status: item?.status || "disabled",
    });
  } finally {
    poolModalApi.setState({ loading: false });
  }
}
async function savePool() {
  const { valid } = await poolFormApi.validate();
  if (!valid) return;
  const values = await poolFormApi.getValues<{
    channelNo: string;
    displayName: string;
    resourceKind: string;
    routingPriority: number;
    routingWeight: number;
    status: string;
  }>();
  poolModalApi.lock();
  try {
    if (editingPool.value) await updateProviderPool(editingPool.value, values);
    else
      await createProviderPool({
        channelNo: values.channelNo,
        displayName: values.displayName,
        resourceKind: values.resourceKind,
        routingPriority: values.routingPriority,
        routingWeight: values.routingWeight,
      });
    message.success(tr("poolSaved"));
    poolModalApi.close();
    await load();
  } catch {
    message.error(tr("operationFailed"));
  } finally {
    poolModalApi.unlock();
  }
}
function providerUpdateNeeded(item: ProviderChannel) {
  const candidate = providerCandidates.value.find((value) => value.pluginId === item.providerPluginId);
  return !!item.providerReleaseVersion && !!candidate && candidate.releaseVersion !== item.providerReleaseVersion;
}
async function refreshChannelProvider(item: ProviderChannel) {
  if (refreshingChannels.value.includes(item.channelNo)) return;
  refreshingChannels.value = [...refreshingChannels.value, item.channelNo];
  try {
    await updateProviderChannel(item, { displayName: item.displayName, endpoint: item.endpoint, status: item.status, refreshProvider: true });
    message.success(tr("providerUpdated"));
    await load();
  } catch { message.error(tr("providerUpdateFailed")); } finally { refreshingChannels.value = refreshingChannels.value.filter((value) => value !== item.channelNo); }
}
async function setChannelStatus(item: ProviderChannel, status: string) {
  try {
    await updateProviderChannel(item, {
      displayName: item.displayName,
      endpoint: item.endpoint,
      tlsCertificateSha256: item.tlsCertificateSha256 || "",
      status,
    });
    await load();
  } catch {
    message.error(tr("operationFailed"));
  }
}

async function verifyChannel(item: ProviderChannel) {
  if (testingChannels.value.includes(item.channelNo)) return;
  testingChannels.value = [...testingChannels.value, item.channelNo];
  try {
    const result = await verifyProviderChannel(item.channelNo);
    if (result.verified) message.success(tr("connectionVerified"));
    else message.warning(verificationMessage(result.statusCode));
    await load();
  } catch {
    message.error(tr("connectionFailed"));
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
function connectionStatusLabel(item: ProviderChannel) {
  if (providerUpdateNeeded(item)) return tr("providerUpdateNeeded");
  if (item.connectionStatus === "healthy") return tr("connectionHealthy");
  if (item.connectionStatus === "degraded") return tr("connectionDegraded");
  return tr("connectionUnknown");
}

function connectionStatusDetail(item: ProviderChannel) {
  if (!item.connectionVerifiedAt) return "";
  return new Intl.DateTimeFormat(currentLocale(), {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(item.connectionVerifiedAt));
}
async function setPoolStatus(item: ProviderPool, status: string) {
  try {
    await updateProviderPool(item, {
      displayName: item.displayName,
      resourceKind: item.resourceKind,
      status,
      routingPriority: item.routingPriority,
      routingWeight: item.routingWeight,
    });
    await load();
  } catch {
    message.error(tr("operationFailed"));
  }
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
  bindingName: string,
  value: string,
  status?: ProviderChannel["secretFields"][number],
) {
  const idempotencyKey = `admin-${Date.now()}-${crypto.randomUUID()}`.slice(
    0,
    128,
  );
  const replacing = status?.ready === true;
  const access = await createProviderSecretAccess(channelNo, {
    operation: replacing ? "replace" : "create",
    bindingName,
    secretDigest: await digest(value),
    expectedUpdatedAt: replacing ? Date.parse(status.updatedAt || "") : 0,
    rotateLimit: 0,
    idempotencyKey,
  });
  const body = {
    bindingName,
    secret: base64(value),
    idempotencyKey,
    accessContext: access.item.token,
    expectedUpdatedAt: status?.updatedAt ? Date.parse(status.updatedAt) : 0,
  };
  if (replacing) await replaceProviderSecret(channelNo, bindingName, body);
  else await createProviderSecret(channelNo, body);
}

function secretReadyMap(item?: ProviderChannel) {
  return Object.fromEntries(
    (item?.secretFields || []).map((field) => [field.key, field.ready]),
  );
}

function channelSecretsComplete(item: ProviderChannel) {
  if (!item.interfaceConfiguration) return false;
  return secretConfigurationComplete(
    normalizeInterfaceConfigurationSchema(item.interfaceConfiguration),
    secretReadyMap(item),
  );
}

function channelName(channelNo: string) {
  return (
    channels.value.find((item) => item.channelNo === channelNo)?.displayName ||
    tr("unknownChannel")
  );
}

function statusLabel(status: string) {
  const known = new Set([
    "deleted",
    "disabled",
    "draining",
    "enabled",
    "maintenance",
  ]);
  return tr(`status.${known.has(status) ? status : "unknown"}`);
}

function resourceKindLabel(kind: string) {
  const known = new Set(["bare_metal", "virtual_server"]);
  return tr(`resourceKind.${known.has(kind) ? kind : "unknown"}`);
}

onMounted(load);
const layoutRoot = useAdminGridLayout(channelGridApi, poolGridApi);
</script>

<template>
  <Page :ref="layoutRoot" class="plugin-admin-page" :auto-content-height="true" data-testid="provider-configuration-page">
    <Card
      :body-style="{
        display: 'flex',
        flexDirection: 'column',
        flex: 1,
        minHeight: 0,
        overflow: 'hidden',
      }"
      :bordered="false"
      class="flex h-full min-h-0 flex-col"
    >
      <template #title>{{ tr("title") }}</template>
      <template #extra>
        <div class="flex flex-wrap gap-2"><PeerLifecycle /><Button :loading="loading" @click="load()">{{
          tr("refresh")
        }}</Button></div></template>
      <Alert class="mb-4" :message="tr('subtitle')" show-icon type="info" />
      <Alert
        v-if="channels.some((item) => !item.interfaceConfiguration)"
        class="mb-3"
        :message="tr('configurationUnavailable')"
        show-icon
        type="warning"
      />
      <Tabs
        v-model:active-key="activeTab"
        class="provider-configuration-tabs min-h-0 flex-1"
      >
        <Tabs.TabPane key="channels" :tab="tr('channels')">
          <div class="flex h-full min-h-0 flex-col">
            <div class="mb-3 flex justify-end gap-2">
              <Button data-testid="provider-show-deleted" @click="showDeletedChannels = !showDeletedChannels; channelGridApi.query()">{{ tr(showDeletedChannels ? 'hideDeleted' : 'showDeleted') }}</Button>
              <Button
                :disabled="providerCandidates.length === 0"
                type="primary"
                @click="openChannel()"
                >{{ tr("newChannel") }}</Button
              >
            </div>
            <Alert
              v-if="providerCandidates.length === 0"
              class="mb-3"
              :message="tr('noConnector')"
              show-icon
              type="warning"
            />
            <div class="min-h-0 flex-1">
              <ChannelGrid>
                <template #usage="{ row }">{{ row.usage ? `${row.usage.activeInstances} / ${row.usage.totalInstances}` : "—" }}</template>
                <template #channel="{ row }"
                  ><div>{{ row.displayName }}</div></template
                >
                <template #account="{ row }">{{
                  tr(
                    providerUpdateNeeded(row) ? "providerUpdateNeeded" : !row.interfaceConfiguration
                      ? "configurationMissing"
                      : channelSecretsComplete(row)
                        ? "configured"
                        : "incomplete",
                  )
                }}</template>
                <template #status="{ row }">{{
                  statusLabel(row.status)
                }}</template>
                <template #connection="{ row }">
                  <div>{{ connectionStatusLabel(row) }}</div>
                  <div class="text-xs text-muted-foreground">
                    {{ connectionStatusDetail(row) }}
                  </div>
                </template>
                <template #action="{ row }"
                  ><div class="flex gap-2 whitespace-nowrap">
                    <Button v-if="providerUpdateNeeded(row) && row.status !== 'deleted'" :loading="refreshingChannels.includes(row.channelNo)" size="small" type="link" @click="refreshChannelProvider(row)">{{ tr("updateProvider") }}</Button>
                    <Button size="small" type="link" @click="openDetail(row)">{{ tr("details") }}</Button>
                    <Button
                      :disabled="!row.interfaceConfiguration"
                      size="small"
                      type="link"
                      @click="openChannel(row)"
                      >{{ tr("edit") }}</Button
                    ><Button
                      :disabled="
                        row.status === 'deleted' || providerUpdateNeeded(row) || !channelSecretsComplete(row)
                      "
                      :loading="testingChannels.includes(row.channelNo)"
                      size="small"
                      type="link"
                      @click="verifyChannel(row)"
                      >{{ tr("testConnection") }}</Button
                    ><Button
                      v-if="row.status === 'disabled'"
                      :disabled="!channelSecretsComplete(row)"
                      size="small"
                      type="link"
                      @click="setChannelStatus(row, 'enabled')"
                      >{{ tr("enable") }}</Button
                    ><Button
                      v-if="row.status === 'enabled'"
                      :disabled="!row.interfaceConfiguration"
                      size="small"
                      type="link"
                      @click="setChannelStatus(row, 'disabled')"
                      >{{ tr("disable") }}</Button
                    ><Button
                      v-if="row.status !== 'deleted'"
                      :disabled="!row.interfaceConfiguration"
                      danger
                      size="small"
                      type="link"
                      @click="setChannelStatus(row, 'deleted')"
                      >{{ tr("delete") }}</Button
                    ><Button
                      v-else
                      :disabled="!row.interfaceConfiguration"
                      size="small"
                      type="link"
                      @click="setChannelStatus(row, 'disabled')"
                      >{{ tr("restore") }}</Button
                    >
                  </div></template
                >
              </ChannelGrid>
            </div>
          </div>
        </Tabs.TabPane>
        <Tabs.TabPane key="pools" :tab="tr('pools')">
          <div class="flex h-full min-h-0 flex-col">
            <div class="mb-3 flex justify-end">
              <Button
                :disabled="loading"
                type="primary"
                @click="openPool()"
                >{{ tr("newPool") }}</Button
              >
            </div>
            <div class="min-h-0 flex-1">
              <PoolGrid>
                <template #pool="{ row }"
                  ><div>{{ row.displayName }}</div></template
                >
                <template #channel="{ row }">{{
                  row.channelName || channelName(row.channelNo)
                }}</template>
                <template #kind="{ row }">{{
                  resourceKindLabel(row.resourceKind)
                }}</template
                ><template #routing="{ row }"
                  >{{ row.routingPriority }} / {{ row.routingWeight }}</template
                ><template #status="{ row }">{{
                  statusLabel(row.status)
                }}</template>
                <template #action="{ row }"
                  ><div class="flex gap-2 whitespace-nowrap">
                    <Button size="small" type="link" @click="openPool(row)">{{
                      tr("edit")
                    }}</Button
                    ><Button
                      v-if="row.status === 'disabled'"
                      size="small"
                      type="link"
                      @click="setPoolStatus(row, 'enabled')"
                      >{{ tr("enable") }}</Button
                    ><Button
                      v-if="row.status === 'enabled'"
                      size="small"
                      type="link"
                      @click="setPoolStatus(row, 'disabled')"
                      >{{ tr("disable") }}</Button
                    ><Button
                      v-if="row.status !== 'deleted'"
                      danger
                      size="small"
                      type="link"
                      @click="setPoolStatus(row, 'deleted')"
                      >{{ tr("delete") }}</Button
                    ><Button
                      v-else
                      size="small"
                      type="link"
                      @click="setPoolStatus(row, 'disabled')"
                      >{{ tr("restore") }}</Button
                    >
                  </div></template
                >
              </PoolGrid>
            </div>
          </div>
        </Tabs.TabPane>
      </Tabs>
    </Card>
    <ChannelModal :title="tr(editingChannel ? 'editChannel' : 'newChannel')"
      ><ChannelForm
    /></ChannelModal>
    <PoolModal :title="tr(editingPool ? 'editPool' : 'newPool')"
      ><PoolForm
    /></PoolModal>
    <DetailDrawer :title="tr('detailTitle')" class="w-[min(900px,95vw)]">
      <template v-if="detail">
        <Tabs v-model:active-key="detailTab" destroy-inactive-tab-pane>
          <Tabs.TabPane key="overview" :tab="tr('details')">
        <Alert class="mb-4" type="info" show-icon :message="tr('usageHint')" />
        <Descriptions bordered :column="1" size="small">
          <Descriptions.Item :label="tr('channel')">{{ detail.displayName }}</Descriptions.Item>
          <Descriptions.Item :label="tr('connector')">{{ connectorDisplayName(detail) }}</Descriptions.Item>
          <Descriptions.Item :label="tr('endpoint')">{{ detail.endpoint }}</Descriptions.Item>
          <Descriptions.Item :label="tr('runtimeStatus')">{{ statusLabel(detail.status) }}</Descriptions.Item>
          <Descriptions.Item :label="tr('connectionStatus')">{{ connectionStatusLabel(detail) }}</Descriptions.Item>
          <Descriptions.Item :label="tr('apiProducts')">{{ detail.usage ? `${detail.usage.activeInstances} / ${detail.usage.totalInstances}` : "—" }}</Descriptions.Item>
          <Descriptions.Item :label="tr('configuredProducts')">{{ detail.usage?.configuredProducts ?? "—" }}</Descriptions.Item>
        </Descriptions>
          </Tabs.TabPane>
          <Tabs.TabPane v-if="detail.catalogAvailable && detail.status === 'enabled' && detail.interfaceInstanceNo" key="catalog" :tab="$t('plugin.idc-resource.providerCatalog.title')">
            <ProviderCatalog :key="detail.interfaceInstanceNo" :interface-no="detail.interfaceInstanceNo" />
          </Tabs.TabPane>
        </Tabs>
      </template>
    </DetailDrawer>
  </Page>
</template>

<style scoped>
:deep(.provider-configuration-tabs > .ant-tabs-content-holder),
:deep(
  .provider-configuration-tabs > .ant-tabs-content-holder > .ant-tabs-content
),
:deep(.provider-configuration-tabs .ant-tabs-tabpane-active) {
  height: 100%;
  min-height: 0;
}
</style>
