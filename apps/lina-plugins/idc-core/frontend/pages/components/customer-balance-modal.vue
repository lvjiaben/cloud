<script setup lang="ts">
import { computed, ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { Alert, message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { createFinancialBalanceAdjustment, createFinancialManualReceipt } from "../financial-client";
import { currencyOptions } from "../currency-client";
import { compareMinorAmounts, majorToMinorAmount, minorToMajorAmount } from "../catalog-money";

type Operation = "recharge" | "gift" | "deduct";
interface BalanceTarget {
 customerNo: string;
 displayName: string;
 operation: Operation;
 balances: { accountTypeCode: string; currencyCode: string; availableBalanceMinor: string }[];
}
const emit = defineEmits<{ success: [] }>();
const target = ref<BalanceTarget>();
const operation = computed(() => target.value?.operation || "recharge");
const tr = (key: string) => $t(`plugin.idc-core.customer.balanceAction.${key}`);
const [Form, formApi] = useVbenForm({ commonConfig: { labelWidth: 150, componentProps: { class: "w-full" } }, schema: [], showDefaultActions: false });
let requestNo = "";
const [Modal, modalApi] = useVbenModal({
 async onOpenChange(open) {
  if (!open) {target.value = undefined; return;}
  target.value = modalApi.getData<BalanceTarget>();
  requestNo = `balance-${crypto.randomUUID()}`;
  await formApi.resetForm();
  modalApi.setState({ loading: true, confirmDisabled: true });
  try {
   const currencies = (await currencyOptions()).currencies;
   const options = currencies.map(currencyCode => {
    const balance = target.value?.balances.find(item => item.currencyCode === currencyCode && item.accountTypeCode === 'customer_balance');
    return {value: currencyCode, label: operation.value === 'deduct' ? `${currencyCode} · ${tr('available')} ${minorToMajorAmount(balance?.availableBalanceMinor || '0', currencyCode)}` : currencyCode};
   });
   await formApi.setState({ schema: [
    { component: "Select", fieldName: "currencyCode", label: tr("currency"), componentProps: {options, "data-testid": "customer-balance-currency"}, rules: z.string().regex(/^[A-Z]{3}$/) },
    { component: "Input", fieldName: "amount", label: tr("amount"), componentProps: {inputmode: 'decimal', "data-testid": "customer-balance-amount"}, rules: z.string().regex(/^\d+(?:\.\d{1,4})?$/) },
    ...(operation.value === 'recharge' ? [
     { component: "Input", fieldName: "proofReference", label: tr("proof") },
     { component: "Input", fieldName: "receivingAccountReference", label: tr("receivingAccount") },
    ] : []),
    { component: "Textarea", fieldName: "evidence", label: tr("reason"), componentProps: {rows: 3, maxlength: 2000, "data-testid": "customer-balance-reason"} },
   ] });
   await formApi.setValues({currencyCode: currencies[0], amount: '', evidence: '', proofReference: '', receivingAccountReference: ''});
   modalApi.setState({confirmDisabled: options.length === 0});
  } finally {modalApi.setState({loading: false});}
 },
 async onConfirm() {
  if (!target.value || !(await formApi.validate()).valid) return false;
  const values = await formApi.getValues<{currencyCode: string; amount: string; evidence: string; proofReference: string; receivingAccountReference: string}>();
  let amountMinor: string;
  try {amountMinor = majorToMinorAmount(values.amount, values.currencyCode); if (compareMinorAmounts(amountMinor, "0") <= 0 || amountMinor.length > 16) throw new Error('amount');}
  catch {message.error(tr('invalidAmount')); return false;}
  if (operation.value === 'deduct') {
   const available = target.value.balances.find(item => item.currencyCode === values.currencyCode && item.accountTypeCode === 'customer_balance')?.availableBalanceMinor || '0';
   if (compareMinorAmounts(amountMinor, available) > 0) {message.error(tr('insufficient')); return false;}
  }
  modalApi.lock(true);
  try {
   const evidence = JSON.stringify({reason: values.evidence?.trim() || "", proofReference: values.proofReference?.trim(), receivingAccountReference: values.receivingAccountReference?.trim()});
   const hash = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(evidence));
   const common = { requestNo, customerNo: target.value.customerNo, currencyCode: values.currencyCode, amountMinor,
    evidenceDigest: [...new Uint8Array(hash)].map(item => item.toString(16).padStart(2, '0')).join('') };
   const result = operation.value === 'recharge'
    ? await createFinancialManualReceipt({...common, reasonCode: 'offline_transfer', businessPurposeCode: 'balance_recharge', proofReference: values.proofReference?.trim() || "", receivingAccountReference: values.receivingAccountReference?.trim() || ""})
    : await createFinancialBalanceAdjustment({...common, reasonCode: operation.value === 'gift' ? 'manual_gift' : 'manual_deduction', deduct: operation.value === 'deduct'});
   message.success(tr(result.statusCode === 'pending_review' ? 'pendingReview' : 'succeeded'));
   emit('success'); modalApi.close();
  } finally {modalApi.lock(false);}
 },
});
</script>
<template>
 <Modal :title="tr(operation)" class="w-[580px]">
  <div data-testid="customer-balance-modal">
   <p class="mb-4 font-medium">{{ target?.displayName }}</p>
   <Alert class="mb-4" type="info" show-icon :message="tr(`${operation}Hint`)" />
   <Form />
   <p class="text-muted-foreground mt-4 text-sm">{{ tr('reviewHint') }}</p>
  </div>
 </Modal>
</template>
