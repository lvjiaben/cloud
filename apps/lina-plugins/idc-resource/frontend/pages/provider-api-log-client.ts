/** Body-free local Provider history; the list never contacts upstream. */
import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./provider-request";
export interface ProviderAPILogItem {
 id: number; channelNo: string; channelName: string; providerPluginId: string;
 methodCode: string; succeeded: boolean; errorCode: string; durationMs: number; startedAt: number;
}
export function providerAPILogs(params: {page:number;limit:number;channelNo?:string;channelName?:string;methodCode?:string;result?:string}) {
 return requestPreservingHttpStatus<{items:ProviderAPILogItem[];total:number}>(requestClient,pluginApiPath("idc-resource","admin/resource/api-logs"),{method:"GET",params});
}
