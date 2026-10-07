<script setup lang="ts">
import type {
  AutomationInterface,
  AutomationProviderCandidate,
} from "../catalog-automation-client";
import type { VbenFormProps } from "@vben/common-ui";

import { ref, watch } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { message, Pagination } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import {
  automationInterfaceList,
  automationProviderList,
  productFulfillmentBindingPut,
} from "../catalog-automation-client";

const emit = defineEmits<{ success: [] }>();
const PROVIDER_PAGE_SIZE = 20;
const INTERFACE_PAGE_SIZE = 20;
const interfaces = ref<AutomationInterface[]>([]);
const providers = ref<AutomationProviderCandidate[]>([]);
const selectedProviderToken = ref("");
const providerPage = ref(1);
const providerTotal = ref(0);
const interfacePage = ref(1);
const interfaceTotal = ref(0);
let currentVersionNo = "";
let currentNamePrefix = "";
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let providerLoadSequence = 0;
let interfaceLoadSequence = 0;

function optionLabel(item: AutomationInterface) {
  return `${item.displayName} · ${formatTimestamp(item.changedAt)}`;
}

async function loadInterfaces(page = 1, namePrefix = currentNamePrefix) {
  if (!selectedProviderToken.value) {
    interfaces.value = [];
    interfaceTotal.value = 0;
    await formApi.setFieldValue("interfaceInstanceNo", undefined, false);
    await formApi.setState({ schema: formSchema() });
    return;
  }
  const sequence = ++interfaceLoadSequence;
  const result = await automationInterfaceList({
    namePrefix: namePrefix?.trim() || undefined,
    newBusinessState: "accepting",
    page,
    pageSize: INTERFACE_PAGE_SIZE,
    providerSelectionToken: selectedProviderToken.value,
  });
  if (sequence !== interfaceLoadSequence) return;
  currentNamePrefix = namePrefix.trim();
  interfacePage.value = page;
  interfaceTotal.value = result.total;
  interfaces.value = result.items;
  await formApi.setFieldValue("interfaceInstanceNo", undefined, false);
  await formApi.setState({ schema: formSchema() });
}

async function handleProviderChange(value?: string) {
  selectedProviderToken.value = value || "";
  currentNamePrefix = "";
  interfacePage.value = 1;
  await loadInterfaces(1, "");
}

async function loadProviders(page = 1) {
  const sequence = ++providerLoadSequence;
  const result = await automationProviderList(page, PROVIDER_PAGE_SIZE);
  if (sequence !== providerLoadSequence) return;
  providerPage.value = page;
  providerTotal.value = result.total;
  providers.value = result.items;
  selectedProviderToken.value = "";
  currentNamePrefix = "";
  interfacePage.value = 1;
  interfaceTotal.value = 0;
  interfaces.value = [];
  await formApi.setFieldValue("providerSelectionToken", undefined, false);
  await formApi.setFieldValue("interfaceInstanceNo", undefined, false);
  await formApi.setState({ schema: formSchema() });
}

function handleSearch(value: string) {
  if (searchTimer) clearTimeout(searchTimer);
  currentNamePrefix = value.trim();
  searchTimer = setTimeout(
    () => void loadInterfaces(1, currentNamePrefix),
    250,
  );
}

function handlePageChange(page: number) {
  return loadInterfaces(page, currentNamePrefix);
}

function formSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "Select",
      componentProps: {
        "data-testid": "catalog-fulfillment-provider",
        onChange: handleProviderChange,
        options: providers.value.map((provider) => ({
          label: provider.displayName,
          value: provider.selectionToken,
        })),
        placeholder: $t(
          "plugin.idc-core.catalogAutomation.binding.providerPlaceholder",
        ),
        showSearch: true,
      },
      fieldName: "providerSelectionToken",
      label: $t("plugin.idc-core.catalogAutomation.binding.provider"),
      rules: z.preprocess(
        (value) => value ?? "",
        z.string().min(1, {
          message: $t(
            "plugin.idc-core.catalogAutomation.binding.providerRequired",
          ),
        }),
      ),
    },
    {
      component: "Select",
      componentProps: {
        "data-testid": "catalog-fulfillment-interface",
        filterOption: false,
        onSearch: handleSearch,
        options: interfaces.value.map((item) => ({
          label: optionLabel(item),
          value: item.interfaceInstanceNo,
        })),
        placeholder: $t(
          "plugin.idc-core.catalogAutomation.binding.placeholder",
        ),
        showSearch: true,
        disabled: !selectedProviderToken.value,
      },
      fieldName: "interfaceInstanceNo",
      label: $t("plugin.idc-core.catalogAutomation.binding.interface"),
      rules: z.preprocess(
        (value) => value ?? "",
        z.string().min(1, {
          message: $t(
            "plugin.idc-core.catalogAutomation.binding.interfaceRequired",
          ),
        }),
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
    interfaceLoadSequence += 1;
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = undefined;
    currentVersionNo = "";
    currentNamePrefix = "";
    providerPage.value = 1;
    providerTotal.value = 0;
    interfacePage.value = 1;
    interfaceTotal.value = 0;
    interfaces.value = [];
    providers.value = [];
    selectedProviderToken.value = "";
    await formApi.resetForm();
  },
  async onConfirm() {
    if (!currentVersionNo) return false;
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const values = await formApi.getValues<{ interfaceInstanceNo: string }>();
    if (
      !interfaces.value.some(
        (item) => item.interfaceInstanceNo === values.interfaceInstanceNo,
      )
    )
      return false;
    modalApi.lock(true);
    try {
      await productFulfillmentBindingPut(
        currentVersionNo,
        values.interfaceInstanceNo,
      );
      message.success(
        $t("plugin.idc-core.catalogAutomation.binding.succeeded"),
      );
      emit("success");
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{ versionNo?: string }>();
    currentVersionNo = data?.versionNo || "";
    await loadProviders(1);
  },
});
</script>

<template>
  <Modal
    :confirm-text="$t('plugin.idc-core.catalogAutomation.binding.confirm')"
    :title="$t('plugin.idc-core.catalogAutomation.binding.title')"
    class="w-[680px]"
  >
    <div data-testid="catalog-fulfillment-binding-modal">
      <div class="mb-4 rounded-md bg-muted p-3 text-sm text-muted-foreground">
        {{ $t("plugin.idc-core.catalogAutomation.binding.note") }}
      </div>
      <Form />
      <div
        v-if="providerTotal > PROVIDER_PAGE_SIZE"
        class="mb-4 flex items-center justify-between gap-3"
      >
        <span class="text-sm text-muted-foreground">
          {{ $t("plugin.idc-core.catalogAutomation.binding.provider") }}
        </span>
        <Pagination
          data-testid="catalog-fulfillment-provider-pagination"
          :current="providerPage"
          :page-size="PROVIDER_PAGE_SIZE"
          :show-size-changer="false"
          :total="providerTotal"
          @change="loadProviders"
        />
      </div>
      <div
        v-if="interfaceTotal > INTERFACE_PAGE_SIZE"
        class="mb-4 flex items-center justify-between gap-3"
      >
        <span class="text-sm text-muted-foreground">
          {{ $t("plugin.idc-core.catalogAutomation.binding.interface") }}
        </span>
        <Pagination
          data-testid="catalog-fulfillment-interface-pagination"
          :current="interfacePage"
          :page-size="INTERFACE_PAGE_SIZE"
          :show-size-changer="false"
          :total="interfaceTotal"
          @change="handlePageChange"
        />
      </div>
      <div
        v-if="selectedProviderToken && interfaces.length === 0"
        class="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-200"
      >
        {{ $t("plugin.idc-core.catalogAutomation.binding.empty") }}
      </div>
      <div
        v-else-if="providers.length === 0"
        class="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-200"
      >
        {{ $t("plugin.idc-core.catalogAutomation.binding.providerEmpty") }}
      </div>
    </div>
  </Modal>
</template>
