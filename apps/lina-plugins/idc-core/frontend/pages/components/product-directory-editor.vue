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
  listProductDirectoryOptions,
  createProductDirectory,
  updateProductDirectory,
} from "../product-directory-client";
const emit = defineEmits<{ success: [] }>();
const current = ref<ProductDirectory>();
const key = "plugin.idc-core.productDirectory.";
const parentMenus = ref<ProductDirectory[]>([]);
const contentPlugins = ref<Array<{label:string;value:string}>>([]);
const pluginPages = ref<Array<{label:string;value:string}>>([]);
function systemPages() {
 return [ ["/account","overview"], ["/account/products/catalog","shop"], ["/account/servers","servers"], ["/account/orders","orders"], ["/account/billing","billing"], ["/account/billing/invoices","invoices"], ["/account/billing/transactions","transactions"], ["/account/billing/renewals","renewals"], ["/account/refunds","refunds"], ["/account/profile","profile"], ["/account/addresses","addresses"], ["/account/verification","verification"], ["/account/support","support"], ["/account/support/new","newTicket"], ["/account/security","security"], ["/account/privacy","privacy"], ["/account/contacts","contacts"], ["/account/ssh-keys","sshKeys"], ["/account/referrals","referrals"], ["/account/billing/invoice-profiles","invoiceProfiles"], ["/account/tax-invoices","taxInvoices"], ["/account/billing/credit","credit"] ].map(([value,name])=>({value,label:$t(key+"pages."+name)}));
}
async function loadPluginPages() {
 const result = await requestClient.get<{list:Array<{id:string;installed:number;enabled:number;runtimeState?:string;customerSurfaces?:Array<{routePath:string;title:string;slot?:string;status?:string;bridgeCapabilities?:string[]}>}>}>("/plugins/dynamic");
 contentPlugins.value = result.list.filter(plugin=>plugin.installed===1&&plugin.enabled===1&&(!plugin.runtimeState||plugin.runtimeState==="normal")).flatMap(plugin=>{const pages=(plugin.customerSurfaces??[]).filter(page=>page.slot==="customer.products"&&page.status!=="inactive"&&page.bridgeCapabilities?.includes("purchased-services.customer.v1"));return pages.length===1?[{value:plugin.id,label:$t(pages[0]!.title)}]:[];});
 pluginPages.value = result.list.filter(plugin=>plugin.enabled===1).flatMap(plugin=>plugin.customerSurfaces ?? []).map(page=>({value:page.routePath,label:$t(page.title)}));
}

function schema(): VbenFormProps["schema"] {
  return [
    {component:"Select",fieldName:"parentDirectoryNo",label:$t(key+"parent"),defaultValue:"",componentProps:{disabled:Boolean(current.value && parentMenus.value.some(row=>row.parentDirectoryNo===current.value!.directoryNo)),options:[{value:"",label:$t(key+"topLevel")},...parentMenus.value.filter(row=>!row.parentDirectoryNo&&row.directoryNo!==current.value?.directoryNo).map(row=>({value:row.directoryNo,label:preferences.app.locale==='zh-CN'?row.nameZhCn:row.nameEnUs||row.nameZhCn}))],showSearch:true,optionFilterProp:"label",'data-testid':"directory-parent"}},
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
      rules: z.string().trim().max(128).optional(),
    },
    {
      component: "Select", fieldName: "navigationType", label: $t(key + "navigationType"), defaultValue: "product",
      componentProps: { disabled: Boolean(current.value?.productCount), options: ["product", "system", "custom", "plugin"].map(value => ({value,label:$t(key+"types."+value)})) }, rules:z.enum(["product","system","custom","plugin"]),
    },
    {
      component: "Input", fieldName: "targetUrl", label:$t(key + "targetUrl"), defaultValue:"",
      componentProps:{maxlength:2048,placeholder:$t(key+"targetHint")},
      dependencies:{show:values=>values.navigationType==="custom",triggerFields:["navigationType"]},
    },
    { component:"Select",fieldName:"systemTarget",label:$t(key+"targetUrl"), componentProps:{options:systemPages(),showSearch:true,optionFilterProp:"label"}, dependencies:{show:values=>values.navigationType==="system",triggerFields:["navigationType"]} },
    { component:"Select",fieldName:"pluginTarget",label:$t(key+"targetUrl"), componentProps:{options:pluginPages.value,showSearch:true,optionFilterProp:"label"}, dependencies:{show:values=>values.navigationType==="plugin",triggerFields:["navigationType"]} },
    {component:"Select",fieldName:"pagePluginIds",label:$t(key+"pagePlugins"),defaultValue:[],componentProps:{options:contentPlugins.value,mode:"multiple",showSearch:true,optionFilterProp:"label",allowClear:true},dependencies:{show:values=>values.navigationType==="product",triggerFields:["navigationType"]}},
    {component:"IconPicker",fieldName:"icon",label:$t(key+"icon"),defaultValue:"lucide:folder",componentProps:{prefix:"lucide",'data-testid':"directory-icon-picker"},rules:z.string().min(1).max(80)},
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
  commonConfig: { componentProps: { class: "w-full" }, labelWidth: 120 },
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
    await loadPluginPages().catch(() => { pluginPages.value = []; contentPlugins.value=[]; message.error($t(key + "pluginLookupFailed")); });
    try {
      parentMenus.value=await listProductDirectoryOptions();
      formApi.setState({ schema: schema() });
      await formApi.setValues(
        current.value
          ? {
              ...current.value,
              parentDirectoryNo:current.value.parentDirectoryNo || "",
              enabled: current.value.statusCode === "enabled",
              systemTarget:current.value.targetUrl, pluginTarget:current.value.targetUrl,
            }
          : { nameZhCn: "", nameEnUs: "", sortOrder: 100, parentDirectoryNo:"", navigationType:"product", targetUrl:"", icon:"lucide:folder" },
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
      pagePluginIds: values.navigationType === "product" ? (values.pagePluginIds ?? []) : [],
      parentDirectoryNo: values.parentDirectoryNo || "",
      navigationType: values.navigationType ?? "product",
      targetUrl: values.navigationType === "system" ? (values.systemTarget ?? "") : values.navigationType === "plugin" ? (values.pluginTarget ?? "") : values.navigationType === "product" ? "" : (values.targetUrl ?? "").trim(),
      icon: values.icon ?? "lucide:folder",
      nameZhCn: values.nameZhCn.trim(),
      nameEnUs: values.nameEnUs?.trim() || values.nameZhCn.trim(),
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
      <p v-if="current?.productCount" class="mb-4 text-muted-foreground">{{ $t(key + "typeLocked") }}</p><Form /></div
  ></Modal>
</template>
