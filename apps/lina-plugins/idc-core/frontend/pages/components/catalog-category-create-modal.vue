<script setup lang="ts">
import type { CatalogCategory } from "../catalog-client";
import type { VbenFormProps } from "@vben/common-ui";

import { ref, watch } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { catalogCategoryCreate, catalogCategoryList } from "../catalog-client";

const emit = defineEmits<{ success: [] }>();
const categories = ref<CatalogCategory[]>([]);

interface CategoryTreeNode {
  children?: CategoryTreeNode[];
  label: string;
  value: string;
}

function localizedName(category: CatalogCategory) {
  return preferences.app.locale === "zh-CN"
    ? category.nameZhCn || category.nameEnUs
    : category.nameEnUs || category.nameZhCn;
}

function categoryTree(categories: CatalogCategory[]) {
  const nodes = new Map<string, CategoryTreeNode>();
  for (const item of categories) {
    nodes.set(item.categoryNo, {
      children: [],
      label: localizedName(item),
      value: item.categoryNo,
    });
  }
  const roots: CategoryTreeNode[] = [];
  for (const item of categories) {
    const node = nodes.get(item.categoryNo)!;
    const parent = nodes.get(item.parentCategoryNo);
    if (parent) parent.children!.push(node);
    else roots.push(node);
  }
  return roots;
}

async function refreshParentOptions() {
  const result = await catalogCategoryList("active");
  categories.value = result.categories;
}

function formSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "TreeSelect",
      componentProps: {
        allowClear: true,
        "data-testid": "catalog-category-create-parent",
        placeholder: $t("plugin.idc-core.catalog.category.parentPlaceholder"),
        showSearch: true,
        treeData: categoryTree(categories.value),
        treeDefaultExpandAll: true,
        treeNodeFilterProp: "label",
      },
      fieldName: "parentCategoryNo",
      label: $t("plugin.idc-core.catalog.category.parent"),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "catalog-category-create-name-zh",
        maxlength: 128,
      },
      fieldName: "nameZhCn",
      label: $t("plugin.idc-core.catalog.create.nameZhCn"),
      rules: z.string().trim().min(1).max(128),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "catalog-category-create-name-en",
        maxlength: 128,
      },
      fieldName: "nameEnUs",
      label: $t("plugin.idc-core.catalog.create.nameEnUs"),
      rules: z.string().trim().min(1).max(128),
    },
    {
      component: "InputNumber",
      componentProps: {
        "data-testid": "catalog-category-create-sort",
        max: 1_000_000,
        min: -1_000_000,
        precision: 0,
      },
      defaultValue: 100,
      fieldName: "sortOrder",
      label: $t("plugin.idc-core.catalog.create.sortOrder"),
      rules: z.number().int().min(-1_000_000).max(1_000_000),
    },
  ];
}

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "idc-plugin-form-surface",
});

watch(
  () => preferences.app.locale,
  () => formApi.setState({ schema: formSchema() }),
);

const [Modal, modalApi] = useVbenModal({
  async onClosed() {
    await formApi.resetForm();
  },
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const values = await formApi.getValues<{
      nameEnUs: string;
      nameZhCn: string;
      parentCategoryNo?: string;
      sortOrder: number;
    }>();
    modalApi.lock(true);
    try {
      await catalogCategoryCreate({
        nameEnUs: values.nameEnUs.trim(),
        nameZhCn: values.nameZhCn.trim(),
        parentCategoryNo: values.parentCategoryNo,
        sortOrder: values.sortOrder,
      });
      message.success($t("plugin.idc-core.catalog.create.categorySucceeded"));
      emit("success");
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    await refreshParentOptions();
    await formApi.setState({ schema: formSchema() });
    await formApi.setValues({ parentCategoryNo: undefined, sortOrder: 100 });
  },
});
</script>

<template>
  <Modal
    :confirm-text="$t('plugin.idc-core.catalog.create.createCategory')"
    :title="$t('plugin.idc-core.catalog.create.categoryTitle')"
    class="w-[620px]"
  >
    <div data-testid="catalog-category-create-modal">
      <div class="creation-note">
        {{ $t("plugin.idc-core.catalog.create.categoryNote") }}
      </div>
      <Form />
    </div>
  </Modal>
</template>

<style scoped>
.creation-note {
  background: hsl(var(--muted));
  border-left: 3px solid rgb(6 182 212);
  border-radius: 8px;
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  margin-bottom: 18px;
  padding: 11px 14px;
}
</style>
