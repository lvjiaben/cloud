<script setup lang="ts">
import type { VbenFormProps } from "@vben/common-ui";
import type {
  AutomationInterface,
  ProductFulfillmentBinding,
} from "../catalog-automation-client";
import type { CatalogUpstreamProductSelection } from "../catalog-client";
import CatalogUpstreamProductForm from "./catalog-upstream-product-form.vue";
import type { CatalogFulfillmentStrategy } from "../catalog-fulfillment-policy-state";
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { preferences } from "@vben/preferences";
import { Alert, Button, message, Pagination, Tag } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { automationInterfaceList } from "../catalog-automation-client";
import { catalogFulfillmentPolicyUpdate } from "../catalog-client";
import {
  catalogFulfillmentStrategies,
  fulfillmentStrategyRequiresBinding,
} from "../catalog-fulfillment-policy-state";

const props = defineProps<{
  categoryNo: string;
  importedReference?: string;
  importedName?: string;
  importedPricingMode?:"markup"|"manual"|"mixed";
  importedMarkupPercent?:string;
  importedManualPrices?:CatalogUpstreamProductSelection["prices"];
  productNo: string;
  versionNo: string;
  strategy?: CatalogFulfillmentStrategy;
  binding?: ProductFulfillmentBinding;
  writable: boolean;
  externalSave?: boolean;
  refresh: () => Promise<void>;
}>();
const emit = defineEmits<{ busy: [value: boolean] }>();
const saving = ref(false);
const upstreamProduct = ref<CatalogUpstreamProductSelection>();
const upstreamForm = ref<InstanceType<typeof CatalogUpstreamProductForm>>();
const upstreamLoading = ref(false);
const catalogAvailable = computed(() => selectedInterface.value === props.binding?.interfaceInstanceNo ? !!props.binding?.catalogAvailable : !!interfaces.value.find(item => item.interfaceInstanceNo === selectedInterface.value)?.catalogAvailable);
const dirty = ref(false);
const selectedInterface = ref("");
const selectedInterfaceLabel = ref("");
const selectedInterfaceState = ref("");
const selectedStrategy = ref<CatalogFulfillmentStrategy>("manual");
const interfaces = ref<AutomationInterface[]>([]);
const loading = ref(false);
const loadFailed = ref(false);
const page = ref(1);
const total = ref(0);
const PAGE_SIZE = 20;
let sequence = 0;
let search = "";
let timer: ReturnType<typeof setTimeout> | undefined;
let editingVersion = "";
let editingProduct = "";
let initialStrategy: CatalogFulfillmentStrategy = "manual";
const initialInterface = ref("");
const tr = (key: string) => $t(`plugin.idc-core.catalogFulfillment.${key}`);
const workspaceText = (key: string) =>
  $t(`plugin.idc-core.catalog.workspace.${key}`);
const selectedState = computed(() => {
  const value =
    selectedInterface.value === props.binding?.interfaceInstanceNo
      ? props.binding.interfaceNewBusinessState
      : selectedInterfaceState.value;
  return value === "accepting" || value === "stopped" ? value : "unknown";
});
const changedBinding = computed(
  () => selectedInterface.value !== initialInterface.value,
);
const stoppedBinding = computed(
  () =>
    props.binding?.interfaceNewBusinessState === "stopped" &&
    selectedInterface.value === props.binding.interfaceInstanceNo,
);

