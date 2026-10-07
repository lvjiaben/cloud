<script setup lang="ts">
import type {
  EnterpriseCaseDetail,
  EnterpriseMaterialItem,
} from "../enterprise-verification-client";

import { computed, ref } from "vue";

import { useAccess } from "../plugin-access";
import { useVbenDrawer, useVbenModal } from "@vben/common-ui";

import {
  Alert,
  Button,
  Descriptions,
  Divider,
  Empty,
  List,
  message,
  Select,
  Space,
  Tag,
} from "ant-design-vue";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import {
  enterpriseApprove,
  enterpriseCaseGet,
  enterpriseMaterialDownload,
  enterpriseReject,
  enterpriseReviewStart,
} from "../enterprise-verification-client";

const emit = defineEmits<{ success: [detail: EnterpriseCaseDetail] }>();
const detail = ref<EnterpriseCaseDetail>();
const busyAction = ref("");
const rejectionReason = ref("");
const confirmationAction = ref<"approve" | "reject" | "review">("review");
const confirmationReason = ref("");
const { hasAccessByCodes } = useAccess();

const permissions = {
  access: "idc:admin:compliance:enterprise-material:access",
  approve: "idc:admin:compliance:enterprise-verification:approve",
  reject: "idc:admin:compliance:enterprise-verification:reject",
  review: "idc:admin:compliance:enterprise-verification:review",
};
const rejectionReasons = [
  "materials_incomplete",
  "materials_unreadable",
  "legal_information_mismatch",
  "registration_identifier_invalid",
  "representative_identity_mismatch",
  "agent_authorization_invalid",
  "registration_status_unverifiable",
  "business_scope_unsupported",
];

const canStart = computed(
  () =>
    !!detail.value &&
    ["pending", "review_required"].includes(detail.value.statusCode) &&
    hasAccessByCodes([permissions.review]),
);
const canApprove = computed(
  () =>
    detail.value?.statusCode === "verifying" &&
    hasAccessByCodes([permissions.approve]),
);
const canReject = computed(
  () =>
    !!detail.value &&
    ["verifying", "review_required"].includes(detail.value.statusCode) &&
    hasAccessByCodes([permissions.reject]),
);
const canAccessMaterial = computed(() =>
  hasAccessByCodes([permissions.access]),
);

const title = computed(() =>
  detail.value
    ? `${$t("plugin.idc-compliance.enterprise.detail.title")} · ${detail.value.caseNo}`
    : $t("plugin.idc-compliance.enterprise.detail.title"),
);

function statusColor(status: string) {
  if (status === "verified") return "success";
  if (status === "verifying") return "processing";
  if (status === "pending" || status === "review_required") return "warning";
  if (status === "rejected" || status === "expired") return "error";
  return "default";
}

function sizeLabel(value: number) {
  if (value < 1024) return `${value} B`;
  return `${(value / 1024 / 1024).toFixed(2)} MiB`;
}

async function reload() {
  if (!detail.value?.caseNo) return;
  detail.value = (await enterpriseCaseGet(detail.value.caseNo)).item;
  emit("success", detail.value);
}

async function runTransition(action: "approve" | "review") {
  if (!detail.value || busyAction.value) return;
  confirmationAction.value = action;
  confirmationReason.value = "";
  confirmationModalApi.open();
}

function runReject() {
  if (!detail.value || !rejectionReason.value || busyAction.value) return;
  confirmationAction.value = "reject";
  confirmationReason.value = rejectionReason.value;
  confirmationModalApi.open();
}

async function downloadMaterial(item: EnterpriseMaterialItem) {
  if (!detail.value || busyAction.value) return;
  busyAction.value = item.materialNo;
  try {
    await enterpriseMaterialDownload(
      detail.value.caseNo,
      item.materialNo,
      item.mediaType,
    );
    message.success($t("plugin.idc-compliance.enterprise.material.downloaded"));
  } finally {
    busyAction.value = "";
  }
}

