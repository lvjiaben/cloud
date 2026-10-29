<script setup lang="ts">
import type {
  ProviderTaskDetail,
  ProviderTaskStatus,
  ProviderTaskTakeoverReason,
} from "../provider-task-client";

import { computed, onBeforeUnmount, ref } from "vue";

import { useRouter } from "vue-router";

import { useAccess } from "../plugin-access";
import { useVbenDrawer } from "@vben/common-ui";

import {
  Alert,
  Button,
  Checkbox,
  Descriptions,
  Empty,
  message,
  Modal,
  Progress,
  Select,
  Tag,
} from "ant-design-vue";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import {
  providerTaskDetail,
  providerTaskManualTakeover,
} from "../provider-task-client";

const emit = defineEmits<{ success: [detail: ProviderTaskDetail] }>();
const takeoverPermission = "idc:admin:resource:task:manual-takeover";
const eligibleStatuses: ProviderTaskStatus[] = [
  "accepted",
  "result_unknown",
  "running",
];
const reasonCodes: ProviderTaskTakeoverReason[] = [
  "provider_result_unresolved",
  "provider_task_stalled",
  "resource_state_conflict",
  "external_coordination_required",
  "compensation_required",
];

const { hasAccessByCodes } = useAccess();
const router = useRouter();
const detail = ref<ProviderTaskDetail>();
const targetScope = ref<{ serviceNo?: string; channelNo?: string }>({});
const hasNonSubmissionProof = computed(() => detail.value?.task.status === 'failed' && detail.value.evidence.some(item => item.evidenceKind === 'owner_provision_request_not_sent'));
const selectedEvidence = ref<string[]>([]);
const reasonCode = ref<ProviderTaskTakeoverReason>();

function normalizeDetail(item: ProviderTaskDetail): ProviderTaskDetail {
  return {
    ...item,
    evidence: item.evidence ?? [],
    residualResources: item.residualResources ?? [],
  };
}
const submitting = ref(false);
let detailRevision = 0;
onBeforeUnmount(() => { detailRevision++; detail.value = undefined; selectedEvidence.value = []; reasonCode.value = undefined; });

const canTakeOver = computed(
  () =>
    !!detail.value &&
    hasAccessByCodes([takeoverPermission]) &&
    eligibleStatuses.includes(detail.value.task.status),
);

const canSubmit = computed(
  () =>
    canTakeOver.value &&
    !!reasonCode.value &&
    selectedEvidence.value.length >= 1 &&
    selectedEvidence.value.length <= 16,
);

const title = computed(() => $t("plugin.idc-resource.tasks.detail.title"));

function statusColor(status: ProviderTaskStatus) {
  if (status === "succeeded") return "success";
  if (status === "accepted" || status === "running") return "processing";
  if (status === "result_unknown") return "warning";
  if (status === "manual_takeover") return "purple";
  if (status === "failed" || status === "canceled") return "error";
  return "default";
}

function taskStatusLabel(status: string) {
  const known = new Set([
    "accepted",
    "canceled",
    "failed",
    "manual_takeover",
    "pending",
    "result_unknown",
    "retry_wait",
    "running",
    "submitting",
    "succeeded",
  ]);
  return $t(
    `plugin.idc-resource.tasks.status.${known.has(status) ? status : "unknown"}`,
  );
}

function residualStatusLabel(status: string) {
  const known = new Set([
    "present",
    "release_failed",
    "release_pending",
    "unknown",
  ]);
  return $t(
    `plugin.idc-resource.tasks.residualStatus.${known.has(status) ? status : "unknown"}`,
  );
}

function reasonOptions() {
  return reasonCodes.map((value) => ({
    label: takeoverReasonLabel(value),
    value,
  }));
}

function takeoverReasonLabel(reason: string) {
  return $t(
    `plugin.idc-resource.tasks.reason.${reasonCodes.includes(reason as ProviderTaskTakeoverReason) ? reason : "unknown"}`,
  );
}

