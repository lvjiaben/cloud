<script setup lang="ts">
import type { ProviderChannelHealth } from "../health-client";

import { onMounted } from "vue";

import { useVbenModal } from "@vben/common-ui";

import { Alert, message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { providerHealthPolicyUpdate } from "../health-client";

const emit = defineEmits<{ success: [item: ProviderChannelHealth] }>();

function schema() {
  const numberField = (
    fieldName: string,
    label: string,
    minimum: number,
    maximum: number,
  ) => ({
    component: "InputNumber" as const,
    componentProps: { class: "w-full", max: maximum, min: minimum },
    fieldName,
    label,
    rules: z.number().int().min(minimum).max(maximum),
  });
  return [
    numberField(
      "windowSampleLimit",
      $t("plugin.idc-resource.health.policy.window"),
      5,
      100,
    ),
    numberField(
      "minimumSamplesToProtect",
      $t("plugin.idc-resource.health.policy.minimum"),
      3,
      100,
    ),
    numberField(
      "failureLimit",
      $t("plugin.idc-resource.health.policy.failures"),
      1,
      100,
    ),
    numberField(
      "unknownResultLimit",
      $t("plugin.idc-resource.health.policy.unknown"),
      1,
      100,
    ),
    numberField(
      "consecutiveFailureLimit",
      $t("plugin.idc-resource.health.policy.consecutive"),
      1,
      100,
    ),
    numberField(
      "protectionDurationSeconds",
      $t("plugin.idc-resource.health.policy.duration"),
      60,
      86400,
    ),
  ];
}

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 160 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1 md:grid-cols-2",
});

onMounted(() => formApi.setState({ schema: schema() }));

const [Modal, modalApi] = useVbenModal({
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const item = modalApi.getData()?.item as ProviderChannelHealth;
    const values = await formApi.getValues();
    if (
      Number(values.minimumSamplesToProtect) >
        Number(values.windowSampleLimit) ||
      Number(values.failureLimit) > Number(values.windowSampleLimit) ||
      Number(values.unknownResultLimit) > Number(values.failureLimit) ||
      Number(values.consecutiveFailureLimit) > Number(values.windowSampleLimit)
    ) {
      message.error($t("plugin.idc-resource.health.policy.invalid"));
      return false;
    }
    modalApi.lock(true);
    try {
      const result = await providerHealthPolicyUpdate(item.channelNo, {
        consecutiveFailureLimit: Number(values.consecutiveFailureLimit),
        expectedVersion: item.policy.version,
        failureLimit: Number(values.failureLimit),
        minimumSamplesToProtect: Number(values.minimumSamplesToProtect),
        protectionDurationSeconds: Number(values.protectionDurationSeconds),
        unknownResultLimit: Number(values.unknownResultLimit),
        windowSampleLimit: Number(values.windowSampleLimit),
      });
      message.success(
        $t("plugin.idc-resource.health.policy.succeeded"),
      );
      emit("success", result.item);
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    const item = modalApi.getData()?.item as ProviderChannelHealth;
    await formApi.setValues({ ...item.policy });
  },
});
</script>

<template>
  <Modal
    class="w-[760px]"
    :confirm-text="$t('plugin.idc-resource.health.policy.confirm')"
    :title="$t('plugin.idc-resource.health.policy.title')"
  >
    <div class="grid gap-5" data-testid="provider-health-policy-modal">
      <Alert
        :description="$t('plugin.idc-resource.health.policy.boundary')"
        :message="$t('plugin.idc-resource.health.policy.safety')"
        show-icon
        type="warning"
      />
      <Form />
    </div>
  </Modal>
</template>
