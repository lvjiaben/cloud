<script setup lang="ts">
import type {
  CatalogProductDetail,
  CatalogProductVersionConfiguration,
} from "../catalog-client";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import {
  Alert,
  Badge,
  Button,
  Skeleton,
  Space,
  Tabs,
  Tag,
  message,
  Modal,
} from "ant-design-vue";
import { $t } from "#/locales";
import { useAccess } from "../plugin-access";
import {
  catalogProductDetail,
  catalogProductRevisionCreate,
  catalogProductVersionConfigurationGet,
  catalogProductDescriptionUpdate,
} from "../catalog-client";
import VbenDataGrid from "./vben-data-grid.vue";
import CatalogPurchaseLinks from "./catalog-purchase-links.vue";
import CatalogProductBasicForm from "./catalog-product-basic-form.vue";
import CatalogFulfillmentPolicyForm from "./catalog-fulfillment-policy-form.vue";
import CatalogOfferOptions from "./catalog-offer-options.vue";
import CatalogProvisioningRules from "./catalog-provisioning-rules.vue";
import CatalogBasePriceTable from "./catalog-base-price-table.vue";
import CatalogPricedBillingRuleForm from "./catalog-priced-billing-rule-form.vue";
import CatalogTaxPolicyModal from "./catalog-tax-policy-modal.vue";
import CatalogProductInventoryForm from "./catalog-product-inventory-form.vue";
import CatalogProductAddons from "./catalog-product-addons.vue";
import CatalogPricesDrawer from "./catalog-prices-drawer.vue";

const props = defineProps<{ productNo: string; section?: string }>();
const emit = defineEmits<{ updated: []; close: [] }>();
const { hasAccessByCodes } = useAccess();
const can = (code: string) => hasAccessByCodes([`idc:admin:catalog:${code}`]);
const tr = (key: string) =>
  $t(`plugin.idc-core.catalog.merchantWorkspace.${key}`);
const detail = ref<CatalogProductDetail>();
const configuration = ref<CatalogProductVersionConfiguration>();
const configurationVersionNo = ref("");
const active = ref("basic");
const loading = ref(false),
  failed = ref(false),
  configurationLoading = ref(false),
  configurationFailed = ref(false);
const importedRows = computed(() => (configuration.value?.customerDeclaration?.fields || []).map(field => ({
  key:field.key,name:preferences.app.locale === 'zh-CN' ? field.label.zhCN : field.label.enUS,
  contents:(configuration.value?.customerResolvedCandidates[field.key] || field.staticCandidates || []).map(choice => preferences.app.locale === 'zh-CN' ? choice.label.zhCN : choice.label.enUS).join('、') || [field.minimum,field.maximum].filter(value => value !== undefined).join(' — ') || '—',
})));
const importedColumns = computed(() => [{key:'name',title:tr('parameterName'),minWidth:180},{key:'contents',title:tr('parameterValues'),minWidth:300}]);
const settingsSaving = ref(false);
const basicForm = ref<InstanceType<typeof CatalogProductBasicForm>>();
const inventoryForm = ref<InstanceType<typeof CatalogProductInventoryForm>>();
const addonForm = ref<InstanceType<typeof CatalogProductAddons>>();
const deploymentForm = ref<InstanceType<typeof CatalogProvisioningRules>>();
const loginForm = ref<InstanceType<typeof CatalogProvisioningRules>>();
const basicFormKey = ref(0);
const basicSavedPending = ref(false);
const basicSubmitting = ref(false);
const footerBusy = computed(()=>settingsSaving.value || basicSubmitting.value);
const offerOptionsForm = ref<InstanceType<typeof CatalogOfferOptions>>();
const basePriceForm = ref<InstanceType<typeof CatalogBasePriceTable>>();
const fulfillmentForm = ref<InstanceType<typeof CatalogFulfillmentPolicyForm>>();
const footerCanSave = computed(()=>active.value === 'basic' ? can('product:update') : active.value === 'pricing' ? canPrice.value : active.value === 'configuration' ? !!canConfigure.value && !configuration.value?.importedConfiguration : active.value === 'fulfillment' && editable.value && can('automation-interface:view') && can('fulfillment-binding:write'));
const footerDisabled = computed(()=>footerBusy.value || detailRefreshFailed.value || (active.value==='configuration' && !offerOptionsForm.value?.dirty) || (active.value==='pricing' && basePriceForm.value?.saveDisabled) || (active.value==='fulfillment' && fulfillmentForm.value?.saveDisabled));
const footerSaveTestId = computed(()=>active.value==='basic' ? 'catalog-description-save' : active.value==='pricing' ? 'catalog-base-price-table-save' : active.value==='configuration' ? 'catalog-configuration-table-save' : 'catalog-fulfillment-policy-save');
const detailRefreshFailed = ref(false),
  retryingDetail = ref(false);
