<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/ai-copilot",
  title: "AI Operations Copilot",
};
</script>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

import { useAccess } from "@vben/access";
import { Page } from "@vben/common-ui";

import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  Skeleton,
  Space,
  Switch,
  Tag,
  message,
} from "ant-design-vue";

import { pluginApiPath, requestClient } from "#/api/request";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

type AIConfig = {
  baseUrl: string;
  budgetMinor: number;
  enabled: boolean;
  keyConfigured: boolean;
  model: string;
  rateLimitPerMinute: number;
  retentionDays: number;
  revision: number;
};

type AIConfigTestResult = {
  message: string;
  reachable: boolean;
  status: string;
};

type AIChatResult = {
  answer: string;
  conversationId: number;
  dataAsOf: string;
  evidence: string[];
  model?: string;
  safety: string;
  uncertainty: string;
};

type AIAuditItem = {
  createdAt: string;
  decision: string;
  reason?: string;
  tool: string;
};

const tr = (key: string, params?: Record<string, number | string>) =>
  $t(`plugin.idc-ai.copilot.${key}`, params);

const emptyConfig = (): AIConfig => ({
  baseUrl: "",
  budgetMinor: 0,
  enabled: false,
  keyConfigured: false,
  model: "",
  rateLimitPerMinute: 30,
  retentionDays: 30,
  revision: 0,
});

const { hasAccessByCodes } = useAccess();
const canViewConfig = computed(() =>
  hasAccessByCodes(["idc:admin:ai:config:view"]),
);
const canManageConfig = computed(() =>
  hasAccessByCodes(["idc:admin:ai:config:manage"]),
);
const canViewAudit = computed(() => hasAccessByCodes(["idc:admin:ai:audit"]));

const config = ref<AIConfig>(emptyConfig());
const apiKey = ref("");
const configLoading = ref(false);
const configSaving = ref(false);
const configTesting = ref(false);
const lastTestedSignature = ref("");

const prompt = ref("");
const answer = ref<AIChatResult>();
const asking = ref(false);
const feedbackSending = ref(false);
const feedbackRating = ref(0);

const audits = ref<AIAuditItem[]>([]);
const auditsLoading = ref(false);

const configSignature = computed(() =>
  JSON.stringify({
    baseUrl: config.value.baseUrl.trim(),
    key: apiKey.value
      ? "new-key"
      : config.value.keyConfigured
        ? "saved-key"
        : "",
    model: config.value.model.trim(),
  }),
);

const configStatusColor = computed(() =>
  config.value.enabled ? "success" : "default",
);

function validateConfiguration() {
  const baseUrl = config.value.baseUrl.trim();
  const model = config.value.model.trim();
  if (!baseUrl || !model) {
    message.warning(tr("message.required"));
    return false;
  }
  let parsed: URL;
  try {
    parsed = new URL(baseUrl);
  } catch {
    message.warning(tr("message.invalidUrl"));
    return false;
  }
  const localAddress =
    parsed.protocol === "http:" && parsed.hostname === "127.0.0.1";
  if (parsed.protocol !== "https:" && !localAddress) {
    message.warning(tr("message.httpsRequired"));
    return false;
  }
  if (!config.value.keyConfigured && !apiKey.value.trim()) {
    message.warning(tr("message.firstKeyRequired"));
    return false;
  }
  return true;
}

async function loadConfig() {
  if (!canViewConfig.value) return;
  configLoading.value = true;
  try {
    config.value = await requestClient.get<AIConfig>(
      pluginApiPath("idc-ai", "admin/ai/config"),
    );
    apiKey.value = "";
    if (config.value.enabled) {
      lastTestedSignature.value = configSignature.value;
    }
  } finally {
    configLoading.value = false;
  }
}

async function testConfiguration() {
  if (!validateConfiguration()) return;
  configTesting.value = true;
  try {
    const result = await requestClient.post<AIConfigTestResult>(
      pluginApiPath("idc-ai", "admin/ai/config/test"),
      {
        apiKey: apiKey.value.trim(),
        baseUrl: config.value.baseUrl.trim(),
        model: config.value.model.trim(),
      },
    );
    if (!result.reachable) {
      lastTestedSignature.value = "";
      message.error(tr("message.testFailed"));
      return;
    }
    lastTestedSignature.value = configSignature.value;
    message.success(tr("message.testSucceeded"));
  } finally {
    configTesting.value = false;
  }
}

