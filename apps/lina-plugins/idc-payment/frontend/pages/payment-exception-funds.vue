<script lang="ts">
export const pluginPageMeta = { routePath: "idc/payment-exception-funds", title: "Unassigned Payment Funds" };
</script>
<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { Page } from "@vben/common-ui";
import PaymentOperations from "./components/payment-operations.vue";
</script>
<template><Page class="plugin-admin-page" :auto-content-height="true" data-testid="payment-exception-funds-page"><PaymentOperations mode="exceptions" /></Page></template>