const confirmationTitle = computed(() =>
  $t(
    `plugin.idc-compliance.enterprise.actions.${confirmationAction.value}Title`,
  ),
);
const confirmationDescription = computed(() =>
  $t(
    `plugin.idc-compliance.enterprise.actions.${confirmationAction.value}Description`,
  ),
);
const confirmationText = computed(() =>
  $t(`plugin.idc-compliance.enterprise.actions.${confirmationAction.value}`),
);

const [ConfirmationModal, confirmationModalApi] = useVbenModal({
  async onConfirm() {
    if (!detail.value || busyAction.value) return false;
    const current = detail.value;
    const action = confirmationAction.value;
    confirmationModalApi.lock(true);
    busyAction.value = action;
    try {
      if (action === "review") {
        await enterpriseReviewStart(current.caseNo, current.version);
      } else if (action === "approve") {
        await enterpriseApprove(current.caseNo, current.version);
      } else {
        await enterpriseReject(
          current.caseNo,
          current.version,
          confirmationReason.value,
        );
        rejectionReason.value = "";
      }
      message.success(
        $t(`plugin.idc-compliance.enterprise.actions.${action}Succeeded`),
      );
      confirmationModalApi.close();
      await reload();
    } finally {
      busyAction.value = "";
      confirmationModalApi.lock(false);
    }
  },
});

const [Drawer, drawerApi] = useVbenDrawer({
  footer: false,
  async onOpenChange(open) {
    if (!open) {
      detail.value = undefined;
      rejectionReason.value = "";
      return;
    }
    const data = drawerApi.getData<{ caseNo?: string }>();
    if (!data?.caseNo) return;
    drawerApi.setState({ loading: true });
    try {
      detail.value = (await enterpriseCaseGet(data.caseNo)).item;
    } finally {
      drawerApi.setState({ loading: false });
    }
  },
});
</script>