const capabilityMessageKeys: Record<string, string> = {
  "balance.query": "balanceQuery",
  "capability.discovery": "capabilityDiscovery",
  "cost.query": "costQuery",
  "provider-capacity.list": "providerCapacity",
  "server.backup": "serverBackup",
  "server.console": "serverConsole",
  "server.expiry.query": "serverExpiryQuery",
  "server.ip.change": "serverIPChange",
  "server.nat": "serverNAT",
  "server.password.reset": "serverPasswordReset",
  "server.provision": "serverProvision",
  "server.query": "serverQuery",
  "server.reboot": "serverReboot",
  "server.reinstall": "serverReinstall",
  "server.renew": "serverRenew",
  "server.rescue.enter": "serverRescueEnter",
  "server.rescue.exit": "serverRescueExit",
  "server.resize": "serverResize",
  "server.resume": "serverResume",
  "server.security-group": "serverSecurityGroup",
  "server.snapshot": "serverSnapshot",
  "server.start": "serverStart",
  "server.stop": "serverStop",
  "server.suspend": "serverSuspend",
  "server.terminate": "serverTerminate",
  "server.traffic-package": "serverTrafficPackage",
  "server.virtual-media": "serverVirtualMedia",
  "ticket.manage": "ticketManage",
};

function capabilityLabel(code: string) {
  return $t(
    `plugin.idc-resource.tasks.capability.${capabilityMessageKeys[code] || "unknown"}`,
  );
}

function upstreamStatusLabel(status: string) {
  const known = new Set([
    "accepted",
    "canceled",
    "failed",
    "pending",
    "result_unknown",
    "running",
    "succeeded",
  ]);
  return $t(
    `plugin.idc-resource.tasks.upstreamStatus.${known.has(status) ? status : "unknown"}`,
  );
}

function evidenceLabel(kind: string) {
  const known = new Set([
    "upstream_http_response",
    "upstream_call_diagnostic",
	"owner_provision_request_not_sent",
    "upstream_result_unknown",
    "upstream_rejection",
    "upstream_call_failed",
    "resource_observation",
    "residualResource",
    "task_observation",
  ]);
  return $t(
    `plugin.idc-resource.tasks.evidenceKind.${known.has(kind) ? kind : "unknown"}`,
  );
}

function evidenceSummaryLabel(summary: string) {
  const keys: Record<string, string> = {
	"The resource owner confirmed no business HTTP request was attempted": "requestNotSent",
    "resource.power_state_on": "resourcePowerOn",
    "task.running": "taskRunning",
    "operation submission outcome could not be proven": "submissionUnknown",
    "upstream rejected the operation before acceptance": "requestRejected",
    "The upstream connection timed out": "timeout",
    "The upstream hostname could not be resolved": "dns",
    "The upstream network connection failed": "network",
    "The upstream API endpoint was not found": "endpoint",
    "The upstream API returned an invalid response": "invalidResponse",
    "The upstream API rejected authentication": "authentication",
    "The connector authentication configuration does not match its request": "configuration",
  };
  return $t(
    `plugin.idc-resource.tasks.evidenceSummary.${keys[summary] || "unknown"}`,
  );
}

function formatHTTPResponse(text: string) {
  try { return JSON.stringify(JSON.parse(text), null, 2) ?? text; }
  catch { return text; }
}

function callFailureLabel(summary: string) {
  const stages = new Set(["request_validation", "authentication", "request_preparation", "http_transport", "response_processing", "provider_dispatch", "provider_result", "owner_setup"]);
  try {
    const value: unknown = JSON.parse(summary);
    if (value && typeof value === "object" && "stage" in value && typeof value.stage === "string" && stages.has(value.stage)) return $t(`plugin.idc-resource.tasks.callStage.${value.stage}`);
  } catch { /* Historical unstructured summaries use the safe fallback. */ }
  return $t("plugin.idc-resource.tasks.evidenceKind.upstream_call_diagnostic");
}

function residualKindLabel(kind: string) {
  const keys: Record<string, string> = {
    bare_metal: "bareMetal",
    ip: "ipAddress",
    ip_address: "ipAddress",
    server: "server",
    virtual_server: "virtualServer",
  };
  return $t(
    `plugin.idc-resource.tasks.residualKind.${keys[kind] || "unknown"}`,
  );
}

function residualReasonLabel(reason: string) {
  const keys: Record<string, string> = {
    provider_rejected: "providerRejected",
    provider_timeout: "providerTimeout",
    release_failed: "releaseFailed",
  };
  return $t(
    `plugin.idc-resource.tasks.residualReason.${keys[reason] || "unknown"}`,
  );
}

