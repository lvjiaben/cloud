import {test,expect} from '@playwright/test';
import {WebsitePage} from '../pages/WebsitePage';

test.describe('TC-14 真实内容发布与撤回',()=>{
 test('TC-14a: 后台发布、修改和下线在官网刷新后可见，原站内容不作回退',async({page})=>{
  test.skip(process.env.WEBSITE_LOCAL_CHECKOUT!=='1','需要本地隔离环境显式授权');
  test.setTimeout(60000);
  const backend=process.env.WEBSITE_API_ORIGIN!;
  expect(['localhost','127.0.0.1']).toContain(new URL(backend).hostname);
  expect(['16630','16634'],'只允许既有隔离后台，禁止当前6030业务库').toContain(new URL(backend).port);
  const login=await (await page.request.post(backend+'/api/v1/auth/login',{data:{username:process.env.WEBSITE_QA_ADMIN_USER,password:process.env.WEBSITE_QA_ADMIN_PASSWORD,clientType:'web'}})).json();
  expect(login.code).toBe(0);
  const headers={Authorization:'Bearer '+login.data.accessToken};
  const root=backend+'/x/idc-core/api/v1/admin/content/items';
  async function call(url:string,method:string,data?:unknown){const response=await page.request.fetch(url,{method,headers,data});const body=await response.json();expect(body.code,body.message).toBe(0);return body.data;}
  const site=new WebsitePage(page);
  for(const [type,path]of [['news','news'],['announcement','announcements'],['knowledge','help']]){
   const title='隔离内容联调（测试专用）'+type+'-'+Date.now();
   const fields={type,visibility:'public',title,summary:'仅验证本地真实发布，不作为正式网站内容。',body:'<p>真实后台发布正文，100% 与下划线 _ 搜索验证。</p>',targetCustomerNos:[],targetServiceNos:[],targetCustomerGroupNos:[]};
   let item=(await call(root,'POST',fields)).item;
   try{
    item=(await call(root+'/'+item.contentNo+'/publications','POST',{version:item.version,confirmed:true})).item;
    const publishedAt=Date.now();await site.open('/'+path+'.html?query='+encodeURIComponent(title));
    await expect(page.locator('[data-content-list]')).toContainText(title);
    expect(Date.now()-publishedAt).toBeLessThan(10000);
    await site.open('/content.html?id='+item.contentNo);
    await expect(page.locator('[data-content-body]')).toContainText('真实后台发布正文，100% 与下划线 _ 搜索验证。');
    const changed=title+' 已修改';
    item=(await call(root+'/'+item.contentNo,'PUT',{...fields,title:changed,version:item.version})).item;
    await page.reload();await expect(page.locator('[data-content-body] h1')).toHaveText(changed);
    await site.capture('real-'+type+'-published',false);
    item=(await call(root+'/'+item.contentNo+'/unpublications','POST',{version:item.version,confirmed:true})).item;
    await site.open('/'+path+'.html?query='+encodeURIComponent(title));
    await expect(page.locator('[data-content-list]')).not.toContainText(title);
    await expect(page.locator('[data-status]')).toHaveText('暂无已发布内容。');
   }finally{
    // Keep only this test's own records offline, preserving their audit trail.
    if(item.status==='published')await call(root+'/'+item.contentNo+'/unpublications','POST',{version:item.version,confirmed:true});
   }
  }
 });
});
