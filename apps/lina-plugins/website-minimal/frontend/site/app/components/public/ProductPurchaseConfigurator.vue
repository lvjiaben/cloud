<script setup lang="ts">
// This component renders the host-governed public configuration projection and
// always asks the server to evaluate conditions and authoritative prices.
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import {
  Alert as AAlert,
  Button as AButton,
  Card as ACard,
  Cascader as ACascader,
  Empty as AEmpty,
  Form as AForm,
  FormItem as AFormItem,
  Input as AInput,
  RadioGroup as ARadioGroup,
  Select as ASelect,
  Skeleton as ASkeleton,
  Switch as ASwitch,
  Textarea as ATextarea,
  TreeSelect as ATreeSelect,
} from "ant-design-vue";

import {
  addGuestCartItem,
  evaluatePurchaseConfiguration,
  getPurchaseConfiguration,
  openGuestCart,
  quotePurchaseConfiguration,
  type PurchaseConfigurationField,
  type PurchaseConfigurationOption,
  type PurchaseConfigurationQuote,
  type PurchaseConfigurationQuoteTerm,
  type PurchaseConfigurationValue,
} from "./purchase-configuration-client";
import type { Locale } from "./public-i18n";

const props = defineProps<{
  currencies: string[];
  locale: Locale;
  publicSlug: string;
}>();

const content = {
  "zh-CN": {
    choose: "选择配置",
    addFailed: "商品没有加入购物车，请保留当前配置后重试。",
    addToCart: "加入购物车",
    configurationEmpty: "这个商品不需要客户填写额外配置。",
    configurationError: "商品配置暂时不可用，请稍后重试。",
    configurationInvalid: "请检查标出的配置后再查看价格。",
    currency: "报价币种",
    fileAfterLogin: "文件将在登录后的安全购买流程中上传",
    fileAllowedExtensions: "允许扩展名",
    fileAllowedMimeTypes: "允许文件类型",
    fileMaximumFiles: "最多文件数",
    fileMaximumSingle: "单个文件上限",
    fileMaximumTotal: "全部文件合计上限",
    fileSafetyScan: "安全扫描",
    fileSafetyScanNo: "不要求",
    fileSafetyScanYes: "需要",
    load: "加载配置",
    quote: "查看准确价格",
    quoteError: "当前价格暂时无法确认，请稍后重试。",
    quoteTitle: "当前权威价格",
    requiresCustomer: "登录并完成客户资料核验后才能形成最终价格。",
    retry: "重新加载",
  },
  "en-US": {
    choose: "Choose configuration",
    addFailed: "The product was not added. Keep this configuration and try again.",
    addToCart: "Add to cart",
    configurationEmpty:
      "This product needs no additional customer configuration.",
    configurationError:
      "Product configuration is temporarily unavailable. Please try again.",
    configurationInvalid:
      "Review the highlighted configuration before checking the price.",
    currency: "Quote currency",
    fileAfterLogin: "Files are uploaded in the secure signed-in purchase flow",
    fileAllowedExtensions: "Allowed extensions",
    fileAllowedMimeTypes: "Allowed file types",
    fileMaximumFiles: "Maximum files",
    fileMaximumSingle: "Per-file limit",
    fileMaximumTotal: "Combined limit",
    fileSafetyScan: "Safety scan",
    fileSafetyScanNo: "Not required",
    fileSafetyScanYes: "Required",
    load: "Load configuration",
    quote: "Check exact price",
    quoteError:
      "The current price cannot be confirmed right now. Please try again.",
    quoteTitle: "Current authoritative price",
    requiresCustomer:
      "Sign in and complete customer verification to receive a final price.",
    retry: "Reload",
  },
} as const;

const text = computed(() => content[props.locale]);
const configurationRevision = ref("");
const fields = ref<PurchaseConfigurationField[]>([]);
const values = ref<PurchaseConfigurationValue[]>([]);
const loading = ref(false);
const failed = ref(false);
const evaluating = ref(false);
const valid = ref(false);
const issues = ref(new Map<string, string>());
const currencyCode = ref(props.currencies.at(0) ?? "");
const quote = ref<PurchaseConfigurationQuote>();
const quoteFailed = ref(false);
const quoteLoading = ref(false);
const addingTermToken = ref("");
const addFailed = ref(false);
let evaluationTimer: ReturnType<typeof globalThis.setTimeout> | undefined;
let evaluationSequence = 0;
let quoteSequence = 0;
let loadSequence = 0;

