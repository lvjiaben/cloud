import {test,expect} from '../support/source-site';
import {NavigationPage} from '../pages/NavigationPage';
import {previewResponse} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-61 原结构导航工具栏',()=>{
 test.describe('用户确认的登录地址',()=>{
  test.use({customerOrigin:'http://127.0.0.1:6032'});
  test('TC-61d: 文档、控制台与注册入口使用现有页面和真实公开能力',async({page,request})=>{
   const site=new NavigationPage(page);await site.open('/');const tools=page.locator('.horizontal-menu-navbar-search');
   await expect(tools.getByRole('link',{name:'文档中心',exact:true})).toHaveAttribute('href','/help.html');
   for(const [name,path] of [['控制台','/account'],['免费注册','/auth/register']]){const link=tools.getByRole('link',{name,exact:true});await expect(link).toHaveAttribute('href','http://127.0.0.1:6032'+path);const response=await request.get((await link.getAttribute('href'))!);expect(response.ok()).toBe(true);expect(response.headers()['content-type']).toContain('text/html');}
   const knowledge=await request.get('http://127.0.0.1:6030/x/idc-core/api/v1/public/content/items?type=knowledge&limit=1');expect(knowledge.ok()).toBe(true);const content=await knowledge.json();expect(content.code).toBe(0);expect(Array.isArray(content.data.items)).toBe(true);
   const settings=await request.get('http://127.0.0.1:6030/api/v1/config/public/frontend');expect(settings.ok()).toBe(true);const config=await settings.json();expect(config.code).toBe(0);expect(typeof config.data.auth.registerEnabled).toBe('boolean');
  });
  test('TC-61c: 登录链接采用已配置的127地址并可读取现有登录页面',async({page,request})=>{
   const site=new NavigationPage(page);await site.open('/');const login=page.locator('.horizontal-menu-navbar-search').getByRole('link',{name:'登录',exact:true});await expect(login).toHaveAttribute('href','http://127.0.0.1:6032/auth/login');const response=await request.get((await login.getAttribute('href'))!);expect(response.ok()).toBe(true);expect(response.headers()['content-type']).toContain('text/html');
  });
 });
 test('TC-61b: 当前真实公开导航树与源码展示一致，商品面板和普通链接分别匹配',async({page,request},info)=>{
  const response=await request.get('http://127.0.0.1:6030/x/idc-portal/api/v1/public/brand');expect(response.ok()).toBe(true);const {data}=await response.json(),items=data.brand.navigation;
  const site=new NavigationPage(page);await site.open('/');const links=page.locator('.horizontal-menu-navbar-menu>li>a');await expect(links).toHaveText(items.map(item=>item.label.zhCN||item.label.enUS));
  for(let i=0;i<items.length;i++){const item=items[i],link=links.nth(i);if(item.group||item.children?.length||item.href==='/products.html'){await link.hover();await expect(link).toHaveAttribute('aria-expanded','true');const panel=page.locator('.lina-open .lina-nav-panel');await expect(panel).toBeVisible();for(const child of item.children||[])await expect(panel).toContainText(child.label.zhCN||child.label.enUS);await site.enterPanel();await expect(panel).toBeVisible();await site.closePanel();}else{await expect(link).not.toHaveClass(/nav-arrow/);await expect(link).toHaveAttribute('href',new URL(item.href,'http://localhost:6680').href);}}
  for(const name of ['文档中心','控制台','登录','免费注册'])await expect(page.locator('.horizontal-menu-navbar-search').getByRole('link',{name,exact:true})).toBeVisible();await site.captureSection('.nav-layoutcontainer',info.outputPath('real-public-navigation-toolbar.png'));
 });
 test('TC-61a: 三搜索范围调用本方参数，关闭恢复焦点且十页入口一致',async({page},info)=>{
  const requests:string[]=[];
  await page.route('**/x/**',async route=>{const request=route.request(),url=new URL(request.url());requests.push(url.pathname+url.search);const response=previewResponse(url,request.method());await route.fulfill({status:response.status,json:{code:response.status===200?0:1,data:response.data}});});
  const site=new NavigationPage(page);
  for(const name of ['index','products','configure','news','announcements','help','downloads','content','about','contact']){await site.open('/'+name+'.html');const tools=page.locator('.horizontal-menu-navbar-search');await expect(tools.getByRole('link',{name:'文档中心',exact:true})).toHaveAttribute('href','/help.html');for(const [label,path] of [['控制台','/account'],['登录','/auth/login'],['免费注册','/auth/register']])await expect(tools.getByRole('link',{name:label,exact:true})).toHaveAttribute('href','http://localhost:6032'+path);}
  await site.open('/');await site.openSearch();await expect(page.locator('.nav-search')).toBeVisible();await expect(page.locator('.nav-search-tab')).toHaveText(['全部产品','新闻资讯','帮助文档']);await site.captureSection('.nav-layoutcontainer',info.outputPath('toolbar-search.png'));await site.escape();await expect(page.locator('.nav-search')).toBeHidden();await expect(page.locator('.search-input')).toBeFocused();
  await site.openSearch();await page.getByRole('button',{name:'关闭搜索',exact:true}).click();await expect(page.locator('.search-input')).toBeFocused();await expect(page.locator('.nav-search')).toBeHidden();await site.openSearch();await page.locator('[data-brand-name]').first().click();await expect(page.locator('.nav-search')).toBeHidden();
  await site.searchIn('news','本方 新闻');await expect(page).toHaveURL(/news.html\?query=/);await expect.poll(()=>requests.some(value=>value.includes('/content/items?')&&new URLSearchParams(value.split('?')[1]).get('query')==='本方 新闻'&&value.includes('type=news'))).toBe(true);
  await site.searchIn('docs','帮助');await expect(page).toHaveURL(/help.html\?query=/);await expect.poll(()=>requests.some(value=>value.includes('type=knowledge')&&new URLSearchParams(value.split('?')[1]).get('query')==='帮助')).toBe(true);
  await site.searchIn('all','模拟商品');await expect(page).toHaveURL(/products.html\?namePrefix=/);await expect.poll(()=>requests.some(value=>value.includes('/catalog/products?')&&new URLSearchParams(value.split('?')[1]).get('namePrefix')==='模拟商品')).toBe(true);
  await site.setViewport(390,844);await site.toggleMenu();await expect(page.getByRole('link',{name:'免费注册',exact:true})).toBeVisible();await site.openSearch();expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(390);await site.captureSection('.nav-layoutcontainer',info.outputPath('mobile-toolbar-search.png'));await site.escape();await site.toggleMenu();await expect(page.locator('.horizontal-menu-navbar-search')).toBeHidden();
 });
});

