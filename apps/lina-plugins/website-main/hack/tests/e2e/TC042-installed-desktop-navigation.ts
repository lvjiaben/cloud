import {test,expect} from '@playwright/test';
import {HeaderNavigationPage} from '../pages/HeaderNavigationPage';

// Read-only acceptance of the explicitly configured local navigation mapping.
test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-42 已安装官网桌面分组导航',()=>{
 test('TC-42a: 真实配置展开后命中子项、键盘关闭且链接落在对应页面',async({page,request},info)=>{
  const response=await request.get('/x/idc-portal/api/v1/public/brand');expect(response.ok()).toBe(true);
  const {data}=await response.json();expect(data.brand.templateEntry.assetBase).toBe('/x-assets/website-main/v0.1.19/');
  const site=new HeaderNavigationPage(page),writes:string[]=[];
  page.on('request',r=>{if(!['GET','HEAD'].includes(r.method()))writes.push(r.method()+' '+r.url());});
  for(const [group,child,path] of [['资讯中心','新闻','news'],['支持服务','资源下载','downloads']]){
   await site.open('/');await expect(site.group(group)).toHaveAttribute('aria-expanded','false');
   await site.toggle(group);await expect(site.group(group)).toHaveAttribute('aria-expanded','true');
   expect((await site.hitTarget(child)).hit).toBe(true);
   await info.attach(group,{path:await site.capture('installed-navigation-'+path,false),contentType:'image/png'});
   await site.toggle(group,'Escape');await expect(site.group(group)).toHaveAttribute('aria-expanded','false');await expect(site.link(child)).toBeHidden();
   await site.toggle(group,'Enter');expect((await site.hitTarget(child)).hit).toBe(true);
   await site.follow(child);await expect(page).toHaveURL(new RegExp('/'+path+'\\.html$'));
   await expect(page.locator('base')).toHaveAttribute('href','/x-assets/website-main/v0.1.19/');
  }
  expect(writes).toEqual([]);
 });
});
