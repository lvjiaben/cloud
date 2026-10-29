<script setup lang="ts">
import RenewalModal from "./renewal-modal.vue";
import type { NativeServerDetail } from "#/api/idc/native-servers";
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Page } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  DescriptionsItem,
  InputPassword,
  Skeleton,
  Space,
  Tag,
} from "ant-design-vue";
import VbenModalCompat from "#/components/vben-modal-compat.vue";
import {
  createNativeOperation,
  getNativeServer,
  resolveNativeServer,
  revealInitialCredential,
} from "#/api/idc/native-servers";
import { $t } from "#/locales";
const route = useRoute();
const router = useRouter();
const item = ref<NativeServerDetail>();
const loading = ref(false);
const failed = ref(false);
const sending = ref(false);
const operationFailed = ref(false);
const pendingAction = ref("");
const confirmationOpen = ref(false);
const confirmed = ref(false);
const passwordOpen = ref(false);
const renewalOpen = ref(false);
const password = ref("");
const credentialFailed = ref(false);
const showAllAddresses = ref(false);
const visibleAddresses = computed(() =>
  showAllAddresses.value
    ? (item.value?.addresses ?? [])
    : (item.value?.addresses ?? []).slice(0, 8),
);
let generation = 0;
let loadVersion = 0;
let disposed = false;
let timer: ReturnType<typeof setTimeout> | undefined;
let polls = 0;
let observationRetries = 0;
let retryKey = "";
let retryAction = "";
const runtimeColor = computed(() =>
  ["active", "running"].includes(item.value?.runtimeState || "")
    ? "success"
    : item.value?.runtimeState === "stopped"
      ? "default"
      : "warning",
);
const title = computed(() =>
  item.value
    ? preferences.app.locale === "zh-CN"
      ? item.value.productNameZhCn
      : item.value.productNameEnUs
    : $t("nativeServers.title"),
);
const busy = computed(
  () =>
    item.value?.operations.some((operation) =>
      ["pending", "submitting", "accepted", "running", "retry_wait"].includes(
        operation.status,
      ),
    ) ?? false,
);
const waitingUntil = computed(() => item.value?.operations.find((operation) => operation.waitingUntil)?.waitingUntil);
const attention = computed(
  () =>
    item.value?.operations.some((operation) =>
      ["result_unknown", "manual_takeover"].includes(operation.status),
    ) ?? false,
);
const canUse = computed(
  () => !failed.value && !sending.value && !loading.value,
);
function translated(group: string, value: string, known: string[]) {
  return $t(
    `nativeServers.${group}.${known.includes(value) ? value : "unknown"}`,
  );
}
function runtime(value: string) {
  return translated("runtime", value, [
    "active",
    "running",
    "stopped",
    "pending",
    "suspended",
    "terminated",
    "failed",
  ]);
}
function entitlement(value: string) {
  return translated("entitlement", value, [
    "active",
    "suspended",
    "expired",
    "terminated",
    "cancelled",
    "pending_activation",
  ]);
}
function operationStatus(value: string) {
  return translated("operationStatus", value, [
    "pending",
    "submitting",
    "accepted",
    "running",
    "retry_wait",
    "succeeded",
    "failed",
    "canceled",
    "result_unknown",
    "manual_takeover",
  ]);
}
function actionLabel(value: string) {
  return translated("power", value, ["start", "stop", "reboot"]);
}
function date(value?: number) {
  return value
    ? new Date(value).toLocaleString(preferences.app.locale)
    : $t("nativeServers.noExpiry");
}
function stopPolling() {
  if (timer) clearTimeout(timer);
  timer = undefined;
}
async function load(quiet = false) {
  stopPolling();
  const current = generation;
  const requestVersion = ++loadVersion;
  if (!quiet) loading.value = true;
  try {
    let serverNo = String(route.params.serverNo || "");
    if (!serverNo && route.params.entitlementNo) {
      const target = await resolveNativeServer(
        String(route.params.entitlementNo),
      );
      if (disposed || current !== generation || requestVersion !== loadVersion)
        return;
      await router.replace(
        `/account/native-servers/${encodeURIComponent(target.serverNo)}`,
      );
      return;
    }
    const detail = await getNativeServer(serverNo);
    if (disposed || current !== generation || requestVersion !== loadVersion)
      return;
    item.value = detail;
    failed.value = false;
    if (detail.runtimeState !== "unknown") observationRetries = 0;
    // Retry only a bounded read; no power or password request is replayed.
    if (!document.hidden) {
      if (busy.value && waitingUntil.value)
        timer = setTimeout(() => {
          if (!document.hidden) {
            polls = 0;
            void load(true);
          }
        }, Math.min(30000, Math.max(1000, waitingUntil.value - Date.now() + 250)));
      else if (busy.value && polls < 24)
        timer = setTimeout(() => {
          if (!document.hidden) {
            polls += 1;
            void load(true);
          }
        }, 5000);
      else if (detail.runtimeState === "unknown" && observationRetries < 3)
        timer = setTimeout(() => {
          if (!document.hidden) {
            observationRetries += 1;
            void load(true);
          }
        }, 5000);
    }
  } catch {
    if (!disposed && current === generation && requestVersion === loadVersion)
      failed.value = true;
  } finally {
    if (
      !disposed &&
      current === generation &&
      requestVersion === loadVersion &&
      !quiet
    )
      loading.value = false;
  }
}
function choose(action: string) {
  if (!canUse.value) return;
  operationFailed.value = false;
  credentialFailed.value = false;
  pendingAction.value = action;
  confirmed.value = false;
  if (action === "start") void submitPower(action);
  else confirmationOpen.value = true;
}
async function submitPower(action: string) {
  if (!item.value || sending.value) return;
  if (retryAction !== action || !retryKey) {
    retryAction = action;
    retryKey = crypto.randomUUID();
  }
  const current = generation;
  sending.value = true;
  operationFailed.value = false;
  try {
    const result = await createNativeOperation(
      item.value.serverNo,
      action,
      retryKey,
      action !== "start",
    );
    if (disposed || current !== generation) return;
    item.value.operations = [
      result.operation,
      ...item.value.operations.filter(
        (operation) => operation.operationNo !== result.operation.operationNo,
      ),
    ].slice(0, 10);
    item.value.availableActions = [];
    retryKey = "";
    retryAction = "";
    confirmationOpen.value = false;
    polls = 0;
    await load(true);
  } catch {
    if (!disposed && current === generation) {
      operationFailed.value = true;
      confirmationOpen.value = false;
    }
  } finally {
    if (!disposed && current === generation) sending.value = false;
  }
}
async function confirmAction() {
  if (!confirmed.value || sending.value) return;
  if (pendingAction.value !== "credential") {
    await submitPower(pendingAction.value);
    return;
  }
  if (!item.value) return;
  const current = generation;
  sending.value = true;
  credentialFailed.value = false;
  try {
    const result = await revealInitialCredential(item.value.serverNo);
    if (disposed || current !== generation) return;
    password.value = result.password;
    item.value.credentialAvailable = false;
    passwordOpen.value = true;
  } catch {
    if (!disposed && current === generation) credentialFailed.value = true;
  } finally {
    if (!disposed && current === generation) {
      sending.value = false;
      confirmationOpen.value = false;
    }
  }
}
function clearPassword() {
  password.value = "";
  passwordOpen.value = false;
}
watch(passwordOpen, (open) => {
  if (!open) password.value = "";
});
watch(
  () => route.fullPath,
  () => {
    generation += 1;
    item.value = undefined;
    failed.value = false;
    sending.value = false;
    operationFailed.value = false;
    credentialFailed.value = false;
    confirmationOpen.value = false;
    clearPassword();
    renewalOpen.value = false;
    showAllAddresses.value = false;
    retryKey = "";
    retryAction = "";
    polls = 0;
    observationRetries = 0;
    void load();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  disposed = true;
  generation += 1;
  stopPolling();
  clearPassword();
  retryKey = "";
});
</script>
<template>
  <Page
    class="customer-inner-page"
    header-class="customer-inner-heading"
    content-class="customer-inner-content"
    :title="title"
    data-testid="native-server-detail"
  >
    <Space class="native-server-toolbar" wrap>
      <Button @click="router.push('/account/servers')">{{
        $t("nativeServers.back")
      }}</Button>
      <Button
        :loading="loading"
        @click="
          polls = 0;
          observationRetries = 0;
          load();
        "
        >{{ $t("nativeServers.refresh") }}</Button
      >
      <Button
        v-if="
          item?.entitlementNo &&
          item.expiresAt &&
          !['terminated', 'cancelled'].includes(item.entitlementStatus)
        "
        data-testid="native-server-renew"
        @click="renewalOpen = true"
        >{{ $t("serverRenewal.title") }}</Button
      >
      <Button
        v-if="item"
        type="link"
        @click="
          router.push(`/account/orders/${encodeURIComponent(item.orderNo)}`)
        "
        >{{ $t("nativeServers.sourceOrder") }}</Button
      >
    </Space>
    <Alert
      v-if="failed"
      class="mb-4"
      type="error"
      show-icon
      :message="$t('nativeServers.loadFailed')"
    />
    <Skeleton v-if="loading && !item" active />
    <template v-if="item">
      <Alert
        v-if="busy"
        class="mb-4"
        type="info"
        show-icon
        :message="waitingUntil ? $t('nativeServers.waitingForProvider', { time: date(waitingUntil) }) : $t('nativeServers.processing')"
      />
      <Alert
        v-if="attention"
        class="mb-4"
        type="warning"
        show-icon
        :message="$t('nativeServers.attention')"
      />
      <Alert
        v-if="item.runtimeState === 'unknown'"
        class="mb-4"
        type="warning"
        show-icon
        :message="$t('nativeServers.observationUnavailable')"
      />
      <Alert
        v-if="operationFailed"
        class="mb-4"
        type="error"
        show-icon
        :message="$t('nativeServers.operationFailed')"
      >
        <template #action
          ><Button :loading="sending" @click="submitPower(retryAction)">{{
            $t("nativeServers.retrySame")
          }}</Button></template
        >
      </Alert>
      <Alert
        v-if="credentialFailed"
        class="mb-4"
        type="error"
        show-icon
        :message="$t('nativeServers.credentialFailed')"
      />
      <section class="native-server-summary">
        <div>
          <span>{{ $t("nativeServers.runtimeState") }}</span
          ><Tag :color="runtimeColor">{{ runtime(item.runtimeState) }}</Tag>
        </div>
        <div>
          <span>{{ $t("nativeServers.entitlementState") }}</span
          ><strong>{{ entitlement(item.entitlementStatus) }}</strong>
        </div>
        <div>
          <span>{{ $t("nativeServers.expiresAt") }}</span
          ><strong>{{ date(item.expiresAt) }}</strong>
        </div>
      </section>
      <Card class="mb-4" :title="$t('nativeServers.overview')">
        <template #extra
          ><Space wrap>
            <Button
              v-for="action in item.availableActions"
              :key="action"
              :aria-label="actionLabel(action)"
              :disabled="!canUse"
              :loading="sending && retryAction === action"
              @click="choose(action)"
              >{{ actionLabel(action) }}</Button
            >
          </Space></template
        >
        <Descriptions
          bordered
          :column="{ xs: 1, sm: 1, lg: 2 }"
          :label-style="{ width: '120px', whiteSpace: 'nowrap' }"
          :content-style="{ overflowWrap: 'anywhere' }"
        >
          <DescriptionsItem :label="$t('nativeServers.ipAddresses')">
            <Space
              v-if="item.addressesConfirmed && item.addresses?.length"
              wrap
            >
              <span
                v-for="address in visibleAddresses"
                :key="address"
                class="native-ip"
                >{{ address }}</span
              >
              <Button
                v-if="item.addresses.length > 8"
                type="link"
                @click="showAllAddresses = !showAllAddresses"
                >{{
                  showAllAddresses
                    ? $t("nativeServers.collapseAddresses")
                    : $t("nativeServers.allAddresses", {
                        count: item.addresses.length,
                      })
                }}</Button
              >
            </Space>
            <span v-else>{{
              item.addressesConfirmed
                ? $t("nativeServers.noAddresses")
                : $t("nativeServers.addressesUnavailable")
            }}</span>
          </DescriptionsItem>
          <DescriptionsItem :label="$t('nativeServers.product')">{{
            title
          }}</DescriptionsItem>
          <DescriptionsItem :label="$t('nativeServers.startedAt')">{{
            date(item.startedAt)
          }}</DescriptionsItem>
          <DescriptionsItem :label="$t('nativeServers.serverNo')"
            ><span class="native-server-reference">{{
              item.serverNo
            }}</span></DescriptionsItem
          >
          <DescriptionsItem :label="$t('nativeServers.observedAt')">{{
            item.observedAt
              ? date(item.observedAt)
              : $t("nativeServers.notObserved")
          }}</DescriptionsItem>
        </Descriptions>
      </Card>
      <Card
        class="mb-4"
        :title="$t('nativeServers.access')"
        data-testid="native-server-credential"
      >
        <p class="mb-3">{{ $t("nativeServers.credentialHint") }}</p>
        <p v-if="busy">{{ $t("nativeServers.processing") }}</p>
        <p v-else-if="item.runtimeState === 'unknown'">
          {{ $t("nativeServers.credentialObservationUnavailable") }}
        </p>
        <Button
          v-else-if="item.credentialAvailable"
          :disabled="!canUse"
          @click="choose('credential')"
          >{{ $t("nativeServers.reveal") }}</Button
        >
        <p v-else>{{ $t("nativeServers.credentialUnavailable") }}</p>
      </Card>
      <Card
        :title="$t('nativeServers.history')"
        data-testid="native-server-operations"
      >
        <p v-if="!item.operations.length">
          {{ $t("nativeServers.noOperations") }}
        </p>
        <ol v-else class="native-operations">
          <li v-for="operation in item.operations" :key="operation.operationNo">
            <div>
              <strong>{{ actionLabel(operation.action) }}</strong
              ><Tag>{{ operationStatus(operation.status) }}</Tag>
            </div>
            <small v-if="operation.waitingUntil" data-testid="operation-waiting-reason">{{ $t('nativeServers.waitingForProvider', { time: date(operation.waitingUntil) }) }}</small>
            <time>{{ date(operation.createdAt) }}</time
            ><small>{{ operation.operationNo }}</small>
          </li>
        </ol>
      </Card>
    </template>
    <RenewalModal
      v-if="item?.entitlementNo"
      v-model:open="renewalOpen"
      :entitlement-no="item.entitlementNo"
      @settled="load()"
    />
    <VbenModalCompat
      v-model:open="confirmationOpen"
      :title="
        pendingAction === 'credential'
          ? $t('nativeServers.reveal')
          : actionLabel(pendingAction)
      "
      :confirm-loading="sending"
      :ok-button-props="{ disabled: !confirmed }"
      :ok-text="$t('nativeServers.confirm')"
      :cancel-text="$t('nativeServers.cancel')"
      @ok="confirmAction"
    >
      <p class="mb-4">
        {{
          pendingAction === "credential"
            ? $t("nativeServers.credentialConfirmation")
            : $t("nativeServers.powerConfirmation")
        }}
      </p>
      <Checkbox v-model:checked="confirmed">{{
        $t("nativeServers.confirmed")
      }}</Checkbox>
    </VbenModalCompat>
    <VbenModalCompat
      v-model:open="passwordOpen"
      :title="$t('nativeServers.initialPassword')"
      :footer="null"
      @cancel="clearPassword"
    >
      <Alert
        class="mb-4"
        type="warning"
        show-icon
        :message="$t('nativeServers.passwordOnce')"
      />
      <InputPassword
        v-if="passwordOpen"
        :value="password"
        readonly
        autocomplete="off"
        :aria-label="$t('nativeServers.initialPassword')"
      />
      <Button class="mt-4" @click="clearPassword">{{
        $t("nativeServers.closePassword")
      }}</Button>
    </VbenModalCompat>
  </Page>
