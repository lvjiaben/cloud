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
  evaluatePurchaseConfiguration,
  getPurchaseConfiguration,
  quotePurchaseConfiguration,
  type PurchaseConfigurationField,
  type PurchaseConfigurationOption,
  type PurchaseConfigurationQuote,
  type PurchaseConfigurationQuoteTerm,
  type PurchaseConfigurationValue,
  type ServerLoginPolicy,
} from "./purchase-configuration-client";
import type { Locale } from "./public-i18n";
import { loginForWebsite, commerceRequest, syncWebsiteSession, websiteAccessToken, savePurchaseDraft, summarizePurchaseConfiguration, type Cart } from "./website-commerce-client";

const props = defineProps<{
  currencies: string[];
  locale: Locale;
  publicSlug: string;
  quoteItemNo?: string;
  quantity?: number;
  serverLoginToken?: string;
  serverLoginHostname?: string;
  serverLoginPasswordConfigured?: boolean;
}>();

const emit=defineEmits<{configured:[]}>();
const localePath=useLocalePath();
const content = {
  "zh-CN": {
    multiServer: "此项包含多台服务器，请分别配置后加入购物车",
    passwordReentry: "登录后请重新填写主机密码，或留空自动生成。",
    choose: "选择配置",
    addFailed: "商品没有加入购物车，请保留当前配置后重试。",
    addToCart: "加入购物车",
    continuePurchase: "登录并继续购买",
    configurationEmpty: "这个商品不需要客户填写额外配置。",
 advancedSettings:"高级配置",
    configurationError: "商品配置暂时不可用，请稍后重试。",
    choicesPending: "上游可选内容尚未就绪。可以继续填写其他配置，稍后重新读取。",
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
    quoteTitle: "计费周期与价格",
    termAmount: "商品与配置",
    setupAmount: "一次性费用",
    taxAmount: "税费（合计已包含）",
    totalAmount: "本次合计",
    purchaseCycles: "本次包含 {count} 个计费周期",
    requiresCustomer: "登录并完成客户资料核验后才能形成最终价格。",
    retry: "重新加载",
    saveConfiguration: "保存配置并继续",
    saveFailed: "配置与当前报价不一致或已失效，请返回商品重新选购。",
  },
  "en-US": {
    choicesPending: "Upstream options are not ready. Continue filling other settings and reload the options shortly.",
    multiServer: "This item contains multiple servers. Configure each server separately before adding it to the cart.",
    passwordReentry: "Enter the server password again after sign-in, or leave it blank to generate one.",
    choose: "Choose configuration",
    addFailed: "The product was not added. Keep this configuration and try again.",
    addToCart: "Add to cart",
    continuePurchase: "Sign in and continue",
    advancedSettings:"Advanced settings",
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
    quoteTitle: "Billing period and price",
    termAmount: "Product and configuration",
    setupAmount: "One-time fee",
    taxAmount: "Tax included in total",
    totalAmount: "Total for this purchase",
    purchaseCycles: "This purchase includes {count} billing periods",
    requiresCustomer:
      "Sign in and complete customer verification to receive a final price.",
    retry: "Reload",
    saveConfiguration: "Save configuration and continue",
    saveFailed: "The configuration no longer matches this quote. Return to the product to select it again.",
  },
} as const;

const text = computed(() => content[props.locale]);

const configurationRevision = ref("");
const fields = ref<PurchaseConfigurationField[]>([]);
const values = ref<PurchaseConfigurationValue[]>([]);
const loading = ref(true);
const failed = ref(false);
const evaluationFailed = ref(false);
const evaluating = ref(false);
const valid = ref(false);
const issues = ref(new Map<string, string>());
const currencyCode = ref(props.currencies.at(0) ?? "");
const quote = ref<PurchaseConfigurationQuote>();
const quoteFailed = ref(false);
const quoteLoading = ref(false);
const addingTermToken = ref("");
const addFailed = ref(false);
const showAdvanced = ref(false);
const serverLoginPolicy = ref<ServerLoginPolicy>();
const hostname = ref(props.serverLoginHostname||'');
const serverPassword = ref('');
const passwordNeedsReentry = ref(false);
const serverLoginIssue = ref('');
const preparedServerLogin = ref<{serverLoginToken:string;hostname:string;passwordConfigured:boolean}>();
const serverPasswordConfigured = computed(()=>preparedServerLogin.value?.passwordConfigured||props.serverLoginPasswordConfigured);
const hasServerLogin = computed(()=>Boolean(serverLoginPolicy.value?.hostnameEnabled||serverLoginPolicy.value?.passwordEnabled));
let evaluationTimer: ReturnType<typeof globalThis.setTimeout> | undefined;
let optionTimer: ReturnType<typeof globalThis.setTimeout> | undefined;
let optionReads = 0;
let evaluationSequence = 0;
let quoteSequence = 0;
let loadSequence = 0;