let sequence = 0,
  configurationSequence = 0,
  productLoadSequence = 0,
  disposed = false;
let candidateTimer: ReturnType<typeof setTimeout> | undefined;
let candidateReads = 0;
function stopCandidateReads() {
  if (candidateTimer) clearTimeout(candidateTimer);
  candidateTimer = undefined;
}
function scheduleCandidateRead() {
  stopCandidateReads();
  if (
    (!hasEmptyChoices.value &&
      !["failed", "retained"].includes(
        configuration.value?.upstreamOptionsStatus || "",
      )) ||
    candidateReads >= 20 ||
    disposed
  )
    return;
  const versionNo = detail.value?.version.versionNo;
  const request = configurationSequence;
  candidateTimer = setTimeout(async () => {
    if (
      document.hidden ||
      disposed ||
      !versionNo ||
      request !== configurationSequence
    )
      return;
    candidateReads++;
    try {
      const result = await catalogProductVersionConfigurationGet(versionNo);
      const current = configuration.value,
        next = result.configuration;
      if (
        disposed ||
        request !== configurationSequence ||
        detail.value?.version.versionNo !== versionNo
      )
        return;
      // Only merge the reconstructible option projection. Unsaved merchant
      // inputs and authoritative configuration versions remain untouched.
      if (current && next && current.version === next.version)
        current.upstreamOptionsStatus = next.upstreamOptionsStatus;
      if (
        current &&
        next &&
        current.version === next.version &&
        JSON.stringify([
          current.productResolvedCandidates,
          current.customerResolvedCandidates,
          current.directoryRelationships,
        ]) !==
          JSON.stringify([
            next.productResolvedCandidates,
            next.customerResolvedCandidates,
            next.directoryRelationships,
          ])
      ) {
        current.productResolvedCandidates = next.productResolvedCandidates;
        current.customerResolvedCandidates = next.customerResolvedCandidates;
        current.directoryRelationships = next.directoryRelationships;
      }
      scheduleCandidateRead();
    } catch {
      /* A manual reload remains available; do not loop on API errors. */
    }
  }, 2000);
}
const editable = computed(() => detail.value?.version.status === "draft");
const canConfigure = computed(
  () =>
    editable.value &&
    configurationVersionNo.value === detail.value?.version.versionNo &&
    can("configuration:write") &&
    !configuration.value?.importedConfiguration,
);
const canPrice = computed(() => editable.value && can("price:write"));
const hasEmptyChoices = computed(() =>
  [
    ...Object.values(configuration.value?.productResolvedCandidates || {}),
    ...Object.values(configuration.value?.customerResolvedCandidates || {}),
  ].some((items) => !items.length),
);
const taxCurrencies = computed(() =>
  [
    ...new Set(detail.value?.prices.map((price) => price.currencyCode) || []),
  ].filter(
    (code) =>
      !detail.value?.taxPolicies.some((policy) => policy.currencyCode === code),
  ),
);
const name = computed(() =>
  preferences.app.locale === "zh-CN"
    ? detail.value?.product.nameZhCn
    : detail.value?.product.nameEnUs,
);
const [TaxModal, taxModal] = useVbenModal({
  connectedComponent: CatalogTaxPolicyModal,
});
const [PeriodModal, periodModal] = useVbenModal({});

