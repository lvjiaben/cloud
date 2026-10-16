<script lang="ts">
import { pluginSlotKeys } from "#/plugins/plugin-slots";

export const pluginSlotMeta = {
  order: 10,
  slotKey: pluginSlotKeys.layoutHeaderActionsBefore,
};
</script>

<script setup lang="ts">
import { computed, watch } from "vue";
import { useRoute } from "vue-router";
import { useVbenDrawer } from "@vben/common-ui";
import { IconifyIcon } from "@vben/icons";
import { Button, Tooltip } from "ant-design-vue";
import { $t } from "#/locales";

const route = useRoute();
const topic = computed(
  () => route.path.match(/\/system\/(user|role|dept|post|menu)\/?$/)?.[1],
);
const help = computed(() => {
  switch (topic.value) {
    case "user":
      return {
        title: $t("plugin.org-core.permissionHelp.user.title"),
        summary: $t("plugin.org-core.permissionHelp.user.summary"),
        note: $t("plugin.org-core.permissionHelp.user.note"),
        steps: [
          $t("plugin.org-core.permissionHelp.user.step1"),
          $t("plugin.org-core.permissionHelp.user.step2"),
          $t("plugin.org-core.permissionHelp.user.step3"),
          $t("plugin.org-core.permissionHelp.user.step4"),
        ],
      };
    case "role":
      return {
        title: $t("plugin.org-core.permissionHelp.role.title"),
        summary: $t("plugin.org-core.permissionHelp.role.summary"),
        note: $t("plugin.org-core.permissionHelp.role.note"),
        steps: [
          $t("plugin.org-core.permissionHelp.role.step1"),
          $t("plugin.org-core.permissionHelp.role.step2"),
          $t("plugin.org-core.permissionHelp.role.step3"),
          $t("plugin.org-core.permissionHelp.role.step4"),
        ],
      };
    case "dept":
      return {
        title: $t("plugin.org-core.permissionHelp.dept.title"),
        summary: $t("plugin.org-core.permissionHelp.dept.summary"),
        note: $t("plugin.org-core.permissionHelp.dept.note"),
        steps: [
          $t("plugin.org-core.permissionHelp.dept.step1"),
          $t("plugin.org-core.permissionHelp.dept.step2"),
          $t("plugin.org-core.permissionHelp.dept.step3"),
          $t("plugin.org-core.permissionHelp.dept.step4"),
        ],
      };
    case "post":
      return {
        title: $t("plugin.org-core.permissionHelp.post.title"),
        summary: $t("plugin.org-core.permissionHelp.post.summary"),
        note: $t("plugin.org-core.permissionHelp.post.note"),
        steps: [
          $t("plugin.org-core.permissionHelp.post.step1"),
          $t("plugin.org-core.permissionHelp.post.step2"),
          $t("plugin.org-core.permissionHelp.post.step3"),
          $t("plugin.org-core.permissionHelp.post.step4"),
        ],
      };
    case "menu":
      return {
        title: $t("plugin.org-core.permissionHelp.menu.title"),
        summary: $t("plugin.org-core.permissionHelp.menu.summary"),
        note: $t("plugin.org-core.permissionHelp.menu.note"),
        steps: [
          $t("plugin.org-core.permissionHelp.menu.step1"),
          $t("plugin.org-core.permissionHelp.menu.step2"),
          $t("plugin.org-core.permissionHelp.menu.step3"),
          $t("plugin.org-core.permissionHelp.menu.step4"),
        ],
      };
    default:
      return undefined;
  }
});
const [HelpDrawer, drawerApi] = useVbenDrawer({
  footer: false,
});
watch(
  () => route.path,
  () => drawerApi.close(),
);
</script>

<template>
  <template v-if="help">
    <Tooltip
      :title="$t('plugin.org-core.permissionHelp.button')"
      placement="left"
    >
      <Button
        data-testid="permission-help-button"
        :aria-label="$t('plugin.org-core.permissionHelp.button')"
        class="permission-help-trigger"
        shape="circle"
        @click="drawerApi.open()"
      >
        <template #icon
          ><IconifyIcon icon="ant-design:question-outlined" class="text-[24px]"
        /></template>
      </Button>
    </Tooltip>
    <HelpDrawer :title="help.title" class="w-full sm:w-[520px]">
      <div
        data-testid="permission-help-content"
        class="space-y-6 p-2 text-sm leading-7"
      >
        <p class="text-muted-foreground">{{ help.summary }}</p>
        <section>
          <h3 class="mb-3 font-semibold">
            {{ $t("plugin.org-core.permissionHelp.stepsTitle") }}
          </h3>
          <ol class="list-decimal space-y-4 pl-5">
            <li v-for="step in help.steps" :key="step" class="pl-1">
              {{ step }}
            </li>
          </ol>
        </section>
        <aside class="rounded-lg border border-border bg-muted/40 p-4">
          <h3 class="mb-2 font-semibold">
            {{ $t("plugin.org-core.permissionHelp.reminder") }}
          </h3>
          <p>{{ help.note }}</p>
        </aside>
      </div>
    </HelpDrawer>
  </template>
</template>

<style scoped>
.permission-help-trigger {
  position: fixed;
  right: 24px;
  bottom: 96px;
  z-index: 40;
  width: 48px;
  height: 48px;
  color: hsl(var(--foreground));
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 50% !important;
  box-shadow:
    0 4px 12px hsl(var(--foreground) / 12%),
    0 12px 28px hsl(var(--foreground) / 8%);
  transition:
    transform 180ms ease,
    box-shadow 180ms ease;
}
.permission-help-trigger:hover {
  color: hsl(var(--primary));
  background: hsl(var(--card));
  border-color: hsl(var(--primary) / 50%);
  box-shadow:
    0 6px 16px hsl(var(--foreground) / 16%),
    0 16px 32px hsl(var(--foreground) / 10%);
  transform: translateY(-2px);
}
.permission-help-trigger:focus-visible {
  outline: 2px solid hsl(var(--primary));
  outline-offset: 3px;
}
@media (prefers-reduced-motion: reduce) {
  .permission-help-trigger {
    transition: none;
  }
  .permission-help-trigger:hover {
    transform: none;
  }
}
</style>