function interfaceOptions() {
  const options = interfaces.value.map((item) => ({
    value: item.interfaceInstanceNo,
    label:
      item.newBusinessState === "stopped"
        ? `${item.displayName} · ${$t("plugin.idc-core.catalogAutomation.state.stopped")}`
        : item.displayName,
    disabled:
      item.newBusinessState !== "accepting" &&
      item.interfaceInstanceNo !== initialInterface.value,
  }));
  // A saved binding stays visible even when it falls outside this search page.
  if (
    props.binding &&
    !options.some((item) => item.value === props.binding!.interfaceInstanceNo)
  ) {
    options.unshift({
      value: props.binding.interfaceInstanceNo,
      label: props.binding.interfaceDisplayName,
      disabled: false,
    });
  }
  if (
    selectedInterface.value &&
    selectedInterfaceLabel.value &&
    !options.some((item) => item.value === selectedInterface.value)
  ) {
    options.unshift({
      value: selectedInterface.value,
      label: selectedInterfaceLabel.value,
      disabled: false,
    });
  }
  options.unshift({
    value: "",
    label: workspaceText("noAutomationInterface"),
    disabled: false,
  });
  return options;
}
function schema(): VbenFormProps["schema"] {
  return [
    {
      fieldName: "interfaceInstanceNo",
      component: "Select",
      label: workspaceText("automationInterface"),
      componentProps: {
        "data-testid": "catalog-fulfillment-interface",
        class: "w-full",
        disabled: !props.writable || saving.value,
        options: interfaceOptions(),
        showSearch: true,
        filterOption: false,
        loading: loading.value,
        onSearch: handleSearch,
        onChange: (value: string) => {
          selectedInterfaceState.value =
            interfaces.value.find((item) => item.interfaceInstanceNo === value)
              ?.newBusinessState || "";
          selectedInterfaceLabel.value =
            interfaceOptions().find((item) => item.value === value)?.label ||
            "";
          selectedInterface.value = value;
          dirty.value = true;
        },
        placeholder: workspaceText("interfaceSearch"),
      },
      rules: z
        .string()
        .refine(
          (value) =>
            !fulfillmentStrategyRequiresBinding(selectedStrategy.value) ||
            !!value,
          { message: workspaceText("interfaceRequired") },
        ),
    },
    {
      fieldName: "strategyCode",
      component: "Select",
      label: workspaceText("provisioningMode"),
      componentProps: {
        "data-testid": "catalog-fulfillment-policy-strategy",
        class: "w-full",
        disabled: !props.writable || saving.value,
        options: catalogFulfillmentStrategies.map((value) => ({
          value,
          label: workspaceText(`provisioningMode_${value}`),
        })),
        onChange: (value: CatalogFulfillmentStrategy) => {
          selectedStrategy.value = value;
          dirty.value = true;
        },
      },
      rules: z.enum([
        "manual",
        "approval_then_plugin",
        "payment_confirmed_plugin",
      ]),
    },
  ];
}
const [Form, formApi] = useVbenForm({
  schema: [],
  showDefaultActions: false,
  commonConfig: { labelWidth: 130 },
  wrapperClass: "grid-cols-1",
});
async function loadInterfaces(targetPage = 1) {
  if (!props.writable) return;
  const current = ++sequence;
  loading.value = true;
  loadFailed.value = false;
  try {
    const result = await automationInterfaceList({
      forProductBinding: true,
      namePrefix: search || undefined,
      page: targetPage,
      pageSize: PAGE_SIZE,
    });
    if (current !== sequence) return;
    interfaces.value = result.items;
    total.value = result.total;
    page.value = targetPage;
  } catch {
    if (current === sequence) loadFailed.value = true;
  } finally {
    if (current === sequence) loading.value = false;
  }
}
function handleSearch(value: string) {
  search = value.trim();
  // Invalidate the in-flight request immediately, before the debounce completes.
  sequence++;
  if (timer) clearTimeout(timer);
  timer = setTimeout(() => void loadInterfaces(1), 250);
}
watch(
  () => [
    props.writable,
    preferences.app.locale,
    saving.value,
    loading.value,
    interfaces.value,
    props.binding,
    selectedStrategy.value,
  ],
  () => formApi.setState({ schema: schema() }),
  { immediate: true },
);
watch(
  () => [
    props.productNo,
    props.versionNo,
    props.strategy,
    props.binding?.interfaceInstanceNo,
  ],
  () => {
    if (dirty.value) {
      // An internal edit copy may advance after another tab saves. Retain the
      // user's inputs only when the authoritative binding/policy is unchanged.
      if (
        editingProduct === props.productNo &&
        initialInterface.value === (props.binding?.interfaceInstanceNo || "") &&
        initialStrategy === (props.strategy || "manual")
      )
        editingVersion = props.versionNo;
      return;
    }
    editingProduct = props.productNo;
    initialStrategy = props.strategy || "manual";
    editingVersion = props.versionNo;
    initialInterface.value = props.binding?.interfaceInstanceNo || "";
    selectedInterface.value = initialInterface.value;
    selectedInterfaceLabel.value = props.binding?.interfaceDisplayName || "";
    selectedStrategy.value = props.strategy || "manual";
    void formApi.setValues({
      interfaceInstanceNo: selectedInterface.value,
      strategyCode: selectedStrategy.value,
    });
  },
  { immediate: true },
);
watch(
  () => props.writable,
  (allowed) => {
    if (allowed) void loadInterfaces();
    else {
      sequence++;
      interfaces.value = [];
      loading.value = false;
    }
  },
  { immediate: true },
);
async function save() {
  if (!props.writable || saving.value || upstreamLoading.value) return;
  if (catalogAvailable.value && changedBinding.value && !upstreamProduct.value) { message.error($t('plugin.idc-core.catalog.upstream.required')); return; }
  if(upstreamProduct.value && !(await upstreamForm.value?.validate()))return;
  saving.value = true;
  emit("busy", true);
  try {
    if (!(await formApi.validate()).valid) return;
    const values = await formApi.getValues<{
      interfaceInstanceNo: string;
      strategyCode: CatalogFulfillmentStrategy;
    }>();
    await catalogFulfillmentPolicyUpdate(
      editingVersion,
      values.strategyCode,
      upstreamProduct.value || changedBinding.value ? values.interfaceInstanceNo : undefined,
      upstreamProduct.value,
    );
    upstreamProduct.value = undefined;
    dirty.value = false;
    message.success(tr("saved"));
    await props.refresh();
  } finally {
    saving.value = false;
    emit("busy", false);
  }
}
onBeforeUnmount(() => {
  sequence++;
  if (timer) clearTimeout(timer);
});
defineExpose({save,dirty:computed(() => selectedInterface.value !== initialInterface.value || selectedStrategy.value !== initialStrategy || !!upstreamProduct.value),saveDisabled:computed(()=>saving.value || upstreamLoading.value)});
</script>

