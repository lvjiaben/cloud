<script setup lang="ts">
import { ref } from 'vue';

import { useVbenDrawer } from '@vben/common-ui';

import {
  Alert,
  Card,
  Collapse,
  CollapsePanel,
  TypographyParagraph,
  TypographyText,
} from 'ant-design-vue';

import { $t } from '#/locales';

import {
  pluginDeveloperGuideExampleSections,
  pluginDeveloperGuideQuickStepCount,
  pluginDeveloperGuideRuleCount,
  pluginDeveloperGuideSectionKeys,
} from './plugin-developer-guide';

const activeKeys = ref<string[]>(['start']);

const sections = pluginDeveloperGuideSectionKeys.map((key) => ({
  key,
  showExample: pluginDeveloperGuideExampleSections.has(key),
}));

const [Drawer] = useVbenDrawer({
  footer: false,
});
</script>

<template>
  <Drawer
    :title="$t('pages.system.plugin.guide.title')"
    class="w-[min(920px,92vw)]"
  >
    <div class="flex flex-col gap-4" data-testid="plugin-developer-guide">
      <Alert
        show-icon
        type="info"
        :message="$t('pages.system.plugin.guide.summary')"
        :description="$t('pages.system.plugin.guide.boundary')"
      />

      <Card size="small" :title="$t('pages.system.plugin.guide.quickTitle')">
        <ol class="m-0 space-y-2 pl-5 leading-6">
          <li v-for="index in pluginDeveloperGuideQuickStepCount" :key="index">
            {{ $t(`pages.system.plugin.guide.quickSteps.step${index}`) }}
          </li>
        </ol>
      </Card>

      <Collapse v-model:active-key="activeKeys">
        <CollapsePanel
          v-for="section in sections"
          :key="section.key"
          :header="
            $t(`pages.system.plugin.guide.sections.${section.key}.title`)
          "
        >
          <TypographyParagraph class="leading-6 whitespace-pre-line">
            {{ $t(`pages.system.plugin.guide.sections.${section.key}.body`) }}
          </TypographyParagraph>
          <pre
            v-if="section.showExample"
            class="max-h-80 overflow-auto rounded-md bg-(--ant-color-fill-tertiary) p-3 text-xs/5"
          ><code>{{ $t(`pages.system.plugin.guide.sections.${section.key}.example`) }}</code></pre>
        </CollapsePanel>
      </Collapse>

      <Card size="small" :title="$t('pages.system.plugin.guide.rulesTitle')">
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <TypographyText
            v-for="index in pluginDeveloperGuideRuleCount"
            :key="index"
            class="leading-6"
          >
            {{ index }}.
            {{ $t(`pages.system.plugin.guide.rules.rule${index}`) }}
          </TypographyText>
        </div>
      </Card>
    </div>
  </Drawer>
</template>
