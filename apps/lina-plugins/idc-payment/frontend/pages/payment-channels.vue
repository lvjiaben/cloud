<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/payment-channels",
  title: "Payment channels",
};
</script>

<script setup lang="ts">
import PaymentChannelModes from './components/payment-channel-modes.vue';
import PaymentPublicAddresses from './components/payment-public-addresses.vue';
import "../../../vben-adapter/frontend/admin.css";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import type {
  PaymentChannel,
  PaymentChannelCreateInput,
  PaymentConfigurationField,
  PaymentProviderCandidate,
} from "./payment-client";
import type { VbenFormProps } from "#/adapter/form";
import type { VxeGridProps } from "#/adapter/vxe-table";

import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";

import { Page, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";

import { Alert, Button, message, Space, Switch, Tag } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import { fixedRightActionColumn } from "../../../vben-adapter/frontend/src";
import { usePaymentPage } from "../src/payment-page";

import {
  createPaymentChannelConfiguration,
  getPaymentChannelConfigurations,
  setPaymentChannelStatus,
  verifyPaymentChannelConnection,
} from "./payment-client";

type ProviderCode = PaymentChannel["providerCode"];
type ConfigurationValue = boolean | string;

const providerCandidates = ref<PaymentProviderCandidate[]>([]);
const directory = usePaymentPage<PaymentChannel, { providerCandidates: PaymentProviderCandidate[] }>(async (page, pageSize) => {
  const result = await getPaymentChannelConfigurations(page, pageSize);
  return { items: result.channels || [], total: result.total, providerCandidates: result.providerCandidates || [] };
}, () => message.error($t("plugin.idc-payment.loadFailed")), () => {
  providerCandidates.value = [];
  clearConfiguration();
  paymentModalApi.close();
}, result => { providerCandidates.value = result.providerCandidates; });
const loading = computed(() => directory.state.loading);
const changing = ref("");
const testing = ref("");
const configuration = reactive<Record<string, ConfigurationValue>>({});
const form = reactive({
  channelNo: "",
  displayName: "",
  enabledModes: [] as string[],
  providerCode: "" as ProviderCode | "",
});

const selectedProvider = computed(() =>
  providerCandidates.value.find(
    (item) => item.providerCode === form.providerCode,
  ),
);
// Provider declaration keys are normalized by the bridge; the channel API uses
// fixed camel-case DTO properties. Keep the form and submit payload aligned.
const configurationInputKeys: Record<string, string> = {
  endpointorigin: "endpointOrigin", merchantid: "merchantId", appid: "appId",
  certificateserialno: "certificateSerialNo", privatekey: "privateKey",
  wechatpaypublickeyid: "wechatPayPublicKeyId", publickey: "publicKey",
  apiv3key: "apiV3Key", jsapiauthorized: "jsapiAuthorized", secret: "secret",
};
const configurationFields = computed(() =>
  (selectedProvider.value?.interfaceConfiguration?.fields || []).map(field => ({
    ...field, key: configurationInputKeys[field.key.toLowerCase()] || field.key,
  })),
);
const providerOptions = computed(() =>
  providerCandidates.value.map((item) => ({
    label: item.displayName || providerLabel(item.providerCode),
    value: item.providerCode,
  })),
);
const modeOptions = computed(() =>
  (selectedProvider.value?.modes || []).map((value) => ({
    label: $t(`plugin.idc-payment.channels.mode.${value}`),
    value,
  })),
);

function columns(): VxeGridProps<PaymentChannel>["columns"] {
  return [
    {
      field: "displayName",
      title: $t("plugin.idc-payment.channels.table.name"),
      width: 190,
    },
    {
      field: "providerCode",
      slots: { default: "provider" },
      title: $t("plugin.idc-payment.channels.table.provider"),
      width: 145,
    },
    {
      field: "merchantReference",
      title: $t("plugin.idc-payment.channels.table.merchant"),
      width: 165,
    },
    {
      field: "modes",
      slots: { default: "modes" },
      title: $t("plugin.idc-payment.channels.table.modes"),
      width: 250,
    },
    {
      field: "verifiedAt",
      slots: { default: "verifiedAt" },
      title: $t("plugin.idc-payment.channels.table.verifiedAt"),
      width: 180,
    },
    {
      field: "statusCode",
      slots: { default: "status" },
      title: $t("plugin.idc-payment.channels.table.status"),
      width: 110,
    },
    fixedRightActionColumn({
      field: "actions",
      slots: { default: "actions" },
      title: $t("plugin.idc-payment.channels.table.actions"),
      width: 280,
    }),
  ];
}

const [Grid, gridApi] = useVbenVxeGrid({
  gridEvents: {
    pageChange: ({ currentPage, pageSize }: {currentPage: number; pageSize: number}) => void directory.load(currentPage, pageSize),
    toolbarButtonClick: ({ code }: {code: string}) => { if (code === "refresh") void loadChannels(); },
  },
  gridOptions: {
    columns: columns(),
    data: [],
    height: "100%",
    pagerConfig: { enabled: true, currentPage: 1, pageSize: 20, pageSizes: [10, 20, 50, 100], total: 0 },
    rowConfig: { keyField: "channelNo" },
    showOverflow: true,
    toolbarConfig: { custom: true, refresh: false, zoom: true },
    id: "idc-payment-channels",
  },
});

const [PaymentForm, paymentFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 120 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1 md:grid-cols-2 gap-x-4",
});

