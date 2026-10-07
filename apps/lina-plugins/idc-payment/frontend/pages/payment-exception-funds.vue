<script lang="ts">
export const pluginPageMeta = { routePath: "idc/payment-exception-funds", title: "Unassigned Payment Funds" };
</script>
<script setup lang="ts">
import { Page } from "@vben/common-ui";
import PaymentOperations from "./components/payment-operations.vue";
</script>
<template><Page :auto-content-height="true" data-testid="payment-exception-funds-page"><PaymentOperations mode="exceptions" /></Page></template>
