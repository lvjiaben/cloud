// Consume framework-neutral public template settings through the documented HTTP API.
import { updatePreferences } from '@vben/preferences';
export async function applyCustomerTemplate(){
 try {
 const response=await fetch('/x/idc-portal/api/v1/public/customer-center/configuration',{cache:'no-store',credentials:'same-origin'});
 if(!response.ok)return;
 const body=await response.json(); const view=body.data??body;
 if(!view.active)return;
 const common=view.common??{},config=view.config??{};
 const logo=common.logoUrl||(view.entry?.assetBase?view.entry.assetBase+'logo.png':'');
 updatePreferences({app:{name:common.siteName||'客户中心'},...(logo?{logo:{source:logo,sourceDark:logo}}:{}),theme:{...(typeof config.primarycolor==='string'?{colorPrimary:config.primarycolor}:{})}},{markUserThemePreference:false});
 document.documentElement.dataset.customerTemplate=view.selectedPlugin;
 if(common.siteTitle)document.title=common.siteTitle;
 }catch{/* Network failures retain the last working customer shell. */}
}
