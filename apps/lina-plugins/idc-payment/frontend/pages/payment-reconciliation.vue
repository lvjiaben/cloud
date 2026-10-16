<script lang="ts">
export const pluginPageMeta = { routePath: "idc/payment-reconciliation", title: "Payment Reconciliation" };
</script>
<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { Page } from "@vben/common-ui";
import PaymentOperations from "./components/payment-operations.vue";
</script>
<template><Page class="plugin-admin-page" :auto-content-height="true" data-testid="payment-reconciliation-page"><PaymentOperations mode="reconciliation" /></Page></template>