const visibleFields = computed(() =>
  fields.value.filter((field) => field.visible),
);

const missingChoices = computed(() => visibleFields.value.some(field =>
  ['single_select','multi_select','tree_select','address'].includes(field.type) && !field.options?.length,
));
function stopOptionRefresh() {
  if (optionTimer) globalThis.clearTimeout(optionTimer);
  optionTimer = undefined;
}
function scheduleOptionRefresh() {
  stopOptionRefresh();
  if (!missingChoices.value || optionReads >= 20) return;
  optionTimer = globalThis.setTimeout(() => {
    if (document.hidden) return;
    optionReads++;
    void evaluateNow();
  }, 2000);
}
function refreshOptions() { optionReads = 0; stopOptionRefresh(); void evaluateNow(); }

const displayedFields=computed(()=>visibleFields.value.filter(field=>!field.advanced||showAdvanced.value));

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

function usesQuantitySlider(field: PurchaseConfigurationField) {
  if(field.type!=='integer')return false;
  if(field.displayControl==='slider')return true;
  if(field.displayControl&&field.displayControl!=='auto')return false;
  const minimum=field.minimumInteger,maximum=field.maximumInteger;
  if(minimum===undefined||maximum===undefined||!/^[-]?\d+$/u.test(minimum)||!/^[-]?\d+$/u.test(maximum))return false;
  const min=Number(minimum),max=Number(maximum),step=Number(field.quantityStep||1);
  return Number.isSafeInteger(min)&&Number.isSafeInteger(max)&&max>min&&Number.isSafeInteger(step)&&step>0;
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
  // Omit a cleared quantity so interactive evaluation restores the first
  // allowed value or declared minimum, including required numeric controls.
  // Numeric zero remains an explicit selection when allowed by the declaration.
  if ((field.type === "integer" || (field.type === "decimal" && (!field.required || (field.minimumDecimal !== undefined && field.minimumDecimal !== "")))) && (input === "" || input === null || input === undefined)) {
    values.value = values.value.filter(value => value.fieldToken !== field.fieldToken);
  }
  // Invalidate the in-flight evaluation immediately, before the debounce.
  // An older response must never replace the customer's latest input.
  evaluationSequence += 1;
  evaluating.value = true;
  valid.value = false;
  quoteLoading.value = false;
  quoteSequence += 1;
  quote.value = undefined;
  scheduleEvaluation();
}

function stringArray(input: unknown) {
  return Array.isArray(input) ? input.map((item) => `${item}`) : [];
}