<template>
  <section
    class="max-w-[760px]"
    data-testid="catalog-fulfillment-policy-modal"
    @keydown.esc.stop
  >
    <Form />
    <CatalogUpstreamProductForm ref="upstreamForm" v-if="catalogAvailable && selectedInterface" :key="selectedInterface"
      :interface-no="selectedInterface" :category-no="categoryNo" :product-name="importedName" :reference="selectedInterface === binding?.interfaceInstanceNo ? importedReference : undefined"
      :saved-pricing-mode="importedPricingMode" :saved-markup-percent="importedMarkupPercent" :saved-prices="importedManualPrices"
      :writable="writable && !saving" @change="upstreamProduct = $event" @loading="upstreamLoading = $event" />
    <div
      v-if="selectedInterface"
      class="mb-4 sm:pl-[130px]"
      data-testid="catalog-fulfillment-interface-state"
    >
      <span class="text-muted-foreground mr-2">{{
        $t("plugin.idc-core.catalogAutomation.binding.salesState")
      }}</span
      ><Tag
        :color="
          selectedState === 'accepting'
            ? 'success'
            : selectedState === 'stopped'
              ? 'warning'
              : 'default'
        "
        >{{
          $t(`plugin.idc-core.catalogAutomation.state.${selectedState}`)
        }}</Tag
      >
    </div>
    <div
      v-if="writable && total > PAGE_SIZE"
      class="mb-5 pl-0 sm:pl-[130px]"
      data-testid="catalog-interface-pagination"
    >
      <Pagination
        :current="page"
        :page-size="PAGE_SIZE"
        :total="total"
        :show-size-changer="false"
        :show-total="
          (count) =>
            $t('plugin.idc-core.catalog.workspace.interfaceCount', { count })
        "
        @change="loadInterfaces"
      />
    </div>
    <Alert
      v-if="loadFailed"
      class="mb-4"
      type="error"
      show-icon
      :message="workspaceText('interfaceLoadFailed')"
      ><template #action
        ><Button
          data-testid="catalog-fulfillment-interface-retry"
          @click="loadInterfaces(page)"
          >{{ workspaceText("retry") }}</Button
        ></template
      ></Alert
    >
    <Alert
      v-if="stoppedBinding"
      class="mb-4"
      type="warning"
      show-icon
      :message="$t('plugin.idc-core.catalogAutomation.binding.stoppedNote')"
    />
    <Alert
      v-if="changedBinding"
      class="mb-4"
      type="info"
      show-icon
      :message="workspaceText('bindingChangeHint')"
    />
    <div v-if="!externalSave" class="mt-3 sm:pl-[130px]">
      <Button
        v-if="writable"
        type="primary"
        :loading="saving" :disabled="upstreamLoading"
        data-testid="catalog-fulfillment-policy-save"
        @click="save"
        >{{ workspaceText("saveAutomation") }}</Button
      >
    </div>
  </section>
</template>
