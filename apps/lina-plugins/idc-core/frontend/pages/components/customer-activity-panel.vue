<script setup lang="ts">
import type { VbenFormSchema } from "#/adapter/form";

import { computed, nextTick, reactive, ref, watch } from "vue";

import { useAccess } from "../plugin-access";

import { Button, Empty, message, Modal as AntModal, Tag } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import VbenModalCompat from "./vben-modal-compat.vue";

import {
  customerBlacklistCreate,
  customerBlacklistDecision,
  customerActivities,
  customerFollowUpPlan,
  customerFollowUpTransition,
  customerNoteCreate,
} from "../customer-client";
import type {
  CustomerActivityItem,
  CustomerActivityKind,
} from "../customer-client";

const props = defineProps<{ customerNo: string }>();
const emit = defineEmits<{ changed: [] }>();
const { hasAccessByCodes } = useAccess();
const canQuery = computed(() => hasAccessByCodes(["idc:admin:customer:query"]));
const canUpdate = computed(() =>
  hasAccessByCodes(["idc:admin:customer:update"]),
);
const active = ref<"blacklist" | "followUp" | "note" | "">("");
const saving = ref(false);
const historyLoading = ref<CustomerActivityKind | "">("");
const histories = reactive<
  Record<CustomerActivityKind, CustomerActivityItem[]>
>({ blacklist: [], follow_up: [], note: [] });
const historyCursors = reactive<Record<CustomerActivityKind, string>>({
  blacklist: "",
  follow_up: "",
  note: "",
});
const historyKinds: CustomerActivityKind[] = ["note", "follow_up", "blacklist"];
const activityOpen = computed({
  get: () => Boolean(active.value),
  set: (open: boolean) => {
    if (!open) close();
  },
});

function translatedOptions(prefix: string, values: string[]) {
  return values.map((value) => ({
    label: $t(`plugin.idc-core.customer.activity.${prefix}.${value}`),
    value,
  }));
}

function activitySchema(
  kind: "blacklist" | "followUp" | "note",
): VbenFormSchema[] {
  if (kind === "note") {
    return [
      {
        component: "Select" as const,
        componentProps: {
          options: translatedOptions("noteCategory", [
            "general",
            "billing",
            "service",
            "sales",
            "risk",
            "compliance",
          ]),
        },
        defaultValue: "general",
        fieldName: "category",
        label: $t("plugin.idc-core.customer.activity.category"),
      },
      {
        component: "Textarea" as const,
        componentProps: { maxlength: 8192, rows: 5, showCount: true },
        fieldName: "content",
        label: $t("plugin.idc-core.customer.activity.content"),
        rules: z.string().trim().min(1).max(8192),
      },
      {
        component: "Select" as const,
        componentProps: {
          allowClear: true,
          options: histories.note
            .filter((item) => item.status === "active")
            .map((item) => ({
              label: item.title,
              value: item.number,
            })),
          placeholder: $t(
            "plugin.idc-core.customer.activity.supersedesPlaceholder",
          ),
        },
        fieldName: "supersedesNo",
        label: $t("plugin.idc-core.customer.activity.supersedesNo"),
      },
    ];
  }
  if (kind === "followUp") {
    return [
      {
        component: "Select" as const,
        componentProps: {
          options: translatedOptions("channel", [
            "phone",
            "email",
            "sms",
            "wechat",
            "meeting",
            "ticket",
            "other",
          ]),
        },
        defaultValue: "phone",
        fieldName: "channel",
        label: $t("plugin.idc-core.customer.activity.channelLabel"),
      },
      {
        component: "Input" as const,
        componentProps: { maxlength: 512 },
        fieldName: "subject",
        label: $t("plugin.idc-core.customer.activity.subject"),
        rules: z.string().trim().min(1).max(512),
      },
      {
        component: "Input" as const,
        componentProps: { type: "datetime-local" },
        fieldName: "plannedAt",
        label: $t("plugin.idc-core.customer.activity.plannedAt"),
        rules: z.string().min(1),
      },
      {
        component: "Textarea" as const,
        componentProps: { maxlength: 8192, rows: 4, showCount: true },
        fieldName: "detail",
        label: $t("plugin.idc-core.customer.activity.detail"),
      },
    ];
  }
  return [
    {
      component: "Select" as const,
      componentProps: {
        options: translatedOptions("scope", [
          "registration",
          "purchase",
          "refund",
          "resource_operation",
          "all_business",
        ]),
      },
      defaultValue: "purchase",
      fieldName: "scope",
      label: $t("plugin.idc-core.customer.activity.scopeLabel"),
    },
    {
      component: "Select" as const,
      componentProps: {
        options: translatedOptions("risk", [
          "low",
          "medium",
          "high",
          "critical",
        ]),
      },
      defaultValue: "medium",
      fieldName: "riskLevel",
      label: $t("plugin.idc-core.customer.activity.riskLabel"),
    },
    {
      component: "Select" as const,
      componentProps: {
        options: translatedOptions("blacklistReason", [
          "payment_risk",
          "account_security",
          "abusive_behavior",
          "suspected_fraud",
          "compliance_risk",
          "other",
        ]),
      },
      defaultValue: "payment_risk",
      fieldName: "reasonCode",
      label: $t("plugin.idc-core.customer.activity.reasonCode"),
      rules: z.string().trim().min(1).max(64),
    },
    {
      component: "Textarea" as const,
      componentProps: { maxlength: 2048, rows: 4, showCount: true },
      fieldName: "reasonDetail",
      label: $t("plugin.idc-core.customer.activity.reasonDetail"),
    },
    {
      component: "Input" as const,
      componentProps: { maxlength: 256 },
      fieldName: "evidenceReference",
      label: $t("plugin.idc-core.customer.activity.evidenceReference"),
    },
    {
      component: "Input" as const,
      componentProps: { type: "datetime-local" },
      fieldName: "expiresAt",
      label: $t("plugin.idc-core.customer.activity.expiresAt"),
    },
  ];
}