const visibleFields = computed(() =>
  fields.value.filter((field) => field.visible),
);

function localized(
  field: PurchaseConfigurationField,
  key: "description" | "label",
) {
  if (key === "label")
    return props.locale === "zh-CN" ? field.labelZhCN : field.labelEnUS;
  return props.locale === "zh-CN"
    ? field.descriptionZhCN
    : field.descriptionEnUS;
}

function optionLabel(option: PurchaseConfigurationOption) {
  return props.locale === "zh-CN" ? option.labelZhCN : option.labelEnUS;
}

function selectOptions(field: PurchaseConfigurationField) {
  return (field.options ?? []).map((option) => ({
    label: optionLabel(option),
    value: option.optionToken,
  }));
}

function treeOptions(
  options: PurchaseConfigurationOption[] = [],
): Array<Record<string, unknown>> {
  return options.map((option) => ({
    children: option.children ? treeOptions(option.children) : undefined,
    label: optionLabel(option),
    value: option.optionToken,
  }));
}

function valueFor(field: PurchaseConfigurationField) {
  return values.value.find((value) => value.fieldToken === field.fieldToken);
}

function ensureValue(field: PurchaseConfigurationField) {
  let current = valueFor(field);
  if (!current) {
    current = { fieldToken: field.fieldToken, type: field.type };
    values.value.push(current);
  }
  return current;
}

function scalarValue(field: PurchaseConfigurationField) {
  const current = valueFor(field);
  if (!current) {
    if (["address", "multi_select"].includes(field.type)) return [];
    if (field.type === "boolean") return false;
    return "";
  }
  if (field.type === "boolean") return current.booleanValue ?? false;
  if (field.type === "date") return current.dateValue ?? "";
  if (field.type === "datetime") return current.dateTimeValue ?? "";
  if (field.type === "decimal") return current.decimalValue ?? "";
  if (field.type === "integer") return current.integerValue ?? "";
  if (field.type === "address") return current.addressOptionTokens ?? [];
  if (field.type === "tree_select")
    return current.treeOptionTokens?.at(0) ?? "";
  if (field.type === "single_select") return current.optionTokens?.at(0);
  if (field.type === "multi_select") return current.optionTokens ?? [];
  return current.textValue ?? "";
}

function stringScalarValue(field: PurchaseConfigurationField) {
  const current = scalarValue(field);
  if (field.type === "datetime" && typeof current === "string") {
    return toLocalDateTimeInput(current);
  }
  return typeof current === "string" ? current : "";
}

function toLocalDateTimeInput(value: string) {
  if (!value) return "";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  const offset = parsed.getTimezoneOffset() * 60_000;
  return new Date(parsed.getTime() - offset).toISOString().slice(0, 16);
}

function toRFC3339DateTime(value: unknown) {
  const normalized = `${value ?? ""}`;
  if (!normalized) return "";
  const parsed = new Date(normalized);
  return Number.isNaN(parsed.getTime()) ? normalized : parsed.toISOString();
}

function booleanScalarValue(field: PurchaseConfigurationField) {
  return scalarValue(field) === true;
}

function selectScalarValue(field: PurchaseConfigurationField) {
  const current = scalarValue(field);
  return typeof current === "string" || Array.isArray(current)
    ? current
    : undefined;
}

function tokenArrayValue(field: PurchaseConfigurationField) {
  const current = scalarValue(field);
  return Array.isArray(current) ? current : [];
}

function setScalarValue(field: PurchaseConfigurationField, input: unknown) {
  const current = ensureValue(field);
  delete current.addressOptionTokens;
  delete current.booleanValue;
  delete current.dateTimeValue;
  delete current.dateValue;
  delete current.decimalValue;
  delete current.fileTokens;
  delete current.integerValue;
  delete current.optionTokens;
  delete current.textValue;
  delete current.treeOptionTokens;
  if (field.type === "boolean") current.booleanValue = input === true;
  else if (field.type === "date") current.dateValue = `${input ?? ""}`;
  else if (field.type === "datetime")
    current.dateTimeValue = toRFC3339DateTime(input);
  else if (field.type === "decimal") current.decimalValue = `${input ?? ""}`;
  else if (field.type === "integer") current.integerValue = `${input ?? ""}`;
  else if (field.type === "address")
    current.addressOptionTokens = stringArray(input);
  else if (field.type === "tree_select")
    current.treeOptionTokens =
      input === undefined || input === null || input === "" ? [] : [`${input}`];
  else if (field.type === "single_select")
    current.optionTokens = input ? [`${input}`] : [];
  else if (field.type === "multi_select")
    current.optionTokens = stringArray(input);
  else current.textValue = `${input ?? ""}`;
  quoteSequence += 1;
  quote.value = undefined;
  scheduleEvaluation();
}