<template>
  <Drawer :title="title" class="w-[880px]">
    <div
      v-if="detail"
      class="detail-shell"
      data-testid="enterprise-case-detail"
    >
      <section class="detail-summary">
        <div>
          <span>{{
            $t("plugin.idc-compliance.enterprise.detail.customer")
          }}</span>
          <h2>{{ detail.maskedLegalName }}</h2>
          <code>{{ detail.customerNo }}</code>
        </div>
        <Space direction="vertical" align="end">
          <Tag :color="statusColor(detail.statusCode)">
            {{
              $t(`plugin.idc-compliance.enterprise.status.${detail.statusCode}`)
            }}
          </Tag>
          <Tag v-if="detail.reverification" color="purple">
            {{ $t("plugin.idc-compliance.enterprise.reverification") }}
          </Tag>
        </Space>
      </section>

      <Alert
        show-icon
        type="info"
        :message="$t('plugin.idc-compliance.enterprise.detail.maskedTitle')"
        :description="
          $t('plugin.idc-compliance.enterprise.detail.maskedDescription')
        "
      />

      <Descriptions bordered :column="2" size="small">
        <Descriptions.Item
          :label="$t('plugin.idc-compliance.enterprise.fields.legalName')"
        >
          {{ detail.legalProfile.legalName }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-compliance.enterprise.fields.registrationId')"
        >
          <code>{{ detail.legalProfile.registrationIdentifier }}</code>
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-compliance.enterprise.fields.country')"
        >
          {{ detail.legalProfile.registrationCountryCode }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-compliance.enterprise.fields.industry')"
        >
          {{ detail.legalProfile.industryCode || "—" }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-compliance.enterprise.fields.representative')"
        >
          {{ detail.legalProfile.legalRepresentativeName }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="
            $t('plugin.idc-compliance.enterprise.fields.representativeId')
          "
        >
          <code>{{ detail.legalProfile.legalRepresentativeId }}</code>
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-compliance.enterprise.fields.applicant')"
        >
          {{
            $t(
              `plugin.idc-compliance.enterprise.applicant.${detail.applicantRoleCode}`,
            )
          }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-compliance.enterprise.fields.submittedAt')"
        >
          {{ formatTimestamp(detail.submittedAt) }}
        </Descriptions.Item>
      </Descriptions>

      <Divider orientation="left">
        {{ $t("plugin.idc-compliance.enterprise.material.title") }}
      </Divider>
      <List
        v-if="detail.materials.length"
        :data-source="detail.materials"
        bordered
      >
        <template #renderItem="{ item }">
          <List.Item>
            <List.Item.Meta>
              <template #title>
                {{
                  $t(
                    `plugin.idc-compliance.enterprise.material.type.${item.materialTypeCode}`,
                  )
                }}
              </template>
              <template #description>
                <code>{{ item.materialNo }}</code> · {{ item.mediaType }} ·
                {{ sizeLabel(item.byteSize) }}
              </template>
            </List.Item.Meta>
            <Button
              v-if="canAccessMaterial"
              :loading="busyAction === item.materialNo"
              :data-testid="`enterprise-material-download-${item.materialNo}`"
              @click="downloadMaterial(item)"
            >
              {{ $t("plugin.idc-compliance.enterprise.material.download") }}
            </Button>
          </List.Item>
        </template>
      </List>
      <Empty
        v-else
        :description="$t('plugin.idc-compliance.enterprise.material.empty')"
      />

      <section v-if="canStart || canApprove || canReject" class="review-panel">
        <div>
          <h3>
            {{ $t("plugin.idc-compliance.enterprise.actions.title") }}
          </h3>
          <p>
            {{ $t("plugin.idc-compliance.enterprise.actions.boundary") }}
          </p>
        </div>
        <Space wrap>
          <Button
            v-if="canStart"
            type="primary"
            data-testid="enterprise-review-start"
            :loading="busyAction === 'review'"
            @click="runTransition('review')"
            >{{ $t("plugin.idc-compliance.enterprise.actions.review") }}</Button
          >
          <Button
            v-if="canApprove"
            type="primary"
            data-testid="enterprise-review-approve"
            :loading="busyAction === 'approve'"
            @click="runTransition('approve')"
            >{{
              $t("plugin.idc-compliance.enterprise.actions.approve")
            }}</Button
          >
          <template v-if="canReject">
            <Select
              v-model:value="rejectionReason"
              class="reason-select"
              data-testid="enterprise-rejection-reason"
              :placeholder="
                $t('plugin.idc-compliance.enterprise.actions.reasonPlaceholder')
              "
              :options="
                rejectionReasons.map((value) => ({
                  label: $t(`plugin.idc-compliance.enterprise.reason.${value}`),
                  value,
                }))
              "
            />
            <Button
              danger
              data-testid="enterprise-review-reject"
              :disabled="!rejectionReason"
              :loading="busyAction === 'reject'"
              @click="runReject"
              >{{
                $t("plugin.idc-compliance.enterprise.actions.reject")
              }}</Button
            >
          </template>
        </Space>
      </section>
    </div>
  </Drawer>
  <ConfirmationModal
    :confirm-text="confirmationText"
    :title="confirmationTitle"
    class="w-[520px]"
  >
    <Alert
      :message="confirmationDescription"
      show-icon
      :type="confirmationAction === 'reject' ? 'warning' : 'info'"
    />
  </ConfirmationModal>
</template>

<style scoped>
.detail-shell {
  display: grid;
  gap: 20px;
}
.detail-summary {
  align-items: flex-start;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 8px;
  color: hsl(var(--foreground));
  display: flex;
  justify-content: space-between;
  padding: 24px;
}
.detail-summary span {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.detail-summary h2 {
  font-size: 20px;
  margin: 6px 0;
}
.detail-summary code {
  color: hsl(var(--muted-foreground));
}
.review-panel {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 0;
  display: grid;
  gap: 14px;
  padding: 20px;
}
.review-panel h3 {
  font-size: 16px;
  font-weight: 700;
  margin: 0 0 4px;
}
.review-panel p {
  color: #64748b;
  margin: 0;
}
.reason-select {
  min-width: 260px;
}
</style>