async function saveConfig() {
  if (!validateConfiguration()) return;
  if (
    config.value.enabled &&
    lastTestedSignature.value !== configSignature.value
  ) {
    message.warning(tr("message.testBeforeEnable"));
    return;
  }
  configSaving.value = true;
  try {
    config.value = await requestClient.put<AIConfig>(
      pluginApiPath("idc-ai", "admin/ai/config"),
      {
        apiKey: apiKey.value.trim(),
        baseUrl: config.value.baseUrl.trim(),
        budgetMinor: config.value.budgetMinor,
        enabled: config.value.enabled,
        model: config.value.model.trim(),
        rateLimitPerMinute: config.value.rateLimitPerMinute,
        retentionDays: config.value.retentionDays,
      },
    );
    apiKey.value = "";
    lastTestedSignature.value = config.value.enabled
      ? configSignature.value
      : "";
    message.success(tr("message.saved"));
  } finally {
    configSaving.value = false;
  }
}

async function ask() {
  const value = prompt.value.trim();
  if (!value) {
    message.warning(tr("message.promptRequired"));
    return;
  }
  asking.value = true;
  feedbackRating.value = 0;
  try {
    answer.value = await requestClient.post<AIChatResult>(
      pluginApiPath("idc-ai", "admin/ai/chat"),
      { prompt: value },
    );
    if (canViewAudit.value) await loadAudits();
  } finally {
    asking.value = false;
  }
}

async function sendFeedback(rating: number) {
  if (!answer.value?.conversationId || feedbackSending.value) return;
  feedbackSending.value = true;
  try {
    await requestClient.post(pluginApiPath("idc-ai", "admin/ai/feedback"), {
      conversationId: answer.value.conversationId,
      rating,
    });
    feedbackRating.value = rating;
    message.success(tr("message.feedbackSaved"));
  } finally {
    feedbackSending.value = false;
  }
}

async function loadAudits() {
  if (!canViewAudit.value) return;
  auditsLoading.value = true;
  try {
    const result = await requestClient.get<{ items: AIAuditItem[] }>(
      pluginApiPath("idc-ai", "admin/ai/audit"),
      { params: { limit: 20 } },
    );
    audits.value = result.items ?? [];
  } finally {
    auditsLoading.value = false;
  }
}

function safetyLabel(value: string) {
  const known = new Set([
    "allowed",
    "budget_exhausted",
    "fallback",
    "refused",
    "throttled",
  ]);
  return known.has(value) ? tr(`safety.${value}`) : tr("safety.reviewed");
}

function safetyColor(value: string) {
  if (value === "allowed") return "success";
  if (value === "fallback") return "processing";
  if (value === "refused") return "error";
  return "warning";
}

function auditDecisionLabel(value: string) {
  if (value === "allowed") return tr("audit.allowed");
  if (value === "rejected") return tr("audit.rejected");
  return tr("audit.reviewed");
}

function auditToolLabel(value: string) {
  const known = new Set([
    "dangerous_operation",
    "readonly_summary",
    "sensitive_material",
  ]);
  return known.has(value) ? tr(`audit.tool.${value}`) : tr("audit.tool.other");
}

function auditReasonLabel(value?: string) {
  if (!value) return tr("audit.noReason");
  const known = new Set([
    "sensitive_material_excluded",
    "write_operation_requested",
  ]);
  return known.has(value) ? tr(`audit.reason.${value}`) : tr("audit.noReason");
}

onMounted(async () => {
  await Promise.all([loadConfig(), loadAudits()]);
});
</script>

