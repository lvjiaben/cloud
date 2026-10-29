import {Page} from '@playwright/test';
// Browser-only synthetic data. Every API request is fulfilled or rejected here;
// no record is created in a backend and no fixture enters public assets.
export const prefix='/x/idc-core/api/v1/public';
export async function controlledBusiness(page:Page){
 const requests:{path:string;method:string;body:any}[]=[];
 const products=[
  {publicSlug:'controlled-alpha',nameZhCN:'测试专用甲',categoryNo:'cat-a',availability:'available',currencies:['CNY','USD']},
  {publicSlug:'controlled-empty',nameZhCN:'测试专用缺货',categoryNo:'cat-b',availability:'unavailable',currencies:['CNY']},
  {publicSlug:'controlled-manual',nameZhCN:'测试专用人工确认',categoryNo:'cat-a',availability:'manual_confirmation',currencies:['CNY']}
 ];
 const state:{contentFailure:boolean;empty:boolean;brand:any}={contentFailure:false,empty:false,brand:{siteName:{zhCN:'受控测试专用网站'},navigation:[{label:'测试产品',group:true,children:[{label:'测试目录',href:'/products.html'}]}]}};
 await page.route('**/x/**',async route=>{
  const r=route.request(),url=new URL(r.url()),path=url.pathname;
  if(!['http://localhost:6680','http://127.0.0.1:6680'].includes(url.origin))return route.fallback();
  requests.push({path:path+url.search,method:r.method(),body:r.postDataJSON()});
  const ok=(data:any)=>route.fulfill({json:{code:0,data}});
  if(path==='/x/idc-portal/api/v1/public/brand')return ok({brand:state.brand});
  if(path===prefix+'/catalog/products'){
   let rows=state.empty?[]:products.filter(p=>(!url.searchParams.get('categoryNo')||p.categoryNo===url.searchParams.get('categoryNo'))&&(!url.searchParams.get('namePrefix')||p.nameZhCN.startsWith(url.searchParams.get('namePrefix')!)));
   const offset=url.searchParams.get('cursor')==='controlled-next'?2:0;
   return ok({products:rows.slice(offset,offset+2),categories:[{categoryNo:'cat-a',nameZhCN:'测试分类甲'},{categoryNo:'cat-b',nameZhCN:'测试分类乙'}],availableCurrencies:['CNY','USD'],nextCursor:rows.length>offset+2?'controlled-next':''});
  }
  if(path===prefix+'/content/items'){
   if(state.contentFailure)return route.fulfill({status:503,json:{code:50,data:null}});
   const type=url.searchParams.get('type')||'news',query=url.searchParams.get('query');
   const start=url.searchParams.get('cursor')?2:1;
   const items=state.empty||query==='无匹配'?[]:[{contentNo:'controlled-'+type+'-'+start,title:'测试专用'+type+start,summary:'非正式内容',publishedAt:1792200000000}];
   return ok({items,nextCursor:!state.empty&&!query&&start===1?'controlled-next':''});
  }
  if(path.startsWith(prefix+'/content/items/')){
   const id=path.split('/').at(-1)!;
   return ok({item:{contentNo:id,title:'测试专用详情',summary:'测试摘要',body:'<p>测试正文 <strong>可见文字</strong></p>',publishedAt:1792200000000}});
  }
  // Fail closed, including writes to payment, cart, upload and suppliers.
  return route.fulfill({status:403,json:{code:403,message:'Controlled fixture has no matching operation',data:null}});
 });
 return {requests,products,state};
}