const [PaymentModal, paymentModalApi] = useVbenModal({
  onCancel: () => paymentModalApi.close(),
  onConfirm: submit,
  onOpenChange: async (open) => {
    if (open) await syncPaymentForm();
  },
});

function providerLabel(value: string) {
  return $t(`plugin.idc-payment.channels.provider.${value}`);
}

function localizedLabel(field: PaymentConfigurationField) {
  const labels = field.label;
  if (preferences.app.locale === "zh-CN") {
    return labels?.zhCN || labels?.enUS || field.key;
  }
  return labels?.enUS || labels?.zhCN || field.key;
}

function candidateLabel(
  candidate: NonNullable<PaymentConfigurationField["staticCandidates"]>[number],
) {
  if (preferences.app.locale === "zh-CN") {
    return candidate.label?.zhCN || candidate.label?.enUS || candidate.value;
  }
  return candidate.label?.enUS || candidate.label?.zhCN || candidate.value;
}

function configurationRule(field: PaymentConfigurationField) {
  if (field.type === "boolean") return z.boolean();
  const maximum = field.maximumLength || 65_535;
  const rule = z.string().max(maximum);
  return field.required ? rule.trim().min(1) : rule.optional();
}

function paymentFormSchema(): VbenFormProps["schema"] {
  const fields: VbenFormProps["schema"] = [
    {
      component: "Select",
      componentProps: {
        onChange: handleProviderChange,
        options: providerOptions.value,
      },
      fieldName: "providerCode",
      label: $t("plugin.idc-payment.channels.form.provider"),
      rules: z.string().min(1),
    },
    {
      component: "Input",
      componentProps: { maxlength: 128 },
      fieldName: "displayName",
      label: $t("plugin.idc-payment.channels.form.name"),
      rules: z.string().trim().min(1).max(128),
    },
    {
      component: "Input",
      componentProps: { maxlength: 64, "data-testid": "payment-channel-number" },
      fieldName: "channelNo",
      label: $t("plugin.idc-payment.channels.form.channelNo"),
      rules: z.string().trim().min(1).max(64),
    },
    {
      component: "Select",
      componentProps: { mode: "multiple", options: modeOptions.value },
      fieldName: "enabledModes",
      formItemClass: "md:col-span-2",
      label: $t("plugin.idc-payment.channels.form.modes"),
      rules: z.array(z.string()),
    },
  ];
  for (const field of configurationFields.value) {
    const isLargeSecret =
      field.type === "secret" && (field.maximumLength || 0) > 8192;
    fields.push({
      component:
        field.type === "boolean"
          ? "Switch"
          : field.type === "single_select"
            ? "Select"
            : isLargeSecret
              ? "Textarea"
              : field.type === "secret"
                ? "InputPassword"
                : "Input",
      componentProps: {
        "data-testid": `payment-channel-${field.key}`,
        ...(field.type === "single_select"
          ? {
              options: (field.staticCandidates || []).map((item) => ({
                label: candidateLabel(item),
                value: item.value,
              })),
            }
          : isLargeSecret
            ? {
                autocomplete: "new-password",
                class: "secret-textarea",
                maxlength: field.maximumLength,
                rows: 5,
              }
            : {
                autocomplete: field.type === "secret" ? "new-password" : "off",
                maxlength: field.maximumLength,
              }),
      },
      fieldName: field.key,
      formItemClass: isLargeSecret ? "md:col-span-2" : undefined,
      label: localizedLabel(field),
      rules: configurationRule(field),
    });
  }
  return fields;
}

