<script setup lang="ts">
import type {
  DirectoryInput,
  ProductDirectory,
} from "../product-directory-client";
import type { VbenFormProps } from "@vben/common-ui";
import { onBeforeUnmount, ref, watch } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { catalogProductList } from "../catalog-client";
import {
  createProductDirectory,
  updateProductDirectory,
} from "../product-directory-client";
const emit = defineEmits<{ success: [] }>();
const current = ref<ProductDirectory>();
const options = ref<Array<{ label: string; value: string }>>([]);
const key = "plugin.idc-core.productDirectory.";
let searchRevision = 0;
let searchTimer: ReturnType<typeof setTimeout> | undefined;
async function searchProducts(value: string) {
  const revision = ++searchRevision;
  const result = await catalogProductList({
    page: 1,
    limit: 50,
    namePrefix: value.trim(),
  });
  const values = await formApi.getValues<{ productNos?: string[] }>();
  if (revision !== searchRevision) return false;
  const selected = new Set(values.productNos ?? []);
  const retained = options.value.filter((item) => selected.has(item.value));
  const products = result.products.map((p) => ({
    label:
      preferences.app.locale === "zh-CN"
        ? p.nameZhCn || p.nameEnUs
        : p.nameEnUs || p.nameZhCn,
    value: p.productNo,
  }));
  options.value = [
    ...new Map(
      [...retained, ...products].map((item) => [item.value, item]),
    ).values(),
  ];
  formApi.setState({ schema: schema() });
  return true;
}
function delayedSearch(value: string) {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    void searchProducts(value).catch(() =>
      message.error($t(key + "lookupFailed")),
    );
  }, 250);
}
function schema(): VbenFormProps["schema"] {
  return [
    {
      component: "Input",
      fieldName: "nameZhCn",
      label: $t(key + "nameZhCn"),
      rules: z.string().trim().min(1).max(128),
    },
    {
      component: "Input",
      fieldName: "nameEnUs",
      label: $t(key + "nameEnUs"),
      rules: z.string().trim().min(1).max(128),
    },
    {
      component: "InputNumber",
      fieldName: "sortOrder",
      label: $t(key + "sortOrder"),
      defaultValue: 100,
      componentProps: { precision: 0, min: 0, max: 1000000 },
      rules: z.number().int(),
    },
    {
      component: "Select",
      fieldName: "productNos",
      label: $t(key + "products"),
      defaultValue: [],
      componentProps: {
        mode: "multiple",
        options: options.value,
        showSearch: true,
        filterOption: false,
        onSearch: delayedSearch,
        maxTagCount: "responsive",
        placeholder: $t(key + "productSearch"),
      },
      rules: z.array(z.string()).max(200),
    },
    ...(current.value
      ? [
          {
            component: "Switch",
            fieldName: "enabled",
            label: $t(key + "enabled"),
            defaultValue: true,
          },
        ]
      : []),
  ];
}
const [Form, formApi] = useVbenForm({
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 110 },
  schema: [],
  showDefaultActions: false,
  wrapperClass: "grid-cols-1",
});
const [Modal, modalApi] = useVbenModal({
  async onClosed() {
    searchRevision++;
    if (searchTimer) clearTimeout(searchTimer);
    await formApi.resetForm();
  },
  async onOpenChange(open) {
    if (!open) return;
    modalApi.setState({ confirmDisabled: true });
    current.value = modalApi.getData<{
      directory?: ProductDirectory;
    }>()?.directory;
    options.value = [];
    // Retain saved association keys even when the product has been retired or is outside the search page.
    options.value = (current.value?.productNos ?? []).map((value) => {
      const product = current.value?.products?.find(
        (item) => item.productNo === value,
      );
      return {
        value,
        label: product
          ? preferences.app.locale === "zh-CN"
            ? product.nameZhCn || product.nameEnUs
            : product.nameEnUs || product.nameZhCn
          : $t(key + "retainedProduct"),
      };
    });
    try {
      formApi.setState({ schema: schema() });
      await formApi.setValues(
        current.value
          ? {
              ...current.value,
              enabled: current.value.statusCode === "enabled",
            }
          : { nameZhCn: "", nameEnUs: "", sortOrder: 100, productNos: [] },
      );
      if (await searchProducts(""))
        modalApi.setState({ confirmDisabled: false });
    } catch {
      message.error($t(key + "lookupFailed"));
    }
  },
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const values = await formApi.getValues<
      DirectoryInput & { enabled: boolean }
    >();
    const input = {
      nameZhCn: values.nameZhCn.trim(),
      nameEnUs: values.nameEnUs.trim(),
      sortOrder: values.sortOrder,
      productNos: values.productNos,
    };
    modalApi.lock(true);
    try {
      if (current.value)
        await updateProductDirectory(current.value.directoryNo, {
          ...input,
          expectedVersion: current.value.version,
          statusCode: values.enabled ? "enabled" : "disabled",
        });
      else await createProductDirectory(input);
      message.success($t(key + "saved"));
      emit("success");
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
});
onBeforeUnmount(() => {
  searchRevision++;
  if (searchTimer) clearTimeout(searchTimer);
});
watch(
  () => preferences.app.locale,
  () => formApi.setState({ schema: schema() }),
);
</script>
<template>
  <Modal :title="$t(key + (current ? 'edit' : 'create'))" class="w-[640px]"
    ><div data-testid="product-directory-editor">
      <p class="mb-4 text-muted-foreground">
        {{ $t(key + "associationNote") }}
      </p>
      <Form /></div
  ></Modal>
</template>