<template>
  <Page
    :description="tr('description')"
    :title="tr('title')"
    class="ai-copilot-page"
  >
    <Alert
      :description="tr('readonlyDescription')"
      :message="tr('readonlyTitle')"
      banner
      class="mb-4 rounded-none"
      show-icon
      type="info"
    />

    <div
      class="grid min-w-0 gap-4 xl:grid-cols-[minmax(360px,0.9fr)_minmax(0,1.4fr)]"
    >
      <Card
        v-if="canViewConfig"
        :bordered="false"
        class="ai-console-card"
        data-testid="ai-copilot-panel"
      >
        <template #title>
          <div class="flex min-w-0 items-center justify-between gap-3">
            <span>{{ tr("config.title") }}</span>
            <Tag :color="configStatusColor">
              {{
                config.enabled ? tr("config.enabled") : tr("config.disabled")
              }}
            </Tag>
          </div>
        </template>

        <Skeleton v-if="configLoading" active :paragraph="{ rows: 7 }" />
        <div v-else class="space-y-4">
          <div class="ai-form-field">
            <label for="ai-base-url">{{ tr("config.baseUrl") }}</label>
            <Input
              id="ai-base-url"
              v-model:value="config.baseUrl"
              :disabled="!canManageConfig"
              :placeholder="tr('config.baseUrlPlaceholder')"
            />
            <p>{{ tr("config.baseUrlHelp") }}</p>
          </div>

          <div class="ai-form-field">
            <label for="ai-model">{{ tr("config.model") }}</label>
            <Input
              id="ai-model"
              v-model:value="config.model"
              :disabled="!canManageConfig"
              :placeholder="tr('config.modelPlaceholder')"
            />
          </div>

          <div class="ai-form-field">
            <label for="ai-api-key">{{ tr("config.apiKey") }}</label>
            <Input
              id="ai-api-key"
              v-model:value="apiKey"
              :disabled="!canManageConfig"
              :placeholder="
                config.keyConfigured
                  ? tr('config.apiKeyKeepPlaceholder')
                  : tr('config.apiKeyPlaceholder')
              "
              autocomplete="new-password"
              type="password"
            />
            <div class="mt-2">
              <Tag :color="config.keyConfigured ? 'success' : 'warning'">
                {{
                  config.keyConfigured
                    ? tr("config.keyConfigured")
                    : tr("config.keyMissing")
                }}
              </Tag>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div class="ai-form-field">
              <label for="ai-budget">{{ tr("config.budget") }}</label>
              <InputNumber
                id="ai-budget"
                v-model:value="config.budgetMinor"
                :disabled="!canManageConfig"
                :min="0"
                :precision="0"
                class="w-full"
              />
              <p>{{ tr("config.budgetHelp") }}</p>
            </div>
            <div class="ai-form-field">
              <label for="ai-rate-limit">{{ tr("config.rateLimit") }}</label>
              <InputNumber
                id="ai-rate-limit"
                v-model:value="config.rateLimitPerMinute"
                :disabled="!canManageConfig"
                :max="1000"
                :min="1"
                :precision="0"
                class="w-full"
              />
            </div>
            <div class="ai-form-field">
              <label for="ai-retention">{{ tr("config.retention") }}</label>
              <InputNumber
                id="ai-retention"
                v-model:value="config.retentionDays"
                :disabled="!canManageConfig"
                :max="3650"
                :min="1"
                :precision="0"
                class="w-full"
              />
            </div>
          </div>

          <div class="ai-enable-row">
            <div>
              <strong>{{ tr("config.enableTitle") }}</strong>
              <p>{{ tr("config.enableHelp") }}</p>
            </div>
            <Switch
              v-model:checked="config.enabled"
              :disabled="!canManageConfig"
            />
          </div>

          <Space v-if="canManageConfig" wrap>
            <Button :loading="configTesting" @click="testConfiguration">
              {{ tr("action.test") }}
            </Button>
            <Button :loading="configSaving" type="primary" @click="saveConfig">
              {{ tr("action.save") }}
            </Button>
          </Space>
        </div>
      </Card>

      <Card :bordered="false" class="ai-console-card">
        <template #title>
          <div>
            <div>{{ tr("chat.title") }}</div>
            <div class="ai-card-subtitle">{{ tr("chat.subtitle") }}</div>
          </div>
        </template>

        <div class="ai-form-field">
          <label for="ai-question">{{ tr("chat.question") }}</label>
          <Input.TextArea
            id="ai-question"
            v-model:value="prompt"
            :auto-size="{ minRows: 4, maxRows: 10 }"
            :maxlength="4000"
            :placeholder="tr('chat.placeholder')"
            show-count
            @keydown.ctrl.enter="ask"
          />
        </div>

        <div class="mt-3 flex items-center justify-between gap-3">
          <span class="text-xs text-[var(--ant-color-text-tertiary)]">
            {{ tr("chat.shortcut") }}
          </span>
          <Button :loading="asking" type="primary" @click="ask">
            {{ tr("action.analyze") }}
          </Button>
        </div>

        <div
          v-if="answer"
          class="ai-answer mt-5"
          data-testid="ai-copilot-answer"
        >
          <div class="flex flex-wrap items-center gap-2 border-b px-4 py-3">
            <strong>{{ tr("chat.answer") }}</strong>
            <Tag :color="safetyColor(answer.safety)">
              {{ safetyLabel(answer.safety) }}
            </Tag>
            <Tag v-if="answer.model">{{ answer.model }}</Tag>
          </div>
          <div class="max-h-[480px] overflow-auto p-4">
            <p class="whitespace-pre-wrap leading-7">{{ answer.answer }}</p>

            <dl class="ai-evidence-grid mt-5">
              <div>
                <dt>{{ tr("chat.dataAsOf") }}</dt>
                <dd>
                  {{
                    answer.dataAsOf
                      ? formatTimestamp(answer.dataAsOf)
                      : tr("chat.notProvided")
                  }}
                </dd>
              </div>
              <div>
                <dt>{{ tr("chat.uncertainty") }}</dt>
                <dd>{{ answer.uncertainty || tr("chat.notProvided") }}</dd>
              </div>
            </dl>

            <div v-if="answer.evidence?.length" class="mt-5">
              <h4 class="mb-2 font-medium">{{ tr("chat.evidence") }}</h4>
              <ul class="list-disc space-y-1 pl-5 text-sm">
                <li v-for="item in answer.evidence" :key="item">{{ item }}</li>
              </ul>
            </div>
          </div>
          <div
            v-if="answer.conversationId"
            class="flex flex-wrap items-center gap-2 border-t px-4 py-3"
          >
            <span class="text-sm text-[var(--ant-color-text-secondary)]">
              {{ tr("feedback.title") }}
            </span>
            <Button
              v-for="rating in 5"
              :key="rating"
              :disabled="feedbackSending"
              :type="feedbackRating === rating ? 'primary' : 'default'"
              size="small"
              @click="sendFeedback(rating)"
            >
              {{ rating }}
            </Button>
          </div>
        </div>
      </Card>
    </div>

    <Card v-if="canViewAudit" :bordered="false" class="ai-console-card mt-4">
      <template #title>
        <div class="flex items-center justify-between gap-3">
          <div>
            <div>{{ tr("audit.title") }}</div>
            <div class="ai-card-subtitle">{{ tr("audit.subtitle") }}</div>
          </div>
          <Button :loading="auditsLoading" size="small" @click="loadAudits">
            {{ tr("action.refresh") }}
          </Button>
        </div>
      </template>

      <Skeleton v-if="auditsLoading && !audits.length" active />
      <Empty v-else-if="!audits.length" :description="tr('audit.empty')" />
      <div v-else class="max-h-[360px] divide-y overflow-auto">
        <div
          v-for="(item, index) in audits"
          :key="`${item.createdAt}-${item.tool}-${index}`"
          class="grid gap-2 px-1 py-3 md:grid-cols-[180px_minmax(180px,1fr)_120px_minmax(0,1fr)] md:items-center"
        >
          <span class="text-sm text-[var(--ant-color-text-secondary)]">
            {{ formatTimestamp(item.createdAt) }}
          </span>
          <strong class="text-sm">{{ auditToolLabel(item.tool) }}</strong>
          <Tag :color="item.decision === 'allowed' ? 'success' : 'error'">
            {{ auditDecisionLabel(item.decision) }}
          </Tag>
          <span class="truncate text-sm text-[var(--ant-color-text-secondary)]">
            {{ auditReasonLabel(item.reason) }}
          </span>
        </div>
      </div>
    </Card>
  </Page>
