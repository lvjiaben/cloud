<script lang="ts">
export const pluginPageMeta = {
  routePath: "system/configuration-center",
  title: "System Configuration",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { computed, ref } from "vue";

import { Page, useVbenDrawer } from "@vben/common-ui";
import { IconifyIcon } from "@vben/icons";

import { Button, TabPane, Tabs, Tag } from "ant-design-vue";

import { $t } from "#/locales";

import ConfigurationSurface from "./components/configuration-surface.vue";
import CurrencyManagement from "./currency-management.vue";
import TrafficPackages from "./components/traffic-packages.vue";
import { useAccess } from "./plugin-access";

type ConfigurationItem = {
  description: string;
  icon: string;
  key: string;
  path: string;
  title: string;
};

const tr = (key: string) => $t(`plugin.idc-core.systemConfiguration.${key}`);
const selected = ref<ConfigurationItem>();
const automationTab = ref("servers");
const access = useAccess();

const [ConfigurationDrawer, configurationDrawerApi] = useVbenDrawer({
  class: "w-[min(96vw,1320px)]",
  destroyOnClose: true,
  footer: false,
});

const groups = computed(() => [
  {
    key: "business",
    title: tr("groups.business"),
    items: [
      item(
        "customerMenus",
        "lucide:panel-left",
        "/idc/member-navigation",
      ),
      item("support", "lucide:messages-square", "/idc/support/settings"),
      item(
        "serverSafety",
        "lucide:shield-check",
        "/idc/resource/server-operation-security",
      ),
      item("ipChange", "lucide:network", "/idc/resource/ip-change-policy"),
      ...(access.hasAccessByCodes(["idc:admin:resource:accessory-price:read"]) ? [item("trafficPackages", "lucide:gauge", "/idc/resource/traffic-packages")] : []),
    ],
  },
  {
    key: "automation",
    title: tr("groups.automation"),
    items: [
      item(
        "automation",
        "lucide:workflow",
        "/idc/catalog/automation-interfaces",
      ),
      item("ai", "lucide:sparkles", "/idc/ai-copilot"),
    ],
  },
  {
    key: "compliance",
    title: tr("groups.compliance"),
    items: [
      item(
        "identity",
        "lucide:scan-face",
        "/idc/compliance/providers/aliyun-face",
      ),
      item("suppliers", "lucide:plug-zap", "/idc/compliance/suppliers"),
    ],
  },
  {
    key: "notifications",
    title: tr("groups.notifications"),
    items: [
      item(
        "notificationChannels",
        "lucide:send",
        "/idc/notifications/channels",
      ),
      item(
        "notificationTemplates",
        "lucide:layout-template",
        "/idc/notifications/templates",
      ),
    ],
  },
  {
    key: "payment",
    title: tr("groups.payment"),
    items: [
 ...(access.hasAccessByCodes(["idc:admin:currency:read"]) ? [item("currencies", "lucide:coins", "/idc/currencies")] : []),
 item("payment", "lucide:credit-card", "/idc/payment-channels"),
 ],
  },
  {
    key: "migration",
    title: tr("groups.migration"),
    items: [
      item(
        "migration",
        "lucide:database-backup",
        "/plugins/idc-zjmfmigrate/migration-checks",
      ),
    ],
  },
]);

function item(key: string, icon: string, path: string): ConfigurationItem {
  return {
    description: tr(`items.${key}.description`),
    icon,
    key,
    path,
    title: tr(`items.${key}.title`),
  };
}

function open(entry: ConfigurationItem) {
  selected.value = entry;
  automationTab.value = "servers";
  configurationDrawerApi.open();
}
</script>

<template>
  <Page class="plugin-admin-page">
    <section
      class="configuration-shell"
      data-testid="system-configuration-center"
    >
      <Tabs destroy-inactive-tab-pane>
        <TabPane v-for="group in groups" :key="group.key" :tab="group.title">
          <div class="configuration-grid">
            <article
              v-for="entry in group.items"
              :key="entry.path"
              class="configuration-item"
            >
              <span class="configuration-icon" aria-hidden="true">
                <IconifyIcon :icon="entry.icon" />
              </span>
              <div>
                <div class="configuration-heading">
                  <strong>{{ entry.title }}</strong>
                  <Tag
                    v-if="entry.path === '/idc/payment-channels'"
                    color="blue"
                  >
                    {{ tr("dynamicPlugin") }}
                  </Tag>
                </div>
                <p>{{ entry.description }}</p>
              </div>
              <Button type="primary" @click="open(entry)">{{
                tr("manage")
              }}</Button>
            </article>
          </div>
        </TabPane>
      </Tabs>
    </section>
    <ConfigurationDrawer :title="selected?.title">
      <Tabs
        v-if="selected?.key === 'automation'"
        v-model:active-key="automationTab"
        class="configuration-drawer-tabs"
        destroy-inactive-tab-pane
      >
        <TabPane key="servers" :tab="tr('automationTabs.servers')">
          <ConfigurationSurface path="/idc/resource/provider-configuration" />
        </TabPane>
        <TabPane key="products" :tab="tr('automationTabs.products')">
          <ConfigurationSurface path="/idc/catalog/automation-interfaces" />
        </TabPane>
      </Tabs>
      <CurrencyManagement v-else-if="selected?.key === 'currencies'" />
      <TrafficPackages v-else-if="selected?.key === 'trafficPackages'" />
      <ConfigurationSurface v-else-if="selected" :path="selected.path" />
    </ConfigurationDrawer>
  </Page>
</template>

<style scoped>
.configuration-shell {
  border: 1px solid hsl(var(--border));
  background: hsl(var(--background));
}

.configuration-shell :deep(.ant-tabs-nav) {
  margin: 0;
  padding: 0 16px;
}

.configuration-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  border-top: 1px solid hsl(var(--border));
}

.configuration-item {
  display: grid;
  min-height: 104px;
  grid-template-columns: 36px minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  padding: 16px;
  border-right: 1px solid hsl(var(--border));
  border-bottom: 1px solid hsl(var(--border));
}

.configuration-item:nth-child(2n) {
  border-right: 0;
}

.configuration-icon {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  background: hsl(var(--primary) / 8%);
  color: hsl(var(--primary));
  font-size: 18px;
}

.configuration-heading {
  display: flex;
  align-items: center;
  gap: 8px;
}

.configuration-item p {
  max-width: 600px;
  margin: 4px 0 0;
  color: hsl(var(--muted-foreground));
  font-size: 13px;
  line-height: 1.55;
}

.configuration-drawer-tabs {
  height: 100%;
}

.configuration-drawer-tabs :deep(.ant-tabs-content-holder),
.configuration-drawer-tabs :deep(.ant-tabs-content),
.configuration-drawer-tabs :deep(.ant-tabs-tabpane-active) {
  height: 100%;
  min-height: 0;
}

@media (max-width: 1000px) {
  .configuration-grid {
    grid-template-columns: 1fr;
  }
  .configuration-item {
    border-right: 0;
  }
}

@media (max-width: 640px) {
  .configuration-item {
    grid-template-columns: 32px minmax(0, 1fr);
  }
  .configuration-item > .ant-btn {
    grid-column: 1 / -1;
    width: 100%;
  }
}
</style>
