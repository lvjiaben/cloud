<script setup lang="ts">
import type { AutomationInterface } from "../catalog-automation-client";
import type { VbenFormProps } from "@vben/common-ui";

import { watch } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { automationInterfaceRename } from "../catalog-automation-client";

const emit = defineEmits<{ success: [] }>();
let currentInterface: AutomationInterface | undefined;

function formSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Input",
      componentProps: {
        "data-testid": "automation-interface-rename-name",
        maxlength: 128,
      },
      fieldName: "displayName",
      label: $t("plugin.idc-core.catalogAutomation.rename.name"),
      rules: z.string().trim().min(1).max(128),
    },
  ];
}

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "idc-plugin-form-surface",
});

watch(
  () => preferences.app.locale,
  () => formApi.setState({ schema: formSchema() }),
);

const [Modal, modalApi] = useVbenModal({
  async onClosed() {
    currentInterface = undefined;
    await formApi.resetForm();
  },
  async onConfirm() {
    if (!currentInterface) return false;
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const values = await formApi.getValues<{ displayName: string }>();
    modalApi.lock(true);
    try {
      await automationInterfaceRename(currentInterface.interfaceInstanceNo, {
        displayName: values.displayName.trim(),
        expectedVersion: currentInterface.version,
      });
      message.success($t("plugin.idc-core.catalogAutomation.rename.succeeded"));
      emit("success");
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    currentInterface = modalApi.getData<AutomationInterface>();
    if (!currentInterface) return;
    await formApi.setState({ schema: formSchema() });
    await formApi.setValues({ displayName: currentInterface.displayName });
  },
});
</script>

<template>
  <Modal
    :confirm-text="$t('plugin.idc-core.catalogAutomation.rename.confirm')"
    :title="$t('plugin.idc-core.catalogAutomation.rename.title')"
    class="w-[620px]"
  >
    <div data-testid="automation-interface-rename-modal">
      <div class="mb-4 rounded-md bg-muted p-3 text-sm text-muted-foreground">
        {{ $t("plugin.idc-core.catalogAutomation.rename.note") }}
      </div>
      <Form />
    </div>
  </Modal>
</template>
