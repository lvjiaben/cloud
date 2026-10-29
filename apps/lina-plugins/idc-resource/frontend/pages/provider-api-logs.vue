<script lang="ts">
export const pluginPageMeta = {
  routePath: "idc/resource/api-logs",
};
</script>
<script setup lang="ts">
import "../../../vben-adapter/frontend/admin.css";
import { onMounted, watch } from "vue";
import { Page } from "@vben/common-ui";
import type { VbenFormProps } from "@vben/common-ui";
import { Alert, Tag, message } from "ant-design-vue";
import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { formatTimestamp } from "#/utils/time";
import { useAdminGridLayout } from "../../../vben-adapter/frontend/use-admin-grid-layout";
import { providerAPILogs, type ProviderAPILogItem } from "./provider-api-log-client";
import { createResourcePage } from "./resource-page";
const props = defineProps<{ channelNo?: string }>();
function tr(key:string) {return $t(`plugin.idc-resource.apiLogs.${key}`);}
const methodCodes=["connection.verify", "capabilities.discover", "configuration.candidates", "provision.parameters.build", "provider-capacity.list", "operation.execute", "task.query", "resource.query", "catalog.products.list", "catalog.products.detail", "catalog.configuration.evaluate", "catalog.billing-terms"];
function methodLabel(code:string){return tr(`methods.${methodCodes.includes(code)?code:"unknown"}`);}
function failureLabel(code:string){
 const labels:Record<string,string>={provider_operation_failed:"operationFailed",provider_operation_unknown:"operationUnknown",provider_invalid_result:"invalidResult"};
 return tr(labels[code]||"callFailed");
}
const directory=createResourcePage<ProviderAPILogItem>({
 onPage:page=>gridApi.setGridOptions({pagerConfig:{currentPage:page.currentPage,pageSize:page.pageSize,total:page.total}}),
 onDenied:()=>{gridApi.setGridOptions({data:[]});},
 onFailure:()=>message.error(tr("loadFailed")),
});
function schema():VbenFormProps["schema"] {return [
 {fieldName:"channelName",label:tr("channelNo"),component:"Input",componentProps:{allowClear:true,maxlength:128}},
 {fieldName:"methodCode",label:tr("method"),component:"Select",componentProps:{allowClear:true,placeholder:tr("methodHint"),options:methodCodes.map(value=>({value,label:methodLabel(value)}))}},
 {fieldName:"result",label:tr("result"),component:"Select",componentProps:{allowClear:true,options:[{label:tr("completed"),value:"succeeded"},{label:tr("failed"),value:"failed"}]}},
 ];}
function columns(){return [
 {field:"id",title:tr("id"),width:90},
 {field:"startedAt",title:tr("time"),minWidth:175,formatter:({cellValue}:{cellValue:number})=>formatTimestamp(cellValue)},
 {field:"channelName",title:tr("interface"),minWidth:160},
 {field:"providerPluginId",title:tr("connector"),minWidth:150},
 {field:"methodCode",title:tr("method"),minWidth:220,formatter:({cellValue}:{cellValue:string})=>methodLabel(cellValue)},
 {field:"succeeded",title:tr("result"),width:130,slots:{default:"result"}},
 {field:"durationMs",title:tr("duration"),width:130},
 {field:"errorCode",title:tr("reason"),minWidth:200,slots:{default:"reason"}},
 ];}
const [Grid,gridApi]=useVbenVxeGrid({
 formOptions:{schema:[],commonConfig:{labelWidth:90},layout:"inline",actionLayout:"inline",actionPosition:"left",showCollapseButton:false,wrapperClass:"plugin-admin-search"},
 gridOptions:{height:"100%",columns:[],rowConfig:{keyField:"id"},pagerConfig:{enabled:true,currentPage:1,pageSize:25,pageSizes:[10,25,50,100],total:0},toolbarConfig:{refresh:true,custom:true,zoom:true},
 proxyConfig:{autoLoad:false,ajax:{query:async({page}:{page:{currentPage:number;pageSize:number}},values:Record<string,unknown>={})=>{
 const filters={channelNo:props.channelNo,channelName:String(values.channelName||"").trim(),methodCode:String(values.methodCode||"").trim(),result:String(values.result||"")};
 return directory.query(page,JSON.stringify(filters),(current,limit)=>providerAPILogs({...filters,page:current,limit}));
 }}}},
});
function labels(){gridApi.setGridOptions({columns:columns()});gridApi.formApi.setState({schema:schema()});}
onMounted(async()=>{labels();await gridApi.query();});
watch(()=>$t("plugin.idc-resource.apiLogs.interface"),labels);
const layoutRoot=useAdminGridLayout(gridApi);
</script>
<template>
 <Page class="plugin-admin-page" auto-content-height data-testid="provider-api-logs-page">
  <div :ref="layoutRoot" class="api-log-shell">
   <Alert type="info" show-icon :message="tr('hint')" />
   <Grid class="min-h-0 w-full">
    <template #result="{row}"><Tag :color="row.succeeded?'green':'red'">{{tr(row.succeeded?'completed':'failed')}}</Tag></template>
    <template #reason="{row}">{{row.errorCode ? failureLabel(row.errorCode) : '—'}}</template>
   </Grid>
  </div>
 </Page>
</template>
<style scoped>
.api-log-shell{display:flex;flex-direction:column;gap:12px;height:100%;min-height:0;}
.api-log-shell :deep(.vxe-grid){flex:1;min-height:0;}
</style>
