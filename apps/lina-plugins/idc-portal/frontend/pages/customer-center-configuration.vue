<script lang="ts">
export const pluginPageMeta = {
  routePath: "system/customer-center-configuration",
  title: "Customer Center Configuration",
};
</script>

<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import CustomerDeveloperGuide from "./customer-developer-guide.vue";

import { Page } from "@vben/common-ui";
import { Alert, Button, Empty, Spin, Tag, message } from "ant-design-vue";
import { computed, onMounted, ref } from "vue";
import { useVbenForm } from "#/adapter/form";
import { $t } from "#/locales";
import { usePortalMountContext } from "./source-context";
import {
  getCustomerCenterConfiguration,
  saveCustomerCenterConfiguration,
  type CustomerCenterCommon,
  type CustomerCenterPlugin,
} from "./customer-center-configuration-client";

const tr = (key: string) => $t(`plugin.idc-portal.customerCenter.${key}`);
const context = usePortalMountContext();

const plugins = ref<CustomerCenterPlugin[]>([]);
const selectedPlugin = ref("");
const activePlugin = ref("");
const loading = ref(false);
const saving = ref(false);
const switching = ref(false);
const ready = ref(false);
const failed = ref(false);
const configCache = new Map<string, Record<string, unknown>>();
const current = computed(() =>
  plugins.value.find((p) => p.pluginId === selectedPlugin.value),
);
const commonKeys = [
  "siteName",
  "siteTitle",
  "companyName",
  "contactEmail",
  "contactPhone",
  "logoUrl",
  "description",
] as const;
const [CommonForm, commonApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1 md:grid-cols-2",
});
const [PluginForm, pluginApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1 md:grid-cols-2",
});
function commonSchema() {
  return commonKeys.map((key) => ({
    fieldName: key,
    label: tr(key),
    component: key === "description" ? "Textarea" : "Input",
    componentProps: {
      "aria-label": tr(key),
      maxlength: key === "description" ? 1000 : 300,
    },
    ...(key === "siteName" ? { rules: "required" } : {}),
  }));
}
async function choose(pluginId: string) {
  if (switching.value) return;
  switching.value = true;
  try {
    if (selectedPlugin.value)
      configCache.set(selectedPlugin.value, await pluginApi.getValues());
    selectedPlugin.value = pluginId;
    const fields = current.value?.schema.fields ?? [];
    pluginApi.setState({
      schema: fields.map((field) => ({
        fieldName: field.key,
        label: context.value.locale.startsWith("zh")
          ? field.label.zhCN
          : field.label.enUS,
        component:
          field.type === "boolean"
            ? "Switch"
            : field.type === "integer"
              ? "InputNumber"
              : field.type === "multi_line"
                ? "Textarea"
                : "Input",
        componentProps: {
          placeholder: context.value.locale.startsWith("zh")
            ? field.description?.zhCN
            : field.description?.enUS,
          "aria-label": context.value.locale.startsWith("zh")
            ? field.label.zhCN
            : field.label.enUS,
          ...(field.maximumLength ? { maxlength: field.maximumLength } : {}),
        },
        ...(field.required ? { rules: "required" } : {}),
      })),
    });
    await pluginApi.resetForm();
    await pluginApi.setValues(configCache.get(pluginId) ?? {});
  } finally {
    switching.value = false;
  }
}
async function load() {
  if (loading.value || saving.value) return;
  loading.value = true;
  ready.value = false;
  failed.value = false;
  try {
    const data = await getCustomerCenterConfiguration();
    plugins.value = data.plugins ?? [];

    activePlugin.value = data.selectedPlugin;
    configCache.clear();
    for (const [pluginId, values] of Object.entries(data.configurations ?? {}))
      configCache.set(pluginId, values);
    if (data.selectedPlugin)
      configCache.set(data.selectedPlugin, data.config ?? {});
    selectedPlugin.value = "";
    await commonApi.setValues(data.common ?? {});
    await choose(data.selectedPlugin);
    ready.value = true;
  } catch {
    failed.value = true;
  } finally {
    loading.value = false;
  }
}
async function save() {
  if (
    !ready.value ||
    !current.value ||
    saving.value ||
    loading.value ||
    switching.value
  )
    return;
  const [commonValidation, pluginValidation] = await Promise.all([
    commonApi.validate(),
    pluginApi.validate(),
  ]);
  if (!commonValidation.valid || !pluginValidation.valid) return;
  saving.value = true;
  try {
    const values = await commonApi.getValues<CustomerCenterCommon>();
    const common = Object.fromEntries(
      commonKeys.map((key) => [key, values[key] ?? ""]),
    ) as CustomerCenterCommon;
    await saveCustomerCenterConfiguration({

      selectedPlugin: selectedPlugin.value,
      common,
      config: await pluginApi.getValues(),
    });
    activePlugin.value = selectedPlugin.value;
    message.success(tr("saved"));
  } catch {
    /* The request client shows the API error. Keep unsaved values for retry. */
  } finally {
    saving.value = false;
  }
}
onMounted(async () => {
  commonApi.setState({ schema: commonSchema() });
  await load();
});
</script>