async function editableDetail() {
  const productNo = props.productNo;
  let current = await catalogProductDetail(productNo);
  if (disposed || productNo !== props.productNo) return current;
  const required = [
    "product:update",
    "product:query",
    "billing:write",
    "price:write",
    "tax:write",
    "fulfillment-binding:write",
    "configuration:write",
  ];
  if (current.version.status === "published" && required.every(can)) {
    await catalogProductRevisionCreate(productNo, current.version.versionNo);
    current = await catalogProductDetail(productNo);
  }
  return current;
}

async function loadConfiguration(current: CatalogProductDetail) {
  stopCandidateReads();
  candidateReads = 0;
  const request = ++configurationSequence;
  configurationLoading.value = true;
  configurationFailed.value = false;
  try {
    const result = await catalogProductVersionConfigurationGet(
      current.version.versionNo,
    );
    if (
      !disposed &&
      request === configurationSequence &&
      detail.value?.version.versionNo === current.version.versionNo
    ) {
      configuration.value = result.configuration ?? undefined;
      configurationVersionNo.value = current.version.versionNo;
      scheduleCandidateRead();
    }
  } catch {
    if (!disposed && request === configurationSequence)
      configurationFailed.value = true;
  } finally {
    if (!disposed && request === configurationSequence)
      configurationLoading.value = false;
  }
}

async function reloadDetail() {
  const request = ++sequence;
  try {
    const current = await editableDetail();
    if (disposed || request !== sequence) return;
    detail.value = current;
    if (can("configuration:write")) await loadConfiguration(current);
    else {
      configurationSequence++;
      configuration.value = undefined;
      configurationVersionNo.value = "";
      configurationLoading.value = false;
      configurationFailed.value = false;
    }
    detailRefreshFailed.value = false;
    if (basicSavedPending.value) {
      basicSavedPending.value = false;
      basicFormKey.value++;
    }
    emit("updated");
  } catch (error) {
    if (!disposed && request === sequence && detail.value)
      detailRefreshFailed.value = true;
    throw error;
  }
}

async function retryDetail() {
  if (retryingDetail.value) return;
  retryingDetail.value = true;
  try {
    await reloadDetail();
  } catch {
    /* The explicit recovery state remains visible until a successful read. */
  } finally {
    retryingDetail.value = false;
  }
}

async function loadProduct() {
  detailRefreshFailed.value = false;
  stopCandidateReads();
  const request = ++productLoadSequence;
  detail.value = undefined;
  configuration.value = undefined;
  configurationVersionNo.value = "";
  loading.value = true;
  failed.value = false;
  active.value = ["basic", "pricing", "configuration", "agents", "fulfillment"].includes(props.section || "") ? props.section! : "basic";
  try {
    await reloadDetail();
  } catch {
    if (!disposed && request === productLoadSequence) failed.value = true;
  } finally {
    if (!disposed && request === productLoadSequence) loading.value = false;
  }
}

