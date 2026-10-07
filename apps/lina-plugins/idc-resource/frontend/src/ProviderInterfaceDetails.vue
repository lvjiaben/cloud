<script setup lang="ts">
import { ref, watch } from "vue";
import type { Channel } from "./api";
import type { PluginHostMountContext } from "./host-context";
import ProviderCatalog from "./ProviderCatalog.vue";

const props = defineProps<{ context: PluginHostMountContext; channel: Channel; summary: Record<string, string> }>();
const tab = ref("overview");
const tr = (key: string) => props.context.t(`plugin.idc-resource.providerConfiguration.${key}`, key);
watch(() => props.channel.interfaceInstanceNo, () => { tab.value = "overview"; });
</script>

<template>
  <section class="provider-configuration-workspace" data-testid="provider-interface-details">
    <nav class="tabs" role="tablist" :aria-label="tr('detailTitle')">
      <button type="button" role="tab" :class="{ active: tab === 'overview' }" :aria-selected="tab === 'overview'" @click="tab = 'overview'">{{ tr('details') }}</button>
      <button v-if="channel.catalogAvailable && channel.status === 'enabled' && channel.interfaceInstanceNo" type="button" role="tab" :class="{ active: tab === 'catalog' }" :aria-selected="tab === 'catalog'" @click="tab = 'catalog'">{{ context.t('plugin.idc-resource.providerCatalog.title') }}</button>
    </nav>
    <dl v-if="tab === 'overview'">
      <template v-for="(value, name) in summary" :key="name">
        <dt>{{ tr(name === 'displayName' ? 'channelName' : name) }}</dt>
        <dd>{{ value }}</dd>
      </template>
    </dl>
    <ProviderCatalog v-else-if="channel.catalogAvailable && channel.status === 'enabled' && channel.interfaceInstanceNo" :key="channel.interfaceInstanceNo" :context="context" :interface-no="channel.interfaceInstanceNo" />
  </section>
</template>