function confirmTakeover() {
  if (!canSubmit.value || !detail.value || !reasonCode.value) return;
  const revision = detailRevision;
  const taskNo = detail.value.task.taskNo;
  Modal.confirm({
    cancelText: $t("common.cancel"),
    content: $t("plugin.idc-resource.tasks.takeover.confirmDescription", {
      count: selectedEvidence.value.length,
    }),
    okButtonProps: { danger: true },
    okText: $t("plugin.idc-resource.tasks.takeover.confirm"),
    title: $t("plugin.idc-resource.tasks.takeover.confirmTitle"),
    async onOk() {
      if (!canSubmit.value || !detail.value || !reasonCode.value || revision !== detailRevision || detail.value.task.taskNo !== taskNo) return;
      submitting.value = true;
      try {
        const result = await providerTaskManualTakeover(
          detail.value.task.taskNo,
          {
            ...targetScope.value,
            evidenceNos: [...selectedEvidence.value],
            expectedVersion: detail.value.task.version,
            reasonCode: reasonCode.value,
          },
        );
        if (revision !== detailRevision) return;
        detail.value = normalizeDetail(result.item);
        selectedEvidence.value = [];
        reasonCode.value = undefined;
        message.success($t("plugin.idc-resource.tasks.takeover.succeeded"));
        emit("success", result.item);
      } finally {
        if (revision === detailRevision) submitting.value = false;
      }
    },
  });
}

const [Drawer, drawerApi] = useVbenDrawer({
  footer: false,
  async onOpenChange(open) {
    const revision = ++detailRevision;
    submitting.value = false;
    if (!open) {
      detail.value = undefined;
      selectedEvidence.value = [];
      reasonCode.value = undefined;
      return;
    }
    const data = drawerApi.getData<{ taskNo?: string; serviceNo?: string; channelNo?: string }>();
    targetScope.value = { serviceNo: data?.serviceNo, channelNo: data?.channelNo };
    if (!data?.taskNo) return;
    drawerApi.setState({ loading: true });
    try {
      const item = (await providerTaskDetail(data.taskNo, targetScope.value)).item;
      if (revision === detailRevision) detail.value = normalizeDetail(item);
    } finally {
      if (revision === detailRevision) drawerApi.setState({ loading: false });
    }
  },
});
</script>

