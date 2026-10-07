<script setup lang="ts">
import type { AutomationProviderCandidate } from "../catalog-automation-client";
import type { VbenFormProps } from "@vben/common-ui";

import { computed, ref, watch } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Button, message, Pagination } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import {
  automationInterfaceCreate,
  automationProviderList,
} from "../catalog-automation-client";
import type { CatalogConfigurationValues } from "../catalog-configuration-state";
import {
  catalogConfigurationPresentation,
  catalogConfigurationValuesToRows,
} from "../catalog-configuration-state";
import CatalogConfigurationForm from "./catalog-configuration-form.vue";
import VbenDrawerCompat from "./vben-drawer-compat.vue";

const emit = defineEmits<{ success: [] }>();
const PROVIDER_PAGE_SIZE = 20;
const providers = ref<AutomationProviderCandidate[]>([]);
const providerPage = ref(1);
const providerTotal = ref(0);
const selectedProviderToken = ref("");
const configurationValues = ref<CatalogConfigurationValues>({});
const complexOpen = ref(false);
const creating = ref(false);
const pendingBase = ref<{ displayName: string; selectionToken: string }>();
const configurationFormRef =
  ref<InstanceType<typeof CatalogConfigurationForm>>();
let providerLoadSequence = 0;
const selectedProvider = computed(() =>
  providers.value.find(
    (provider) => provider.selectionToken === selectedProviderToken.value,
  ),
);
const isComplexConfiguration = computed(
  () =>
    selectedProvider.value?.interfaceConfiguration &&
    catalogConfigurationPresentation(
      selectedProvider.value.interfaceConfiguration,
    ) === "drawer",
);

function syncConfirmAvailability(selectionToken = selectedProviderToken.value) {
  const provider = providers.value.find(
    (item) => item.selectionToken === selectionToken,
  );
  modalApi.setState({
    confirmDisabled: !provider || !provider.interfaceConfiguration,
  });
}

async function createInterface(values: {
  displayName: string;
  selectionToken: string;
}) {
  creating.value = true;
  modalApi.lock(true);
  try {
    await automationInterfaceCreate({
      configuration: selectedProvider.value?.interfaceConfiguration
        ? catalogConfigurationValuesToRows(
            selectedProvider.value.interfaceConfiguration,
            "interface",
            configurationValues.value,
          )
        : [],
      displayName: values.displayName.trim(),
      selectionToken: values.selectionToken,
    });
    message.success($t("plugin.idc-core.catalogAutomation.create.succeeded"));
    emit("success");
    complexOpen.value = false;
    modalApi.close();
  } finally {
    creating.value = false;
    modalApi.lock(false);
  }
}

async function submitComplexConfiguration() {
  if (!pendingBase.value || creating.value) return;
  await configurationFormRef.value?.validate();
  await createInterface(pendingBase.value);
}

async function loadProviders(page: number) {
  const sequence = ++providerLoadSequence;
  const result = await automationProviderList(page, PROVIDER_PAGE_SIZE);
  if (sequence !== providerLoadSequence) return;
  const configurableProviders = result.items.filter(
    (provider) => provider.interfaceConfiguration,
  );
  providerPage.value = page;
  providerTotal.value = configurableProviders.length;
  providers.value = configurableProviders;
  await formApi.setFieldValue("selectionToken", undefined);
  await formApi.setState({ schema: formSchema() });
  modalApi.setState({ confirmDisabled: true });
}

function formSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Select",
      componentProps: {
        "data-testid": "automation-interface-provider",
        options: providers.value.map((provider) => ({
          label: provider.displayName,
          value: provider.selectionToken,
        })),
        placeholder: $t(
          "plugin.idc-core.catalogAutomation.create.providerPlaceholder",
        ),
        showSearch: true,
        onChange: (value: string) => {
          selectedProviderToken.value = value;
          configurationValues.value = {};
          syncConfirmAvailability(value);
        },
      },
      fieldName: "selectionToken",
      label: $t("plugin.idc-core.catalogAutomation.create.provider"),
      rules: z.preprocess(
        (value) => value ?? "",
        z.string().min(1, {
          message: $t(
            "plugin.idc-core.catalogAutomation.create.providerRequired",
          ),
        }),
      ),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "automation-interface-display-name",
        maxlength: 128,
        placeholder: $t(
          "plugin.idc-core.catalogAutomation.create.namePlaceholder",
        ),
      },
      fieldName: "displayName",
      label: $t("plugin.idc-core.catalogAutomation.create.name"),
      rules: z.preprocess(
        (value) => value ?? "",
        z
          .string()
          .trim()
          .min(1, {
            message: $t(
              "plugin.idc-core.catalogAutomation.create.nameRequired",
            ),
          })
          .max(128),
      ),
    },
  ];
}

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 118 },
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
    providerLoadSequence += 1;
    providers.value = [];
    providerPage.value = 1;
    providerTotal.value = 0;
    selectedProviderToken.value = "";
    configurationValues.value = {};
    complexOpen.value = false;
    pendingBase.value = undefined;
    modalApi.setState({ confirmDisabled: true });
    await formApi.resetForm();
  },
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const values = await formApi.getValues<{
      displayName: string;
      selectionToken: string;
    }>();
    if (
      !providers.value.some(
        (provider) => provider.selectionToken === values.selectionToken,
      )
    )
      return false;
    if (isComplexConfiguration.value) {
      pendingBase.value = values;
      complexOpen.value = true;
      return false;
    }
    await configurationFormRef.value?.validate();
    await createInterface(values);
  },
  async onOpenChange(open) {
    if (!open) return;
    modalApi.setState({ confirmDisabled: true });
    await loadProviders(1);
  },
});
</script>

<template>
  <Modal
    :confirm-text="$t('plugin.idc-core.catalogAutomation.create.confirm')"
    :title="$t('plugin.idc-core.catalogAutomation.create.title')"
    class="w-[620px]"
  >
    <div data-testid="automation-interface-create-modal">
      <div class="mb-4 rounded-md bg-muted p-3 text-sm text-muted-foreground">
        {{ $t("plugin.idc-core.catalogAutomation.create.note") }}
      </div>
      <Form />
      <CatalogConfigurationForm
        v-if="
          selectedProvider?.interfaceConfiguration && !isComplexConfiguration
        "
        ref="configurationFormRef"
        v-model="configurationValues"
        class="mt-4"
        :declaration="selectedProvider.interfaceConfiguration"
        surface="interface"
      />
      <div
        v-if="providerTotal > PROVIDER_PAGE_SIZE"
        class="mb-4 flex justify-end"
      >
        <Pagination
          data-testid="automation-interface-provider-pagination"
          :current="providerPage"
          :page-size="PROVIDER_PAGE_SIZE"
          :show-size-changer="false"
          :total="providerTotal"
          @change="loadProviders"
        />
      </div>
      <div
        v-if="providers.length === 0"
        class="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-200"
      >
        {{ $t("plugin.idc-core.catalogAutomation.create.noProviders") }}
      </div>
    </div>
  </Modal>
  <VbenDrawerCompat
    v-if="isComplexConfiguration && selectedProvider?.interfaceConfiguration"
    v-model:open="complexOpen"
    :title="$t('plugin.idc-core.catalogAutomation.configuration.title')"
  >
    <CatalogConfigurationForm
      ref="configurationFormRef"
      v-model="configurationValues"
      :declaration="selectedProvider.interfaceConfiguration"
      surface="interface"
    />
    <div class="mt-4 flex justify-end gap-2">
      <Button class="mr-2" :disabled="creating" @click="complexOpen = false">
        {{ $t("plugin.idc-core.catalogAutomation.configuration.back") }}
      </Button>
      <Button
        :loading="creating"
        type="primary"
        @click="submitComplexConfiguration"
      >
        {{
          $t("plugin.idc-core.catalogAutomation.configuration.saveAndCreate")
        }}
      </Button>
    </div>
  </VbenDrawerCompat>
</template>