async function load() {
  stopOptionRefresh(); optionReads = 0;
  const sequence = ++loadSequence;
  loading.value = true;
  failed.value = false;
  quote.value = undefined;
  try {
    const response = await getPurchaseConfiguration(props.publicSlug);
    if (sequence !== loadSequence) return;
    configurationRevision.value = response.configurationRevision;
    fields.value = response.fields;
    serverLoginPolicy.value = response.serverLoginPolicy;
    values.value = response.values;
    const resumeKey='lina.website.configuration.'+props.publicSlug;
    const pending=sessionStorage.getItem(resumeKey);
    if(pending){
      sessionStorage.removeItem(resumeKey);
      const draft=JSON.parse(pending);
      if(draft.configurationRevision===configurationRevision.value){values.value=draft.values;hostname.value=draft.hostname||'';passwordNeedsReentry.value=draft.passwordNeedsReentry===true;}
    }
    valid.value = response.ready ?? false;
    issues.value = new Map();
    evaluationFailed.value = false;
    if(pending || response.ready === undefined) await evaluateNow();
    if(pending) void syncWebsiteSession().then(() => { if(websiteAccessToken()) return loadQuote(); }).catch(() => {});
  } catch {
    if (sequence === loadSequence) failed.value = true;
  } finally {
    if (sequence === loadSequence) { loading.value = false; if (!failed.value) scheduleOptionRefresh(); }
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
  evaluationFailed.value = false;
  try {
    const response = await evaluatePurchaseConfiguration(
      props.publicSlug,
      configurationRevision.value,
      values.value,
    );
    if (sequence !== evaluationSequence) return;
    configurationRevision.value = response.configurationRevision;
    fields.value = response.fields;
    serverLoginPolicy.value = response.serverLoginPolicy;
    values.value = response.values;
    valid.value = response.valid;
    issues.value = new Map(
      response.issues
        .filter((issue) => issue.fieldToken)
        .map((issue) => [issue.fieldToken as string, issue.issueCode]),
    );
  } catch {
    if (sequence === evaluationSequence) {evaluationFailed.value = true; valid.value = false;}
  } finally {
    if (sequence === evaluationSequence) { evaluating.value = false; if (!evaluationFailed.value) scheduleOptionRefresh(); }
  }
}

async function loadQuote() {
  if (!valid.value || evaluating.value || !currencyCode.value) return;
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
  if (!quote.value || evaluating.value || quoteLoading.value || term.status !== "final" || addingTermToken.value || !quote.value.terms.some(item => item.billingTermToken === term.billingTermToken)) return;
  const selectedQuote = quote.value;
  const selectedValues: PurchaseConfigurationValue[] = JSON.parse(JSON.stringify(values.value));
  const selectedSummary = summarizePurchaseConfiguration(fields.value, selectedValues);
  addingTermToken.value = term.billingTermToken;
  addFailed.value = false;
  try {
    if (!await syncWebsiteSession()) { await signInAndQuote(); return; }
    const serverLoginToken=await prepareServerLogin();
    const current=await commerceRequest<{cart:Cart|null}>('account/commerce/cart');
    if(!current.cart)await commerceRequest('account/commerce/carts','POST',{currencyCode:selectedQuote.currencyCode});
    const before=(await commerceRequest<{cart:Cart}>('account/commerce/cart')).cart;
    const input={billingTermToken:term.billingTermToken,configurationRevision:selectedQuote.configurationRevision,currencyCode:selectedQuote.currencyCode,publicSlug:props.publicSlug,purchaseCycles:term.minimumPurchaseCycles,quantity:1,values:selectedValues,...(serverLoginToken?{serverLoginToken}:{})};
    await commerceRequest('account/commerce/cart-items','POST',{...input,expectedCartVersion:before.version});
    const after=(await commerceRequest<{cart:Cart}>('account/commerce/cart')).cart;
    const added=after.items.filter(item=>!before.items.some(previous=>previous.cartItemNo===item.cartItemNo));
    if(after.version===before.version+1&&added.length===1&&added[0]?.configurationReference){
      savePurchaseDraft(added[0].configurationReference,{publicSlug:props.publicSlug,configurationRevision:input.configurationRevision,values:input.values,summary:selectedSummary,preview:{currencyCode:selectedQuote.currencyCode,totalAmountMinor:term.totalAmount,termZhCN:term.labelZhCN,termEnUS:term.labelEnUS}});
    }
    serverPassword.value='';passwordNeedsReentry.value=false;
    await navigateTo(localePath('/cart'));

  } catch {
    addFailed.value = true;
  } finally {
    addingTermToken.value = "";
  }
}

async function saveConfiguration(){
  if(!props.quoteItemNo||!valid.value||addingTermToken.value)return;
  addingTermToken.value=props.quoteItemNo;addFailed.value=false;
  try{
    const serverLoginToken=await prepareServerLogin();
    await commerceRequest(`account/commerce/quote-items/${encodeURIComponent(props.quoteItemNo)}/purchase-configuration`,'POST',{publicSlug:props.publicSlug,configurationRevision:configurationRevision.value,values:values.value,...(serverLoginToken?{serverLoginToken}:{})});
    serverPassword.value='';passwordNeedsReentry.value=false;
    emit('configured');
  }catch{addFailed.value=true;}finally{addingTermToken.value='';}
}

async function prepareServerLogin() {
  serverLoginIssue.value='';
  if(!hasServerLogin.value)return '';
  const nextHostname=serverLoginPolicy.value?.hostnameEnabled?hostname.value:'';
  const nextPassword=serverLoginPolicy.value?.passwordEnabled?serverPassword.value:'';
  const previous=preparedServerLogin.value;
  const previousToken=previous?.serverLoginToken||props.serverLoginToken;
  const previousHostname=previous?.hostname??props.serverLoginHostname??'';
  if(previousToken&&nextHostname===previousHostname&&!nextPassword)return previous?.serverLoginToken||'';
  if(!previousToken&&!nextHostname&&!nextPassword)return '';
  if((props.quantity??1)>1){serverLoginIssue.value=text.value.multiServer;throw new Error('server_login_requires_single');}
  const result=await commerceRequest<{serverLoginToken:string;hostname:string;passwordConfigured:boolean}>(`account/catalog/products/${encodeURIComponent(props.publicSlug)}/purchase-configuration/server-login`,'POST',{configurationRevision:configurationRevision.value,...(previousToken?{previousServerLoginToken:previousToken}:{}),...(nextHostname?{hostname:nextHostname}:{}),...(nextPassword?{password:nextPassword}:{})});
  preparedServerLogin.value=result;
  hostname.value=result.hostname;
  serverPassword.value='';passwordNeedsReentry.value=false;
  return result.serverLoginToken;
}

async function signInAndQuote() {
  sessionStorage.setItem('lina.website.configuration.'+props.publicSlug,JSON.stringify({configurationRevision:configurationRevision.value,values:values.value,hostname:hostname.value,passwordNeedsReentry:Boolean(serverPassword.value)||Boolean(preparedServerLogin.value?.passwordConfigured)}));
  if (await loginForWebsite()) {sessionStorage.removeItem('lina.website.configuration.'+props.publicSlug);await loadQuote();}
}

function invalidateQuote() {
  quoteSequence += 1;
  quote.value = undefined;
  quoteFailed.value = false;
  quoteLoading.value = false;
}

// Display only amounts supplied by the quote owner. Older/incomplete payloads
// may omit the breakdown; do not invent zero fees or calculate prices here.
function hasQuoteBreakdown(term: PurchaseConfigurationQuoteTerm) {
  return [term.subtotalAmount,term.setupAmount,term.taxAmount].every(amount => typeof amount === 'string' && /^-?\d+$/u.test(amount));
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

onMounted(() => {
  // Public configuration does not depend on the cross-origin session probe.
  void syncWebsiteSession().catch(() => {});
  void load();
});
onBeforeUnmount(() => {
  serverPassword.value='';
  stopOptionRefresh();
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
    <template v-else>
      <AAlert v-if="missingChoices" type="info" show-icon :message="text.choicesPending" class="mb-4" data-testid="configuration-options-pending"><template #action><AButton size="small" :loading="evaluating" @click="refreshOptions">{{ text.retry }}</AButton></template></AAlert>
      <AAlert v-if="evaluationFailed" type="warning" show-icon :message="text.configurationError" class="mb-4" data-testid="configuration-evaluation-error">
        <template #action><AButton size="small" :loading="evaluating" @click="evaluateNow">{{text.retry}}</AButton></template>
      </AAlert>
      <AEmpty
        v-if="visibleFields.length === 0 && !hasServerLogin"
        :description="text.configurationEmpty"
      />
      <AForm v-else class="purchase-configuration-form" layout="vertical">
        <AButton v-if="visibleFields.some(field=>field.advanced)" class="mb-4" data-testid="purchase-advanced-toggle" @click="showAdvanced=!showAdvanced">{{ text.advancedSettings }}</AButton>
        <template v-for="(field,index) in displayedFields" :key="field.fieldToken">
        <h3 v-if="(props.locale==='zh-CN'?field.groupZhCN:field.groupEnUS)&&(index===0||field.groupZhCN!==displayedFields[index-1]?.groupZhCN||field.groupEnUS!==displayedFields[index-1]?.groupEnUS)" class="mb-3 font-semibold">{{props.locale==='zh-CN'?field.groupZhCN:field.groupEnUS}}</h3>
        <AFormItem
          :html-for="['single_select','multi_select'].includes(field.type) ? `purchase-${field.fieldToken}` : undefined"
          :help="
            issues.has(field.fieldToken)
              ? text.configurationInvalid
              : localized(field, 'description')
          "
          :required="field.required"
          :validate-status="issues.has(field.fieldToken) ? 'error' : undefined"
        >
          <template #label>{{ localized(field, "label") }}<span v-if="field.unit">（{{field.unit}}）</span></template>
          <AInput
            v-if="field.type === 'single_line'"
            :aria-label="localized(field, 'label')"
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
          <div v-else-if="usesQuantitySlider(field)" :data-testid="`purchase-slider-${field.fieldToken}`">
            <PurchaseQuantitySlider :disabled="field.disabled" :minimum="field.minimumInteger || '0'" :maximum="field.maximumInteger || '0'" :step="field.quantityStep || '1'" :value="stringScalarValue(field)" :label="localized(field,'label')" :unit="field.unit" :locale="locale" @update:value="setScalarValue(field,$event)" />
          </div>
          <ARadioGroup v-else-if="field.type === 'integer' && field.displayControl === 'radio'" option-type="button" button-style="solid" :disabled="field.disabled" :options="(field.numericChoices || []).map(value=>({value,label:(props.locale==='zh-CN'?field.numericLabels?.[value]?.zhCN:field.numericLabels?.[value]?.enUS)||value}))" :value="stringScalarValue(field)" @update:value="setScalarValue(field, $event)" />
          <ASelect v-else-if="field.type === 'integer' && field.displayControl === 'select'" :disabled="field.disabled" :options="(field.numericChoices || []).map(value=>({value,label:(props.locale==='zh-CN'?field.numericLabels?.[value]?.zhCN:field.numericLabels?.[value]?.enUS)||value}))" :value="stringScalarValue(field) || undefined" @update:value="setScalarValue(field, $event)" />
          <AInput
            v-else-if="field.type === 'integer' || field.type === 'decimal'"
            :disabled="field.disabled"
            :aria-label="localized(field, 'label')"
            :inputmode="field.type === 'integer' ? 'numeric' : 'decimal'"
            type="number"
            :min="field.minimumInteger || field.minimumDecimal"
            :max="field.maximumInteger || field.maximumDecimal"
            :step="field.type === 'integer' ? (field.quantityStep || '1') : 'any'"
            :value="stringScalarValue(field)"
            @update:value="setScalarValue(field, $event)"
          />
          <ASwitch
            v-else-if="field.type === 'boolean'"
            :checked="booleanScalarValue(field)"
            :disabled="field.disabled"
            @update:checked="setScalarValue(field, $event)"
          />
          <PurchaseOperatingSystem v-else-if="field.type === 'single_select' && field.presentation === 'operating_system'" :options="field.options || []" :value="selectScalarValue(field) as string | undefined" :disabled="field.disabled" :label="localized(field,'label')" :locale="locale" @update:value="setScalarValue(field,$event)" />
          <PurchaseRegion v-else-if="field.type === 'single_select' && field.presentation === 'region'" :options="field.options || []" :value="selectScalarValue(field) as string | undefined" :disabled="field.disabled" :label="localized(field,'label')" :locale="locale" @update:value="setScalarValue(field,$event)" />
          <ARadioGroup
            v-else-if="
              field.type === 'single_select' &&
              (field.displayControl === 'radio' || ((!field.displayControl || field.displayControl === 'auto') && (field.options?.length ?? 0) <= 5))
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
            :aria-label="localized(field, 'label')"
            :id="`purchase-${field.fieldToken}`"
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
        </template>
      </AForm>
      <AAlert v-if="passwordNeedsReentry&&hasServerLogin" :message="text.passwordReentry" type="info" show-icon />
      <AAlert v-if="serverLoginIssue" :message="serverLoginIssue" type="warning" show-icon data-testid="purchase-server-login-issue" />
      <PurchaseServerLogin v-if="hasServerLogin&&serverLoginPolicy" v-model:hostname="hostname" v-model:password="serverPassword" :policy="serverLoginPolicy" :password-configured="serverPasswordConfigured" :locale="locale" :disabled="Boolean(addingTermToken)" />

      <AAlert
        v-if="!valid && !evaluating"
        class="catalog-alert"
        :message="text.configurationInvalid"
        show-icon
        type="warning"
      />
      <AAlert
        v-if="addFailed && !serverLoginIssue"
        :message="props.quoteItemNo?text.saveFailed:text.addFailed"
        show-icon
        type="error"
      />
      <div v-if="!props.quoteItemNo" class="purchase-configuration-actions">
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

      <AButton v-if="props.quoteItemNo" :disabled="!valid || evaluating" :loading="Boolean(addingTermToken)" type="primary" @click="saveConfiguration">{{text.saveConfiguration}}</AButton>
      <AAlert
        v-if="quoteFailed"
        :message="text.quoteError"
        show-icon
        type="error"
      />
      <ACard v-if="quote && !props.quoteItemNo" class="purchase-configuration-quote" size="small">
        <h3>{{ text.quoteTitle }}</h3>
        <div
          v-for="term in quote.terms"
          :key="term.billingTermToken"
          class="purchase-quote-term"
        >
          <div class="purchase-quote-heading">
            <strong>{{
              props.locale === "zh-CN" ? term.labelZhCN : term.labelEnUS
            }}</strong>
            <p v-if="term.minimumPurchaseCycles > 1" class="purchase-summary-note" data-testid="public-quote-purchase-cycles">{{ text.purchaseCycles.replace('{count}', String(term.minimumPurchaseCycles)) }}</p>
            <p v-if="term.status === 'requires_customer_context'">
              {{ text.requiresCustomer }}
            </p>
          </div>
          <dl v-if="term.status === 'final' && hasQuoteBreakdown(term)" class="purchase-quote-breakdown" data-testid="public-quote-breakdown">
            <div><dt>{{ text.termAmount }}</dt><dd data-testid="public-quote-subtotal">{{ formatAmount(term.subtotalAmount, quote.currencyCode) }}</dd></div>
            <div><dt>{{ text.setupAmount }}</dt><dd data-testid="public-quote-setup">{{ formatAmount(term.setupAmount, quote.currencyCode) }}</dd></div>
            <div><dt>{{ text.taxAmount }}</dt><dd data-testid="public-quote-tax">{{ formatAmount(term.taxAmount, quote.currencyCode) }}</dd></div>
          </dl>
          <div class="purchase-quote-action">
            <div v-if="term.status === 'final'" class="purchase-quote-total" data-testid="public-quote-total">
              <span>{{ text.totalAmount }}</span>
              <strong>{{ formatAmount(term.totalAmount, quote.currencyCode) }}</strong>
            </div>
            <AButton
              v-if="term.status === 'final'"
              :loading="addingTermToken === term.billingTermToken"
              data-testid="public-add-to-cart"
              type="primary"
              @click="addToCart(term)"
            >
              {{ text.addToCart }}
            </AButton>
            <AButton v-else type="primary" @click="signInAndQuote">{{ text.continuePurchase }}</AButton>
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

.purchase-configuration-quote :deep(.ant-card-body) { padding:12px; }
.purchase-quote-heading { flex:1 1 100%;min-width:0; }
.purchase-quote-term { flex-wrap:wrap;min-width:0; }
.purchase-quote-breakdown { flex:1 1 100%;display:grid;gap:8px;margin:4px 0 8px;font-size:12px; }
.purchase-quote-breakdown > div { display:flex;justify-content:space-between;gap:16px; }
.purchase-quote-breakdown dt { color:#64748b;min-width:0; }
.purchase-quote-breakdown dd { margin:0;text-align:right;overflow-wrap:anywhere;min-width:0;flex-shrink:0;max-width:65%; }
.purchase-quote-total { display:flex;flex-direction:column;gap:4px;min-width:0; }
.purchase-quote-total span { color:#64748b;font-size:12px; }
.purchase-quote-total strong { font-size:18px;overflow-wrap:anywhere; }
.purchase-quote-action {
  align-items: flex-end;
  display: flex;
  flex-wrap:wrap;
  justify-content:space-between;
  width:100%;
  gap: 12px;
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