const [ActivityForm, activityFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 120 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});

function close() {
  active.value = "";
}

async function openActivity(kind: "blacklist" | "followUp" | "note") {
  active.value = kind;
  await nextTick();
  await activityFormApi.setState({ schema: activitySchema(kind) });
  await activityFormApi.resetForm();
  if (kind === "note") {
    await activityFormApi.setValues({ category: "general" });
  } else if (kind === "followUp") {
    await activityFormApi.setValues({ channel: "phone" });
  } else {
    await activityFormApi.setValues({
      reasonCode: "payment_risk",
      riskLevel: "medium",
      scope: "purchase",
    });
  }
}

function activityStatus(item: CustomerActivityItem) {
  return $t(
    `plugin.idc-core.customer.activity.status.${item.kind}.${item.status}`,
  );
}

function activityCategory(item: CustomerActivityItem) {
  const prefix =
    item.kind === "note"
      ? "noteCategory"
      : item.kind === "follow_up"
        ? "channel"
        : "scope";
  return $t(`plugin.idc-core.customer.activity.${prefix}.${item.category}`);
}

function activityTitle(item: CustomerActivityItem) {
  if (item.kind !== "blacklist") return item.title;
  return $t(
    `plugin.idc-core.customer.activity.blacklistReason.${item.title}`,
  );
}

function activityTime(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "";
  return new Date(value).toLocaleString();
}

function statusColor(item: CustomerActivityItem) {
  if (["active", "completed"].includes(item.status)) return "green";
  if (["cancelled", "expired", "rejected", "revoked", "withdrawn"].includes(item.status)) {
    return "default";
  }
  return "blue";
}

async function loadHistory(kind: CustomerActivityKind, reset = false) {
  if (historyLoading.value) return;
  historyLoading.value = kind;
  try {
    const result = await customerActivities(props.customerNo, {
      cursor: reset ? undefined : historyCursors[kind] || undefined,
      kind,
      limit: 10,
    });
    histories[kind] = reset
      ? result.items
      : [...histories[kind], ...result.items];
    historyCursors[kind] = result.nextCursor;
  } finally {
    historyLoading.value = "";
  }
}