<template>
  <Page class="plugin-admin-page">
    <div class="website-configuration" data-plugin-frontend-root="idc-portal">
      <header class="website-heading">
        <div>
          <h1>{{ tr("title") }}</h1>
          <p>{{ tr("intro") }}</p>
        </div>
        <div class="flex gap-2">
          <CustomerDeveloperGuide />
          <Button :disabled="loading || saving || switching" @click="load">{{
            tr("reload")
          }}</Button>
          <Button
            type="primary"
            :loading="saving"
            :disabled="!ready || !current || loading || switching"
            @click="save"
            >{{ tr("save") }}</Button
          >
        </div>
      </header>
      <Alert
        v-if="failed"
        class="mb-5 website-error"
        type="error"
        show-icon
        :message="tr('loadError')"
      />
      <Spin :spinning="loading">
        <section class="website-section">
          <h2>{{ tr("plugin") }}</h2>
          <p>{{ tr("pluginHint") }}</p>
          <Empty v-if="ready && !plugins.length" :description="tr('empty')">
            <p>{{ tr('emptyHint') }}</p>
            <RouterLink to="/system/plugin">
              <Button type="primary">{{ tr('managePlugins') }}</Button>
            </RouterLink>
          </Empty>
          <div v-else class="website-plugins">
            <button
              v-for="plugin in plugins"
              :key="plugin.pluginId"
              type="button"
              :disabled="saving || !ready || switching"
              :class="[
                'website-plugin',
                { selected: selectedPlugin === plugin.pluginId },
              ]"
              :aria-pressed="selectedPlugin === plugin.pluginId"
              @click="choose(plugin.pluginId)"
            >
              <span class="website-plugin-marker" aria-hidden="true">{{
                selectedPlugin === plugin.pluginId ? "✓" : ""
              }}</span>
              <span>{{ plugin.name }}</span>
              <Tag v-if="activePlugin === plugin.pluginId" color="green">{{
                tr("active")
              }}</Tag>
            </button>
          </div>
        </section>
<div v-show="ready" class="website-forms">
          <section class="website-section">
            <h2>{{ tr("common") }}</h2>
            <p>{{ tr("commonHint") }}</p>
            <CommonForm :disabled="saving" />
          </section>
          <section v-show="current" class="website-section">
            <h2>{{ tr("content") }} · {{ current?.name }}</h2>
            <p>{{ tr("contentHint") }}</p>
            <PluginForm :disabled="saving" />
            <p v-if="!current?.schema.fields.length">{{ tr("noFields") }}</p>
          </section>
        </div>
      </Spin>
    </div>
  </Page>
</template>

<style scoped>
.website-configuration {
  width: 100%;
  max-width: 1120px;
  margin: 0 auto;
  padding: 0;
}
.website-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.website-heading h1 {
  font-size: 18px;
  font-weight: 650;
  margin: 0 0 7px;
}
.website-heading p,
.website-section > p {
  color: hsl(var(--muted-foreground));
  margin-bottom: 12px;
}
.website-heading p {
  margin: 0;
}
.website-section {
  padding: 20px;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  margin-bottom: 12px;
}
.website-section h2 {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 7px;
}
.website-plugins {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.website-plugin {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  padding: 10px 14px;
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  background: hsl(var(--background));
  text-align: left;
}
.website-plugin.selected {
  border-color: hsl(var(--primary));
  background: hsl(var(--primary) / 0.05);
}
.website-plugin-marker {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border: 1px solid hsl(var(--border));
  border-radius: 50%;
}
.selected .website-plugin-marker {
  color: white;
  background: hsl(var(--primary));
  border-color: hsl(var(--primary));
}
@media (max-width: 640px) {
  .website-heading {
    align-items: flex-start;
    flex-direction: column;
  }
  .website-section {
    padding: 20px 16px;
  }
  .website-plugin {
    width: 100%;
  }
}
</style>