function stringArray(input: unknown) {
  return Array.isArray(input) ? input.map((item) => `${item}`) : [];
}

async function load() {
  const sequence = ++loadSequence;
  loading.value = true;
  failed.value = false;
  quote.value = undefined;
  try {
    const response = await getPurchaseConfiguration(props.publicSlug);
    if (sequence !== loadSequence) return;
    configurationRevision.value = response.configurationRevision;
    fields.value = response.fields;
    values.value = response.values;
    await evaluateNow();
  } catch {
    if (sequence === loadSequence) failed.value = true;
  } finally {
    if (sequence === loadSequence) loading.value = false;
  }
}

function scheduleEvaluation() {
  if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
  evaluationTimer = globalThis.setTimeout(() => void evaluateNow(), 250);
}

async function evaluateNow() {
  if (!configurationRevision.value) return;
  const sequence = ++evaluationSequence;
  evaluating.value = true;
  try {
    const response = await evaluatePurchaseConfiguration(
      props.publicSlug,
      configurationRevision.value,
      values.value,
    );
    if (sequence !== evaluationSequence) return;
    configurationRevision.value = response.configurationRevision;
    fields.value = response.fields;
    values.value = response.values;
    valid.value = response.valid;
    issues.value = new Map(
      response.issues
        .filter((issue) => issue.fieldToken)
        .map((issue) => [issue.fieldToken as string, issue.issueCode]),
    );
  } catch {
    if (sequence === evaluationSequence) failed.value = true;
  } finally {
    if (sequence === evaluationSequence) evaluating.value = false;
  }
}

async function loadQuote() {
  if (!valid.value || !currencyCode.value) return;
  const sequence = ++quoteSequence;
  quoteFailed.value = false;
  quoteLoading.value = true;
  try {
    const response = await quotePurchaseConfiguration(
      props.publicSlug,
      configurationRevision.value,
      currencyCode.value,
      values.value,
    );
    if (sequence === quoteSequence) quote.value = response;
  } catch {
    if (sequence === quoteSequence) {
      quote.value = undefined;
      quoteFailed.value = true;
    }
  } finally {
    if (sequence === quoteSequence) quoteLoading.value = false;
  }
}

async function addToCart(term: PurchaseConfigurationQuoteTerm) {
  if (!quote.value || term.status !== "final" || addingTermToken.value) return;
  addingTermToken.value = term.billingTermToken;
  addFailed.value = false;
  try {
    await openGuestCart(quote.value.currencyCode);
    const response = await addGuestCartItem({
      billingTermToken: term.billingTermToken,
      configurationRevision: quote.value.configurationRevision,
      currencyCode: quote.value.currencyCode,
      publicSlug: props.publicSlug,
      purchaseCycles: term.minimumPurchaseCycles,
      quantity: 1,
      values: values.value,
    });
    if (!response.cart.reference) throw new Error("guest_cart_reference_missing");
    const customerURL = useRuntimeConfig().public.customerUrl;
    await navigateTo(new URL("/account/cart", customerURL).toString(), {
      external: true,
    });
  } catch {
    addFailed.value = true;
  } finally {
    addingTermToken.value = "";
  }
}

function invalidateQuote() {
  quoteSequence += 1;
  quote.value = undefined;
  quoteFailed.value = false;
  quoteLoading.value = false;
}

function formatAmount(amount: string, currency: string) {
  if (!/^-?\d+$/u.test(amount)) return amount;
  const minor = BigInt(amount);
  const digits =
    new Intl.NumberFormat(props.locale, {
      currency,
      style: "currency",
    }).resolvedOptions().maximumFractionDigits ?? 2;
  const divisor = 10n ** BigInt(digits);
  const absolute = minor < 0n ? -minor : minor;
  const fraction = `${absolute % divisor}`.padStart(digits, "0");
  const normalized = `${minor < 0n ? "-" : ""}${absolute / divisor}${digits ? `.${fraction}` : ""}`;
  return `${currency} ${normalized}`;
}