function paymentFormValues() {
  return {
    ...configuration,
    channelNo: form.channelNo,
    displayName: form.displayName,
    enabledModes: [...form.enabledModes],
    providerCode: form.providerCode,
  };
}

async function syncPaymentForm() {
  paymentFormApi.setState({ schema: paymentFormSchema() });
  await paymentFormApi.setValues(paymentFormValues());
}

function clearConfiguration() {
  for (const key of Object.keys(configuration)) delete configuration[key];
}

function resetForm(providerCode?: ProviderCode) {
  const candidate =
    providerCandidates.value.find(
      (item) => item.providerCode === providerCode,
    ) || providerCandidates.value[0];
  clearConfiguration();
  form.channelNo = "";
  form.displayName = candidate?.displayName || "";
  form.providerCode = candidate?.providerCode || "";
  form.enabledModes = (candidate?.modes || []).filter(
    (mode) => mode !== "wechat_jsapi",
  );
  for (const field of configurationFields.value) {
    configuration[field.key] = field.type === "boolean" ? false : "";
  }
  if (candidate && "endpointOrigin" in configuration) {
    configuration.endpointOrigin = candidate.endpointOrigin;
  }
}

async function handleProviderChange(value: ProviderCode) {
  resetForm(value);
  await syncPaymentForm();
}

async function openCreate() {
  resetForm();
  paymentModalApi.setState({
    title: $t("plugin.idc-payment.channels.form.title"),
  });
  paymentModalApi.open();
}

async function loadChannels() {
  await directory.load();
}

async function submit() {
  const validation = await paymentFormApi.validate();
  if (!validation.valid) {
    message.warning($t("plugin.idc-payment.channels.messages.required"));
    return;
  }
  const values = await paymentFormApi.getValues();
  const candidate = providerCandidates.value.find(
    (item) => item.providerCode === values.providerCode,
  );
  if (!candidate) return;
  const payload: PaymentChannelCreateInput = {
    apiV3Key: String(values.apiV3Key || ""),
    appId: String(values.appId || ""),
    certificateSerialNo: String(values.certificateSerialNo || ""),
    channelNo: String(values.channelNo).trim(),
    displayName: String(values.displayName).trim(),
    enabledModes: Array.isArray(values.enabledModes)
      ? values.enabledModes.map(String)
      : [],
    endpointOrigin: String(values.endpointOrigin || candidate.endpointOrigin),
    jsapiAuthorized: Boolean(values.jsapiAuthorized),
    merchantId: String(values.merchantId || ""),
    merchantReference: String(values.merchantId || ""),
    privateKey: String(values.privateKey || ""),
    providerCode: candidate.providerCode,
    publicKey: String(values.publicKey || ""),
    secret: String(values.secret || ""),
    wechatPayPublicKeyId: String(values.wechatPayPublicKeyId || ""),
  };
  paymentModalApi.setState({ confirmLoading: true });
  try {
    await createPaymentChannelConfiguration(payload);
    message.success($t("plugin.idc-payment.channels.messages.created"));
    paymentModalApi.close();
    await loadChannels();
  } finally {
    paymentModalApi.setState({ confirmLoading: false });
  }
}

async function testConnection(channel: PaymentChannel) {
  testing.value = channel.channelNo;
  try {
    await verifyPaymentChannelConnection(channel);
    message.success($t("plugin.idc-payment.channels.messages.verified"));
    await loadChannels();
  } finally {
    testing.value = "";
  }
}

async function toggleChannel(channel: PaymentChannel, enabled: boolean) {
  if (enabled && !channel.verifiedAt) {
    message.warning($t("plugin.idc-payment.channels.messages.verifyFirst"));
    return;
  }
  changing.value = channel.channelNo;
  try {
    await setPaymentChannelStatus(channel, enabled ? "enabled" : "disabled");
    message.success($t("plugin.idc-payment.channels.messages.statusSaved"));
    await loadChannels();
  } finally {
    changing.value = "";
  }
}