async function optionSaved(_saved: CatalogProductVersionConfiguration) {
  // Reload the final editable version before exposing newly enabled price
  // fields. The save response belongs to the version just made effective.
  await reloadDetail();
}
async function saveBasic() {
  if (basicSubmitting.value || settingsSaving.value || detailRefreshFailed.value || !detail.value) return;
  basicSubmitting.value = true;
  try {
  const description = await basicForm.value?.prepare();
  const stock = await inventoryForm.value?.prepare();
  const addons = addonForm.value?.prepare();
  const login = await loginForm.value?.prepare();
  if (!description || stock?.valid === false || addons?.valid === false || login?.valid === false) return;
  settingsSaving.value = true;
    await catalogProductDescriptionUpdate(detail.value.product.productNo,{
      ...description,versionNo:detail.value.version.versionNo,
      ...(stock?.input ? {inventory:stock.input} : {}),
      ...(addons?.input ? {addonProductNos:addons.input} : {}),
      ...(login?.input ? {loginRules:login.input} : {}),
    });
    basicSavedPending.value = true;
    message.success($t('plugin.idc-core.catalog.description.saved'));
    await reloadDetail();
  } finally {settingsSaving.value=false;basicSubmitting.value=false;}
}
async function saveActive() {
  if (footerDisabled.value) return;
  if (active.value==='basic') await saveBasic();
  else if (active.value==='pricing') await basePriceForm.value?.save();
  else if (active.value==='configuration') await offerOptionsForm.value?.save();
  else if (active.value==='fulfillment') await fulfillmentForm.value?.save();
}
const dirtySections = computed(() => [
  ...(!basicSavedPending.value && (basicForm.value?.dirty || inventoryForm.value?.dirty || addonForm.value?.dirty || loginForm.value?.dirty) ? ["basic"] : []),
  ...(basePriceForm.value?.dirty ? ["pricing"] : []),
  ...(offerOptionsForm.value?.dirty ? ["configuration"] : []),
  ...(fulfillmentForm.value?.dirty || deploymentForm.value?.dirty ? ["fulfillment"] : []),
]);
const hasUnsavedChanges = computed(() => dirtySections.value.length > 0);
let pendingClose: Promise<boolean> | undefined;
let navigationConfirmed = false;
async function requestClose() {
  if (navigationConfirmed) return true;
  if (footerBusy.value) return false;
  const basicDirty = !basicSavedPending.value && await basicForm.value?.isDirty();
  const stockDirty = !basicSavedPending.value && await inventoryForm.value?.isDirty();
  if (!dirtySections.value.length && !basicDirty && !stockDirty) return true;
  if (pendingClose) return pendingClose;
  pendingClose = new Promise<boolean>((resolve) => {
    Modal.confirm({
      centered: true,
      title: $t("plugin.idc-core.catalog.editorSafety.leaveTitle"),
      content: $t("plugin.idc-core.catalog.editorSafety.leaveDescription"),
      okText: $t("plugin.idc-core.catalog.editorSafety.discard"),
      cancelText: $t("plugin.idc-core.catalog.editorSafety.continue"),
      okButtonProps: { danger: true },
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    });
  });
  try { return await pendingClose; } finally { pendingClose = undefined; }
}
function beforeUnload(event: BeforeUnloadEvent) {
  if (!dirtySections.value.length) return;
  event.preventDefault();
  event.returnValue = "";
}
onMounted(() => window.addEventListener("beforeunload", beforeUnload));
// Connected drawers do not inherit the owning page's matched route record.
// Guard before route deactivation so cancelling keeps the drawer and draft mounted.
const router = useRouter();
const removeNavigationGuard = router.beforeEach(async (to, from) => {
  if (to.path === from.path) return true;
  const allowed = await requestClose();
  navigationConfirmed = allowed;
  return allowed;
});
const removeNavigationResult = router.afterEach((_to, _from, failure) => {
  if (failure) navigationConfirmed = false;
});
defineExpose({saveActive,footerCanSave,footerDisabled,footerSaveTestId,footerBusy,hasUnsavedChanges,requestClose});
function openTax() {
  if (detail.value)
    taxModal
      .setData({
        versionNo: detail.value.version.versionNo,
        currencies: taxCurrencies.value,
      })
      .open();
}

watch(
  () => props.productNo,
  () => {
    void loadProduct();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  removeNavigationGuard();
  removeNavigationResult();
  window.removeEventListener("beforeunload", beforeUnload);
  stopCandidateReads();
  disposed = true;
  sequence++;
  configurationSequence++;
});
</script>

