<script setup lang="ts">
import type { PropType } from "vue";

import type { DeptTreeNode } from "./post-client";

import { onMounted, ref } from "vue";

import { IconifyIcon } from "@vben/icons";

import { Empty, InputSearch, Skeleton, Tree } from "ant-design-vue";

defineOptions({ inheritAttrs: false });

const props = withDefaults(defineProps<Props>(), {
  showSearch: true,
});

const emit = defineEmits<{
  reload: [];
  select: [];
}>();

interface Props {
  api: () => Promise<DeptTreeNode[]>;
  showSearch?: boolean;
}

const selectDeptId = defineModel("selectDeptId", {
  required: true,
  type: Array as PropType<string[]>,
});

const searchValue = ref("");
const deptTree = ref<DeptTreeNode[]>([]);
const expandedDeptIds = ref<Array<number | string>>([]);
const loading = ref(true);

function collectDeptIds(nodes: DeptTreeNode[]): Array<number | string> {
  return nodes.flatMap((node) => [
    node.id,
    ...collectDeptIds(node.children ?? []),
  ]);
}

async function loadTree(resetSelection = true) {
  loading.value = true;
  try {
    const result = await props.api();
    deptTree.value = result;
    expandedDeptIds.value = collectDeptIds(result);
    if (resetSelection) {
      searchValue.value = "";
      selectDeptId.value = [];
    }
  } finally {
    loading.value = false;
  }
}

async function handleReload() {
  await loadTree();
  emit("reload");
}

async function refreshTree() {
  await loadTree(false);
}

onMounted(() => loadTree());

defineExpose({ refreshTree });
</script>

<template>
  <div :class="$attrs.class as string">
    <Skeleton
      :loading="loading"
      :paragraph="{ rows: 8 }"
      active
      class="border border-border bg-background p-2"
    >
      <div class="flex h-full flex-col overflow-y-auto bg-background">
        <div
          v-if="showSearch"
          class="sticky left-0 top-0 z-10 border-b border-border bg-background p-2"
        >
          <InputSearch
            v-model:value="searchValue"
            :placeholder="$t('pages.common.search')"
            allow-clear
            size="small"
          >
            <template #enterButton>
              <a-button @click="handleReload">
                <IconifyIcon
                  class="text-primary"
                  icon="ant-design:sync-outlined"
                />
              </a-button>
            </template>
          </InputSearch>
        </div>
        <div class="h-full overflow-x-hidden px-2 py-1">
          <Tree
            v-if="deptTree.length > 0"
            v-bind="$attrs"
            v-model:expanded-keys="expandedDeptIds"
            v-model:selected-keys="selectDeptId"
            :field-names="{ title: 'label', key: 'id' }"
            :show-line="{ showLeafIcon: false }"
            :tree-data="deptTree as any"
            :virtual="false"
            default-expand-all
            @select="emit('select')"
          >
            <template #title="{ label }">
              <span v-if="searchValue && label.includes(searchValue)">
                {{ label.substring(0, label.indexOf(searchValue)) }}
                <span class="text-primary">{{ searchValue }}</span>
                {{
                  label.substring(
                    label.indexOf(searchValue) + searchValue.length,
                  )
                }}
              </span>
              <span v-else>{{ label }}</span>
            </template>
          </Tree>
          <Empty
            v-else
            :description="$t('plugin.org-core.post.messages.noDepartments')"
            :image="Empty.PRESENTED_IMAGE_SIMPLE"
            class="mt-5"
          />
        </div>
      </div>
    </Skeleton>
  </div>
</template>