</template>
<style scoped>
.native-server-toolbar {
  margin-bottom: 24px;
}
.native-server-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
  margin: 0 0 24px;
  padding: 24px;
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
  background: hsl(var(--card));
}
.native-server-summary > div {
  display: grid;
  justify-items: start;
  gap: 12px;
}
.native-server-summary span {
  font-size: 13px;
  color: hsl(var(--muted-foreground));
}
.native-server-summary strong {
  font-size: 17px;
  font-weight: 500;
}
.native-server-reference {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}
@media (max-width: 700px) {
  .native-server-summary {
    grid-template-columns: 1fr;
    gap: 18px;
  }
  .native-server-summary > div {
    grid-template-columns: 100px 1fr;
    align-items: center;
  }
}
.native-ip {
  font-family: ui-monospace, monospace;
  overflow-wrap: anywhere;
}
.native-operations {
  margin: 0;
  padding: 0;
  list-style: none;
}
.native-operations li {
  display: grid;
  grid-template-columns: minmax(160px, 1fr) auto;
  gap: 6px 20px;
  padding: 14px 0;
  border-bottom: 1px solid var(--customer-border, #e5e7eb);
}
.native-operations li > div {
  display: flex;
  align-items: center;
  gap: 12px;
}
.native-operations small {
  grid-column: 1 / -1;
  color: #64748b;
  overflow-wrap: anywhere;
}
@media (max-width: 640px) {
  .native-operations li {
    grid-template-columns: 1fr;
  }
}
</style>
