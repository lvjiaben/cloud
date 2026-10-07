<script lang="ts">
export const pluginPageMeta = { routePath: "idc/payment-reconciliation", title: "Payment Reconciliation" };
</script>
<script setup lang="ts">
import { Page } from "@vben/common-ui";
import PaymentOperations from "./components/payment-operations.vue";
</script>
<template><Page :auto-content-height="true" data-testid="payment-reconciliation-page"><PaymentOperations mode="reconciliation" /></Page></template>