watch(() => ({ ...directory.state }), (state) => gridApi.setGridOptions({
  data: state.items, loading: state.loading,
  pagerConfig: { currentPage: state.currentPage, pageSize: state.pageSize, total: state.total },
}));
watch(
  () => preferences.app.locale,
  () => gridApi.setGridOptions({ columns: columns() }),
);

onMounted(loadChannels);
onBeforeUnmount(() => directory.invalidate());
const layoutRoot = useAdminGridLayout(gridApi);
</script>

<template>
  <Page class="plugin-admin-page" :auto-content-height="true">
    <div :ref="layoutRoot" class="channel-page">

      <Alert
        v-if="providerCandidates.length === 0"
        show-icon
        type="warning"
        :message="$t('plugin.idc-payment.channels.noProviderTitle')"
        :description="$t('plugin.idc-payment.channels.noProviderDescription')"
      />

      <div class="table-surface" data-testid="payment-channels-table">
        <Grid>
          <template #toolbar-tools>
            <PaymentPublicAddresses />
            <Button :loading="loading" @click="loadChannels">{{ $t("plugin.idc-payment.refresh") }}</Button>
            <Button
              type="primary"
              :disabled="providerCandidates.length === 0"
              @click="openCreate"
            >
              {{ $t("plugin.idc-payment.channels.actions.add") }}
            </Button>
          </template>
          <template #provider="{ row }">
            {{ providerLabel(row.providerCode) }}
          </template>
          <template #modes="{ row }">
            <Space wrap :size="4">
              <Tag v-for="mode in row.modes" :key="mode">
                {{ $t(`plugin.idc-payment.channels.mode.${mode}`) }}
              </Tag>
            </Space>
          </template>
          <template #verifiedAt="{ row }">
            <span v-if="row.verifiedAt">
              {{ formatTimestamp(row.verifiedAt) }}
            </span>
            <Tag v-else color="warning">
              {{ $t("plugin.idc-payment.channels.notVerified") }}
            </Tag>
          </template>
          <template #status="{ row }">
            <Tag :color="row.statusCode === 'enabled' ? 'success' : 'default'">
              {{ $t(`plugin.idc-payment.channels.status.${row.statusCode}`) }}
            </Tag>
          </template>
          <template #actions="{ row }">
            <Space>
              <PaymentChannelModes :channel="row" :available-modes="providerCandidates.find(item => item.providerCode === row.providerCode)?.modes || []" @updated="loadChannels" />
              <Button
                size="small"
                :loading="testing === row.channelNo"
                @click="testConnection(row)"
              >
                {{ $t("plugin.idc-payment.channels.actions.test") }}
              </Button>
              <Switch
                :checked="row.statusCode === 'enabled'"
                :disabled="!row.verifiedAt && row.statusCode !== 'enabled'"
                :loading="changing === row.channelNo"
                @change="(value) => toggleChannel(row, Boolean(value))"
              />
            </Space>
          </template>
        </Grid>
      </div>
    </div>

    <PaymentModal class="w-[min(96vw,760px)]">
      <Alert
        class="mb-4"
        show-icon
        type="warning"
        :message="$t('plugin.idc-payment.channels.form.verifyNotice')"
      />
      <Alert
        v-if="form.providerCode === 'epay'"
        class="mb-4"
        show-icon
        type="info"
        :message="$t('plugin.idc-payment.channels.form.epayVerifyNotice')"
      />
      <PaymentForm />
    </PaymentModal>
  </Page>
</template>

<style scoped>
.channel-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  min-width: 0;
  gap: 12px;
}

.page-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.page-heading h1 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  line-height: 28px;
}

.page-heading p {
  margin: 4px 0 0;
  color: hsl(var(--muted-foreground));
}

.table-surface {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid hsl(var(--border));
  background: hsl(var(--card));
}

.secret-textarea {
  -webkit-text-security: disc;
}

:deep(.ant-alert),
:deep(.ant-btn),
:deep(.ant-input),
:deep(.ant-input-affix-wrapper),
:deep(.ant-select-selector),
:deep(.vxe-grid) {
  border-radius: 0 !important;
}
</style>
