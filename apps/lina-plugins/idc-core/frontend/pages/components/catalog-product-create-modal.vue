<script setup lang="ts">
import type { CatalogProductCreateResult } from "../catalog-client";
import type { CatalogCategory } from "../catalog-client";
import type { VbenFormProps } from "@vben/common-ui";

import { ref, watch } from "vue";

import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";

import { Alert, Button, message } from "ant-design-vue";

import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";

import { catalogCategoryList, catalogProductCreate } from "../catalog-client";
import { useAccess } from "../plugin-access";
import { listProductDirectoryOptions, type ProductDirectory } from "../product-directory-client";

const emit = defineEmits<{ success: [created: CatalogProductCreateResult] }>();
const categories = ref<CatalogCategory[]>([]);
const memberMenus = ref<ProductDirectory[]>([]);
const { hasAccessByCodes } = useAccess();
const advancedOpen = ref(false);

function localizedCategory(category: CatalogCategory) {
  return preferences.app.locale === "zh-CN"
    ? category.nameZhCn || category.nameEnUs
    : category.nameEnUs || category.nameZhCn;
}

interface CategoryTreeNode {
  children?: CategoryTreeNode[];
  label: string;
  value: string;
}

function categoryTree(items: CatalogCategory[]) {
  const nodes = new Map<string, CategoryTreeNode>();
  for (const item of items) {
    nodes.set(item.categoryNo, {
      children: [],
      label: localizedCategory(item),
      value: item.categoryNo,
    });
  }
  const roots: CategoryTreeNode[] = [];
  for (const item of items) {
    const node = nodes.get(item.categoryNo)!;
    const parent = nodes.get(item.parentCategoryNo);
    if (parent) parent.children!.push(node);
    else roots.push(node);
  }
  return roots;
}

function formSchema(): VbenFormProps["schema"] {
  const fields: NonNullable<VbenFormProps["schema"]> = [
    ...(hasAccessByCodes(["idc:admin:product-directory:list"]) ? [{ component: "Select", fieldName: "directoryNo", label: $t("plugin.idc-core.productDirectory.productMenu"), defaultValue: "", componentProps: { "data-testid": "catalog-product-create-member-menu", allowClear: false, options: memberMenus.value.filter(item => item.statusCode === "enabled" && (!item.navigationType || item.navigationType === "product")).map(item => ({value: item.directoryNo, label: preferences.app.locale === "zh-CN" ? item.nameZhCn : item.nameEnUs})), placeholder: $t("plugin.idc-core.productDirectory.productMenu") }, rules: z.string().trim().min(1, $t("plugin.idc-core.catalog.description.required")) }] : []),
    {
      component: "TreeSelect",
      componentProps: {
        "data-testid": "catalog-product-create-category",
        treeData: categoryTree(categories.value),
        placeholder: $t("plugin.idc-core.catalog.create.categoryPlaceholder"),
        showSearch: true,
        treeDefaultExpandAll: true,
        treeNodeFilterProp: "label",
      },
      fieldName: "categoryNo",
      label: $t("plugin.idc-core.catalog.create.category"),
      rules: z.string().length(36),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "catalog-product-create-slug",
        maxlength: 128,
      },
      fieldName: "publicSlug",
      label: $t("plugin.idc-core.catalog.create.publicSlug"),
      rules: z
        .string()
        .trim()
        .min(1)
        .max(128)
        .regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "catalog-product-create-name-zh",
        maxlength: 256,
      },
      fieldName: "nameZhCn",
      label: $t("plugin.idc-core.catalog.create.nameZhCn"),
      rules: z.string().trim().min(1).max(256),
    },
    {
      component: "Input",
      componentProps: {
        "data-testid": "catalog-product-create-name-en",
        maxlength: 256,
      },
      fieldName: "nameEnUs",
      label: $t("plugin.idc-core.catalog.create.nameEnUs"),
      rules: z.string().trim().min(1).max(256),
    },
    {
      component: "Textarea",
      componentProps: {
        "data-testid": "catalog-product-create-summary-zh",
        maxlength: 1024,
        rows: 2,
      },
      fieldName: "summaryZhCn",
      label: $t("plugin.idc-core.catalog.create.summaryZhCn"),
      rules: z.string().trim().min(1, $t("plugin.idc-core.catalog.description.required")).max(1024),
    },
    {
      component: "Textarea",
      componentProps: {
        "data-testid": "catalog-product-create-summary-en",
        maxlength: 1024,
        rows: 2,
      },
      fieldName: "summaryEnUs",
      label: $t("plugin.idc-core.catalog.create.summaryEnUs"),
      rules: z.string().trim().min(1, $t("plugin.idc-core.catalog.description.required")).max(1024),
    },
    {
      component: "InputNumber",
      componentProps: {
        "data-testid": "catalog-product-create-sort",
        max: 1_000_000,
        min: -1_000_000,
        precision: 0,
      },
      defaultValue: 100,
      fieldName: "sortOrder",
      label: $t("plugin.idc-core.catalog.create.sortOrder"),
      rules: z.number().int().min(-1_000_000).max(1_000_000),
    },
    {
      component: "Switch",
      componentProps: { "data-testid": "catalog-product-create-inventory-tracking" },
      defaultValue: false,
      fieldName: "inventoryTrackingEnabled",
      label: $t("plugin.idc-core.catalog.inventory.trackingEnabled"),
    },
    {
      component: "InputNumber",
      componentProps: {
        "data-testid": "catalog-product-create-sellable-quantity",
        min: 0,
        precision: 0,
      },
      defaultValue: 0,
      dependencies: {
        show: (values) => values.inventoryTrackingEnabled === true,
        triggerFields: ["inventoryTrackingEnabled"],
      },
      fieldName: "sellableQuantity",
      label: $t("plugin.idc-core.catalog.inventory.sellableQuantity"),
      rules: z.number().int().min(0),
    },
  ];
  const order = ["nameZhCn", "nameEnUs", "categoryNo", "directoryNo", "publicSlug", "sortOrder", "summaryZhCn", "summaryEnUs", "inventoryTrackingEnabled", "sellableQuantity"];
  const requiredFields = new Set(['nameZhCn','nameEnUs','categoryNo','publicSlug','summaryZhCn','summaryEnUs','directoryNo','inventoryTrackingEnabled','sellableQuantity']);
  for(const field of fields) {
   if(requiredFields.has(field.fieldName)) continue;
   const originalShow = field.dependencies?.show;
   field.dependencies = {...field.dependencies, triggerFields: field.dependencies?.triggerFields ?? ['nameZhCn'], show: (values, actions) => advancedOpen.value && (typeof originalShow === 'function' ? originalShow(values, actions) : originalShow !== false)};
  }
  return fields.sort((a,b) => order.indexOf(a.fieldName) - order.indexOf(b.fieldName));
}

