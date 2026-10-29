<script setup lang="ts">
// Invoice payment reuses the server-owned payment context and verified channels.
import type { FinancialInvoice } from "#/api/idc/customer-finance";
import type { CustomerPaymentChannel } from "#/api/idc/customer-payments";
import { computed, nextTick, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Alert, Button, Select } from "ant-design-vue";
import QRCode from "qrcode";
import VbenModalCompat from "#/components/vben-modal-compat.vue";
import { getFinancialInvoice } from "#/api/idc/customer-finance";
import { preparePayment } from "#/api/idc/customer-commerce";
import {
  createCustomerOrderPayment,
  listCustomerPaymentChannels,
} from "#/api/idc/customer-payments";
import { $t } from "#/locales";
const props = defineProps<{ invoice: FinancialInvoice }>();
const emit = defineEmits<{ invoiceUpdated: [FinancialInvoice] }>();
const route = useRoute(),
  router = useRouter();
const alreadyPaid = ref(false);
const paidInvoice = computed(
  () =>
    props.invoice.statusCode === "paid" &&
    BigInt(props.invoice.outstandingAmountMinor) === 0n,
);
const open = ref(false),
  busy = ref(false),
  failed = ref(false),
  contextNo = ref(""),
  identityRequired = ref(false),
  notPayable = ref(false),
  qr = ref(""),
  attemptNo = ref("");
const pluginAvailable = ref(false),
  channelsLoaded = ref(false);
const channels = ref<CustomerPaymentChannel[]>([]),
  channelNo = ref(""),
  modeCode = ref("");
