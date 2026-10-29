<script setup lang="ts">
import { useRouter } from "vue-router";
import { Page } from "@vben/common-ui";
import { Button, Empty, Skeleton, Space } from "ant-design-vue";
import { $t } from "#/locales";
import NativeServerList from "./list.vue";
import { useNativeServerAccess } from "./use-native-server-access";
const router = useRouter();
const { available, ready } = useNativeServerAccess();
</script>
<template>
  <Page
    class="customer-inner-page"
    header-class="customer-inner-heading"
    content-class="customer-inner-content"
    :title="$t('accountConsole.servers')"
    data-testid="customer-server-console"
  >
    <header class="server-console-heading">
      <p>{{ $t("accountConsole.serverHint") }}</p>
      <Space wrap
        ><Button @click="router.push('/account/orders')">{{
          $t("accountConsole.orders")
        }}</Button
        ><Button
          type="primary"
          @click="router.push('/account/products/catalog')"
          >{{ $t("accountConsole.purchase") }}</Button
        ></Space
      >
    </header>
    <Skeleton v-if="!ready" active :paragraph="{ rows: 4 }" />
    <NativeServerList v-else-if="available" />
    <Empty v-else :description="$t('accountConsole.cloudUnavailable')" />
    <footer class="server-console-footer">
      <Button
        type="link"
        @click="router.push('/account/product-directories/all')"
        >{{ $t("accountConsole.allServices") }}</Button
      ><Button
        v-if="available"
        type="link"
        @click="router.push('/account/servers/legacy')"
        >{{ $t("accountConsole.otherServerServices") }}</Button
      >
    </footer>
  </Page>
</template>
<style scoped>
.server-console-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
  margin-bottom: 22px;
}
.server-console-heading p {
  margin: 0;
  color: hsl(var(--muted-foreground));
  font-size: 14px;
  line-height: 1.7;
}
.server-console-footer {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 12px;
}
@media (max-width: 700px) {
  .server-console-heading {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
