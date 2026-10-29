import {test,expect} from '../support/source-site';
import {controlledBusiness} from '../support/controlled-business';
import {HeaderNavigationPage} from '../pages/HeaderNavigationPage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-41 桌面分组导航不能被原滚动容器裁切',()=>{
 test('TC-41a: 两个分组的子项实际绘制、命中且可进入正确页面',async({page},info)=>{
  const fixture=await controlledBusiness(page),site=new HeaderNavigationPage(page);
  fixture.state.brand.navigation=[{label:'首页',href:'/'},{label:'产品与服务',href:'/products.html'},
   {label:'资讯中心',group:true,children:[{label:'新闻',href:'/news.html'},{label:'公告',href:'/announcements.html'}]},
   {label:'支持服务',group:true,children:[{label:'帮助文档',href:'/help.html'},{label:'资源下载',href:'/downloads.html'}]},
   {label:'关于我们',href:'/about.html'},{label:'联系我们',href:'/contact.html'}];
  for(const [group,child,path] of [['资讯中心','新闻','news'],['支持服务','资源下载','downloads']]){
   await site.open('/index.html');await expect(site.group(group)).toHaveAttribute('aria-expanded','false');
   await site.toggle(group);await expect(site.group(group)).toHaveAttribute('aria-expanded','true');
   const hit=await site.hitTarget(child);expect(hit.hit,JSON.stringify(hit)).toBe(true);
   expect(hit.y+hit.height).toBeLessThanOrEqual(900);
   await info.attach(group,{path:await site.capture('desktop-navigation-'+path,false),contentType:'image/png'});
   await site.follow(child);await expect(page).toHaveURL(new RegExp('/'+path+'\\.html$'));
  }
  expect(fixture.requests.every(request=>request.method==='GET')).toBe(true);
 });
 test('TC-41b: 三级分组键盘展开与关闭，隐藏子项不抢占页面',async({page},info)=>{
  const fixture=await controlledBusiness(page),site=new HeaderNavigationPage(page);
  fixture.state.brand.navigation=[{label:'测试支持',group:true,children:[{label:'测试资料',group:true,children:[{label:'测试下载',href:'/downloads.html'}]}]}];
  await site.open('/index.html');await site.toggle('测试支持','Enter');await site.toggle('测试资料','Space');
  const hit=await site.hitTarget('测试下载');expect(hit.hit,JSON.stringify(hit)).toBe(true);
  expect(hit.x+hit.width).toBeLessThanOrEqual(1440);await expect(page).toHaveURL(/index.html$/);
  await info.attach('三级菜单',{path:await site.capture('desktop-navigation-third-level',false),contentType:'image/png'});
  await site.toggle('测试资料','Escape');await expect(site.group('测试资料')).toHaveAttribute('aria-expanded','false');await expect(site.link('测试下载')).toBeHidden();
  await site.toggle('测试支持','Escape');await expect(site.group('测试支持')).toHaveAttribute('aria-expanded','false');
 });
});