<template>
  <Drawer :title="title" class="w-[820px]">
    <div
      v-if="detail"
      class="task-detail-shell"
      data-testid="provider-task-detail"
    >
      <section class="task-detail-summary">
        <div>
          <div class="task-detail-kicker">
            {{ $t("plugin.idc-resource.tasks.detail.kicker") }}
          </div>
          <h2>
            {{
              detail.task.channelName ||
              $t("plugin.idc-resource.tasks.channelUnavailable")
            }}
          </h2>
          <span>{{ capabilityLabel(detail.task.capabilityCode) }}</span>
        </div>
        <Tag :color="statusColor(detail.task.status)">
          {{ taskStatusLabel(detail.task.status) }}
        </Tag>
      </section>

      <Alert
        v-if="canTakeOver"
        :description="$t('plugin.idc-resource.tasks.takeover.boundary')"
        :message="$t('plugin.idc-resource.tasks.takeover.safety')"
        show-icon
        type="warning"
      />

      <Alert v-if="hasNonSubmissionProof" type="info" show-icon :message="$t('plugin.idc-resource.tasks.evidenceSummary.requestNotSent')" :description="$t('plugin.idc-resource.tasks.detail.orderHandlingHelp')" />

      <Alert
        v-if="detail.task.capabilityCode === 'server.provision' && detail.task.status === 'result_unknown'"
        data-testid="provider-provision-unknown"
        :message="$t('plugin.idc-resource.tasks.detail.provisionUnknown')"
        :description="$t('plugin.idc-resource.tasks.detail.provisionUnknownHelp')"
        show-icon type="warning"
      />
      <div v-if="detail.task.capabilityCode === 'server.provision'" class="task-section">
        <p v-if="!hasNonSubmissionProof">{{ $t('plugin.idc-resource.tasks.detail.orderHandlingHelp') }}</p>
        <Button v-if="hasAccessByCodes(['idc:admin:product-fulfillment:task:list'])" @click="drawerApi.close(); router.push('/idc/product-fulfillment/tasks')">{{ $t('plugin.idc-resource.tasks.detail.orderHandling') }}</Button>
      </div>

      <Descriptions bordered :column="2" size="small">
        <Descriptions.Item
          :label="$t('plugin.idc-resource.tasks.table.capability')"
        >
          {{ capabilityLabel(detail.task.capabilityCode) }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-resource.tasks.table.resource')"
        >
          {{ $t("plugin.idc-resource.tasks.detail.boundServerResource") }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-resource.tasks.table.attempts')"
        >
          {{ detail.task.attemptCount }} / {{ detail.task.maxAttempts }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-resource.tasks.table.observations')"
        >
          {{ detail.task.observationAttemptCount }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-resource.tasks.detail.updatedAt')"
        >
          {{ formatTimestamp(detail.task.updatedAt) }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-resource.tasks.table.status')"
        >
          {{ taskStatusLabel(detail.task.status) }}
        </Descriptions.Item>
      </Descriptions>

      <section class="task-section">
        <div class="task-section-title">
          {{ $t("plugin.idc-resource.tasks.detail.upstream") }}
        </div>
        <div v-if="detail.upstream" class="upstream-card">
          <div>
            <span>{{
              $t("plugin.idc-resource.tasks.detail.upstreamStatus")
            }}</span>
            <strong>{{
              upstreamStatusLabel(detail.upstream.statusCode)
            }}</strong>
          </div>
          <div>
            <span>{{
              $t("plugin.idc-resource.tasks.detail.upstreamObservedAt")
            }}</span>
            <strong>{{ formatTimestamp(detail.upstream.observedAt) }}</strong>
          </div>
          <div class="upstream-progress">
            <span>{{ $t("plugin.idc-resource.tasks.detail.progress") }}</span>
            <Progress :percent="detail.upstream.progress" size="small" />
          </div>
        </div>
        <Empty v-else :image="Empty.PRESENTED_IMAGE_SIMPLE" :description="hasNonSubmissionProof ? $t('plugin.idc-resource.tasks.evidenceSummary.requestNotSent') : undefined" />
      </section>

      <section v-if="detail.residualResources.length" class="task-section">
        <div class="task-section-title evidence-title">
          <span>{{
            $t("plugin.idc-resource.tasks.detail.residualResources")
          }}</span>
          <Tag color="warning">{{ detail.residualResources.length }} / 16</Tag>
        </div>
        <Alert
          :description="
            $t('plugin.idc-resource.tasks.detail.residualResourcesDescription')
          "
          show-icon
          type="warning"
        />
        <div class="residual-list">
          <div
            v-for="item in detail.residualResources"
            :key="`${item.resourceKind}:${item.resourceRef}`"
            class="residual-card"
          >
            <div>
              <strong>{{ residualKindLabel(item.resourceKind) }}</strong>
            </div>
            <Tag color="warning">{{ residualStatusLabel(item.status) }}</Tag>
            <span>{{ residualReasonLabel(item.reasonCode) }}</span>
          </div>
        </div>
      </section>

      <section class="task-section">
        <div class="task-section-title evidence-title">
          <span>{{ $t("plugin.idc-resource.tasks.detail.evidence") }}</span>
          <Tag>{{ detail.evidence.length }} / 50</Tag>
        </div>
        <Checkbox.Group v-model:value="selectedEvidence" class="evidence-list">
          <label
            v-for="item in detail.evidence"
            :key="item.evidenceNo"
            :data-testid="`provider-task-evidence-${item.evidenceNo}`"
            class="evidence-card"
          >
            <Checkbox :disabled="!canTakeOver" :value="item.evidenceNo" />
            <div>
              <strong>{{ item.evidenceKind === 'upstream_http_response' ? $t('plugin.idc-resource.tasks.detail.httpResponse') : item.evidenceKind === 'upstream_call_diagnostic' ? callFailureLabel(item.summary) : evidenceSummaryLabel(item.summary) }}</strong>
              <pre v-if="item.evidenceKind === 'upstream_http_response' || item.evidenceKind === 'upstream_call_diagnostic'" class="max-h-64 overflow-auto whitespace-pre-wrap break-all text-sm" :data-testid="item.evidenceKind === 'upstream_http_response' ? 'resource-http-response' : 'resource-call-diagnostic'">{{ formatHTTPResponse(item.summary) }}</pre>
              <span>{{ evidenceLabel(item.evidenceKind) }}</span>
              <time>{{ formatTimestamp(item.observedAt) }}</time>
            </div>
          </label>
        </Checkbox.Group>
        <Empty
          v-if="detail.evidence.length === 0"
          :description="$t('plugin.idc-resource.tasks.detail.noEvidence')"
          :image="Empty.PRESENTED_IMAGE_SIMPLE"
        />
      </section>

      <section v-if="canTakeOver" class="takeover-panel">
        <div>
          <strong>{{ $t("plugin.idc-resource.tasks.takeover.title") }}</strong>
          <span>{{
            $t("plugin.idc-resource.tasks.takeover.description")
          }}</span>
        </div>
        <Select
          v-model:value="reasonCode"
          :options="reasonOptions()"
          :placeholder="
            $t('plugin.idc-resource.tasks.takeover.reasonPlaceholder')
          "
          class="w-full"
          data-testid="provider-task-takeover-reason"
        />
        <Button
          :disabled="!canSubmit"
          :loading="submitting"
          danger
          data-testid="provider-task-takeover-submit"
          type="primary"
          @click="confirmTakeover"
        >
          {{ $t("plugin.idc-resource.tasks.takeover.action") }}
        </Button>
      </section>

      <section v-if="detail.task.takeover" class="takeover-record">
        <div>
          <span>{{ $t("plugin.idc-resource.tasks.takeover.record") }}</span>
          <strong>{{
            takeoverReasonLabel(detail.task.takeover.reasonCode)
          }}</strong>
        </div>
        <small>
          {{
            $t("plugin.idc-resource.tasks.takeover.evidenceCount", {
              count: detail.task.takeover.evidenceCount,
            })
          }}
          ·
          {{ formatTimestamp(detail.task.takeover.createdAt) }}
        </small>
      </section>
    </div>
  </Drawer>
</template>

<style scoped>
.task-detail-shell {
  display: grid;
  gap: 20px;
}
.task-detail-summary {
  align-items: center;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
  color: hsl(var(--foreground));
  display: flex;
  justify-content: space-between;
  padding: 24px;
}
.task-detail-kicker {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
  letter-spacing: 0.16em;
}
.task-detail-summary h2 {
  font-size: 20px;
  margin: 10px 0 4px;
}
.task-detail-summary code {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
}
.task-section {
  display: grid;
  gap: 12px;
}
.task-section-title {
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.04em;
}
.evidence-title {
  align-items: center;
  display: flex;
  justify-content: space-between;
}
.upstream-card {
  background: hsl(var(--muted) / 45%);
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  padding: 16px;
}
.upstream-card > div {
  display: flex;
  flex-direction: column;
  gap: 5px;
}
.upstream-card span {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
}
.upstream-card code {
  overflow-wrap: anywhere;
}
.evidence-list {
  display: grid;
  gap: 9px;
  width: 100%;
}
.residual-list {
  display: grid;
  gap: 9px;
}
.residual-card {
  align-items: center;
  border: 1px solid rgb(245 158 11 / 35%);
  border-radius: 0;
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  padding: 14px;
}
.residual-card > div {
  display: grid;
  gap: 3px;
}
.residual-card code {
  overflow-wrap: anywhere;
}
.evidence-card {
  align-items: flex-start;
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  cursor: pointer;
  display: flex;
  gap: 12px;
  padding: 14px;
  transition:
    border-color 0.2s ease,
    transform 0.2s ease;
}
.evidence-card:hover {
  border-color: rgb(217 119 6 / 55%);
  transform: translateY(-1px);
}
.evidence-card > div {
  display: grid;
  gap: 3px;
  min-width: 0;
}
.evidence-card span,
.evidence-card time {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
}
.evidence-card code {
  font-size: 11px;
  overflow-wrap: anywhere;
}
.takeover-panel {
  background: rgb(255 251 235);
  border: 1px solid rgb(245 158 11 / 35%);
  border-radius: 0;
  display: grid;
  gap: 13px;
  padding: 18px;
}
:global(.dark) .takeover-panel {
  background: rgb(69 26 3 / 18%);
}
.takeover-panel > div {
  display: grid;
  gap: 4px;
}
.takeover-panel span {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}
.takeover-record {
  align-items: center;
  background: rgb(88 28 135 / 7%);
  border: 1px solid rgb(147 51 234 / 20%);
  border-radius: 0;
  display: grid;
  gap: 4px;
  padding: 16px;
}
.takeover-record > div {
  display: flex;
  gap: 10px;
}
.takeover-record span,
.takeover-record small {
  color: hsl(var(--muted-foreground));
}
@media (max-width: 720px) {
  .upstream-card {
    grid-template-columns: 1fr;
  }
  .residual-card {
    grid-template-columns: 1fr;
  }
}
</style>