function formatBytes(bytes = 0) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KiB", "MiB", "GiB"];
  let value = bytes;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value >= 10 || Number.isInteger(value) ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`;
}

onMounted(() => void load());
onBeforeUnmount(() => {
  if (evaluationTimer) globalThis.clearTimeout(evaluationTimer);
  evaluationSequence += 1;
  quoteSequence += 1;
  loadSequence += 1;
});
</script>

<template>
  <ACard
    class="catalog-detail-card"
    :bordered="true"
    data-testid="public-product-configurator"
  >
    <div class="product-card-heading">
      <h2>{{ text.choose }}</h2>
      <AButton v-if="failed" size="small" @click="load">{{
        text.retry
      }}</AButton>
    </div>
    <ASkeleton v-if="loading" active :paragraph="{ rows: 5 }" />
    <AAlert
      v-else-if="failed"
      :message="text.configurationError"
      show-icon
      type="warning"
    />
    <AEmpty
      v-else-if="visibleFields.length === 0"
      :description="text.configurationEmpty"
    />
    <template v-else>
      <AForm class="purchase-configuration-form" layout="vertical">
        <AFormItem
          v-for="field in visibleFields"
          :key="field.fieldToken"
          :help="
            issues.has(field.fieldToken)
              ? text.configurationInvalid
              : localized(field, 'description')
          "
          :required="field.required"
          :validate-status="issues.has(field.fieldToken) ? 'error' : undefined"
        >
          <template #label>{{ localized(field, "label") }}</template>
          <AInput
            v-if="field.type === 'single_line'"
            :disabled="field.disabled"
            :maxlength="field.maximumLength"
            :value="stringScalarValue(field)"
            @update:value="setScalarValue(field, $event)"
          />
          <ATextarea
            v-else-if="field.type === 'multi_line'"
            :disabled="field.disabled"
            :maxlength="field.maximumLength"
            :rows="4"
            :value="stringScalarValue(field)"
            @update:value="setScalarValue(field, $event)"
          />
          <AInput
            v-else-if="field.type === 'integer' || field.type === 'decimal'"
            :disabled="field.disabled"
            :inputmode="field.type === 'integer' ? 'numeric' : 'decimal'"
            :value="stringScalarValue(field)"
            @update:value="setScalarValue(field, $event)"
          />
          <ASwitch
            v-else-if="field.type === 'boolean'"
            :checked="booleanScalarValue(field)"
            :disabled="field.disabled"
            @update:checked="setScalarValue(field, $event)"
          />
          <ARadioGroup
            v-else-if="
              field.type === 'single_select' &&
              (field.options?.length ?? 0) <= 5
            "
            :disabled="field.disabled"
            :options="selectOptions(field)"
            :value="selectScalarValue(field)"
            @update:value="setScalarValue(field, $event)"
          />
          <ASelect
            v-else-if="
              field.type === 'single_select' || field.type === 'multi_select'
            "
            :disabled="field.disabled"
            :max-tag-count="field.type === 'multi_select' ? 3 : undefined"
            :mode="field.type === 'multi_select' ? 'multiple' : undefined"
            :options="selectOptions(field)"
            show-search
            :value="selectScalarValue(field)"
            @update:value="setScalarValue(field, $event)"
          />
          <AInput
            v-else-if="field.type === 'date' || field.type === 'datetime'"
            :disabled="field.disabled"
            :type="field.type === 'date' ? 'date' : 'datetime-local'"
            :value="stringScalarValue(field)"
            @update:value="setScalarValue(field, $event)"
          />
          <ATreeSelect
            v-else-if="field.type === 'tree_select'"
            :disabled="field.disabled"
            :tree-data="treeOptions(field.options)"
            :value="selectScalarValue(field)"
            allow-clear
            show-search
            tree-node-filter-prop="label"
            @update:value="setScalarValue(field, $event)"
          />
          <ACascader
            v-else-if="field.type === 'address'"
            :disabled="field.disabled"
            :options="treeOptions(field.options)"
            :value="tokenArrayValue(field)"
            change-on-select
            show-search
            @update:value="setScalarValue(field, $event)"
          />
          <AAlert
            v-else-if="field.type === 'file'"
            :message="text.fileAfterLogin"
            show-icon
            type="info"
          >
            <template #description>
              <dl
                v-if="field.uploadPolicy"
                class="purchase-file-policy"
                data-testid="purchase-file-policy"
              >
                <div>
                  <dt>{{ text.fileMaximumFiles }}</dt>
                  <dd>{{ field.uploadPolicy.maximumFiles }}</dd>
                </div>
                <div>
                  <dt>{{ text.fileMaximumSingle }}</dt>
                  <dd>{{ formatBytes(field.uploadPolicy.maximumFileBytes) }}</dd>
                </div>
                <div>
                  <dt>{{ text.fileMaximumTotal }}</dt>
                  <dd>{{ formatBytes(field.uploadPolicy.maximumTotalBytes) }}</dd>
                </div>
                <div>
                  <dt>{{ text.fileAllowedExtensions }}</dt>
                  <dd>{{ field.uploadPolicy.allowedExtensions.join(", ") }}</dd>
                </div>
                <div>
                  <dt>{{ text.fileAllowedMimeTypes }}</dt>
                  <dd>{{ field.uploadPolicy.allowedMimeTypes.join(", ") }}</dd>
                </div>
                <div>
                  <dt>{{ text.fileSafetyScan }}</dt>
                  <dd>
                    {{
                      field.uploadPolicy.safetyScanRequired
                        ? text.fileSafetyScanYes
                        : text.fileSafetyScanNo
                    }}
                  </dd>
                </div>
              </dl>
            </template>
          </AAlert>
        </AFormItem>
      </AForm>

      <AAlert
        v-if="!valid && !evaluating"
        class="catalog-alert"
        :message="text.configurationInvalid"
        show-icon
        type="warning"
      />
      <AAlert
        v-if="addFailed"
        :message="text.addFailed"
        show-icon
        type="error"
      />
      <div class="purchase-configuration-actions">
        <ASelect
          v-model:value="currencyCode"
          :aria-label="text.currency"
          :options="currencies.map((item) => ({ label: item, value: item }))"
          style="min-width: 112px"
          @change="invalidateQuote"
        />
        <AButton
          :disabled="!valid || evaluating"
          :loading="evaluating || quoteLoading"
          type="primary"
          @click="loadQuote"
        >
          {{ text.quote }}
        </AButton>
      </div>

      <AAlert
        v-if="quoteFailed"
        :message="text.quoteError"
        show-icon
        type="error"
      />
      <ACard v-if="quote" class="purchase-configuration-quote" size="small">
        <h3>{{ text.quoteTitle }}</h3>
        <div
          v-for="term in quote.terms"
          :key="term.billingTermToken"
          class="purchase-quote-term"
        >
          <div>
            <strong>{{
              props.locale === "zh-CN" ? term.labelZhCN : term.labelEnUS
            }}</strong>
            <p v-if="term.status === 'requires_customer_context'">
              {{ text.requiresCustomer }}
            </p>
          </div>
          <div class="purchase-quote-action">
            <strong>{{
              formatAmount(term.totalAmount, quote.currencyCode)
            }}</strong>
            <AButton
              v-if="term.status === 'final'"
              :loading="addingTermToken === term.billingTermToken"
              data-testid="public-add-to-cart"
              type="primary"
              @click="addToCart(term)"
            >
              {{ text.addToCart }}
            </AButton>
          </div>
        </div>
      </ACard>
    </template>
  </ACard>
</template>

<style scoped>
.purchase-configuration-actions,
.purchase-quote-term {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.purchase-configuration-actions {
  border-top: 1px solid var(--ant-color-border-secondary, #f0f0f0);
  margin-top: 8px;
  padding-top: 20px;
}

.purchase-configuration-form :deep(.ant-form-item-label > label) {
  align-items: center;
}

.purchase-configuration-quote {
  margin-top: 16px;
}

.purchase-file-policy {
  display: grid;
  gap: 6px;
  margin: 10px 0 0;
}

.purchase-file-policy div {
  display: grid;
  gap: 8px;
  grid-template-columns: minmax(110px, 0.4fr) minmax(0, 1fr);
}

.purchase-file-policy dt {
  color: var(--ant-color-text-secondary, #666);
}

.purchase-file-policy dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.purchase-quote-term {
  border-top: 1px solid var(--ant-color-border-secondary, #f0f0f0);
  padding: 12px 0;
}

.purchase-quote-term p {
  margin: 4px 0 0;
}

.purchase-quote-action {
  align-items: flex-end;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

@media (max-width: 520px) {
  .purchase-configuration-actions,
  .purchase-quote-term {
    align-items: stretch;
    flex-direction: column;
  }

  .purchase-quote-action {
    align-items: stretch;
  }
}
</style>
