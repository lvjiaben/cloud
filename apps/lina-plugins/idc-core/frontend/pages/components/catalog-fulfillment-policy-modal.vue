<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type { CatalogFulfillmentStrategy } from "../catalog-fulfillment-policy-state";

import { ref, watch } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { catalogFulfillmentPolicyUpdate } from "../catalog-client";
import {
  catalogFulfillmentStrategies,
  fulfillmentStrategyRequiresBinding,
} from "../catalog-fulfillment-policy-state";

const emit = defineEmits<{ success: [] }>();
const versionNo = ref("");
const hasBinding = ref(false);
const submitting = ref(false);

function formSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Select",
      componentProps: {
        "data-testid": "catalog-fulfillment-policy-strategy",
        options: catalogFulfillmentStrategies.map((value) => ({
          disabled:
            fulfillmentStrategyRequiresBinding(value) && !hasBinding.value,
          label: $t(`plugin.idc-core.catalogFulfillment.strategy.${value}`),
          value,
        })),
      },
      fieldName: "strategyCode",
      label: $t("plugin.idc-core.catalogFulfillment.field.strategy"),
      rules: z.enum([
        "manual",
        "approval_then_plugin",
        "payment_confirmed_plugin",
      ]),
    },
  ];
}

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 128 },
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
    versionNo.value = "";
    hasBinding.value = false;
    submitting.value = false;
    modalApi.setState({ confirmDisabled: false });
    await formApi.resetForm();
  },
  async onConfirm() {
    if (submitting.value) return false;
    submitting.value = true;
    modalApi.setState({ confirmDisabled: true });
    modalApi.lock(true);
    try {
      const { valid } = await formApi.validate();
      if (!valid || !versionNo.value) return false;
      const values = await formApi.getValues<{
        strategyCode: CatalogFulfillmentStrategy;
      }>();
      if (
        fulfillmentStrategyRequiresBinding(values.strategyCode) &&
        !hasBinding.value
      )
        return false;
      await catalogFulfillmentPolicyUpdate(
        versionNo.value,
        values.strategyCode,
      );
      message.success($t("plugin.idc-core.catalogFulfillment.saved"));
      emit("success");
      modalApi.close();
    } finally {
      submitting.value = false;
      modalApi.setState({ confirmDisabled: false });
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{
      currentStrategy?: CatalogFulfillmentStrategy;
      hasBinding?: boolean;
      versionNo?: string;
    }>();
    versionNo.value = data?.versionNo || "";
    hasBinding.value = Boolean(data?.hasBinding);
    submitting.value = false;
    modalApi.setState({ confirmDisabled: false });
    await formApi.setState({ schema: formSchema() });
    await formApi.setFieldValue(
      "strategyCode",
      data?.currentStrategy || "manual",
    );
  },
});
</script>

<template>
  <Modal
    :confirm-disabled="submitting"
    :confirm-loading="submitting"
    :confirm-text="$t('plugin.idc-core.catalogFulfillment.confirm')"
    :title="$t('plugin.idc-core.catalogFulfillment.title')"
    class="w-[640px]"
  >
    <div data-testid="catalog-fulfillment-policy-modal">
      <div class="mb-4 rounded-md bg-muted p-3 text-sm text-muted-foreground">
        {{ $t("plugin.idc-core.catalogFulfillment.note") }}
      </div>
      <Form />
      <div
        v-if="!hasBinding"
        class="mt-3 rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-200"
      >
        {{ $t("plugin.idc-core.catalogFulfillment.bindFirst") }}
      </div>
    </div>
  </Modal>
</template>
