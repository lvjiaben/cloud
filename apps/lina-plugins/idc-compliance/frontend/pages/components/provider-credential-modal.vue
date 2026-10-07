<script setup lang="ts">
import type { ProviderConfiguration } from "../provider-configuration-client";

import { onMounted, ref } from "vue";

import { useVbenModal } from "@vben/common-ui";

import { Alert, message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import {
  providerConfigurationRecover,
  providerConfigurationUpdate,
} from "../provider-configuration-client";

type CredentialMode = "configure" | "recover";

const emit = defineEmits<{ success: [item: ProviderConfiguration] }>();
const mode = ref<CredentialMode>("configure");
const configuration = ref<ProviderConfiguration | null>(null);
const operationKey = ref("");

function formSchema(selectedMode: CredentialMode) {
  const result = [];
  if (selectedMode === "configure") {
    result.push({
      component: "Input" as const,
      componentProps: {
        "data-testid": "aliyun-provider-scene-id",
        maxlength: 64,
        placeholder: $t(
          "plugin.idc-compliance.provider.form.scenePlaceholder",
        ),
      },
      fieldName: "sceneId",
      label: $t("plugin.idc-compliance.provider.form.sceneId"),
      rules: z.string().regex(/^\d{1,64}$/, {
        message: $t("plugin.idc-compliance.provider.form.sceneInvalid"),
      }),
    });
  }
  result.push(
    {
      component: "Input" as const,
      componentProps: {
        "data-testid": "aliyun-provider-access-key-id",
        autocomplete: "off",
        maxlength: 256,
        placeholder: $t(
          "plugin.idc-compliance.provider.form.accessKeyIdPlaceholder",
        ),
      },
      fieldName: "accessKeyId",
      label: $t("plugin.idc-compliance.provider.form.accessKeyId"),
      rules: z.string().min(8).max(256),
    },
    {
      component: "InputPassword" as const,
      componentProps: {
        "data-testid": "aliyun-provider-access-key-secret",
        autocomplete: "new-password",
        maxlength: 512,
        placeholder: $t(
          "plugin.idc-compliance.provider.form.accessKeySecretPlaceholder",
        ),
      },
      fieldName: "accessKeySecret",
      label: $t("plugin.idc-compliance.provider.form.accessKeySecret"),
      rules: z.string().min(16).max(512),
    },
  );
  return result;
}

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 138 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});

onMounted(() => formApi.setState({ schema: formSchema(mode.value) }));

const [Modal, modalApi] = useVbenModal({
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const values = await formApi.getValues();
    modalApi.lock(true);
    try {
      if (mode.value === "recover") {
        const result = await providerConfigurationRecover({
          accessKeyId: String(values.accessKeyId),
          accessKeySecret: String(values.accessKeySecret),
        });
        message.success(
          $t(
            result.reusedExistingMutation
              ? "plugin.idc-compliance.provider.recovery.reused"
              : "plugin.idc-compliance.provider.recovery.completed",
          ),
        );
        emit("success", result.item);
      } else {
        const current = configuration.value;
        const result = await providerConfigurationUpdate({
          accessKeyId: String(values.accessKeyId),
          accessKeySecret: String(values.accessKeySecret),
          expectedVersion: current?.version ?? 0,
          idempotencyKey: operationKey.value,
          sceneId: String(values.sceneId),
        });
        message.success(
          $t("plugin.idc-compliance.provider.form.saved"),
        );
        emit("success", result.item);
      }
      await formApi.resetForm();
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) {
      await formApi.resetForm();
      operationKey.value = "";
      configuration.value = null;
      return;
    }
    const data = modalApi.getData<{
      configuration: ProviderConfiguration;
      mode: CredentialMode;
    }>();
    configuration.value = data.configuration;
    mode.value = data.mode;
    operationKey.value = `aliyun-face-config-${crypto.randomUUID()}`;
    await formApi.setState({ schema: formSchema(mode.value) });
    await formApi.resetForm();
    if (mode.value === "configure" && configuration.value.sceneId) {
      await formApi.setValues({ sceneId: configuration.value.sceneId });
    }
  },
});
</script>

<template>
  <Modal
    class="w-[720px]"
    :confirm-text="
      $t(
        mode === 'recover'
          ? 'plugin.idc-compliance.provider.recovery.confirm'
          : 'plugin.idc-compliance.provider.form.confirm',
      )
    "
    :title="
      $t(
        mode === 'recover'
          ? 'plugin.idc-compliance.provider.recovery.title'
          : configuration?.credentialConfigured
            ? 'plugin.idc-compliance.provider.form.replaceTitle'
            : 'plugin.idc-compliance.provider.form.title',
      )
    "
  >
    <div
      class="credential-dialog"
      data-testid="aliyun-provider-credential-modal"
    >
      <div class="dialog-steps" aria-hidden="true">
        <span class="active">01</span><i></i><span class="active">02</span
        ><i></i><span>03</span>
      </div>
      <Alert
        :description="
          $t(
            mode === 'recover'
              ? 'plugin.idc-compliance.provider.recovery.description'
              : 'plugin.idc-compliance.provider.form.description',
          )
        "
        :message="
          $t(
            mode === 'recover'
              ? 'plugin.idc-compliance.provider.recovery.safety'
              : 'plugin.idc-compliance.provider.form.safety',
          )
        "
        show-icon
        :type="mode === 'recover' ? 'warning' : 'info'"
      />
      <Form />
      <p class="credential-footnote">
        {{ $t("plugin.idc-compliance.provider.form.reauth") }}
      </p>
    </div>
  </Modal>
</template>

<style scoped>
.credential-dialog {
  display: grid;
  gap: 20px;
}
.dialog-steps {
  align-items: center;
  color: rgb(100 116 139);
  display: flex;
  font-family: ui-monospace, monospace;
  font-size: 11px;
  font-weight: 800;
  gap: 10px;
  letter-spacing: 0.12em;
}
.dialog-steps span {
  align-items: center;
  border: 1px solid rgb(148 163 184 / 45%);
  border-radius: 50%;
  display: flex;
  height: 28px;
  justify-content: center;
  width: 28px;
}
.dialog-steps span.active {
  background: rgb(15 118 110);
  border-color: rgb(15 118 110);
  color: white;
}
.dialog-steps i {
  background: rgb(203 213 225);
  height: 1px;
  width: 34px;
}
.credential-footnote {
  border-left: 3px solid rgb(15 118 110);
  color: rgb(71 85 105);
  font-size: 12px;
  line-height: 1.7;
  margin: 0;
  padding-left: 12px;
}
</style>