async function quickFollowUp(
  item: CustomerActivityItem,
  action: "cancel" | "complete",
) {
  await customerFollowUpTransition(props.customerNo, item.number, {
    action,
    idempotencyKey: `customer-follow-up-decision-${crypto.randomUUID()}`,
  });
  await loadHistory("follow_up", true);
  emit("changed");
}

async function quickBlacklist(
  item: CustomerActivityItem,
  action: "approve" | "reject" | "revoke",
) {
  AntModal.confirm({
    title: $t(
      "plugin.idc-core.customer.activity.confirmBlacklistTitle",
    ),
    content: $t(
      "plugin.idc-core.customer.activity.confirmBlacklistContent",
    ),
    async onOk() {
      await customerBlacklistDecision(props.customerNo, item.number, {
        action,
        idempotencyKey: `customer-blacklist-decision-${crypto.randomUUID()}`,
      });
      await loadHistory("blacklist", true);
      emit("changed");
    },
  });
}

watch(
  () => props.customerNo,
  async () => {
    histories.note = [];
    histories.follow_up = [];
    histories.blacklist = [];
    historyCursors.note = "";
    historyCursors.follow_up = "";
    historyCursors.blacklist = "";
    for (const kind of ["note", "follow_up", "blacklist"] as const) {
      await loadHistory(kind, true);
    }
  },
  { immediate: true },
);

