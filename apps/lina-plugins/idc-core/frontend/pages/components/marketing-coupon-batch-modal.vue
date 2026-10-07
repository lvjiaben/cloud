<script setup lang="ts">
import { useVbenModal } from "@vben/common-ui";

import { message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { marketingCouponBatchGenerate } from "../marketing-client";

const emit = defineEmits<{ success: [] }>();
let requestNo = "";

function formSchema() {
  return [
    {
      component: "Input" as const,
      componentProps: { "data-testid": "marketing-coupon-batch-campaign" },
      fieldName: "campaignNo",
      label: $t("plugin.idc-core.marketing.couponBatch.campaignNo"),
      rules: z.string().trim().min(1).max(40),
    },
    {
      component: "Input" as const,
      componentProps: { "data-testid": "marketing-coupon-batch-benefit" },
      fieldName: "benefitNo",
      label: $t("plugin.idc-core.marketing.couponBatch.benefitNo"),
      rules: z.string().trim().min(1).max(40),
    },
    {
      component: "InputNumber" as const,
      componentProps: {
        "data-testid": "marketing-coupon-batch-count",
        max: 1000,
        min: 1,
      },
      defaultValue: 100,
      fieldName: "count",
      label: $t("plugin.idc-core.marketing.couponBatch.count"),
      rules: z.number().int().min(1).max(1000),
    },
    {
      component: "InputNumber" as const,
      componentProps: {
        "data-testid": "marketing-coupon-batch-length",
        max: 24,
        min: 8,
      },
      defaultValue: 12,
      fieldName: "codeLength",
      label: $t("plugin.idc-core.marketing.couponBatch.codeLength"),
      rules: z.number().int().min(8).max(24),
    },
  ];
}

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
});

async function resetState() {
  requestNo = `coupon-batch-${crypto.randomUUID()}`;
  await formApi.resetForm();
}

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  onClosed: () => void resetState(),
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const model = await formApi.getValues<{
      benefitNo: string;
      campaignNo: string;
      codeLength: number;
      count: number;
    }>();
    modalApi.lock(true);
    try {
      const blob = await marketingCouponBatchGenerate({
        benefitNo: model.benefitNo.trim(),
        campaignNo: model.campaignNo.trim(),
        codeLength: model.codeLength,
        count: model.count,
        requestNo,
      });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `coupon-codes-${requestNo}.xlsx`;
      anchor.style.display = "none";
      document.body.append(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 0);
      message.success(
        $t("plugin.idc-core.marketing.couponBatch.downloaded"),
      );
      emit("success");
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    await formApi.setState({ schema: formSchema() });
    await resetState();
    await formApi.setValues({ codeLength: 12, count: 100 });
  },
});
</script>

<template>
  <Modal
    :title="$t('plugin.idc-core.marketing.couponBatch.title')"
    class="w-[560px]"
  >
    <p class="mb-4 text-sm text-gray-500">
      {{ $t("plugin.idc-core.marketing.couponBatch.warning") }}
    </p>
    <Form />
  </Modal>
</template>