<template>
  <div class="merchant-workspace" data-testid="catalog-product-detail-drawer">
    <Skeleton v-if="loading && !detail" active :paragraph="{ rows: 6 }" />
    <Alert v-if="failed" type="error" show-icon :message="tr('loadFailed')"
      ><template #action
        ><Button @click="loadProduct">{{ tr("reload") }}</Button></template
      ></Alert
    >
    <template v-if="detail">
      <header class="merchant-product-header">
        <div>
          <span class="merchant-eyebrow">{{ tr("saveHint") }}</span>
          <h2>{{ name }}</h2>
        </div>
        <Space
          ><Badge
            :status="detail.product.status === 'active' ? 'success' : 'default'"
            :text="
              $t(`plugin.idc-core.catalog.status.${detail.product.status}`)
            "
          /></Space
        >
      </header>
      <Alert
        v-if="detailRefreshFailed"
        class="merchant-save-recovery"
        type="error"
        show-icon
        :message="tr('savedStateRefreshFailed')"
        data-testid="catalog-detail-refresh-failed"
      >
        <template #action
          ><Button
            type="primary"
            :loading="retryingDetail"
            data-testid="catalog-detail-refresh-retry"
            @click="retryDetail"
            >{{ tr("reload") }}</Button
          ></template
        >
      </Alert>
      <Alert
        v-if="
          hasEmptyChoices ||
          ['failed', 'retained'].includes(
            configuration?.upstreamOptionsStatus || '',
          )
        "
        class="mb-5"
        data-testid="catalog-upstream-sync-status"
        type="warning"
        show-icon
        :message="
          tr(
            configuration?.upstreamOptionsStatus === 'failed'
              ? 'upstreamSyncFailed'
              : configuration?.upstreamOptionsStatus === 'retained'
                ? 'upstreamSyncRetained'
                : 'upstreamSyncPending',
          )
        "
        ><template #action
          ><Button
            :loading="configurationLoading"
            @click="loadConfiguration(detail)"
            >{{ tr("reload") }}</Button
          ></template
        ></Alert
      >
      <Alert v-if="dirtySections.length" class="mb-4" type="info" show-icon data-testid="catalog-unsaved-changes" :message="$t('plugin.idc-core.catalog.editorSafety.unsaved', { sections: dirtySections.map(section => section === 'configuration' ? tr('productConfiguration') : section === 'pricing' ? tr('packagePrice') : section === 'fulfillment' ? tr('automation') : tr('basic')).join('、') })" />
      <Tabs
        :inert="footerBusy || detailRefreshFailed || undefined"
        :destroy-inactive-tab-pane="false"
        v-model:active-key="active"
        class="merchant-tabs"
        data-testid="catalog-edit-sections"
      >
        <Tabs.TabPane key="basic"
          ><template #tab
            ><span data-testid="catalog-editor-tab-basic">{{
              tr("basic")
            }}</span></template
          >
          <div :key="basicFormKey" class="merchant-basic-form">
          <CatalogProductBasicForm ref="basicForm"
            :product="detail.product"
            :writable="can('product:update') && !settingsSaving"
          />
          <CatalogPurchaseLinks :slug="detail.product.publicSlug" :category-no="detail.product.categoryNo" />
          <section class="merchant-section mt-6">
            <CatalogProductInventoryForm ref="inventoryForm" :inventory="detail.inventory" :version-no="detail.version.versionNo"
              :writable="editable && can('product:create') && !settingsSaving" />
          </section>
          <CatalogProductAddons ref="addonForm" :product-no="detail.product.productNo" :version="detail.product.descriptionVersion || 1"
            :writable="can('product:update') && !settingsSaving" />
          <CatalogProvisioningRules ref="loginForm" external-save v-if="configuration?.productDeclaration && !configuration.importedConfiguration"
            mode="login" :version-no="configurationVersionNo" :detail="detail" :configuration="configuration"
            :writable="!!canConfigure && configurationVersionNo === detail.version.versionNo && !settingsSaving"
            :refresh="reloadDetail" @busy="settingsSaving = $event" />
          </div>
        </Tabs.TabPane>
        <Tabs.TabPane key="pricing"><template #tab><span data-testid="catalog-editor-tab-pricing">{{ tr('packagePrice') }}</span></template>
          <section
            class="merchant-section"
            data-testid="catalog-detail-base-prices"
          >
            <div class="merchant-section-title">
              <h3>{{ tr("packagePrice") }}</h3>
              <Button v-if="editable && can('billing:write') && can('price:write') && !configuration?.importedConfiguration"
                type="primary" data-testid="catalog-priced-period-add" @click="periodModal.open()">{{ tr('add') }}</Button>
            </div>
            <CatalogBasePriceTable
              ref="basePriceForm" external-save
              @busy="settingsSaving = $event"
              :version-no="detail.version.versionNo"
              :billing-rules="detail.billingRules"
              :prices="detail.prices"
              :writable="canPrice"
              :refresh="reloadDetail"
            />
          </section>
          <section class="merchant-tax" data-testid="catalog-detail-tax">
            <div>
              <strong>{{ $t("plugin.idc-core.catalog.detail.tax") }}</strong>
              <p v-if="taxCurrencies.length">
                {{
                  $t("plugin.idc-core.catalog.workspace.taxRulesRequired", {
                    currencies: taxCurrencies.join("、"),
                  })
                }}
              </p>
              <div v-else class="mt-2">
                <Tag v-for="tax in detail.taxPolicies" :key="tax.taxPolicyNo"
                  >{{ tax.currencyCode }} ·
                  {{
                    $t(
                      `plugin.idc-core.catalog.detail.code.taxTreatment.${tax.taxTreatment}`,
                    )
                  }}</Tag
                >
              </div>
            </div>
            <Button
              v-if="editable && can('tax:write') && taxCurrencies.length"
              data-testid="catalog-tax-add"
              @click="openTax"
              >{{ $t("plugin.idc-core.catalog.commerce.addTax") }}</Button
            >
          </section>
        </Tabs.TabPane>
        <Tabs.TabPane key="configuration"><template #tab><span data-testid="catalog-editor-tab-configuration">{{ tr('productConfiguration') }}</span></template>
          <Skeleton
            data-testid="catalog-settings-loading"
            v-if="configurationLoading && !configuration"
            active
            :paragraph="{ rows: 3 }"
          />
          <Alert
            data-testid="catalog-settings-failed"
            v-else-if="configurationFailed"
            class="mb-5"
            type="error"
            show-icon
            :message="tr('localConfigurationFailed')"
            ><template #action
              ><Button
                data-testid="catalog-settings-retry"
                @click="loadConfiguration(detail)"
                >{{ tr("reload") }}</Button
              ></template
            ></Alert
          >
          <VbenDataGrid v-if="configuration?.importedConfiguration" :columns="importedColumns" :data="importedRows" row-key="key" :toolbar="false" :height="420" data-testid="catalog-imported-configuration" />
          <CatalogOfferOptions ref="offerOptionsForm" :external-pricing-dirty="basePriceForm?.dirty"
            v-else-if="configuration?.productDeclaration"
            :version-no="configurationVersionNo"
            :detail="detail"
            :configuration="configuration"
            :writable="
              !!canConfigure &&
              configurationVersionNo === detail.version.versionNo
            "
            :can-price="canPrice && can('configuration:write')"
            :refresh="reloadDetail"
            @saved="optionSaved"
            @busy="settingsSaving = $event"
          />
        </Tabs.TabPane>
        <Tabs.TabPane v-if="hasAccessByCodes(['idc:admin:sales-channel:pricing:manage'])" key="agents">
          <template #tab><span data-testid="catalog-editor-tab-agents">{{ tr('agentPrices') }}</span></template>
          <CatalogPricesDrawer inline target="level" :product="{productNo:detail.product.productNo, name:name || ''}" />
        </Tabs.TabPane>
        <Tabs.TabPane key="fulfillment"
          ><template #tab
            ><span data-testid="catalog-editor-tab-fulfillment">{{
              tr("automation")
            }}</span></template
          >
          <section class="merchant-section">
            <CatalogFulfillmentPolicyForm
              ref="fulfillmentForm" external-save
              :category-no="detail.product.categoryNo" :imported-reference="configuration?.importedProductReference" :imported-name="configuration?.importedProductName"
              :imported-pricing-mode="configuration?.importedPricingMode" :imported-markup-percent="configuration?.importedMarkupPercent" :imported-manual-prices="configuration?.importedManualPrices"
              :product-no="detail.product.productNo"
              :version-no="detail.version.versionNo"
              :strategy="detail.fulfillmentPolicy?.strategyCode"
              :binding="detail.fulfillmentBinding ?? undefined"
              :writable="
                editable &&
                can('automation-interface:view') &&
                can('fulfillment-binding:write')
              "
              :refresh="reloadDetail"
              @busy="settingsSaving = $event"
            />
          </section>

          <Skeleton
            data-testid="catalog-settings-loading"
            v-if="configurationLoading && !configuration"
            active
            :paragraph="{ rows: 3 }"
          />
          <Alert
            data-testid="catalog-settings-failed"
            v-else-if="configurationFailed"
            type="error"
            show-icon
            :message="tr('localConfigurationFailed')"
            ><template #action
              ><Button
                data-testid="catalog-settings-retry"
                @click="loadConfiguration(detail)"
                >{{ tr("reload") }}</Button
              ></template
            ></Alert
          >
          <CatalogProvisioningRules
            ref="deploymentForm"
            v-if="configuration?.productDeclaration && !configuration.importedConfiguration" mode="deployment"
            :version-no="configurationVersionNo" :detail="detail" :configuration="configuration"
            :writable="!!canConfigure && configurationVersionNo === detail.version.versionNo"
            :refresh="reloadDetail" @busy="settingsSaving = $event"
          />
        </Tabs.TabPane>
      </Tabs>
    </template>
    <TaxModal @success="reloadDetail" />
    <PeriodModal :title="tr('add')" :footer="false" class="w-[680px] max-w-[95vw]">
      <CatalogPricedBillingRuleForm v-if="detail" :version-no="detail.version.versionNo" :billing-rules="detail.billingRules"
        :refresh="reloadDetail" @saved="periodModal.close()" @busy="settingsSaving = $event" />
    </PeriodModal>
  </div>
