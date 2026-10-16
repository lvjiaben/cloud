<script setup lang="ts">
import type {
  DirectoryInput,
  ProductDirectory,
} from "../product-directory-client";
import type { VbenFormProps } from "@vben/common-ui";
import { ref, watch } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";
import { message } from "ant-design-vue";
import { useVbenForm, z } from "#/adapter/form";
import { $t } from "#/locales";
import { requestClient } from "#/api/request";
import {
  createProductDirectory,
  updateProductDirectory,
} from "../product-directory-client";
const emit = defineEmits<{ success: [] }>();
const current = ref<ProductDirectory>();
const key = "plugin.idc-core.productDirectory.";
const pluginPages = ref<Array<{label:string;value:string}>>([]);
function systemPages() {
 return [ ["/account","overview"], ["/account/products/catalog","shop"], ["/account/product-directories","services"], ["/account/servers","servers"], ["/account/cart","cart"], ["/account/orders","orders"], ["/account/billing","billing"], ["/account/billing/invoices","invoices"], ["/account/billing/transactions","transactions"], ["/account/billing/renewals","renewals"], ["/account/refunds","refunds"], ["/account/profile","profile"], ["/account/addresses","addresses"], ["/account/verification","verification"], ["/account/support","support"] ].map(([value,name])=>({value,label:$t(key+"pages."+name)}));
}
async function loadPluginPages() {
 const result = await requestClient.get<{list:Array<{enabled:number;customerSurfaces?:Array<{routePath:string;title:string}>}>}>("/plugins/dynamic");
 pluginPages.value = result.list.filter(plugin=>plugin.enabled===1).flatMap(plugin=>plugin.customerSurfaces ?? []).map(page=>({value:page.routePath,label:$t(page.title)}));
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
      component: "Select", fieldName: "navigationType", label: $t(key + "navigationType"), defaultValue: "product",
      componentProps: { options: ["product", "system", "custom", "plugin"].map(value => ({value,label:$t(key+"types."+value)})) }, rules:z.enum(["product","system","custom","plugin"]),
    },
    {
      component: "Input", fieldName: "targetUrl", label:$t(key + "targetUrl"), defaultValue:"",
      componentProps:{maxlength:2048,placeholder:$t(key+"targetHint")},
      dependencies:{show:values=>values.navigationType==="custom",triggerFields:["navigationType"]},
    },
    { component:"Select",fieldName:"systemTarget",label:$t(key+"targetUrl"), componentProps:{options:systemPages(),showSearch:true,optionFilterProp:"label"}, dependencies:{show:values=>values.navigationType==="system",triggerFields:["navigationType"]} },
    { component:"Select",fieldName:"pluginTarget",label:$t(key+"targetUrl"), componentProps:{options:pluginPages.value,showSearch:true,optionFilterProp:"label"}, dependencies:{show:values=>values.navigationType==="plugin",triggerFields:["navigationType"]} },
    { component:"Select",fieldName:"icon",label:$t(key+"icon"),defaultValue:"lucide:folder",componentProps:{options:[{value:"lucide:layout-dashboard",label:$t(key+"pages.overview")},{value:"lucide:shopping-cart",label:$t(key+"pages.cart")},...["folder","server","shopping-bag","file-text","wallet","life-buoy","globe","user","link","plug"].map(name=>({value:"lucide:"+name,label:$t(key+"icons."+name)}))]},rules:z.string().min(1).max(80) },
    {
      component: "InputNumber",
      fieldName: "sortOrder",
      label: $t(key + "sortOrder"),
      defaultValue: 100,
      componentProps: { precision: 0, min: 0, max: 1000000 },
      rules: z.number().int(),
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
    await formApi.resetForm();
  },
  async onOpenChange(open) {
    if (!open) return;
    modalApi.setState({ confirmDisabled: true });
    current.value = modalApi.getData<{
      directory?: ProductDirectory;
    }>()?.directory;
    await loadPluginPages().catch(() => { pluginPages.value = []; message.error($t(key + "pluginLookupFailed")); });
    try {
      formApi.setState({ schema: schema() });
      await formApi.setValues(
        current.value
          ? {
              ...current.value,
              enabled: current.value.statusCode === "enabled",
              systemTarget:current.value.targetUrl, pluginTarget:current.value.targetUrl,
            }
          : { nameZhCn: "", nameEnUs: "", sortOrder: 100, navigationType:"product", targetUrl:"", icon:"lucide:folder" },
      );
      modalApi.setState({ confirmDisabled: false });
    } catch {
      message.error($t(key + "lookupFailed"));
    }
  },
  async onConfirm() {
    const { valid } = await formApi.validate();
    if (!valid) return false;
    const values = await formApi.getValues<
      DirectoryInput & { enabled: boolean; systemTarget?:string; pluginTarget?:string }
    >();
    const input = {
      navigationType: values.navigationType ?? "product",
      targetUrl: values.navigationType === "system" ? (values.systemTarget ?? "") : values.navigationType === "plugin" ? (values.pluginTarget ?? "") : values.navigationType === "product" ? "" : (values.targetUrl ?? "").trim(),
      icon: values.icon ?? "lucide:folder",
      nameZhCn: values.nameZhCn.trim(),
      nameEnUs: values.nameEnUs.trim(),
      sortOrder: values.sortOrder,
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
watch(
  () => preferences.app.locale,
  () => formApi.setState({ schema: schema() }),
);
</script>
<template>
  <Modal :title="$t(key + (current ? 'edit' : 'create'))" class="w-[640px]"
    ><div data-testid="product-directory-editor">
      <Form /></div
  ></Modal>
</template>
