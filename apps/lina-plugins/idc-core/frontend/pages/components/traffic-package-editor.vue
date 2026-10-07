<script setup lang="ts">
import type { TrafficPackage, TrafficProductChoice } from "../traffic-package-client";
import { onBeforeUnmount, ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { Alert, message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { majorToMinorAmount, minorToMajorAmount } from "../catalog-money";
import { saveTrafficPackage, searchTrafficProducts } from "../traffic-package-client";

const emit = defineEmits<{ success: [] }>();
const editing = ref<TrafficPackage>();
const ready = ref(false);
const choices = new Map<string, TrafficProductChoice>();
const tr = (key: string) => $t(`plugin.idc-core.trafficPackages.${key}`);
let searchTimer: ReturnType<typeof setTimeout> | undefined;
let searchVersion = 0;
const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 155 },
  showDefaultActions: false, wrapperClass: "grid-cols-1", schema: [],
});
function updateChoices() {
  return formApi.updateSchema([{ fieldName: "productNos", componentProps: {
    options: [...choices.values()].map(item => ({ label: preferences.app.locale === "zh-CN" ? item.nameZhCN : item.nameEnUS, value: item.productNo })),
  } }]);
}
function search(keyword: string) {
  clearTimeout(searchTimer);
  const version = ++searchVersion;
  searchTimer = setTimeout(async () => {
    try {
    const { items } = await searchTrafficProducts(keyword);
    if (version !== searchVersion) return;
    const values = await formApi.getValues<{ productNos?: string[] }>();
    const retained = new Set(values.productNos || []);
    for (const key of choices.keys()) if (!retained.has(key)) choices.delete(key);
    for (const item of items) choices.set(item.productNo, item);
    await updateChoices();
    } catch {
      // The shared request client displays the error; preserve selected names.
    }
  }, 250);
}
onBeforeUnmount(() => { clearTimeout(searchTimer); searchVersion++; });
const [Modal, modalApi] = useVbenModal({
  class: "w-[min(94vw,720px)]", onConfirm: save,
  async onOpenChange(open) {
    if (!open) { ready.value = false; searchVersion++; clearTimeout(searchTimer); return; }
    ready.value = false;
    modalApi.setState({ confirmDisabled: true, loading: true });
    const version = ++searchVersion;
    try {
    editing.value = modalApi.getData<{ item?: TrafficPackage }>()?.item;
    choices.clear();
    const item = editing.value;
    await formApi.setState({ schema: [
      { fieldName: "nameZhCN", component: "Input", label: tr("nameZhCN"), rules: z.string().trim().min(1).max(120), componentProps: { maxlength: 120 } },
      { fieldName: "nameEnUS", component: "Input", label: tr("nameEnUS"), rules: z.string().trim().min(1).max(120), componentProps: { maxlength: 120 } },
      { fieldName: "trafficGb", component: "InputNumber", label: tr("capacity"), rules: z.coerce.number().int().min(1).max(1_000_000), componentProps: { min: 1, max: 1_000_000, precision: 0, addonAfter: "GB" } },
      { fieldName: "currencyCode", component: "Input", label: tr("currency"), rules: z.string().regex(/^[A-Z]{3}$/), componentProps: { maxlength: 3, disabled: !!item } },
      { fieldName: "amount", component: "Input", label: tr("price"), rules: z.string().regex(/^\d{1,10}(?:\.\d{1,4})?$/), componentProps: { inputmode: "decimal", maxlength: 15 } },
      { fieldName: "stockMode", component: "RadioGroup", label: tr("stockMode"), rules: "required", componentProps: { optionType: "button", buttonStyle: "solid", options: [{ label: tr("unlimited"), value: "unlimited" }, { label: tr("limited"), value: "limited" }] } },
      { fieldName: "stockQuantity", component: "InputNumber", label: tr("totalStock"), rules: z.coerce.number().int().min(0).max(1_000_000_000), componentProps: { min: 0, max: 1_000_000_000, precision: 0 }, dependencies: { triggerFields: ["stockMode"], show: values => values.stockMode === "limited" } },
      { fieldName: "productNos", component: "Select", label: tr("products"), rules: z.array(z.string()).min(1).max(100), componentProps: { mode: "multiple", showSearch: true, filterOption: false, onSearch: search, options: [], maxTagCount: "responsive" } },
      { fieldName: "enabled", component: "Switch", label: tr("enabled"), componentProps: { class: "w-auto" } },
    ] });
    await formApi.resetForm();
    const { items } = await searchTrafficProducts("", item?.productNos);
    if (version !== searchVersion) return;
    for (const item of items) choices.set(item.productNo, item);
    // Preserve missing selections without displaying their internal references.
    for (const productNo of item?.productNos || []) {
      if (!choices.has(productNo)) choices.set(productNo, {
        productNo, nameZhCN: tr("unavailableProduct"), nameEnUS: tr("unavailableProduct"),
      });
    }
    await updateChoices();
    await formApi.setValues({ nameZhCN: item?.nameZhCN || "", nameEnUS: item?.nameEnUS || "", trafficGb: item?.trafficGb || 100, currencyCode: item?.currencyCode || "CNY", amount: item ? minorToMajorAmount(String(item.amountMinor), item.currencyCode) : "0.00", stockMode: item?.stockQuantity == null ? "unlimited" : "limited", stockQuantity: item?.stockQuantity ?? 0, productNos: item?.productNos || [], enabled: item?.enabled ?? false });
    if (version !== searchVersion) return;
    ready.value = true;
    modalApi.setState({ confirmDisabled: false });
    } catch {
      // The shared client reports lookup errors. Do not enable a partially
      // initialized editor or expose untranslated product references.
    } finally {
      if (version === searchVersion) modalApi.setState({ loading: false });
    }
  },
});

async function save() {
  if (!ready.value) return false;
  const { valid } = await formApi.validate();
  if (!valid) return false;
  const values = await formApi.getValues<{ nameZhCN: string; nameEnUS: string; trafficGb: number; currencyCode: string; amount: string; stockMode: string; stockQuantity: number; productNos: string[]; enabled: boolean }>();
  let amountMinor: number;
  try {
    amountMinor = Number(majorToMinorAmount(values.amount, values.currencyCode));
    if (!Number.isSafeInteger(amountMinor) || amountMinor > 1_000_000_000) throw new Error("invalid amount");
  } catch { message.error(tr("invalidAmount")); return false; }
  modalApi.lock();
  try {
    await saveTrafficPackage({ nameZhCN: values.nameZhCN.trim(), nameEnUS: values.nameEnUS.trim(), trafficGb: Number(values.trafficGb), currencyCode: values.currencyCode, amountMinor, stockQuantity: values.stockMode === "limited" ? Number(values.stockQuantity) : null, productNos: values.productNos, enabled: values.enabled }, editing.value);
    message.success(tr("saved")); emit("success"); modalApi.close();
  } finally { modalApi.unlock(); }
}
</script>

<template>
  <Modal :title="tr(editing ? 'edit' : 'create')">
    <Alert class="mb-4" :message="tr('stockHelp')" type="info" show-icon />
    <Form v-show="ready" />
    <p class="mt-3 text-sm text-muted-foreground">{{ tr('historyHelp') }}</p>
  </Modal>
</template>
