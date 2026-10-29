import {test,expect} from '../support/source-site';
import {NavigationPage} from '../pages/NavigationPage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});

test('TC-104: 已缓存的公开导航先显示，品牌接口返回后同步最新菜单',async({page},info)=>{
 const cached=[
  {label:'首页',href:'/'},
  {label:'产品与服务',href:'/products.html',group:true,children:[]},
  {label:'生态合作',href:'/content.html?type=partner'},
  {label:'支持与服务',href:'/help.html'},
  {label:'了解我们',href:'/about.html'}
 ];
 let release!:()=>void;
 const gate=new Promise<void>(resolve=>{release=resolve;});
 await page.addInitScript(items=>localStorage.setItem('lina.public-navigation.v1',JSON.stringify({savedAt:Date.now(),items})),cached);
 await page.route('**/x/idc-portal/api/v1/public/brand',async route=>{
  await gate;
  await route.fulfill({json:{code:0,data:{brand:{siteName:{zhCN:'测试官网'},navigation:[{label:'最新发布菜单',href:'/help.html'}]}}}});
 });
 try{
  await page.goto('/');
  const menu=page.locator('.horizontal-menu-navbar-menu');
  await expect(menu).toContainText('首页',{timeout:1000});
  await expect(menu).toContainText('产品与服务');
  await page.screenshot({path:info.outputPath('cached-navigation-before-brand.png')});
 }finally{release();}
 await expect(page.locator('.horizontal-menu-navbar-menu')).toContainText('最新发布菜单');
 await expect(page.locator('.horizontal-menu-navbar-menu')).toContainText('生态合作');
 await page.screenshot({path:info.outputPath('fresh-navigation-after-brand.png')});
});

test('TC-104b: 商品菜单先展示已缓存目录，后台刷新完成后保持正确商品',async({page},info)=>{
 const category={categoryNo:'cat-fast',nameZhCN:'本地服务器分类'};
 const products=[{publicSlug:'cached-node',nameZhCN:'缓存中的云服务器',summaryZhCN:'本地快照商品',categoryNo:'cat-fast'}];
 let release!:()=>void,started!:()=>void,catalogCalls=0;
 const gate=new Promise<void>(resolve=>{release=resolve;}),requestStarted=new Promise<void>(resolve=>{started=resolve;});
 await page.route('**/x/**',route=>{
  const path=new URL(route.request().url()).pathname;
  if(path==='/x/idc-portal/api/v1/public/brand')return route.fulfill({json:{code:0,data:{brand:{siteName:{zhCN:'缓存菜单测试'},navigation:[{label:'产品与服务',href:'/products.html'}]}}}});
  if(path==='/x/idc-core/api/v1/public/content/items')return route.fulfill({json:{code:0,data:{items:[]}}});
  return route.fulfill({json:{code:0,data:{categories:[category],products}}});
 });
 await page.route('**/x/idc-core/api/v1/public/catalog/products?*',async route=>{
  const url=new URL(route.request().url());if(url.searchParams.get('limit')!=='12')return route.fulfill({json:{code:0,data:{categories:[category],products}}});
  catalogCalls++;
  if(catalogCalls===1)return route.fulfill({json:{code:0,data:{categories:[category],products}}});
  started();await gate;await route.fulfill({json:{code:0,data:{categories:[category],products:[{...products[0],nameZhCN:'权威更新服务器'}]}}});
 });
 const site=new NavigationPage(page);
 try{
  await site.open('/');await expect.poll(()=>page.evaluate(()=>Boolean(localStorage.getItem('lina.public-catalog.v1')))).toBe(true);
  await site.hoverTitle('产品与服务');
  const panel=site.panel();await expect(panel.getByRole('link',{name:'缓存中的云服务器 本地快照商品'})).toHaveAttribute('href','http://localhost:6680/products.html?product=cached-node');
  await requestStarted;await page.screenshot({path:info.outputPath('catalog-cache-before-refresh.png')});
 }finally{release();}
 await expect(site.panel().getByRole('link',{name:'权威更新服务器 本地快照商品'})).toHaveAttribute('href','http://localhost:6680/products.html?product=cached-node');
 await page.screenshot({path:info.outputPath('catalog-fresh-after-refresh.png')});
});
