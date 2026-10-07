<script setup lang="ts">
import type { CatalogCategory } from "../catalog-client";
import type { VbenFormProps } from "@vben/common-ui";

import { ref, watch } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { catalogCategoryList, catalogCategoryUpdate } from "../catalog-client";

const emit = defineEmits<{ success: [] }>();
let currentCategory: CatalogCategory | undefined;
const parentCategories = ref<CatalogCategory[]>([]);

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

function descendantNos(categories: CatalogCategory[], categoryNo: string) {
  const excluded = new Set<string>([categoryNo]);
  let changed = true;
  while (changed) {
    changed = false;
    for (const item of categories) {
      if (
        !excluded.has(item.categoryNo) &&
        excluded.has(item.parentCategoryNo)
      ) {
        excluded.add(item.categoryNo);
        changed = true;
      }
    }
  }
  return excluded;
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

function formSchema(): VbenFormProps["schema"] {
  return [
    {
      component: "TreeSelect",
      componentProps: {
        allowClear: true,
        "data-testid": "catalog-category-edit-parent",
        showSearch: true,
        treeData: categoryTree(parentCategories.value),
        treeDefaultExpandAll: true,
        treeNodeFilterProp: "label",
      },
      fieldName: "parentCategoryNo",
      label: $t("plugin.idc-core.catalog.category.parent"),
    },
    {
      component: "Input",
      componentProps: { maxlength: 128 },
      fieldName: "nameZhCn",
      label: $t("plugin.idc-core.catalog.create.nameZhCn"),
      rules: z.string().trim().min(1).max(128),
    },
    {
      component: "Input",
      componentProps: { maxlength: 128 },
      fieldName: "nameEnUs",
      label: $t("plugin.idc-core.catalog.create.nameEnUs"),
      rules: z.string().trim().min(1).max(128),
    },
    {
      component: "InputNumber",
      componentProps: { max: 1_000_000, min: -1_000_000, precision: 0 },
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
    currentCategory = undefined;
    await formApi.resetForm();
  },
  async onConfirm() {
    if (!currentCategory) return false;
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
      await catalogCategoryUpdate(currentCategory.categoryNo, {
        nameEnUs: values.nameEnUs.trim(),
        nameZhCn: values.nameZhCn.trim(),
        parentCategoryNo: values.parentCategoryNo,
        sortOrder: values.sortOrder,
        version: currentCategory.version,
      });
      message.success($t("plugin.idc-core.catalog.category.updateSucceeded"));
      emit("success");
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    currentCategory = modalApi.getData<CatalogCategory>();
    if (!currentCategory) return;
    const result = await catalogCategoryList("active");
    const excluded = descendantNos(
      result.categories,
      currentCategory.categoryNo,
    );
    parentCategories.value = result.categories.filter(
      (item) => !excluded.has(item.categoryNo),
    );
    await formApi.setState({ schema: formSchema() });
    await formApi.setValues({
      nameEnUs: currentCategory.nameEnUs,
      nameZhCn: currentCategory.nameZhCn,
      parentCategoryNo: currentCategory.parentCategoryNo || undefined,
      sortOrder: currentCategory.sortOrder,
    });
  },
});
</script>

<template>
  <Modal
    :title="$t('plugin.idc-core.catalog.category.editTitle')"
    class="w-[620px]"
  >
    <div data-testid="catalog-category-edit-modal"><Form /></div>
  </Modal>
</template>