async function save() {
  const { valid } = await activityFormApi.validate();
  if (!valid) {
    message.warning($t("plugin.idc-core.customer.activity.required"));
    return;
  }
  const values = await activityFormApi.getValues<Record<string, unknown>>();
  saving.value = true;
  try {
    if (active.value === "note") {
      await customerNoteCreate(props.customerNo, {
        category: String(values.category) as "general",
        content: String(values.content).trim(),
        supersedesNo: String(values.supersedesNo || "").trim() || undefined,
        idempotencyKey: `customer-note-${crypto.randomUUID()}`,
      });
    } else if (active.value === "followUp") {
      const plannedAt = Date.parse(String(values.plannedAt));
      if (!Number.isFinite(plannedAt)) {
        message.warning(
          $t("plugin.idc-core.customer.activity.required"),
        );
        return;
      }
      await customerFollowUpPlan(props.customerNo, {
        assignedUserId: 0,
        channel: String(values.channel) as "phone",
        detail: String(values.detail || "").trim(),
        plannedAt,
        subject: String(values.subject).trim(),
        idempotencyKey: `customer-follow-up-${crypto.randomUUID()}`,
      });
    } else if (active.value === "blacklist") {
      const expiresAtText = String(values.expiresAt || "");
      const expiresAt = expiresAtText ? Date.parse(expiresAtText) : 0;
      if (expiresAtText && !Number.isFinite(expiresAt)) {
        message.warning(
          $t("plugin.idc-core.customer.activity.required"),
        );
        return;
      }
      await customerBlacklistCreate(props.customerNo, {
        evidenceReference: String(values.evidenceReference || "").trim(),
        expiresAt,
        reasonCode: String(values.reasonCode).trim(),
        reasonDetail: String(values.reasonDetail || "").trim(),
        riskLevel: String(values.riskLevel) as "medium",
        scope: String(values.scope) as "purchase",
        startsAt: 0,
        idempotencyKey: `customer-blacklist-${crypto.randomUUID()}`,
      });
    }
    message.success($t("plugin.idc-core.customer.activity.saved"));
    emit("changed");
    if (active.value === "note") await loadHistory("note", true);
    else if (active.value === "followUp") await loadHistory("follow_up", true);
    else await loadHistory("blacklist", true);
    close();
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <section v-if="canUpdate" class="activity-panel">
    <div>
      <strong>{{
        $t("plugin.idc-core.customer.activity.title")
      }}</strong>
      <p>{{ $t("plugin.idc-core.customer.activity.hint") }}</p>
    </div>
    <div class="activity-actions">
      <Button @click="openActivity('note')">{{
        $t("plugin.idc-core.customer.activity.addNote")
      }}</Button>
      <Button @click="openActivity('followUp')">{{
        $t("plugin.idc-core.customer.activity.planFollowUp")
      }}</Button>
      <Button danger @click="openActivity('blacklist')">{{
        $t("plugin.idc-core.customer.activity.proposeBlacklist")
      }}</Button>
    </div>
  </section>

  <section v-if="canQuery" class="history-grid">
    <article v-for="kind in historyKinds" :key="kind" class="history-column">
      <header>
        {{ $t(`plugin.idc-core.customer.activity.history.${kind}`) }}
      </header>
      <div v-if="histories[kind].length" class="history-list">
        <div
          v-for="item in histories[kind]"
          :key="item.number"
          class="history-item"
        >
          <div class="history-item-head">
            <span>{{ activityCategory(item) }}</span>
            <Tag :color="statusColor(item)">{{ activityStatus(item) }}</Tag>
          </div>
          <strong>{{ activityTitle(item) }}</strong>
          <p v-if="item.detail">{{ item.detail }}</p>
          <time>{{ activityTime(item.updatedAt || item.createdAt) }}</time>
          <div
            v-if="
              canUpdate && kind === 'follow_up' && item.status === 'planned'
            "
            class="history-buttons"
          >
            <Button
              size="small"
              type="link"
              @click="quickFollowUp(item, 'complete')"
              >{{
                $t("plugin.idc-core.customer.activity.complete")
              }}</Button
            >
            <Button
              size="small"
              type="link"
              @click="quickFollowUp(item, 'cancel')"
              >{{
                $t("plugin.idc-core.customer.activity.cancel")
              }}</Button
            >
          </div>
          <div v-if="canUpdate && kind === 'blacklist'" class="history-buttons">
            <template v-if="item.status === 'pending'">
              <Button
                size="small"
                type="link"
                @click="quickBlacklist(item, 'approve')"
                >{{
                  $t("plugin.idc-core.customer.activity.approve")
                }}</Button
              >
              <Button
                danger
                size="small"
                type="link"
                @click="quickBlacklist(item, 'reject')"
                >{{
                  $t("plugin.idc-core.customer.activity.reject")
                }}</Button
              >
            </template>
            <Button
              v-else-if="item.status === 'active'"
              danger
              size="small"
              type="link"
              @click="quickBlacklist(item, 'revoke')"
              >{{
                $t("plugin.idc-core.customer.activity.revoke")
              }}</Button
            >
          </div>
        </div>
      </div>
      <Empty v-else :image="Empty.PRESENTED_IMAGE_SIMPLE" />
      <Button
        v-if="historyCursors[kind]"
        :loading="historyLoading === kind"
        size="small"
        type="link"
        @click="loadHistory(kind)"
      >
        {{ $t("plugin.idc-core.customer.overview.loadMore") }}
      </Button>
    </article>
  </section>

  <VbenModalCompat
    v-model:open="activityOpen"
    :confirm-loading="saving"
    :title="
      $t(`plugin.idc-core.customer.activity.modal.${active || 'note'}`)
    "
    @ok="save"
  >
    <ActivityForm />
  </VbenModalCompat>
</template>

<style scoped>
.activity-panel {
  align-items: center;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  padding: 14px 16px;
}

.activity-panel p {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  margin: 4px 0 0;
}

.activity-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.history-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.history-column {
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  min-width: 0;
  padding: 12px;
}

.history-column > header {
  font-size: 13px;
  font-weight: 700;
  margin-bottom: 10px;
}

.history-list,
.history-item {
  display: grid;
  gap: 8px;
}

.history-item {
  background: hsl(var(--muted) / 45%);
  border-radius: 0;
  padding: 10px;
}

.history-item-head,
.history-buttons {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.history-item-head > span,
.history-item time,
.history-item p {
  color: hsl(var(--muted-foreground));
  font-size: 11px;
  margin: 0;
  overflow-wrap: anywhere;
}

@media (width <= 720px) {
  .activity-panel {
    align-items: stretch;
    flex-direction: column;
  }

  .activity-actions {
    justify-content: flex-start;
  }

  .history-grid {
    grid-template-columns: 1fr;
  }
}
</style>
