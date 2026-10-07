<script setup lang="ts">
import type {
  ProviderCapacityChannelOption,
  ProviderCapacityKind,
} from "../provider-capacity-client";

import { computed, onMounted } from "vue";

import { useVbenModal } from "@vben/common-ui";

import { Alert, message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { providerCapacitySyncCreate } from "../provider-capacity-client";

const emit = defineEmits<{ success: [observationNo: string] }>();

const channels = computed<ProviderCapacityChannelOption[]>(() =>
  Array.isArray(modalApi.getData()?.channels) ? modalApi.getData().channels : [],
);

function buildFormSchema() {
  return [
    {
      component: "Select" as const,
      componentProps: () => ({
        "data-testid": "provider-capacity-sync-channel",
        options: channels.value.map((channel) => ({
          label: channel.displayName,
          value: channel.channelNo,
        })),
        placeholder: $t(
          "plugin.idc-resource.providerCapacity.sync.channelPlaceholder",
        ),
      }),
      fieldName: "channelNo",
      label: $t("plugin.idc-resource.providerCapacity.sync.channel"),
      rules: z.string().trim().min(1).max(64),
    },
    {
      component: "RadioGroup" as const,
      componentProps: {
        "data-testid": "provider-capacity-sync-kind",
        buttonStyle: "solid",
        optionType: "button",
        options: ["region", "line", "plan", "image", "stock"].map(
          (value) => ({
            label: $t(
              `plugin.idc-resource.providerCapacity.kind.${value}`,
            ),
            value,
          }),
        ),
      },
      defaultValue: "plan",
      fieldName: "kind",
      label: $t("plugin.idc-resource.providerCapacity.sync.kind"),
      rules: z.enum(["region", "line", "plan", "image", "stock"]),
    },
  ];
}

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 104 },
  schema: [],
  showDefaultActions: false,
});

onMounted(() => {
  formApi.setState({ schema: buildFormSchema() });
});

const [Modal, modalApi] = useVbenModal({
  async onClosed() {
    await formApi.resetForm();
  },
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const values = await formApi.getValues<{
      channelNo: string;
      kind: ProviderCapacityKind;
    }>();
    modalApi.lock(true);
    try {
      const result = await providerCapacitySyncCreate(values.channelNo, {
        kind: values.kind,
      });
      message.success(
        $t("plugin.idc-resource.providerCapacity.sync.succeeded"),
      );
      emit("success", result.observationNo);
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData() as {
      channels?: ProviderCapacityChannelOption[];
      selectedChannelNo?: string;
    };
    await formApi.setValues({
      channelNo: data.selectedChannelNo || data.channels?.[0]?.channelNo || "",
      kind: "plan",
    });
  },
});
</script>

<template>
  <Modal
    class="w-[680px]"
    :confirm-text="$t('plugin.idc-resource.providerCapacity.sync.confirm')"
    :title="$t('plugin.idc-resource.providerCapacity.sync.title')"
  >
    <div class="sync-modal" data-testid="provider-capacity-sync-modal">
      <Alert
        :description="$t('plugin.idc-resource.providerCapacity.sync.boundaryDescription')"
        show-icon
        :message="$t('plugin.idc-resource.providerCapacity.sync.boundaryTitle')"
        type="info"
      />
      <Form />
    </div>
  </Modal>
</template>

<style scoped>
.sync-modal {
  display: grid;
  gap: 20px;
}

</style>
