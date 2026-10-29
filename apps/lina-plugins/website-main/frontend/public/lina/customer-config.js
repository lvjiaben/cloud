// Ordinary deployments may supply this public address without rebuilding pages.
export async function loadCustomerConfiguration(win=window,fetcher=fetch){
 if(win.__LINA_TEMPLATE__?.customerUrl)return;
 try{
  const response=await fetcher('/lina/runtime-config.json',{credentials:'omit',cache:'no-store',signal:AbortSignal.timeout(5000)});
  if(!response.ok)return;
  const {customerUrl}=await response.json();
  if(typeof customerUrl!=='string'||!customerUrl)return;
  const url=new URL(customerUrl);
  if(!['http:','https:'].includes(url.protocol)||url.username||url.password)return;
  win.__LINA_TEMPLATE__={...win.__LINA_TEMPLATE__,customerUrl:url.href};
 }catch{/* Missing deployment configuration leaves customer links unavailable. */}
}