const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 118 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "idc-plugin-form-surface grid-cols-1 md:grid-cols-2 gap-x-4",
});

async function loadCategories() {
  const result = await catalogCategoryList();
  categories.value = result.categories;
  modalApi.setState({ confirmDisabled: result.categories.length === 0 });
}

watch(
  () => preferences.app.locale,
  () => formApi.setState({ schema: formSchema() }),
);

const [Modal, modalApi] = useVbenModal({
  async onClosed() {
    advancedOpen.value = false;
    categories.value = [];
    modalApi.setState({ confirmDisabled: true });
    await formApi.resetForm();
  },
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) {advancedOpen.value=true;await formApi.setState({schema:formSchema()});return false;}
    const values = await formApi.getValues<{
      directoryNo?: string;
      categoryNo: string;
      nameEnUs: string;
      nameZhCn: string;
      publicSlug: string;
      sortOrder: number;
      summaryEnUs: string;
      summaryZhCn: string;
	  inventoryTrackingEnabled: boolean;
	  sellableQuantity: number;
    }>();
    if (!categories.value.some((item) => item.categoryNo === values.categoryNo))
      return false;
    modalApi.lock(true);
    try {
      const created = await catalogProductCreate({
        ...values,
        directoryNo: values.directoryNo ?? "",
        nameEnUs: values.nameEnUs.trim(),
        nameZhCn: values.nameZhCn.trim(),
        publicSlug: values.publicSlug.trim().toLowerCase(),
        summaryEnUs: values.summaryEnUs.trim(),
        summaryZhCn: values.summaryZhCn.trim(),
      });
      message.success($t("plugin.idc-core.catalog.create.productSucceeded"));
      await modalApi.close();
      emit("success",created);
    } finally {
      modalApi.lock(false);
    }
  },
  async onOpenChange(open) {
    if (!open) return;
    modalApi.setState({ confirmDisabled: true });
    if (hasAccessByCodes(["idc:admin:product-directory:list"])) { memberMenus.value = await listProductDirectoryOptions(); }
    await loadCategories();
    await formApi.setState({ schema: formSchema() });
    await formApi.setValues({
      categoryNo: categories.value.some(item => item.categoryNo === modalApi.getData<{ categoryNo?: string }>()?.categoryNo) ? modalApi.getData<{ categoryNo?: string }>().categoryNo : undefined,
      directoryNo: "",
	  inventoryTrackingEnabled: false,
	  sellableQuantity: 0,
      publicSlug: `product-${globalThis.crypto.randomUUID().slice(0,8)}`,
      sortOrder: 100,
      summaryEnUs: "",
      summaryZhCn: "",
    });
  },
});
async function toggleAdvanced() {
 const values = await formApi.getValues(); advancedOpen.value = !advancedOpen.value;
 await formApi.setState({schema:formSchema()}); await formApi.setValues(values);
}
</script>

<template>
  <Modal
    :confirm-text="$t('plugin.idc-core.catalog.create.createProduct')"
    :title="$t('plugin.idc-core.catalog.create.productTitle')"
    class="w-[800px] max-w-[96vw]"
  >
    <div data-testid="catalog-product-create-modal">
      <Alert class="mb-5" type="info" show-icon :message="$t('plugin.idc-core.catalog.screen.createHint')" />
      <Form />
      <Button class="mt-2" type="link" data-testid="catalog-create-advanced" @click="toggleAdvanced">{{ $t(advancedOpen ? 'plugin.idc-core.catalog.screen.hideSettings' : 'plugin.idc-core.catalog.screen.moreSettings') }}</Button>
      <div v-if="categories.length === 0" class="empty-category-note">
        {{ $t("plugin.idc-core.catalog.create.noCategories") }}
      </div>
    </div>
  </Modal>
</template>

<style scoped>
.creation-note,
.empty-category-note {
  border-radius: 8px;
  color: hsl(var(--muted-foreground));
  font-size: 12px;
  margin-bottom: 16px;
  padding: 10px 14px;
}

.creation-note {
  background: hsl(var(--muted));
  border-left: 3px solid rgb(6 182 212);
}

.empty-category-note {
  background: rgb(245 158 11 / 10%);
  border: 1px solid rgb(245 158 11 / 30%);
  margin: 8px 0 0 118px;
}
</style>
