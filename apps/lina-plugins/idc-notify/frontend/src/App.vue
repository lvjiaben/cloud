<script setup lang="ts">
import {
  computed,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import type { PluginHostMountContext } from "./host-context";
import type {
  Channel,
  ChannelTestResult,
  Delivery,
  Eligibility,
  EventItem,
  EventPolicy,
  TemplateItem,
} from "./notification/api";
import { createNotificationAPI } from "./notification/api";
import { messages } from "./notification/messages";
import { createTranslator } from "./runtime/i18n";
import { PluginRequestError } from "./runtime/request";

type MappingRow = { key: string; variable: string };
type TemplateEditor = {
  templateNo: string;
  version: number;
  eventCode: string;
  channelCode: string;
  locale: string;
  subject: string;
  body: string;
  status: string;
  smsProviderCode: string;
  providerTemplateId: string;
  smsVariableOrder: string[];
  smsDataRows: MappingRow[];
};

const props = defineProps<{ context: PluginHostMountContext }>();
const api = createNotificationAPI(() => props.context);
const tr = createTranslator(() => props.context, messages);
const page = computed(() =>
  props.context.routePath.includes("/templates")
      ? "templates"
      : props.context.routePath.includes("/deliveries")
        ? "deliveries"
        : "channels",
);
const loading = ref(false);
const loadingMore = ref(false);
const error = ref("");
const success = ref("");
const channels = ref<Channel[]>([]);
const events = ref<EventItem[]>([]);
const eventPolicies = ref<EventPolicy[]>([]);
const templates = ref<TemplateItem[]>([]);
const deliveries = ref<Delivery[]>([]);
const nextCursor = ref("");
const selectedDelivery = ref<Delivery | null>(null);
const eligibility = ref<Eligibility | null>(null);
const checking = ref(false);
const actionReason = ref("");
const preview = ref<{ subject: string; body: string } | null>(null);
const activeTemplateSection = ref<"templates" | "policies">("templates");
const activeTemplateChannel = ref("all");
const editor = reactive<TemplateEditor>({
  templateNo: "",
  version: 0,
  eventCode: "",
  channelCode: "email",
  locale: "zh-CN",
  subject: "",
  body: "",
  status: "draft",
  smsProviderCode: "",
  providerTemplateId: "",
  smsVariableOrder: [],
  smsDataRows: [],
});
const selectedChannel = ref<Channel | null>(null);
const channelPanel = ref<"configuration" | "test">("configuration");
const config = reactive({
  host: "",
  port: 465,
  security: "implicit_tls",
  username: "",
  password: "",
  fromAddress: "",
  fromName: "",
  smsProvider: "tencent_cloud_sms_v20210111",
  region: "ap-guangzhou",
  smsSdkAppId: "",
  signName: "",
  secretId: "",
  secretKey: "",
  accessKeyId: "",
  accessKeySecret: "",
  smsBaoUsername: "",
  smsBaoApiKey: "",
  productId: "",
});
const testForm = reactive({
  target: "",
  templateNo: "",
  templateVersion: 0,
  variables: {} as Record<string, string>,
});
const testResult = ref<ChannelTestResult | null>(null);
const cooldownClock = ref(Date.now());
const eligibilityObservedAt = ref(0);
let cooldownTimer: number | undefined;
let deliverySelectionGeneration = 0;
let eligibilityRequestGeneration = 0;
let lastCooldownRefreshKey = "";

const title = computed(() =>
  tr(`title${page.value[0].toUpperCase()}${page.value.slice(1)}`),
);
const subtitle = computed(() =>
  tr(`subtitle${page.value[0].toUpperCase()}${page.value.slice(1)}`),
);
const selectedEvent = computed(() =>
  events.value.find((item) => item.code === editor.eventCode),
);
const editorVariables = computed(() => [
  ...(selectedEvent.value?.requiredVariables ?? []),
  ...(selectedEvent.value?.optionalVariables ?? []),
]);
const activeSMSChannel = computed(() =>
  channels.value.find((item) => item.code === "sms"),
);
const testTemplates = computed(() =>
  templates.value.filter(
    (item) =>
      item.channelCode === selectedChannel.value?.code &&
      item.status === "active" &&
      (item.channelCode !== "sms" ||
        item.sms?.providerCode === selectedChannel.value?.providerCode),
  ),
);
// The filter is a channel directory, not a summary of templates that already exist.
// Keeping every channel visible lets operators create the first template for a channel
// without the tab mysteriously disappearing when its template list is empty.
const templateChannels = computed(() => [
  ...new Set(channels.value.map((item) => item.code)),
]);
const visibleTemplates = computed(() =>
  activeTemplateChannel.value === "all"
    ? templates.value
    : templates.value.filter(
        (item) => item.channelCode === activeTemplateChannel.value,
      ),
);
const configurableChannelCodes = new Set([
  "email",
  "sms",
]);
const resendCooldownRemainingSeconds = computed(() => {
  const current = eligibility.value;
  if (!current?.nextAllowedAt || !eligibilityObservedAt.value) return 0;
  const elapsed = Math.max(
    0,
    cooldownClock.value - eligibilityObservedAt.value,
  );
  const authorityNow = current.evaluatedAt + elapsed;
  return Math.max(0, Math.ceil((current.nextAllowedAt - authorityNow) / 1000));
});
const resendCooldownLabel = computed(() => {
  const seconds = resendCooldownRemainingSeconds.value;
  return tr("cooldownRemaining", {
    minutes: Math.floor(seconds / 60),
    seconds: seconds % 60,
  });
});
function formatTime(value?: number) {
  return value
    ? new Intl.DateTimeFormat(props.context.locale, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(value))
    : "—";
}
function statusLabel(value: string) {
  const key = `status_${value}`;
  return key in messages ? tr(key) : tr("statusUnknown");
}
function resultLabel(value: string) {
  const key = `result_${value}`;
  return key in messages ? tr(key) : statusLabel(value);
}
function channelLabel(value: string, displayName = "") {
  const key = `channel_${value}`;
  return key in messages ? tr(key) : displayName || tr("channelUnknown");
}
function eventLabel(value: string) {
  const key = `event_${value.replaceAll(".", "_")}`;
  return key in messages ? tr(key) : tr("eventUnknown");
}
function channelDisplayName(code: string) {
  const item = channels.value.find((channel) => channel.code === code);
  return channelLabel(code, item?.displayName);
}
function providerName(code: string) {
  const key = `provider_${code}`;
  return key in messages ? tr(key) : tr("providerUnknown");
}
function providerLabel(item: Channel) {
  if (!item.configured) return "—";
  const saved = item.credentialMaskedTail
    ? tr("providerCredentialTail", { tail: item.credentialMaskedTail })
    : tr("providerConfigured");
  return item.providerCode
    ? `${providerName(item.providerCode)} · ${saved}`
    : saved;
}
function recipientLabel(item: Delivery) {
  return item.recipientMask || "—";
}
function reasonLabel(value: string) {
  const key = `reason_${value}`;
  return key in messages ? tr(key) : tr("reason_unavailable");
}
function variableLabel(value: string) {
  const key = `variable_${value}`;
  return key in messages ? tr(key) : tr("variableUnknown");
}
function configurableChannel(item: Channel) {
  return configurableChannelCodes.has(item.code);
}
function requiredTestVariable(key: string) {
  const template = templates.value.find(
    (item) =>
      item.templateNo === testForm.templateNo &&
      item.version === testForm.templateVersion,
  );
  const event = events.value.find((item) => item.code === template?.eventCode);
  return event?.requiredVariables.includes(key) ?? false;
}
function clearNotices() {
  error.value = "";
  success.value = "";
}
function accessWasRevoked(value: unknown) {
  return (
    value instanceof PluginRequestError &&
    [401, 403, 404].includes(value.status)
  );
}

function resetDeliverySelection() {
  deliverySelectionGeneration++;
  eligibilityRequestGeneration++;
  selectedDelivery.value = null;
  eligibility.value = null;
  checking.value = false;
  eligibilityObservedAt.value = 0;
}

async function load() {
  loading.value = true;
  clearNotices();
  try {
    if (page.value === "channels") {
      const [channelPage, eventPage, templatePage] = await Promise.all([
        api.channels(),
        api.events(),
        api.templates(),
      ]);
      channels.value = channelPage.items;
      events.value = eventPage.items;
      templates.value = templatePage.items;
    }
    if (page.value === "templates") {
      const [channelPage, eventPage, policyPage, templatePage] =
        await Promise.all([
          api.channels(),
          api.events(),
          api.eventPolicies(),
          api.templates(),
        ]);
      channels.value = channelPage.items;
      events.value = eventPage.items;
      eventPolicies.value = policyPage.items;
      templates.value = templatePage.items;
      if (!editor.eventCode && events.value[0])
        editor.eventCode = events.value[0].code;
    }
    if (page.value === "deliveries") {
      const [channelPage, deliveryPage] = await Promise.all([
        api.channels(),
        api.deliveries(),
      ]);
      channels.value = channelPage.items;
      deliveries.value = deliveryPage.items;
      nextCursor.value = deliveryPage.nextCursor;
      resetDeliverySelection();
    }
  } catch {
    error.value = tr("reason_unavailable");
  } finally {
    loading.value = false;
  }
}

async function loadMoreDeliveries() {
  if (!nextCursor.value || loadingMore.value) return;
  loadingMore.value = true;
  clearNotices();
  try {
    const result = await api.deliveries(nextCursor.value);
    deliveries.value.push(...result.items);
    nextCursor.value = result.nextCursor;
  } catch {
    error.value = tr("reason_unavailable");
  } finally {
    loadingMore.value = false;
  }
}
async function channelAction(
  item: Channel,
  action: "clear" | "disable" | "enable",
) {
  if (!confirm(tr("confirmAction"))) return;
  loading.value = true;
  clearNotices();
  try {
    const response =
      action === "disable"
        ? await api.disableChannel(item)
        : action === "enable"
          ? await api.enableChannel(item)
          : await api.clearChannel(item);
    const index = channels.value.findIndex(
      (channel) => channel.code === item.code,
    );
    if (index >= 0) channels.value[index] = response.item;
    if (selectedChannel.value?.code === item.code)
      selectedChannel.value = response.item;
    success.value = tr("saved");
  } catch {
    error.value = tr("reason_unavailable");
  } finally {
    loading.value = false;
  }
}

function openConfiguration(item: Channel) {
  selectedChannel.value = item;
  channelPanel.value = "configuration";
  testResult.value = null;
  clearNotices();
  Object.assign(config, {
    host: "",
    port: 465,
    security: "implicit_tls",
    username: "",
    password: "",
    fromAddress: "",
    fromName: "",
    smsProvider:
      item.code === "sms" && item.providerCode
        ? item.providerCode
        : "tencent_cloud_sms_v20210111",
    region: "ap-guangzhou",
    smsSdkAppId: "",
    signName: "",
    secretId: "",
    secretKey: "",
    accessKeyId: "",
    accessKeySecret: "",
    smsBaoUsername: "",
    smsBaoApiKey: "",
    productId: "",
  });
  if (item.configuration.smtp) Object.assign(config, item.configuration.smtp);
  if (item.configuration.aliyunSms)
    Object.assign(config, item.configuration.aliyunSms);
  if (item.configuration.smsBao)
    Object.assign(config, item.configuration.smsBao);
  if (item.configuration.tencentSms)
    Object.assign(config, item.configuration.tencentSms);
}

function clearSMSSecrets() {
  config.secretId = "";
  config.secretKey = "";
  config.accessKeyId = "";
  config.accessKeySecret = "";
  config.smsBaoUsername = "";
  config.smsBaoApiKey = "";
}

function onSMSProviderChange() {
  clearSMSSecrets();
  config.signName = "";
  config.productId = "";
  config.smsSdkAppId = "";
  config.region = "ap-guangzhou";
}

async function saveSMSConfiguration(item: Channel) {
  if (config.smsProvider === "aliyun_sms_v20170525") {
    return api.configureAliyunSMS(item, {
      signName: config.signName,
      accessKeyId: config.accessKeyId,
      accessKeySecret: config.accessKeySecret,
    });
  }
  if (config.smsProvider === "smsbao_http_v1") {
    return api.configureSMSBao(item, {
      signName: config.signName,
      productId: config.productId,
      username: config.smsBaoUsername,
      apiKey: config.smsBaoApiKey,
    });
  }
  return api.configureTencentSMS(item, {
    region: config.region,
    smsSdkAppId: config.smsSdkAppId,
    signName: config.signName,
    secretId: config.secretId,
    secretKey: config.secretKey,
  });
}

async function saveConfiguration() {
  const item = selectedChannel.value;
  if (!item) return;
  loading.value = true;
  clearNotices();
  try {
    const response =
      item.code === "email"
        ? await api.configureSMTP(item, {
            host: config.host,
            port: Number(config.port),
            security: config.security,
            username: config.username,
            password: config.password,
            fromAddress: config.fromAddress,
            fromName: config.fromName,
          })
        : await saveSMSConfiguration(item);
    config.password = "";
    clearSMSSecrets();
    const index = channels.value.findIndex(
      (channel) => channel.code === item.code,
    );
    if (index >= 0) channels.value[index] = response.item;
    selectedChannel.value = response.item;
    success.value = tr("configurationSaved");
  } catch {
    config.password = "";
    clearSMSSecrets();
    error.value = tr("configurationFailed");
  } finally {
    loading.value = false;
  }
}

function openTest(item: Channel) {
  selectedChannel.value = item;
  channelPanel.value = "test";
  testResult.value = null;
  testForm.target = "";
  testForm.templateNo = "";
  testForm.templateVersion = 0;
  testForm.variables = {};
  clearNotices();
  const first = templates.value.find(
    (template) =>
      template.channelCode === item.code && template.status === "active",
  );
  if (first) selectTestTemplate(`${first.templateNo}:${first.version}`);
}
function selectTestTemplate(value: string) {
  const split = value.lastIndexOf(":");
  testForm.templateNo = value.slice(0, split);
  testForm.templateVersion = Number(value.slice(split + 1));
  const template = templates.value.find(
    (item) =>
      item.templateNo === testForm.templateNo &&
      item.version === testForm.templateVersion,
  );
  const event = events.value.find((item) => item.code === template?.eventCode);
  testForm.variables = Object.fromEntries(
    [
      ...(event?.requiredVariables ?? []),
      ...(event?.optionalVariables ?? []),
    ].map((key) => [key, ""]),
  );
}
async function sendTest() {
  const item = selectedChannel.value;
  if (
    !item ||
    !testForm.templateNo ||
    !testForm.target
  )
    return;
  const prompt =
    item.code === "sms"
      ? tr("confirmSmsTest")
      : tr("confirmRealTest");
  if (!confirm(prompt)) return;
  loading.value = true;
  clearNotices();
  try {
    testResult.value = (
            await api.testChannel(item, {
              target: testForm.target,
              templateNo: testForm.templateNo,
              templateVersion: testForm.templateVersion,
              variables: { ...testForm.variables },
            })
          ).item;
    testForm.target = "";
    success.value = tr("testCompleted");
  } catch {
    testForm.target = "";
    error.value = tr("testFailed");
  } finally {
    loading.value = false;
  }
}

function editTemplate(item: TemplateItem) {
  Object.assign(editor, {
    templateNo: item.templateNo,
    version: item.version,
    eventCode: item.eventCode,
    channelCode: item.channelCode,
    locale: item.locale,
    subject: item.subject,
    body: item.body,
    status: item.status,
    smsProviderCode: item.sms?.providerCode ?? "",
    providerTemplateId:
      item.sms?.templateId ?? "",
    smsVariableOrder: [...(item.sms?.variableOrder ?? [])],
    smsDataRows: Object.entries(item.sms?.dataMapping ?? {}).map(
      ([key, variable]) => ({ key, variable }),
    ),
  });
  preview.value = null;
}
function resetProviderMapping() {
  editor.smsProviderCode =
    editor.channelCode === "sms"
      ? (activeSMSChannel.value?.providerCode ?? "")
      : "";
  editor.providerTemplateId = "";
  editor.smsVariableOrder = [];
  editor.smsDataRows = [];
}
async function updateEventPolicy(item: EventPolicy) {
  clearNotices();
  try {
    const response = await api.updateEventPolicy(item, !item.enabled);
    const index = eventPolicies.value.findIndex(
      (policy) =>
        policy.eventCode === item.eventCode &&
        policy.recipientKind === item.recipientKind &&
        policy.channelCode === item.channelCode,
    );
    if (index >= 0) eventPolicies.value[index] = response.item;
    success.value = tr("eventPolicySaved");
  } catch {
    error.value = tr("reason_task_state_changed");
    await load();
  }
}
function addSMSVariable() {
  if (editorVariables.value[0])
    editor.smsVariableOrder.push(editorVariables.value[0]);
}
function addSMSDataMapping() {
  editor.smsDataRows.push({
    key: "",
    variable: editorVariables.value[0] ?? "",
  });
}
async function saveTemplate() {
  loading.value = true;
  clearNotices();
  try {
    const payload: Partial<TemplateItem> & TemplateEditor = {
      ...editor,
      smsVariableOrder: [...editor.smsVariableOrder],
      smsDataRows: editor.smsDataRows.map((row) => ({ ...row })),
    };
    if (editor.channelCode === "sms")
      payload.sms = {
        providerCode: editor.smsProviderCode,
        templateId: editor.providerTemplateId,
        variableOrder: [...editor.smsVariableOrder],
        dataMapping: Object.fromEntries(
          editor.smsDataRows.map((row) => [row.key, row.variable]),
        ),
      };
    await api.saveTemplate(payload);
    await load();
    success.value = tr("templateSaved");
  } catch {
    error.value = tr("reason_unavailable");
  } finally {
    loading.value = false;
  }
}
function previewVariables() {
  return Object.fromEntries(
    editorVariables.value.map((key) => [key, `[${variableLabel(key)}]`]),
  );
}
async function previewTemplate() {
  const current = templates.value.find(
    (item) =>
      item.templateNo === editor.templateNo && item.version === editor.version,
  );
  if (!current) return;
  try {
    preview.value = await api.previewTemplate(current, previewVariables());
  } catch {
    error.value = tr("reason_unavailable");
  }
}
async function openDelivery(item: Delivery) {
  clearNotices();
  const selection = ++deliverySelectionGeneration;
  eligibilityRequestGeneration++;
  // Switch the visible detail immediately. Keeping the previous detail on
  // screen lets an operator type into the wrong record while the new detail
  // request is still in flight, and that input would then be discarded.
  selectedDelivery.value = item;
  eligibility.value = null;
  eligibilityObservedAt.value = 0;
  actionReason.value = "";
  checking.value = true;
  try {
    const current = (await api.delivery(item.taskNo)).item;
    if (
      selection !== deliverySelectionGeneration ||
      page.value !== "deliveries"
    )
      return;
    selectedDelivery.value = current;
    checking.value = false;
    await checkEligibility(selection);
  } catch {
    if (
      selection !== deliverySelectionGeneration ||
      page.value !== "deliveries"
    )
      return;
    resetDeliverySelection();
    error.value = tr("reason_unavailable");
  }
}
async function checkEligibility(
  selection = deliverySelectionGeneration,
  preserveNotice = false,
) {
  const current = selectedDelivery.value;
  if (!current || checking.value || selection !== deliverySelectionGeneration)
    return;
  const taskNo = current.taskNo;
  const request = ++eligibilityRequestGeneration;
  checking.value = true;
  if (!preserveNotice) error.value = "";
  try {
    const result = (await api.eligibility(taskNo)).item;
    if (
      request !== eligibilityRequestGeneration ||
      selection !== deliverySelectionGeneration ||
      selectedDelivery.value?.taskNo !== taskNo ||
      page.value !== "deliveries"
    )
      return;
    eligibility.value = result;
    eligibilityObservedAt.value = Date.now();
    cooldownClock.value = eligibilityObservedAt.value;
  } catch (caught) {
    const isCurrent =
      request === eligibilityRequestGeneration &&
      selection === deliverySelectionGeneration &&
      selectedDelivery.value?.taskNo === taskNo &&
      page.value === "deliveries";
    if (isCurrent && accessWasRevoked(caught)) {
      resetDeliverySelection();
      error.value = tr("reason_unavailable");
      return;
    }
    if (isCurrent) {
      error.value = tr("eligibilityCheckFailed");
    }
  } finally {
    if (request === eligibilityRequestGeneration) checking.value = false;
  }
}
async function resend() {
  const source = selectedDelivery.value;
  if (!source || !actionReason.value.trim() || !confirm(tr("confirmAction")))
    return;
  try {
    const created = (await api.resend(source, actionReason.value)).item;
    await load();
    const selection = ++deliverySelectionGeneration;
    selectedDelivery.value = created;
    actionReason.value = "";
    await checkEligibility(selection);
  } catch {
    error.value = tr("reason_task_state_changed");
    await checkEligibility(deliverySelectionGeneration, true);
  }
}
async function closeUnknown() {
  const source = selectedDelivery.value;
  if (!source || !actionReason.value.trim() || !confirm(tr("confirmAction")))
    return;
  try {
    const closed = (await api.closeUnknown(source, actionReason.value)).item;
    await load();
    deliverySelectionGeneration++;
    selectedDelivery.value = closed;
    eligibility.value = null;
    eligibilityObservedAt.value = 0;
    actionReason.value = "";
  } catch {
    error.value = tr("reason_task_state_changed");
  }
}
watch(page, () => {
  resetDeliverySelection();
  void load();
});
watch(resendCooldownRemainingSeconds, (current, previous) => {
  const item = eligibility.value;
  if (
    previous <= 0 ||
    current > 0 ||
    !item?.nextAllowedAt ||
    page.value !== "deliveries"
  )
    return;
  const refreshKey = `${item.taskNo}:${item.nextAllowedAt}`;
  if (lastCooldownRefreshKey === refreshKey) return;
  lastCooldownRefreshKey = refreshKey;
  void checkEligibility();
});
onMounted(() => {
  void load();
  cooldownTimer = window.setInterval(() => {
    cooldownClock.value = Date.now();
  }, 1000);
});
onBeforeUnmount(() => {
  if (cooldownTimer !== undefined) window.clearInterval(cooldownTimer);
  cooldownTimer = undefined;
  resetDeliverySelection();
});
</script>

<template>
  <main class="notify-workspace">
    <header class="workspace-toolbar">
      <div>
        <h1>{{ title }}</h1>
        <p>{{ subtitle }}</p>
      </div>
      <button class="button secondary" :disabled="loading" @click="load">
        {{ tr("refresh") }}
      </button>
    </header>
    <p v-if="error" class="notice error" role="alert">{{ error }}</p>
    <p v-if="success" class="notice success" role="status">{{ success }}</p>
    <p v-if="loading" class="notice info">{{ tr("loading") }}</p>

    <section
      v-if="page === 'channels'"
      class="channel-layout"
      :class="{ 'single-column': !selectedChannel }"
    >
      <div class="panel">
        <p class="notice info">{{ tr("providerReady") }}</p>
        <div class="card-grid">
          <article
            v-for="item in channels"
            :key="item.code"
            class="channel-card"
          >
            <div class="card-heading">
              <div>
                <p class="eyebrow">{{ tr("notificationChannel") }}</p>
                <h2>{{ channelLabel(item.code, item.displayName) }}</h2>
              </div>
              <span class="status" :data-state="item.status">{{
                statusLabel(item.status)
              }}</span>
            </div>
            <dl>
              <div>
                <dt>{{ tr("configured") }}</dt>
                <dd>
                  {{ item.configured ? tr("configured") : tr("notConfigured") }}
                </dd>
              </div>
              <div>
                <dt>{{ tr("provider") }}</dt>
                <dd>{{ providerLabel(item) }}</dd>
              </div>
              <div>
                <dt>{{ tr("pending") }}</dt>
                <dd>{{ item.pendingQueryCount }}</dd>
              </div>
            </dl>
            <p
              v-if="!configurableChannel(item)"
              class="notice info"
            >
              {{ tr("channelAdapterPending") }}
            </p>
            <p v-if="item.pendingQueryCount > 0" class="notice warning">
              {{ tr("clearBlockedByPending") }}
            </p>
            <div class="actions">
              <button
                v-if="configurableChannel(item)"
                class="button primary"
                @click="openConfiguration(item)"
              >
                {{
                  item.configured ? tr("replaceConfiguration") : tr("configure")
                }}</button
              ><button
                v-if="
                  item.configured &&
                  ['email', 'sms'].includes(
                    item.code,
                  ) &&
                  item.status === 'enabled'
                "
                class="button secondary"
                @click="openTest(item)"
              >
                {{ tr("testSend") }}</button
              ><button
                v-if="item.status === 'enabled'"
                class="button secondary"
                @click="channelAction(item, 'disable')"
              >
                {{ tr("disable") }}</button
              ><button
                v-if="item.status === 'disabled'"
                class="button secondary"
                @click="channelAction(item, 'enable')"
              >
                {{ tr("enable") }}</button
              ><button
                v-if="item.status === 'disabled' && item.configured"
                class="button danger"
                :disabled="item.pendingQueryCount > 0"
                @click="channelAction(item, 'clear')"
              >
                {{ tr("clear") }}
              </button>
            </div>
          </article>
        </div>
      </div>
      <aside v-if="selectedChannel" class="panel channel-wizard">
        <div class="card-heading">
          <div>
            <p class="eyebrow">
              {{
                channelPanel === "configuration"
                  ? tr("configurationWizard")
                  : tr("testSend")
              }}
            </p>
            <h2>
              {{
                channelLabel(selectedChannel.code, selectedChannel.displayName)
              }}
            </h2>
          </div>
          <button
            class="icon-button"
            :aria-label="tr('close')"
            @click="selectedChannel = null"
          >
            ×
          </button>
        </div>
        <form
          v-if="channelPanel === 'configuration'"
          @submit.prevent="saveConfiguration"
        >
          <p v-if="selectedChannel.code === 'email'" class="notice warning">
            {{ tr("smtpTlsHelp") }}
          </p>
          <template v-if="selectedChannel.code === 'email'"
            ><div class="two">
              <label
                >{{ tr("smtpHost")
                }}<input
                  v-model.trim="config.host"
                  required
                  maxlength="253"
                  autocomplete="off" /></label
              ><label
                >{{ tr("smtpPort")
                }}<select v-model.number="config.port">
                  <option :value="465">465</option>
                  <option :value="587">587</option>
                </select></label
              >
            </div>
            <label
              >{{ tr("securityMode")
              }}<select v-model="config.security">
                <option value="implicit_tls">{{ tr("implicitTls") }}</option>
                <option value="starttls">{{ tr("startTls") }}</option>
              </select></label
            ><label
              >{{ tr("username")
              }}<input
                v-model.trim="config.username"
                required
                maxlength="320"
                autocomplete="off" /></label
            ><label
              >{{ tr("password")
              }}<input
                v-model="config.password"
                required
                type="password"
                maxlength="4096"
                autocomplete="new-password" /></label
            ><label
              >{{ tr("fromAddress")
              }}<input
                v-model.trim="config.fromAddress"
                required
                type="email"
                maxlength="320" /></label
            ><label
              >{{ tr("fromName")
              }}<input v-model.trim="config.fromName" maxlength="128" /></label
          ></template>
          <template v-else-if="selectedChannel.code === 'sms'">
            <p class="notice warning">{{ tr("smsProviderSwitchWarning") }}</p>
            <label
              >{{ tr("smsProvider")
              }}<select
                v-model="config.smsProvider"
                @change="onSMSProviderChange"
              >
                <option value="tencent_cloud_sms_v20210111">
                  {{ tr("provider_tencent_cloud_sms_v20210111") }}
                </option>
                <option value="aliyun_sms_v20170525">
                  {{ tr("provider_aliyun_sms_v20170525") }}
                </option>
                <option value="smsbao_http_v1">
                  {{ tr("provider_smsbao_http_v1") }}
                </option>
              </select></label
            >
            <template
              v-if="config.smsProvider === 'tencent_cloud_sms_v20210111'"
              ><label
                >{{ tr("tencentRegion")
                }}<input
                  v-model.trim="config.region"
                  required
                  maxlength="32" /></label
              ><label
                >{{ tr("smsSdkAppId")
                }}<input
                  v-model.trim="config.smsSdkAppId"
                  required
                  inputmode="numeric"
                  maxlength="20" /></label
              ><label
                >{{ tr("smsSignName")
                }}<input
                  v-model.trim="config.signName"
                  required
                  maxlength="64" /></label
              ><label
                >{{ tr("secretId")
                }}<input
                  v-model="config.secretId"
                  required
                  type="password"
                  maxlength="256"
                  autocomplete="new-password" /></label
              ><label
                >{{ tr("secretKey")
                }}<input
                  v-model="config.secretKey"
                  required
                  type="password"
                  maxlength="256"
                  autocomplete="new-password" /></label
            ></template>
            <template v-else-if="config.smsProvider === 'aliyun_sms_v20170525'"
              ><label
                >{{ tr("smsSignName")
                }}<input
                  v-model.trim="config.signName"
                  required
                  maxlength="64" /></label
              ><label
                >{{ tr("aliyunAccessKeyId")
                }}<input
                  v-model="config.accessKeyId"
                  required
                  type="password"
                  maxlength="256"
                  autocomplete="new-password" /></label
              ><label
                >{{ tr("aliyunAccessKeySecret")
                }}<input
                  v-model="config.accessKeySecret"
                  required
                  type="password"
                  maxlength="256"
                  autocomplete="new-password" /></label
            ></template>
            <template v-else
              ><p class="notice info">{{ tr("smsBaoTemplateHelp") }}</p>
              <label
                >{{ tr("smsSignName")
                }}<input
                  v-model.trim="config.signName"
                  required
                  minlength="3"
                  maxlength="10" /></label
              ><label
                >{{ tr("smsBaoProductId")
                }}<input
                  v-model.trim="config.productId"
                  inputmode="numeric"
                  maxlength="32" /></label
              ><label
                >{{ tr("smsBaoUsername")
                }}<input
                  v-model="config.smsBaoUsername"
                  required
                  type="password"
                  maxlength="128"
                  autocomplete="new-password" /></label
              ><label
                >{{ tr("smsBaoApiKey")
                }}<input
                  v-model="config.smsBaoApiKey"
                  required
                  type="password"
                  maxlength="256"
                  autocomplete="new-password" /></label
            ></template>
          </template>
          <button class="button primary" type="submit" :disabled="loading">
            {{ tr("saveConfiguration") }}
          </button>
        </form>
        <form v-else @submit.prevent="sendTest">
          <p class="notice warning">
            {{
              selectedChannel.code === "sms"
                ? tr("smsCostWarning")
                : tr("realDeliveryWarning")
            }}
          </p>
          <label
            >{{ tr("testTemplate")
            }}<select
              :value="
                testForm.templateNo
                  ? `${testForm.templateNo}:${testForm.templateVersion}`
                  : ''
              "
              required
              @change="
                selectTestTemplate(($event.target as HTMLSelectElement).value)
              "
            >
              <option value="" disabled>{{ tr("chooseTemplate") }}</option>
              <option
                v-for="item in testTemplates"
                :key="`${item.templateNo}:${item.version}`"
                :value="`${item.templateNo}:${item.version}`"
              >
                {{ eventLabel(item.eventCode) }} ·
                {{
                  item.locale === "zh-CN" ? tr("localeZhCN") : tr("localeEnUS")
                }}
                · v{{ item.version }}
              </option>
            </select></label
          ><label
            >{{ tr(`testTarget_${selectedChannel.code}`)
            }}<input
              v-model="testForm.target"
              required
              maxlength="512"
              autocomplete="off" /></label
          ><label v-for="(_, key) in testForm.variables" :key="key"
            >{{ variableLabel(key)
            }}<span v-if="!requiredTestVariable(key)"
              >（{{ tr("optional") }}）</span
            ><input
              v-model="testForm.variables[key]"
              :required="requiredTestVariable(key)"
              maxlength="4096"
              autocomplete="off" /></label
          ><button
            class="button danger"
            type="submit"
            :disabled="loading || !testTemplates.length"
          >
            {{ tr("sendRealTest") }}
          </button>
          <p v-if="!testTemplates.length" class="notice info">
            {{ tr("activeTemplateRequired") }}
          </p>
          <div v-if="testResult" class="result-card">
            <strong>{{ resultLabel(testResult.resultClass) }}</strong>
            <p>
              {{ tr("maskedTarget") }}：{{
                testResult.targetMask
              }}
            </p>
          </div>
        </form>
      </aside>
    </section>

    <section
      v-else-if="page === 'templates'"
      class="split"
      :class="{ 'single-column': activeTemplateSection === 'policies' }"
    >
      <div class="panel">
        <div
          class="template-tabs workspace-tabs"
          role="tablist"
          :aria-label="`${tr('titleTemplates')} / ${tr('eventPolicies')}`"
        >
          <button
            class="template-tab"
            :class="{ active: activeTemplateSection === 'templates' }"
            type="button"
            role="tab"
            :aria-selected="activeTemplateSection === 'templates'"
            @click="activeTemplateSection = 'templates'"
          >
            {{ tr("titleTemplates") }}</button
          ><button
            class="template-tab"
            :class="{ active: activeTemplateSection === 'policies' }"
            type="button"
            role="tab"
            :aria-selected="activeTemplateSection === 'policies'"
            @click="activeTemplateSection = 'policies'"
          >
            {{ tr("eventPolicies") }}
          </button>
        </div>
        <template v-if="activeTemplateSection === 'templates'">
          <div
            class="template-tabs"
            role="tablist"
            :aria-label="tr('titleTemplates')"
          >
            <button
              class="template-tab"
              :class="{ active: activeTemplateChannel === 'all' }"
              type="button"
              role="tab"
              :aria-selected="activeTemplateChannel === 'all'"
              @click="activeTemplateChannel = 'all'"
            >
              {{ tr("allChannels") }}</button
            ><button
              v-for="channel in templateChannels"
              :key="channel"
              class="template-tab"
              :class="{ active: activeTemplateChannel === channel }"
              type="button"
              role="tab"
              :aria-selected="activeTemplateChannel === channel"
              @click="activeTemplateChannel = channel"
            >
              {{ channelDisplayName(channel) }}
            </button>
          </div>
          <div class="template-scroll">
            <div
              v-for="item in visibleTemplates"
              :key="`${item.templateNo}-${item.version}`"
              class="row"
            >
              <div>
                <strong>{{ eventLabel(item.eventCode) }}</strong>
                <p>
                  {{ channelDisplayName(item.channelCode) }} ·
                  {{
                    item.locale === "zh-CN"
                      ? tr("localeZhCN")
                      : tr("localeEnUS")
                  }}
                  · v{{ item.version }}
                </p>
              </div>
              <button
                class="button secondary"
                type="button"
                @click="editTemplate(item)"
              >
                {{ tr("details") }}
              </button>
            </div>
            <p v-if="!visibleTemplates.length" class="empty">
              {{ tr("noData") }}
            </p>
          </div>
        </template>
        <template v-else>
          <p class="notice info">{{ tr("eventPolicyHelp") }}</p>
          <div class="policy-scroll">
            <div
              v-for="item in eventPolicies"
              :key="`${item.eventCode}-${item.recipientKind}-${item.channelCode}`"
              class="row"
            >
              <div>
                <strong>{{ eventLabel(item.eventCode) }}</strong>
                <p>
                  {{ tr(`recipient_${item.recipientKind}`) }} ·
                  {{ channelDisplayName(item.channelCode) }} · v{{
                    item.version
                  }}
                </p>
              </div>
              <button
                class="button secondary"
                type="button"
                :aria-pressed="item.enabled"
                @click="updateEventPolicy(item)"
              >
                {{ item.enabled ? tr("disable") : tr("enable") }}
              </button>
            </div>
          </div>
        </template>
      </div>
      <form
        v-if="activeTemplateSection === 'templates'"
        class="panel editor"
        @submit.prevent="saveTemplate"
      >
        <label
          >{{ tr("event")
          }}<select v-model="editor.eventCode">
            <option
              v-for="event in events"
              :key="event.code"
              :value="event.code"
            >
              {{ eventLabel(event.code) }}
            </option>
          </select></label
        >
        <div class="two">
          <label
            >{{ tr("channel")
            }}<select
              v-model="editor.channelCode"
              @change="resetProviderMapping"
            >
              <option
                v-for="item in channels"
                :key="item.code"
                :value="item.code"
              >
                {{ channelLabel(item.code, item.displayName) }}
              </option>
            </select></label
          ><label
            >{{ tr("locale")
            }}<select v-model="editor.locale">
              <option value="zh-CN">{{ tr("localeZhCN") }}</option>
              <option value="en-US">{{ tr("localeEnUS") }}</option>
            </select></label
          >
        </div>
        <label>{{ tr("subject") }}<input v-model="editor.subject" /></label
        ><label
          >{{ tr("body") }}<textarea v-model="editor.body" rows="8" required />
        </label>
        <fieldset v-if="editor.channelCode === 'sms'" class="mapping">
          <legend>{{ tr("smsTemplateMapping") }}</legend>
          <p v-if="!activeSMSChannel?.configured" class="notice warning">
            {{ tr("smsProviderRequired") }}
          </p>
          <p v-else class="notice info">
            {{ tr("currentSmsProvider") }}：{{
              providerName(
                editor.smsProviderCode || activeSMSChannel.providerCode,
              )
            }}
          </p>
          <p
            v-if="
              editor.smsProviderCode &&
              editor.smsProviderCode !== activeSMSChannel?.providerCode
            "
            class="notice warning"
          >
            {{ tr("smsTemplateProviderMismatch") }}
          </p>
          <template
            v-if="editor.smsProviderCode === 'tencent_cloud_sms_v20210111'"
            ><label
              >{{ tr("providerTemplateId")
              }}<input
                v-model.trim="editor.providerTemplateId"
                required
                inputmode="numeric"
                maxlength="32"
            /></label>
            <div
              v-for="(_, index) in editor.smsVariableOrder"
              :key="index"
              class="mapping-row"
            >
              <select v-model="editor.smsVariableOrder[index]">
                <option
                  v-for="variable in editorVariables"
                  :key="variable"
                  :value="variable"
                >
                  {{ variableLabel(variable) }}
                </option></select
              ><button
                class="button danger"
                type="button"
                @click="editor.smsVariableOrder.splice(index, 1)"
              >
                {{ tr("remove") }}
              </button>
            </div>
            <button
              class="button secondary"
              type="button"
              :disabled="!editorVariables.length"
              @click="addSMSVariable"
            >
              {{ tr("addOrderedParameter") }}
            </button></template
          ><template
            v-else-if="editor.smsProviderCode === 'aliyun_sms_v20170525'"
            ><label
              >{{ tr("aliyunTemplateCode")
              }}<input
                v-model.trim="editor.providerTemplateId"
                required
                maxlength="128"
            /></label>
            <div
              v-for="(row, index) in editor.smsDataRows"
              :key="index"
              class="mapping-row"
            >
              <input
                v-model.trim="row.key"
                required
                :placeholder="tr('aliyunTemplateParameter')"
                maxlength="64"
              /><select v-model="row.variable">
                <option
                  v-for="variable in editorVariables"
                  :key="variable"
                  :value="variable"
                >
                  {{ variableLabel(variable) }}
                </option></select
              ><button
                class="button danger"
                type="button"
                @click="editor.smsDataRows.splice(index, 1)"
              >
                {{ tr("remove") }}
              </button>
            </div>
            <button
              class="button secondary"
              type="button"
              :disabled="!editorVariables.length"
              @click="addSMSDataMapping"
            >
              {{ tr("addNamedParameter") }}
            </button></template
          >
          <p
            v-else-if="editor.smsProviderCode === 'smsbao_http_v1'"
            class="notice info"
          >
            {{ tr("smsBaoBodyMappingHelp") }}
          </p>
        </fieldset>
        <label
          >{{ tr("status")
          }}<select v-model="editor.status">
            <option value="draft">{{ statusLabel("draft") }}</option>
            <option value="active">{{ statusLabel("active") }}</option>
            <option value="disabled">{{ statusLabel("disabled") }}</option>
          </select></label
        >
        <div class="actions">
          <button class="button primary" type="submit">{{ tr("save") }}</button
          ><button
            class="button secondary"
            type="button"
            :disabled="!editor.templateNo"
            @click="previewTemplate"
          >
            {{ tr("preview") }}
          </button>
        </div>
        <div v-if="preview" class="preview">
          <strong>{{ preview.subject }}</strong>
          <p>{{ preview.body }}</p>
        </div>
      </form>
    </section>

    <section v-else class="split">
      <div class="panel">
        <button
          v-for="item in deliveries"
          :key="item.taskNo"
          class="row row-button"
          @click="openDelivery(item)"
        >
          <div>
            <strong>{{ eventLabel(item.eventCode) }}</strong>
            <p>
              {{ channelDisplayName(item.channelCode) }} ·
              {{ formatTime(item.queuedAt) }}
            </p>
          </div>
          <span class="status" :data-state="item.status">{{
            statusLabel(item.status)
          }}</span>
        </button>
        <p v-if="!deliveries.length" class="empty">{{ tr("noData") }}</p>
        <button
          v-if="nextCursor"
          class="button secondary load-more"
          :disabled="loadingMore"
          @click="loadMoreDeliveries"
        >
          {{ loadingMore ? tr("loading") : tr("loadMore") }}
        </button>
      </div>
      <aside class="panel detail">
        <template v-if="selectedDelivery"
          ><h2>{{ eventLabel(selectedDelivery.eventCode) }}</h2>
          <dl>
            <div>
              <dt>{{ tr("status") }}</dt>
              <dd>{{ statusLabel(selectedDelivery.status) }}</dd>
            </div>
            <div>
              <dt>{{ tr("channel") }}</dt>
              <dd>{{ channelDisplayName(selectedDelivery.channelCode) }}</dd>
            </div>
            <div>
              <dt>{{ tr("recipient") }}</dt>
              <dd>{{ recipientLabel(selectedDelivery) }}</dd>
            </div>
            <div>
              <dt>{{ tr("retryCount") }}</dt>
              <dd>{{ selectedDelivery.retrySequence }}/3</dd>
            </div>
            <div v-if="selectedDelivery.nextAttemptAt">
              <dt>{{ tr("nextRetryAt") }}</dt>
              <dd>{{ formatTime(selectedDelivery.nextAttemptAt) }}</dd>
            </div>
            <div>
              <dt>{{ tr("queuedAt") }}</dt>
              <dd>{{ formatTime(selectedDelivery.queuedAt) }}</dd>
            </div>
            <div v-if="eligibility">
              <dt>{{ tr("remainingManualResends") }}</dt>
              <dd>{{ eligibility.remainingCount }}</dd>
            </div>
            <div v-if="eligibility?.nextAllowedAt">
              <dt>{{ tr("nextActionAt") }}</dt>
              <dd>{{ formatTime(eligibility.nextAllowedAt) }}</dd>
            </div>
          </dl>
          <div v-if="eligibility && !eligibility.allowed" class="reason-list">
            <p v-for="reason in eligibility.reasons" :key="reason">
              {{ reasonLabel(reason) }}
            </p>
          </div>
          <p v-if="resendCooldownRemainingSeconds > 0" class="notice info">
            {{ resendCooldownLabel }}
          </p>
          <label
            >{{ tr("reason")
            }}<textarea v-model="actionReason" rows="3" maxlength="500" />
          </label>
          <div class="actions">
            <button
              class="button secondary"
              :disabled="checking"
              @click="checkEligibility()"
            >
              {{ checking ? tr("checking") : tr("recheck") }}</button
            ><button
              class="button primary"
              :disabled="!eligibility?.allowed || !actionReason.trim()"
              @click="resend"
            >
              {{ tr("resend") }}</button
            ><button
              v-if="selectedDelivery.status === 'manual_review'"
              class="button danger"
              :disabled="!actionReason.trim()"
              @click="closeUnknown"
            >
              {{ tr("closeUnknown") }}
            </button>
          </div></template
        >
        <p v-else class="empty">{{ tr("selectDelivery") }}</p>
      </aside>
    </section>
  </main>
</template>
