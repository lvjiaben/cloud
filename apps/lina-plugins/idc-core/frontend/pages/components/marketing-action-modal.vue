<script setup lang="ts">
import type {
  MarketingPointsAccountItem,
  MarketingPointsLotItem,
  MarketingPromotionAssetItem,
  MarketingValueGrantFinancialAdjustment,
  MarketingValueGrantItem,
} from "../marketing-client";
import type { VbenFormSchema } from "#/adapter/form";

import { computed, ref } from "vue";

import { useVbenModal } from "@vben/common-ui";

import { Alert, message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import {
  marketingAssetIssue,
  marketingAssetRevoke,
  marketingPointsCredit,
  marketingPointsReverse,
  marketingValueGrantAdjustmentReview,
  marketingValueGrantPostRequest,
  marketingValueGrantReverseRequest,
} from "../marketing-client";

type ActionKind =
  | "asset_issue"
  | "asset_revoke"
  | "grant_post"
  | "grant_reverse"
  | "grant_review"
  | "points_credit"
  | "points_reverse";

interface ModalData {
  action: ActionKind;
  asset?: MarketingPromotionAssetItem;
  benefitNo?: string;
  campaignNo?: string;
  lot?: MarketingPointsLotItem;
  pointsAccount?: MarketingPointsAccountItem;
  adjustment?: MarketingValueGrantFinancialAdjustment;
  grant?: MarketingValueGrantItem;
}

const emit = defineEmits<{ success: [action: ActionKind] }>();
const action = ref<ActionKind>("asset_issue");
const asset = ref<MarketingPromotionAssetItem>();
const lot = ref<MarketingPointsLotItem>();
const pointsAccount = ref<MarketingPointsAccountItem>();
const adjustment = ref<MarketingValueGrantFinancialAdjustment>();
const grant = ref<MarketingValueGrantItem>();
const requestNo = ref("");

function reasonOptions() {
  return [
    "customer_service",
    "campaign_fulfillment",
    "refund_correction",
    "risk_control",
    "manual_correction",
    "migration_reconciliation",
  ].map((value) => ({
    label: $t(`plugin.idc-core.marketing.actions.reason.${value}`),
    value,
  }));
}

const title = computed(() =>
  $t(`plugin.idc-core.marketing.actions.${action.value}.title`),
);
const maximumPoints = computed(() => {
  if (action.value !== "points_reverse" || !lot.value) return 1_000_000_000_000;
  return Math.max(1, lot.value.originalPoints - lot.value.reversedPoints);
});

function actionFormSchema(selectedAction: ActionKind): VbenFormSchema[] {
  const schema: VbenFormSchema[] = [];
  if (selectedAction === "asset_issue" || selectedAction === "points_credit") {
    schema.push({
      component: "Input" as const,
      componentProps: {
        "data-testid": "marketing-action-customer",
        maxlength: 128,
      },
      fieldName: "customerNo",
      label: $t("plugin.idc-core.marketing.assets.customer"),
      rules: z.string().trim().min(1).max(128),
    });
  }
  if (selectedAction === "asset_issue") {
    schema.push(
      {
        component: "Input" as const,
        componentProps: {
          "data-testid": "marketing-action-campaign",
          maxlength: 128,
        },
        fieldName: "campaignNo",
        label: $t("plugin.idc-core.marketing.actions.campaignNo"),
        rules: z.string().trim().min(1).max(128),
      },
      {
        component: "Input" as const,
        componentProps: {
          "data-testid": "marketing-action-benefit",
          maxlength: 128,
        },
        fieldName: "benefitNo",
        label: $t("plugin.idc-core.marketing.actions.benefitNo"),
        rules: z.string().trim().min(1).max(128),
      },
    );
  }
  if (selectedAction === "grant_reverse") {
    schema.push({
      component: "InputNumber" as const,
      componentProps: {
        "data-testid": "marketing-action-amount-minor",
        max: Math.max(1, grant.value?.availableAmountMinor ?? 1),
        min: 1,
        precision: 0,
      },
      fieldName: "amountMinor",
      label: $t("plugin.idc-core.marketing.actions.amountMinor"),
      rules: z
        .number()
        .int()
        .min(1)
        .max(Math.max(1, grant.value?.availableAmountMinor ?? 1)),
    });
  }
  if (selectedAction === "grant_review") {
    schema.push({
      component: "Select" as const,
      componentProps: {
        "data-testid": "marketing-action-decision",
        options: [
          {
            label: $t("plugin.idc-core.marketing.actions.approve"),
            value: "approve",
          },
          {
            label: $t("plugin.idc-core.marketing.actions.reject"),
            value: "reject",
          },
        ],
      },
      fieldName: "decision",
      label: $t("plugin.idc-core.marketing.actions.decision"),
      rules: z.enum(["approve", "reject"]),
    });
  }
  if (
    selectedAction === "points_credit" ||
    selectedAction === "points_reverse"
  ) {
    schema.push({
      component: "InputNumber" as const,
      componentProps: {
        "data-testid": "marketing-action-points",
        max: maximumPoints.value,
        min: 1,
        precision: 0,
      },
      fieldName: "points",
      label: $t("plugin.idc-core.marketing.actions.points"),
      rules: z.number().int().min(1).max(maximumPoints.value),
    });
  }
  schema.push({
    component: "Select" as const,
    componentProps: {
      "data-testid": "marketing-action-reason",
      options: reasonOptions(),
    },
    fieldName: "reasonCode",
    label: $t("plugin.idc-core.marketing.actions.reasonLabel"),
    rules: z.string().min(1),
  });
  return schema;
}

const [ActionForm, actionFormApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 130 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});