</template>

<style scoped>
.merchant-workspace :deep(.ant-tabs-nav) {
  position: sticky;
  top: 0;
  z-index: 2;
  background: hsl(var(--background));
}
.merchant-workspace {
  color: hsl(var(--foreground));
  padding: 0 18px 28px;
}
.merchant-product-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 6px 0 22px;
  border-bottom: 1px solid hsl(var(--border));
}
.merchant-product-header h2 {
  margin: 6px 0 0;
  font-size: 24px;
  font-weight: 600;
  letter-spacing: -0.4px;
}
.merchant-eyebrow {
  font-size: 12px;
  color: hsl(var(--muted-foreground));
}
.merchant-tabs[inert] {
  opacity: 0.65;
}
.merchant-save-recovery {
  position: fixed;
  right: 24px;
  bottom: 80px;
  z-index: 50;
  width: min(820px, calc(100vw - 160px));
  box-shadow: 0 8px 24px rgb(0 0 0 / 12%);
}
.merchant-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 24px;
}
.merchant-tabs :deep(.ant-tabs-tab) {
  padding: 17px 6px;
  font-size: 15px;
}
.merchant-tabs :deep(.ant-tabs-nav-list) {
  flex-wrap: wrap;
}
.merchant-section {
  margin-bottom: 28px;
}
.merchant-section-title h3 {
  font-size: 16px;
  font-weight: 600;
  margin: 0;
}
.merchant-section-title {display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:18px;}
.merchant-section-title p,
.merchant-tax p {
  font-size: 13px;
  line-height: 1.7;
  color: hsl(var(--muted-foreground));
  margin: 8px 0 18px;
}
.merchant-tax {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  margin-top: 26px;
  padding: 20px;
  background: hsl(var(--muted) / 30%);
  border: 1px solid hsl(var(--border));
}

@media (max-width: 760px) {
  .merchant-save-recovery {
    right: 16px;
    bottom: 16px;
    left: 16px;
    width: auto;
  }
  .merchant-product-header {
    align-items: flex-start;
  }
  .merchant-product-header h2 {
    font-size: 20px;
  }
  .merchant-tax {
    flex-wrap: wrap;
  }
  .merchant-tabs :deep(.ant-tabs-tab) {
    margin-left: 12px;
  }
  .merchant-tabs :deep(.ant-tabs-nav-list) {
    gap: 0;
  }
}
</style>
