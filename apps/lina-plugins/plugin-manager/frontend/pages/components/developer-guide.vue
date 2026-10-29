<script setup lang="ts">
// Render the host's published guide messages without importing host view code.
import { ref } from "vue";
import { useVbenDrawer } from "@vben/common-ui";
import { Alert, Collapse, CollapsePanel } from "ant-design-vue";
import { $t } from "#/locales";

const [Drawer] = useVbenDrawer({ footer: false });
const expanded = ref(["start"]);
const sections = [
  "start",
  "dimensions",
  "manifest",
  "extensions",
  "source",
  "dynamic",
  "surfaces",
  "pageBridge",
  "interfaces",
  "fulfillment",
  "secrets",
  "testing",
  "package",
  "lifecycle",
];
// The project permits single-language plugins; the shared guide predates that rule.
const projectNotes: Record<string, string> = {
  "quickSteps.step5":
    "补齐声明语言、单元测试、集成测试和真实浏览器测试；未启用多语言的插件允许单语言。",
  "sections.manifest.body":
    "清单声明稳定 ID、版本、运行类型、分发方式及实际需要的依赖、业务扩展点、页面和宿主服务。多语言按 i18n.enabled 及声明语言启用；不要根据插件名字猜业务能力。",
  "rules.rule4": "启用 i18n 的插件按声明语言维护翻译；未启用时允许单语言。",
};
const t = (key: string) =>
  projectNotes[key] ?? $t(`pages.system.plugin.guide.${key}`);
</script>

<template>
  <Drawer :title="t('title')" class="plugin-developer-guide-drawer">
    <article data-testid="plugin-developer-guide" class="space-y-5">
      <Alert
        type="info"
        show-icon
        :message="t('summary')"
        :description="t('boundary')"
      />
      <section class="space-y-3">
        <h2 class="text-base font-medium">{{ t("quickTitle") }}</h2>
        <ol class="list-decimal space-y-2 pl-5 leading-6">
          <li v-for="step in 6" :key="step">
            {{ t(`quickSteps.step${step}`) }}
          </li>
        </ol>
      </section>
      <Collapse v-model:active-key="expanded">
        <CollapsePanel
          v-for="section in sections"
          :key="section"
          :header="t(`sections.${section}.title`)"
        >
          <p class="whitespace-pre-line leading-7">
            {{ t(`sections.${section}.body`) }}
          </p>
          <pre
            v-if="['manifest', 'testing', 'package'].includes(section)"
            class="mt-3 max-h-80 overflow-auto rounded bg-muted p-3 text-xs leading-5"
          ><code>{{ t(`sections.${section}.example`) }}</code></pre>
        </CollapsePanel>
      </Collapse>
      <section class="space-y-3">
        <h2 class="text-base font-medium">{{ t("rulesTitle") }}</h2>
        <ol class="list-decimal space-y-2 pl-5 leading-6">
          <li v-for="rule in 8" :key="rule">{{ t(`rules.rule${rule}`) }}</li>
        </ol>
      </section>
    </article>
  </Drawer>
</template>

<style>
.plugin-developer-guide-drawer {
  width: min(920px, 100vw);
  max-width: 100vw;
}
</style>
