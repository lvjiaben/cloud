<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { Alert, Button, Descriptions, Spin } from "ant-design-vue";
import { preferences } from "@vben/preferences";
import { pluginApiPath, requestClient } from "#/api/request";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import { businessLabel, displayName, parseServerPage, type ServerItem } from "../../../idc-zjmfcloud/frontend/src/customer/servers/model";
const props = defineProps<{ entitlementNo: string }>();
const item = ref<ServerItem>();
const loading = ref(false);
const failed = ref(false);
const locale = computed(() => preferences.app.locale === "en-US" ? "en-US" : "zh-CN");
const text = (key: string) => $t(`plugin.idc-resource.serverAccessories.${key}`);
let generation = 0;
async function load() {
  const current = ++generation;
  item.value = undefined;
  loading.value = true;
  failed.value = false;
  try {
    const response = await requestClient.get(pluginApiPath("idc-resource", "admin/resource/zjmfcloud-servers"), { params: { entitlementNo: props.entitlementNo, page: 1, limit: 1 } });
    if (current !== generation) return;
    const page = parseServerPage(response);
    if (!page || page.items.length !== 1) throw new Error("unavailable server entitlement");
    item.value = page.items[0];
  } catch {
    if (current === generation) failed.value = true;
  } finally {
    if (current === generation) loading.value = false;
  }
}
watch(() => props.entitlementNo, () => { void load(); }, { immediate: true });
onBeforeUnmount(() => ++generation);
</script>
<template>
  <section data-testid="customer-native-server-detail" class="flex flex-col gap-4">
    <Button :loading="loading" @click="load">{{ text('refresh') }}</Button>
    <Alert v-if="failed" type="error" :message="text('message.loadFailed')" />
    <Spin :spinning="loading">
      <Descriptions v-if="item" bordered :column="1">
        <Descriptions.Item :label="text('field.product')">{{ displayName(item, locale) }}</Descriptions.Item>
        <Descriptions.Item :label="text('field.serviceNo')">{{ item.serverNo }}</Descriptions.Item>
        <Descriptions.Item :label="text('field.runtime')">{{ businessLabel(item.state, locale) }}</Descriptions.Item>
        <Descriptions.Item :label="$t('plugin.idc-resource.tasks.table.channel')">{{ item.channelName }} · {{ item.poolName }}</Descriptions.Item>
        <Descriptions.Item :label="$t('plugin.idc-resource.tasks.table.updatedAt')">{{ formatTimestamp(item.updatedAt) }}</Descriptions.Item>
      </Descriptions>
    </Spin>
  </section>
</template>
