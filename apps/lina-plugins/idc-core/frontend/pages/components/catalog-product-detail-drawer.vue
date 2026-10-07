<script setup lang="ts">
import type {
  CatalogProductDetail,
  CatalogProductVersionCost,
  CatalogProductVersionConfiguration,
} from "../catalog-client";
import type { CatalogConfigurationValues } from "../catalog-configuration-state";
import type { ContractPolicyRule } from "../contract-policy-client";

import { computed, ref } from "vue";
import { useVbenDrawer, useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import {
  Badge,
  Button,
  Card,
  Descriptions,
  Empty,
  message,
  Popconfirm,
  Space,
  Tag,
} from "ant-design-vue";

import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";

import { useAccess } from "../plugin-access";
import { productFulfillmentBindingDelete } from "../catalog-automation-client";
import { canWriteCatalogFulfillmentBinding } from "../catalog-automation-state";
import {
  catalogProductVersionConfigurationGet,
  catalogProductVersionCostList,
  catalogProductDetail,
  catalogProductVersionConfigurationEvaluate,
  catalogProductVersionConfigurationUpdate,
  catalogProductVersionConfigurationFileUpload,
  catalogProductVersionConfigurationFileRevoke,
} from "../catalog-client";
import {
  catalogConfigurationRowsToValues,
  catalogConfigurationValuesToRows,
} from "../catalog-configuration-state";
import CatalogFulfillmentBindingModal from "./catalog-fulfillment-binding-modal.vue";
import CatalogFulfillmentPolicyModal from "./catalog-fulfillment-policy-modal.vue";
import CatalogConfigurationSection from "./catalog-configuration-section.vue";
import CatalogConfigurationPriceModal from "./catalog-configuration-price-modal.vue";
import CatalogProductCostModal from "./catalog-product-cost-modal.vue";
import CatalogProductInventoryModal from "./catalog-product-inventory-modal.vue";
import ContractPolicyEditor from "./contract-policy-editor.vue";

const detail = ref<CatalogProductDetail>();
const versionConfiguration = ref<CatalogProductVersionConfiguration>();
const versionCosts = ref<CatalogProductVersionCost[]>([]);
const { hasAccessByCodes } = useAccess();
let requestSequence = 0;

const canEditContractPolicy = computed(
  () =>
    detail.value?.version.status === "draft" &&
    hasAccessByCodes(["idc:admin:catalog:contract-policy:write"]),
);
const canWriteBinding = computed(() =>
  canWriteCatalogFulfillmentBinding(
    detail.value?.version.status,
    hasAccessByCodes(["idc:admin:catalog:automation-interface:view"]),
    hasAccessByCodes(["idc:admin:catalog:fulfillment-binding:write"]),
  ),
);
const bindingActionInFlight = ref(false);
const canEditConfiguration = computed(
  () =>
    detail.value?.version.status === "draft" &&
    hasAccessByCodes(["idc:admin:catalog:configuration:write"]),
);
const canViewConfiguration = computed(() =>
  hasAccessByCodes(["idc:admin:catalog:configuration:write"]),
);
const canManageCosts = computed(() =>
  hasAccessByCodes(["idc:admin:catalog:cost:write"]),
);
const canManageConfigurationPrices = computed(
  () =>
    detail.value?.version.status === "draft" &&
    hasAccessByCodes(["idc:admin:catalog:configuration:write"]) &&
    hasAccessByCodes(["idc:admin:catalog:price:write"]),
);
const canEditInventory = computed(() => detail.value?.version.status === "draft" && hasAccessByCodes(["idc:admin:catalog:product:create"]));

const title = computed(() => {
  if (!detail.value) return $t("plugin.idc-core.catalog.detail.title");
  return `${$t("plugin.idc-core.catalog.detail.title")} · ${localizedName(detail.value.product)}`;
});

const pricesByRule = computed(() => {
  const result = new Map<string, CatalogProductDetail["prices"]>();
  for (const price of detail.value?.prices ?? []) {
    const items = result.get(price.billingRuleNo) ?? [];
    items.push(price);
    result.set(price.billingRuleNo, items);
  }
  return result;
});

function localizedName(value: { nameEnUs: string; nameZhCn: string }) {
  return preferences.app.locale === "zh-CN"
    ? value.nameZhCn || value.nameEnUs
    : value.nameEnUs || value.nameZhCn;
}

function localizedSummary() {
  if (!detail.value) return "";
  const product = detail.value.product;
  return preferences.app.locale === "zh-CN"
    ? product.summaryZhCn || product.summaryEnUs
    : product.summaryEnUs || product.summaryZhCn;
}

function statusBadge(status: string) {
  if (["active", "published"].includes(status)) return "success";
  if (status === "draft") return "processing";
  if (status === "suspended") return "warning";
  return "default";
}

function codeLabel(domain: string, value: string) {
  return $t(`plugin.idc-core.catalog.detail.code.${domain}.${value}`);
}

function formatMoney(amountMinor: string, currencyCode: string) {
  const rawAmount = String(amountMinor).trim();
  const normalizedCurrency = currencyCode.trim().toUpperCase();
  try {
    if (!/^-?\d+$/.test(rawAmount)) throw new Error("invalid minor amount");
    const locale = preferences.app.locale === "zh-CN" ? "zh-CN" : "en-US";
    const currencyFormatter = new Intl.NumberFormat(locale, {
      currency: normalizedCurrency,
      currencyDisplay: "narrowSymbol",
      style: "currency",
    });
    // Currency formatting always resolves the fraction-digit bounds.
    const digits = currencyFormatter.resolvedOptions().maximumFractionDigits as number;
    const amount = BigInt(rawAmount);
    const absoluteAmount = amount < 0n ? -amount : amount;
    const divisor = 10n ** BigInt(digits);
    const integer = absoluteAmount / divisor;
    const fraction = (absoluteAmount % divisor)
      .toString()
      .padStart(digits, "0");
    const formattedInteger = new Intl.NumberFormat(locale, {
      maximumFractionDigits: 0,
      useGrouping: true,
    }).format(integer);
    const formattedNumber = digits
      ? `${formattedInteger}.${fraction}`
      : formattedInteger;
    const currencyParts = currencyFormatter.formatToParts(0);
    const currencyPart =
      currencyParts.find((part) => part.type === "currency")?.value ||
      normalizedCurrency;
    const currencyIndex = currencyParts.findIndex(
      (part) => part.type === "currency",
    );
    const integerIndex = currencyParts.findIndex(
      (part) => part.type === "integer",
    );
    const separator = currencyParts
      .slice(
        Math.min(currencyIndex, integerIndex) + 1,
        Math.max(currencyIndex, integerIndex),
      )
      .filter((part) => part.type === "literal")
      .map((part) => part.value)
      .join("");
    const sign = amount < 0n ? "-" : "";
    return currencyIndex < integerIndex
      ? `${sign}${currencyPart}${separator}${formattedNumber}`
      : `${sign}${formattedNumber}${separator}${currencyPart}`;
  } catch {
    return `${normalizedCurrency || currencyCode} ${rawAmount}`.trim();
  }
}

function pricesFor(ruleNo: string) {
  return pricesByRule.value.get(ruleNo) ?? [];
}

function formatTaxRate(value: null | number) {
  if (value === null) return $t("plugin.idc-core.catalog.detail.notApplicable");
  return `${(value / 100).toFixed(2).replace(/\.00$/, "")}%`;
}

function applyContractPolicy(rule: ContractPolicyRule) {
  if (!detail.value) return;
  detail.value.version.contractPolicyCode = rule.policyCode;
  detail.value.version.contractTemplateNo = rule.templateNo;
  detail.value.version.contractPolicyVersion = rule.version;
  detail.value.version.requiresContract = rule.policyCode === "required";
}

const [BindingModal, bindingModalApi] = useVbenModal({
  connectedComponent: CatalogFulfillmentBindingModal,
});

const [PolicyModal, policyModalApi] = useVbenModal({
  connectedComponent: CatalogFulfillmentPolicyModal,
});

const [CostModal, costModalApi] = useVbenModal({ connectedComponent: CatalogProductCostModal });
const [InventoryModal, inventoryModalApi] = useVbenModal({ connectedComponent: CatalogProductInventoryModal });
const [ConfigurationPriceModal, configurationPriceModalApi] = useVbenModal({ connectedComponent: CatalogConfigurationPriceModal });

function openConfigurationPrice() {
  if (!detail.value) return;
  configurationPriceModalApi.setData({
    billingRules: detail.value.billingRules.filter((rule) => rule.status === "active"),
    currencyCodes: [...new Set(detail.value.taxPolicies.map((item) => item.currencyCode))],
    versionNo: detail.value.version.versionNo,
  });
  configurationPriceModalApi.open();
}

function configurationPriceLabel(price: CatalogProductDetail["prices"][number]) {
  if (!price.configurationPriceDimensionNo) {
    return $t("plugin.idc-core.catalog.detail.basePrice");
  }
  const field = localizedFulfillmentText(price.configurationFieldLabelZhCn, price.configurationFieldLabelEnUs);
  const candidate = localizedFulfillmentText(price.configurationCandidateLabelZhCn, price.configurationCandidateLabelEnUs);
  return `${field} · ${candidate}`;
}

function openInventory() {
  if (!detail.value) return;
  inventoryModalApi.setData({ inventory: detail.value.inventory, versionNo: detail.value.version.versionNo });
  inventoryModalApi.open();
}

function applyInventory(inventory: CatalogProductDetail["inventory"]) {
  if (detail.value) detail.value.inventory = inventory;
}

function currencyCodes() {
  return [...new Set((detail.value?.prices ?? []).map((item) => item.currencyCode))];
}

function openCost(cost?: CatalogProductVersionCost) {
  if (!detail.value) return;
  costModalApi.setData({ cost, currencyCodes: currencyCodes(), versionNo: detail.value.version.versionNo });
  costModalApi.open();
}

async function reloadCosts() {
  if (!detail.value || !canManageCosts.value) { versionCosts.value = []; return; }
  versionCosts.value = (await catalogProductVersionCostList(detail.value.version.versionNo)).costs;
}

function openBinding() {
  if (!detail.value) return;
  bindingModalApi.setData({ versionNo: detail.value.version.versionNo });
  bindingModalApi.open();
}

function openFulfillmentPolicy() {
  if (!detail.value) return;
  policyModalApi.setData({
    currentStrategy: detail.value.fulfillmentPolicy?.strategyCode,
    hasBinding: Boolean(detail.value.fulfillmentBinding),
    versionNo: detail.value.version.versionNo,
  });
  policyModalApi.open();
}

function localizedFulfillmentText(zhCn: string, enUs: string) {
  return preferences.app.locale === "zh-CN" ? zhCn || enUs : enUs || zhCn;
}

async function reloadDetail() {
  if (!detail.value) return;
  const result = await catalogProductDetail(detail.value.product.productNo);
  detail.value = result;
  await reloadCosts();
  if (!result.fulfillmentBinding || !canViewConfiguration.value) {
    versionConfiguration.value = undefined;
    return;
  }
  const configurationResult = await catalogProductVersionConfigurationGet(
    result.version.versionNo,
  );
  versionConfiguration.value = configurationResult.configuration ?? undefined;
}

async function saveConfiguration(values: CatalogConfigurationValues) {
  if (!detail.value || !versionConfiguration.value?.productDeclaration) return;
  const result = await catalogProductVersionConfigurationUpdate(
    detail.value.version.versionNo,
    {
      merchantValues: catalogConfigurationValuesToRows(
        versionConfiguration.value.productDeclaration,
        "product",
        values,
      ),
      expectedVersion: versionConfiguration.value.version,
    },
  );
  versionConfiguration.value = result.configuration;
}

async function evaluateConfiguration(values: CatalogConfigurationValues) {
  if (!detail.value || !versionConfiguration.value?.productDeclaration) {
    throw new Error("configuration_unavailable");
  }
  const result = await catalogProductVersionConfigurationEvaluate(
    detail.value.version.versionNo,
    {
      expectedVersion: versionConfiguration.value.version,
      merchantValues: catalogConfigurationValuesToRows(
        versionConfiguration.value.productDeclaration,
        "product",
        values,
      ),
    },
  );
  return result.evaluation;
}

function uploadProductConfigurationFile(fieldKey: string, file: File) {
  if (!detail.value || !versionConfiguration.value)
    throw new Error("configuration_unavailable");
  return catalogProductVersionConfigurationFileUpload(
    detail.value.version.versionNo,
    versionConfiguration.value.version,
    fieldKey,
    file,
  );
}
function revokeProductConfigurationFile(fileToken: string) {
  if (!detail.value || !versionConfiguration.value)
    throw new Error("configuration_unavailable");
  return catalogProductVersionConfigurationFileRevoke(
    detail.value.version.versionNo,
    versionConfiguration.value.version,
    fileToken,
  );
}

function configurationTitle(surface: "customer" | "interface" | "product") {
  if (preferences.app.locale === "zh-CN") {
    return surface === "product" ? "后台商品配置" : "客户购买配置";
  }
  return surface === "product"
    ? "Product configuration"
    : "Customer purchase configuration";
}

async function unbindAutomation() {
  if (!detail.value || bindingActionInFlight.value) return;
  bindingActionInFlight.value = true;
  try {
    await productFulfillmentBindingDelete(detail.value.version.versionNo);
    message.success(
      $t("plugin.idc-core.catalogAutomation.binding.unbindSucceeded"),
    );
    await reloadDetail();
  } finally {
    bindingActionInFlight.value = false;
  }
}

const [Drawer, drawerApi] = useVbenDrawer({
  footer: false,
  async onOpenChange(open) {
    const sequence = ++requestSequence;
    if (!open) {
      detail.value = undefined;
      versionConfiguration.value = undefined;
      versionCosts.value = [];
      return;
    }
    const data = drawerApi.getData<{ productNo?: string }>();
    if (!data?.productNo) return;
    drawerApi.setState({ loading: true });
    try {
      const result = await catalogProductDetail(data.productNo);
      if (sequence !== requestSequence) return;
      detail.value = result;
      await reloadCosts();
      if (!result.fulfillmentBinding || !canViewConfiguration.value) {
        versionConfiguration.value = undefined;
        return;
      }
      const configurationResult = await catalogProductVersionConfigurationGet(
        result.version.versionNo,
      );
      if (sequence === requestSequence)
        versionConfiguration.value =
          configurationResult.configuration ?? undefined;
    } finally {
      if (sequence === requestSequence) drawerApi.setState({ loading: false });
    }
  },
});
</script>

<template>
  <Drawer :title="title" class="w-3/4">
    <div
      v-if="detail"
      class="grid gap-4"
      data-testid="catalog-product-detail-drawer"
    >
      <Card :bordered="true" size="small">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <h2 class="m-0 text-lg font-semibold">
              {{ localizedName(detail.product) }}
            </h2>
            <p class="text-muted-foreground mb-0 mt-2">
              {{ localizedSummary() }}
            </p>
          </div>
          <div class="flex shrink-0 flex-col items-end gap-2">
            <Badge
              :status="statusBadge(detail.product.status)"
              :text="
                $t(`plugin.idc-core.catalog.status.${detail.product.status}`)
              "
            />
            <Tag color="blue">v{{ detail.version.versionNumber }}</Tag>
          </div>
        </div>
      </Card>

      <Descriptions bordered :column="2" size="small">
        <Descriptions.Item
          :label="$t('plugin.idc-core.catalog.detail.category')"
        >
          {{
            preferences.app.locale === "zh-CN"
              ? detail.product.categoryNameZhCn ||
                detail.product.categoryNameEnUs
              : detail.product.categoryNameEnUs ||
                detail.product.categoryNameZhCn
          }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-core.catalog.detail.publicSlug')"
        >
          /{{ detail.product.publicSlug }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-core.catalog.detail.contract')"
        >
          {{
            $t(
              `plugin.idc-core.contractPolicy.policy.${detail.version.contractPolicyCode}`,
            )
          }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-core.catalog.detail.versionStatus')"
        >
          <Badge
            :status="statusBadge(detail.version.status)"
            :text="codeLabel('versionStatus', detail.version.status)"
          />
        </Descriptions.Item>
        <Descriptions.Item :label="$t('plugin.idc-core.catalog.detail.source')">
          {{ $t(`plugin.idc-core.catalog.source.${detail.version.source}`) }}
        </Descriptions.Item>
        <Descriptions.Item
          :label="$t('plugin.idc-core.catalog.detail.updatedAt')"
        >
          {{ formatTimestamp(detail.product.updatedAt) }}
        </Descriptions.Item>
      </Descriptions>

      <ContractPolicyEditor
        v-if="canEditContractPolicy"
        level="product"
        :policy-code="detail.version.contractPolicyCode"
        :target-no="detail.version.versionNo"
        :template-no="detail.version.contractTemplateNo"
        :version="detail.version.contractPolicyVersion"
        @updated="applyContractPolicy"
      />

      <CatalogConfigurationSection
        v-if="versionConfiguration?.productDeclaration"
        :can-edit="canEditConfiguration"
        :declaration="versionConfiguration.productDeclaration"
        :evaluate="evaluateConfiguration"
        :resolved-candidates="versionConfiguration.productResolvedCandidates"
        :revoke-file="revokeProductConfigurationFile"
        :save="saveConfiguration"
        surface="product"
        :title="configurationTitle('product')"
        :values="
          catalogConfigurationRowsToValues(
            versionConfiguration.merchantValues,
            versionConfiguration.productDeclaration,
          )
        "
        :upload-file="uploadProductConfigurationFile"
        @saved="reloadDetail"
      />
      <CatalogConfigurationSection
        v-if="versionConfiguration?.customerDeclaration"
        :declaration="versionConfiguration.customerDeclaration"
        :resolved-candidates="versionConfiguration.customerResolvedCandidates"
        surface="customer"
        :title="configurationTitle('customer')"
      />

      <Card
        :bordered="true"
        :title="$t('plugin.idc-core.catalogFulfillment.summaryTitle')"
        data-testid="catalog-fulfillment-policy-summary"
        size="small"
      >
        <template #extra>
          <Button
            v-if="canWriteBinding"
            data-testid="catalog-fulfillment-policy-open"
            type="primary"
            @click="openFulfillmentPolicy"
          >
            {{
              $t(
                detail.fulfillmentPolicy
                  ? "plugin.idc-core.catalogFulfillment.change"
                  : "plugin.idc-core.catalogFulfillment.set",
              )
            }}
          </Button>
        </template>
        <Descriptions
          v-if="detail.fulfillmentPolicy"
          bordered
          :column="2"
          size="small"
        >
          <Descriptions.Item
            :label="$t('plugin.idc-core.catalogFulfillment.field.strategy')"
          >
            {{
              $t(
                `plugin.idc-core.catalogFulfillment.strategy.${detail.fulfillmentPolicy.strategyCode}`,
              )
            }}
          </Descriptions.Item>
          <Descriptions.Item
            :label="$t('plugin.idc-core.catalogFulfillment.field.granularity')"
          >
            {{
              $t(
                `plugin.idc-core.catalogFulfillment.quantityHandling.${detail.fulfillmentPolicy.quantityHandling}`,
              )
            }}
          </Descriptions.Item>
          <Descriptions.Item
            v-if="detail.fulfillmentPolicy.capabilityZhCn || detail.fulfillmentPolicy.capabilityEnUs"
            :label="$t('plugin.idc-core.catalogFulfillment.field.capability')"
            :span="2"
          >
            {{
              localizedFulfillmentText(
                detail.fulfillmentPolicy.capabilityZhCn,
                detail.fulfillmentPolicy.capabilityEnUs,
              )
            }}
          </Descriptions.Item>
        </Descriptions>
        <Empty
          v-else
          :description="$t('plugin.idc-core.catalogFulfillment.notSet')"
          :image="Empty.PRESENTED_IMAGE_SIMPLE"
        />
      </Card>

      <Card :bordered="true" :title="$t('plugin.idc-core.catalog.inventory.title')" size="small" data-testid="catalog-product-inventory">
        <template v-if="canEditInventory" #extra><Button @click="openInventory">{{ $t("plugin.idc-core.catalog.inventory.edit") }}</Button></template>
        <Descriptions :column="2" size="small">
          <Descriptions.Item :label="$t('plugin.idc-core.catalog.inventory.trackingEnabled')">{{ $t(detail.inventory.trackingEnabled ? "plugin.idc-core.catalog.detail.yes" : "plugin.idc-core.catalog.detail.no") }}</Descriptions.Item>
          <Descriptions.Item :label="$t('plugin.idc-core.catalog.inventory.sellableQuantity')">{{ detail.inventory.trackingEnabled ? detail.inventory.sellableQuantity : $t("plugin.idc-core.catalog.inventory.unlimited") }}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Card
        :bordered="true"
        :title="$t('plugin.idc-core.catalogAutomation.binding.summaryTitle')"
        data-testid="catalog-fulfillment-binding-summary"
        size="small"
      >
        <template #extra>
          <Space v-if="canWriteBinding">
            <Button
              data-testid="catalog-fulfillment-binding-open"
              :disabled="bindingActionInFlight"
              type="primary"
              @click="openBinding"
            >
              {{
                $t(
                  detail.fulfillmentBinding
                    ? "plugin.idc-core.catalogAutomation.binding.replace"
                    : "plugin.idc-core.catalogAutomation.binding.bind",
                )
              }}
            </Button>
            <Popconfirm
              v-if="detail.fulfillmentBinding"
              :cancel-text="
                $t('plugin.idc-core.catalogAutomation.actions.cancel')
              "
              :ok-text="
                $t('plugin.idc-core.catalogAutomation.binding.confirmUnbind')
              "
              :title="
                $t('plugin.idc-core.catalogAutomation.binding.unbindConfirm')
              "
              @confirm="unbindAutomation"
            >
              <Button
                data-testid="catalog-fulfillment-binding-remove"
                :disabled="bindingActionInFlight"
              >
                {{ $t("plugin.idc-core.catalogAutomation.binding.unbind") }}
              </Button>
            </Popconfirm>
          </Space>
        </template>

        <Descriptions
          v-if="detail.fulfillmentBinding"
          bordered
          :column="2"
          size="small"
        >
          <Descriptions.Item
            :label="$t('plugin.idc-core.catalogAutomation.binding.current')"
          >
            {{ detail.fulfillmentBinding.interfaceDisplayName }}
          </Descriptions.Item>
          <Descriptions.Item
            :label="$t('plugin.idc-core.catalogAutomation.binding.salesState')"
          >
            <Badge
              :status="
                detail.fulfillmentBinding.interfaceNewBusinessState ===
                'accepting'
                  ? 'success'
                  : 'warning'
              "
              :text="
                $t(
                  `plugin.idc-core.catalogAutomation.state.${detail.fulfillmentBinding.interfaceNewBusinessState}`,
                )
              "
            />
          </Descriptions.Item>
          <Descriptions.Item
            :label="$t('plugin.idc-core.catalogAutomation.binding.evidence')"
          >
            <Tag
              :color="
                detail.fulfillmentBinding.evidenceVerified
                  ? 'success'
                  : 'warning'
              "
            >
              {{
                $t(
                  detail.fulfillmentBinding.evidenceVerified
                    ? "plugin.idc-core.catalogAutomation.binding.evidenceVerified"
                    : "plugin.idc-core.catalogAutomation.binding.evidenceNeedsReview",
                )
              }}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item
            :label="$t('plugin.idc-core.catalogAutomation.binding.boundAt')"
          >
            {{ formatTimestamp(detail.fulfillmentBinding.boundAt) }}
          </Descriptions.Item>
        </Descriptions>
        <Empty
          v-else
          :description="
            $t('plugin.idc-core.catalogAutomation.binding.notBound')
          "
          :image="Empty.PRESENTED_IMAGE_SIMPLE"
        />
        <div
          v-if="
            detail.fulfillmentBinding?.interfaceNewBusinessState === 'stopped'
          "
          class="mt-3 rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-200"
        >
          {{ $t("plugin.idc-core.catalogAutomation.binding.stoppedNote") }}
        </div>
      </Card>

      <Card
        :bordered="true"
        :title="$t('plugin.idc-core.catalog.detail.pricing')"
        data-testid="catalog-detail-pricing"
        size="small"
      >
        <template v-if="canManageConfigurationPrices" #extra>
          <Button type="primary" @click="openConfigurationPrice">
            {{ $t("plugin.idc-core.catalog.configurationPrice.add") }}
          </Button>
        </template>
        <div v-if="detail.billingRules.length" class="grid gap-3">
          <div
            v-for="rule in detail.billingRules"
            :key="rule.billingRuleNo"
            class="rounded-md border p-3"
          >
            <div class="mb-2 flex flex-wrap items-center gap-2">
              <Tag color="blue">{{ codeLabel("charge", rule.chargeType) }}</Tag>
              <span class="text-muted-foreground">
                {{ rule.periodCount }} ×
                {{ codeLabel("period", rule.periodUnit) }}
              </span>
              <span class="text-muted-foreground">{{
                codeLabel("pricePhase", rule.pricePhase)
              }}</span>
            </div>
            <div class="grid gap-2 sm:grid-cols-2">
              <div
                v-for="price in pricesFor(rule.billingRuleNo)"
                :key="price.priceNo"
                class="bg-muted/30 rounded-md px-3 py-2"
              >
                <span class="text-muted-foreground block text-xs">
                  {{ configurationPriceLabel(price) }}
                </span>
                <strong>{{
                  formatMoney(price.amountMinor, price.currencyCode)
                }}</strong>
                <span class="ml-2 text-xs">
                  {{ $t("plugin.idc-core.catalog.configurationPrice.setupFee") }}
                  {{ formatMoney(price.setupFeeMinor, price.currencyCode) }}
                </span>
                <span
                  v-if="price.renewalAmountMinor !== null"
                  class="ml-2 text-xs"
                >
                  {{ $t("plugin.idc-core.catalog.detail.renewal") }}
                  {{
                    formatMoney(price.renewalAmountMinor, price.currencyCode)
                  }}
                </span>
              </div>
            </div>
          </div>
        </div>
        <Empty v-else :image="Empty.PRESENTED_IMAGE_SIMPLE" />
      </Card>

      <Card v-if="canManageCosts" :bordered="true" :title="$t('plugin.idc-core.catalog.cost.title')" size="small" data-testid="catalog-product-costs">
        <template #extra><Button type="primary" @click="openCost()">{{ $t("plugin.idc-core.catalog.cost.add") }}</Button></template>
        <div v-if="versionCosts.length" class="grid gap-2">
          <div v-for="cost in versionCosts" :key="cost.costNo" class="flex items-center justify-between gap-4 rounded-md border p-3">
            <div class="grid min-w-0 flex-1 grid-cols-2 gap-2 sm:grid-cols-4">
              <div><span class="text-muted-foreground block text-xs">{{ $t("plugin.idc-core.catalog.cost.currency") }}</span><strong>{{ cost.currencyCode }}</strong></div>
              <div><span class="text-muted-foreground block text-xs">{{ $t("plugin.idc-core.catalog.cost.period") }}</span><span>{{ cost.periodCount }} × {{ codeLabel("period", cost.periodUnit) }}</span></div>
              <div><span class="text-muted-foreground block text-xs">{{ $t("plugin.idc-core.catalog.cost.unitCost") }}</span><span>{{ formatMoney(cost.unitCostMinor, cost.currencyCode) }}</span></div>
              <div><span class="text-muted-foreground block text-xs">{{ $t("plugin.idc-core.catalog.cost.validity") }}</span><span>{{ formatTimestamp(cost.effectiveFrom) }}<template v-if="cost.effectiveUntil"> — {{ formatTimestamp(cost.effectiveUntil) }}</template></span></div>
            </div>
            <Button class="shrink-0" @click="openCost(cost)">{{ $t("plugin.idc-core.catalog.cost.edit") }}</Button>
          </div>
        </div>
        <Empty v-else :description="$t('plugin.idc-core.catalog.cost.empty')" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
      </Card>

      <Card
        :bordered="true"
        :title="$t('plugin.idc-core.catalog.detail.tax')"
        data-testid="catalog-detail-tax"
        size="small"
      >
        <div v-if="detail.taxPolicies.length" class="grid gap-3 sm:grid-cols-2">
          <div
            v-for="policy in detail.taxPolicies"
            :key="policy.taxPolicyNo"
            class="rounded-md border p-3"
          >
            <div class="mb-2 flex items-center justify-between">
              <strong>{{ policy.currencyCode }}</strong>
              <Tag>{{ codeLabel("taxTreatment", policy.taxTreatment) }}</Tag>
            </div>
            <div class="text-muted-foreground flex justify-between text-sm">
              <span>{{ $t("plugin.idc-core.catalog.detail.taxRate") }}</span>
              <span>{{ formatTaxRate(policy.taxRateBasisPoints) }}</span>
            </div>
            <div
              class="text-muted-foreground mt-1 flex justify-between text-sm"
            >
              <span>{{
                $t("plugin.idc-core.catalog.detail.priceIncludesTax")
              }}</span>
              <span>{{
                $t(
                  policy.priceIncludesTax
                    ? "plugin.idc-core.catalog.detail.yes"
                    : "plugin.idc-core.catalog.detail.no",
                )
              }}</span>
            </div>
          </div>
        </div>
        <Empty v-else :image="Empty.PRESENTED_IMAGE_SIMPLE" />
      </Card>
      <BindingModal @success="reloadDetail" />
      <PolicyModal @success="reloadDetail" />
  <CostModal @success="reloadCosts" />
      <InventoryModal @success="applyInventory" />
      <ConfigurationPriceModal @success="reloadDetail" />
    </div>
  </Drawer>
</template>