</template>

<style scoped>
.ai-copilot-page :deep(.ant-card) {
  border-radius: 0;
}

.ai-console-card {
  border: 1px solid var(--ant-color-border-secondary);
  box-shadow: none;
}

.ai-console-card :deep(.ant-card-head) {
  min-height: 48px;
  border-bottom-color: var(--ant-color-border-secondary);
}

.ai-card-subtitle {
  margin-top: 3px;
  color: var(--ant-color-text-secondary);
  font-size: 12px;
  font-weight: 400;
}

.ai-form-field > label {
  display: block;
  margin-bottom: 7px;
  color: var(--ant-color-text);
  font-size: 14px;
  font-weight: 500;
}

.ai-form-field > p,
.ai-enable-row p {
  margin: 6px 0 0;
  color: var(--ant-color-text-tertiary);
  font-size: 12px;
  line-height: 1.6;
}

.ai-enable-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 14px 16px;
  border: 1px solid var(--ant-color-border-secondary);
  background: var(--ant-color-fill-quaternary);
}

.ai-answer {
  border: 1px solid var(--ant-color-border-secondary);
  background: var(--ant-color-bg-container);
}

.ai-evidence-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.ai-evidence-grid > div {
  padding: 12px;
  border-left: 2px solid var(--ant-color-primary);
  background: var(--ant-color-fill-quaternary);
}

.ai-evidence-grid dt {
  margin-bottom: 5px;
  color: var(--ant-color-text-tertiary);
  font-size: 12px;
}

.ai-evidence-grid dd {
  margin: 0;
  color: var(--ant-color-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

@media (max-width: 640px) {
  .ai-evidence-grid {
    grid-template-columns: 1fr;
  }
}
</style>