async function resetState() {
  asset.value = undefined;
  lot.value = undefined;
  pointsAccount.value = undefined;
  adjustment.value = undefined;
  grant.value = undefined;
  requestNo.value = "";
  await actionFormApi.resetForm();
}

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  onClosed: () => void resetState(),
  async onConfirm() {
    const { valid } = await actionFormApi.validate();
    if (!valid) return false;
    const model = await actionFormApi.getValues<Record<string, unknown>>();
    modalApi.lock(true);
    try {
      if (action.value === "asset_issue") {
        await marketingAssetIssue({
          benefitNo: String(model.benefitNo).trim(),
          campaignNo: String(model.campaignNo).trim(),
          customerNo: String(model.customerNo).trim(),
          reasonCode: String(model.reasonCode),
          requestNo: requestNo.value,
        });
      } else if (action.value === "asset_revoke" && asset.value) {
        await marketingAssetRevoke(asset.value.assetNo, {
          expectedUpdatedAt: asset.value.updatedAt,
          reasonCode: String(model.reasonCode),
          requestNo: requestNo.value,
        });
      } else if (action.value === "points_credit") {
        await marketingPointsCredit(String(model.customerNo).trim(), {
          points: Number(model.points),
          reasonCode: String(model.reasonCode),
          requestNo: requestNo.value,
        });
      } else if (
        action.value === "points_reverse" &&
        pointsAccount.value &&
        lot.value
      ) {
        await marketingPointsReverse(pointsAccount.value.customerNo, {
          awardSourceReference: lot.value.sourceReference,
          awardSourceType: lot.value.sourceType,
          points: Number(model.points),
          reasonCode: String(model.reasonCode),
          requestNo: requestNo.value,
        });
      } else if (action.value === "grant_post" && grant.value) {
        await marketingValueGrantPostRequest(grant.value.grantNo, {
          reasonCode: String(model.reasonCode),
          requestNo: requestNo.value,
        });
      } else if (action.value === "grant_reverse" && grant.value) {
        await marketingValueGrantReverseRequest(grant.value.grantNo, {
          amountMinor: String(model.amountMinor),
          reasonCode: String(model.reasonCode),
          requestNo: requestNo.value,
        });
      } else if (action.value === "grant_review" && adjustment.value) {
        await marketingValueGrantAdjustmentReview(
          adjustment.value.adjustmentNo,
          {
            decision: String(model.decision) as "approve" | "reject",
            reasonCode: String(model.reasonCode),
          },
        );
      } else {
        throw new Error("marketing action context is incomplete");
      }
      message.success($t("plugin.idc-core.marketing.actions.success"));
      emit("success", action.value);
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    await resetState();
    const data = modalApi.getData<ModalData>();
    action.value = data.action;
    asset.value = data.asset;
    lot.value = data.lot;
    pointsAccount.value = data.pointsAccount;
    adjustment.value = data.adjustment;
    grant.value = data.grant;
    const customerNo =
      data.pointsAccount?.customerNo ?? data.asset?.customerNo ?? "";
    const points = data.lot
      ? Math.max(1, data.lot.originalPoints - data.lot.reversedPoints)
      : 1;
    const amountMinor = Math.max(1, data.grant?.availableAmountMinor ?? 1);
    const reasonCode =
      action.value === "asset_revoke"
        ? "risk_control"
        : action.value === "points_reverse"
          ? "refund_correction"
          : action.value === "grant_reverse"
            ? "refund_correction"
            : action.value === "grant_review"
              ? "manual_correction"
              : action.value === "grant_post"
                ? "campaign_fulfillment"
                : action.value === "points_credit"
                  ? "manual_correction"
                  : "customer_service";
    requestNo.value = `marketing-${action.value}-${crypto.randomUUID()}`;
    await actionFormApi.setState({ schema: actionFormSchema(action.value) });
    await actionFormApi.setValues({
      amountMinor,
      benefitNo: data.benefitNo ?? "",
      campaignNo: data.campaignNo ?? "",
      customerNo,
      decision: "approve",
      points,
      reasonCode,
    });
  },
});
</script>

<template>
  <Modal :title="title" class="w-[620px]">
    <Alert
      class="mb-5"
      show-icon
      type="info"
      :message="$t('plugin.idc-core.marketing.actions.auditTitle')"
      :description="$t('plugin.idc-core.marketing.actions.auditNote')"
    />
    <div
      v-if="action !== 'asset_issue' && action !== 'points_credit'"
      class="mb-4 rounded border p-3"
    >
      <div>
        {{ $t("plugin.idc-core.marketing.actions.target") }}：
        <code>{{
          asset?.assetNo ||
          pointsAccount?.customerNo ||
          grant?.grantNo ||
          adjustment?.adjustmentNo
        }}</code>
      </div>
      <div v-if="action === 'points_reverse'" class="mt-2">
        {{ $t("plugin.idc-core.marketing.actions.source") }}：
        <code>{{ lot?.sourceType }} / {{ lot?.sourceReference }}</code>
      </div>
    </div>
    <ActionForm />
  </Modal>
</template>