const available = computed(
  () =>
    Boolean(props.invoice.orderNo) &&
    ["pending_payment", "overdue"].includes(props.invoice.statusCode) &&
    BigInt(props.invoice.outstandingAmountMinor) > 0n,
);
const selected = computed(() =>
  channels.value.find((item) => item.channelNo === channelNo.value),
);
const modeOptions = computed(() =>
  (selected.value?.modes ?? [])
    .filter((mode) => mode !== "wechat_jsapi")
    .map((value) => ({ value, label: $t(`finance.recharge.modes.${value}`) })),
);
watch(channelNo, () => {
  modeCode.value = modeOptions.value[0]?.value || "";
});
async function begin() {
  if (!available.value || busy.value) return;
  open.value = true;
  busy.value = true;
  failed.value = false;
  try {
    const { preparation } = await preparePayment(props.invoice.orderNo!);
    identityRequired.value = preparation.statusCode === "awaiting_identity";
    notPayable.value = !preparation.ready && !identityRequired.value;
    contextNo.value = preparation.contextNo;
    if (!preparation.ready) {
      const current = (await getFinancialInvoice(props.invoice.invoiceNo))
        .detail.invoice;
      emit("invoiceUpdated", current);
      alreadyPaid.value =
        current.statusCode === "paid" &&
        BigInt(current.outstandingAmountMinor) === 0n;
      return;
    }
    if (!channelsLoaded.value) {
      channels.value = (
        await listCustomerPaymentChannels(props.invoice.currencyCode, true)
      ).channels.filter((item) =>
        item.currencies.includes(props.invoice.currencyCode),
      );
      channelsLoaded.value = true;
    }
    if (!channels.value.some((item) => item.channelNo === channelNo.value))
      channelNo.value = channels.value[0]?.channelNo || "";
    if (!modeOptions.value.some((item) => item.value === modeCode.value))
      modeCode.value = modeOptions.value[0]?.value || "";
  } catch {
    failed.value = true;
  } finally {
    busy.value = false;
  }
}
function identity() {
  const url = new URL("/account/identity", location.origin);
  url.searchParams.set("payment_context", contextNo.value);
  url.searchParams.set("return_to", route.fullPath);
  void router.push(url.pathname + url.search);
}
async function pay() {
  if (
    busy.value ||
    identityRequired.value ||
    notPayable.value ||
    !contextNo.value ||
    !channelNo.value ||
    !modeCode.value
  )
    return;
  busy.value = true;
  failed.value = false;
  try {
    const payment = await createCustomerOrderPayment({
      paymentContextNo: contextNo.value,
      channelNo: channelNo.value,
      modeCode: modeCode.value,
      balanceAmountMinor: "0",
    });
    attemptNo.value = payment.attemptNo;
    if (payment.redirectMethod === "QR") {
      qr.value = await QRCode.toDataURL(payment.redirectUrl, {
        width: 280,
        margin: 2,
      });
      return;
    }
    const url = new URL(payment.redirectUrl);
    if (
      !["http:", "https:"].includes(url.protocol) ||
      url.username ||
      url.password
    )
      throw new Error("invalid payment URL");
    if (payment.redirectMethod === "GET") {
      window.location.assign(url.toString());
      return;
    }
    if (payment.redirectMethod === "POST") {
      const form = document.createElement("form");
      form.action = url.toString();
      form.method = "POST";
      form.hidden = true;
      for (const [name, value] of Object.entries(payment.redirectFields)) {
        const input = document.createElement("input");
        input.type = "hidden";
        input.name = name;
        input.value = value;
        form.append(input);
      }
      document.body.append(form);
      form.submit();
      return;
    }
    throw new Error("unsupported payment mode");
  } catch {
    failed.value = true;
  } finally {
    busy.value = false;
  }
}
onMounted(async () => {
  if (!available.value) return;
  try {
    channels.value = (
      await listCustomerPaymentChannels(props.invoice.currencyCode, true)
    ).channels.filter((item) =>
      item.currencies.includes(props.invoice.currencyCode),
    );
    channelsLoaded.value = true;
    pluginAvailable.value = true;
  } catch (error) {
    const status = (error as { response?: { status?: number } }).response
      ?.status;
    pluginAvailable.value = ![403, 404, 501].includes(status ?? 0);
  }
  if (pluginAvailable.value && route.query.pay === "1") {
    await nextTick();
    await begin();
  }
});
</script>
<template>
  <Alert
    v-if="paidInvoice"
    :message="$t('finance.onlinePayment.alreadyPaid')"
    type="info"
    show-icon
    class="online-payment"
  />
  <Button
    v-if="available && pluginAvailable"
    class="online-payment"
    type="primary"
    data-testid="invoice-online-pay"
    @click="begin"
    >{{ $t("finance.onlinePayment.action") }}</Button
  >
  <VbenModalCompat
    v-model:open="open"
    :title="$t('finance.onlinePayment.title')"
    :footer="null"
  >
    <div class="online-payment-form">
      <Alert
        v-if="failed"
        :message="$t('finance.onlinePayment.failed')"
        type="error"
        show-icon
        ><template #action
          ><Button @click="begin">{{
            $t("finance.actions.retry")
          }}</Button></template
        ></Alert
      >
      <Alert
        v-if="alreadyPaid"
        :message="$t('finance.onlinePayment.alreadyPaid')"
        type="info"
        show-icon
      />
      <template v-else-if="identityRequired"
        ><Alert
          :message="$t('finance.onlinePayment.identity')"
          show-icon
          type="warning"
        /><Button type="primary" @click="identity">{{
          $t("commerce.actions.verify")
        }}</Button></template
      >
      <Alert
        v-else-if="notPayable"
        :message="$t('finance.onlinePayment.notPayable')"
        type="warning"
        show-icon
      />
      <template v-else-if="qr"
        ><img
          :src="qr"
          :alt="$t('finance.recharge.scanTitle')"
          data-testid="invoice-payment-qr"
        />
        <p>{{ $t("finance.recharge.scanDescription") }}</p>
        <Button
          type="primary"
          @click="
            router.push('/payment/return/' + encodeURIComponent(attemptNo))
          "
          >{{ $t("finance.recharge.checkStatus") }}</Button
        ></template
      >
      <template v-else>
        <Alert
          v-if="!busy && !channels.length"
          :message="$t('finance.onlinePayment.noChannels')"
          type="info"
          show-icon
        />
        <label
          >{{ $t("finance.recharge.channel")
          }}<Select
            v-model:value="channelNo"
            :disabled="busy"
            :options="
              channels.map((item) => ({
                label: item.displayName,
                value: item.channelNo,
              }))
            "
            data-testid="invoice-payment-channel"
        /></label>
        <label
          >{{ $t("finance.recharge.mode")
          }}<Select
            v-model:value="modeCode"
            :disabled="busy"
            :options="modeOptions"
            data-testid="invoice-payment-mode"
        /></label>
        <Button
          :disabled="!modeCode || !channelNo || !contextNo"
          :loading="busy"
          type="primary"
          data-testid="invoice-payment-submit"
          @click="pay"
          >{{ $t("finance.onlinePayment.continue") }}</Button
        >
      </template>
    </div>
  </VbenModalCompat>
</template>
<style scoped>
.online-payment {
  margin: 16px 12px 0 0;
}
.online-payment-form {
  display: grid;
  gap: 16px;
}
.online-payment-form label {
  display: grid;
  gap: 8px;
}
.online-payment-form img {
  max-width: 100%;
  margin: auto;
}
</style>
