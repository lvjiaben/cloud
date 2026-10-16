import path from 'node:path';
import { mkdir } from 'node:fs/promises';
import { readFileSync } from 'node:fs';
import { test as base, expect, type Page } from '@playwright/test';
import { config } from '../../../../../../hack/tests/fixtures/config';

const endpoint = '/x/idc-portal/api/v1/admin/site/configuration';
const test = base.extend<{adminPage: Page}>({
 adminPage: async ({page},use)=>{
  page.setDefaultTimeout(10000);
  const login=await page.request.post(config.apiBaseURL+'auth/login',{data:{username:config.adminUser,password:config.adminPass,clientType:'desktop'}});
  const session=await login.json();
  expect(session.code).toBe(0);
  const state=JSON.parse(readFileSync(path.resolve('temp/storage-state/admin.json'),'utf8'));
  const entries=state.origins[0].localStorage;
  const access=entries.find((item:{name:string})=>item.name.endsWith('core-access'));
  const data=JSON.parse(access.value);
  data.accessToken=session.data.accessToken;data.refreshToken=session.data.refreshToken;access.value=JSON.stringify(data);
  await page.addInitScript(items=>{if(sessionStorage.getItem('website-test-session'))return;for(const item of items)localStorage.setItem(item.name,item.value);for(const key of Object.keys(localStorage))if(key.startsWith('linapro:i18n:runtime:'))localStorage.removeItem(key);sessionStorage.setItem('website-test-session','1');},entries);
  await page.route(/\/(?:api\/v1|x\/[^/]+\/api\/v1)\//,async route=>{const url=new URL(route.request().url());const response=await route.fetch({url:new URL(url.pathname+url.search,config.backendBaseURL).href});await route.fulfill({response});});
  await use(page);
 }
});

test.describe('TC-2 官网导航', () => {
 test('TC-2a: 保存头尾导航，刷新恢复，拒绝伪造页面并显示开发文档', async ({ adminPage }) => {
  await adminPage.goto('/admin/system/website-configuration');
  const token = await adminPage.evaluate(() => { const k=Object.keys(localStorage).find(k=>k.endsWith('-core-access'));return k?JSON.parse(localStorage.getItem(k)??'{}').accessToken:''; });
  const headers = { Authorization: 'Bearer '+token };
  async function call(url: string, method='GET', data?: unknown) { const r=await adminPage.request.fetch(config.backendBaseURL+url,{method,headers,data,timeout:15000});return r.json(); }
  const original = (await call(endpoint)).data;
  const selected = original.plugins.find((p:any)=>p.pluginId==='website-default');
  expect(selected.pages.some((p:any)=>p.id==='products')).toBe(true);
  const navigation = { header:[{id:'nav-home',parentId:'',nameZh:'导航验收首页',nameEn:'Navigation home',pageId:'home',url:'',sortOrder:0},{id:'nav-login',parentId:'nav-home',nameZh:'会员登录验收',nameEn:'Login test',pageId:'customer-login',url:'',sortOrder:1},{id:'nav-register',parentId:'nav-home',nameZh:'会员注册验收',nameEn:'Register test',pageId:'customer-register',url:'',sortOrder:2}], footer:[{id:'nav-docs',parentId:'',nameZh:'接口文档验收',nameEn:'Docs test',pageId:'',url:'https://example.test/docs',sortOrder:0}] };
  const input={selectedPlugin:selected.pluginId,common:original.common,config:original.configurations?.[selected.pluginId]??original.config??{},navigation};
  try {
   expect((await call(endpoint,'PUT',input)).code).toBe(0);
   await adminPage.reload();
   const root=adminPage.locator('[data-plugin-frontend-root="idc-portal"]');
   await expect(root.getByRole('heading',{name:'官网导航',exact:true})).toBeVisible();
   await expect(root).toContainText('导航验收首页');
   await expect(root).toContainText('会员登录验收');
   await expect(root).not.toContainText('plugin.idc-portal.');
   await root.getByText('底部导航',{exact:true}).click();
   await expect(root).toContainText('接口文档验收');
   await root.getByRole('button',{name:/添.*加.*导.*航/}).click();
   await expect(adminPage.getByRole('dialog',{name:'添加导航',exact:true})).toBeVisible();
   await mkdir(path.resolve('../../temp/20261016'),{recursive:true});
   await adminPage.screenshot({path:path.resolve('../../temp/20261016/113000-website-navigation-dialog.png'),fullPage:true});
   const editor=adminPage.getByRole('dialog',{name:'添加导航',exact:true});
   await editor.getByLabel('中文名称',{exact:true}).fill('临时导航验收');
   await editor.getByLabel('英文名称',{exact:true}).fill('Temporary navigation');
   await editor.getByRole('button',{name:/确.*认/}).click();
   await expect(root).toContainText('临时导航验收');
   await root.getByRole('button',{name:/保.*存.*并.*启.*用/}).click();
   await expect.poll(async()=>((await call(endpoint)).data.navigation.footer.length)).toBe(2);
   const row=root.getByRole('row').filter({hasText:'临时导航验收'});
   await row.getByRole('button',{name:/编.*辑/}).click();
   const edit=adminPage.getByRole('dialog',{name:'编辑',exact:true});
   await edit.getByLabel('中文名称',{exact:true}).fill('临时导航已修改');
   await edit.getByRole('button',{name:/确.*认/}).click();
   await expect(root).toContainText('临时导航已修改');
   await root.getByRole('row').filter({hasText:'临时导航已修改'}).getByRole('button',{name:/删.*除/}).click();
   await adminPage.getByRole('button',{name:/确.*定/}).last().click();
   await root.getByRole('button',{name:/保.*存.*并.*启.*用/}).click();
   await expect.poll(async()=>((await call(endpoint)).data.navigation.footer.length)).toBe(1);
   const bad=structuredClone(input);bad.navigation.header[0].pageId='forged-page';
   expect((await call(endpoint,'PUT',bad)).code).not.toBe(0);
   expect((await call(endpoint)).data.navigation).toEqual(navigation);
   const publicBrand=(await call('/x/idc-portal/api/v1/public/brand')).data.brand;
   expect(publicBrand.navigation[0].label.zhCN).toBe('导航验收首页');
   expect(publicBrand.navigation[0].children[0]).toMatchObject({href:'/auth/login',customer:true});
   expect(publicBrand.footerNavigation[0]).toMatchObject({href:'https://example.test/docs',external:true});
   await root.getByRole('button',{name:/开.*发.*文.*档/}).click();
   await expect(adminPage.getByRole('dialog',{name:'开发文档'}).locator('article')).toBeVisible();
   await expect(adminPage.getByRole('dialog',{name:'开发文档'})).toContainText('/x/idc-portal/api/v1/public/brand');
   const guide=adminPage.getByRole('dialog',{name:'开发文档'});
   await expect(guide.getByPlaceholder('搜索接口，如 login、orders、wallet、support')).toBeVisible();
   await expect(guide).not.toContainText('plugin.idc-portal.');
   await guide.getByPlaceholder('搜索接口，如 login、orders、wallet、support').fill('/public/brand');
   await expect(guide.locator('summary')).toContainText('/x/idc-portal/api/v1/public/brand');
   await guide.locator('summary').click();
   await expect(guide.locator('details pre')).toContainText('responses');
   const popup=adminPage.waitForEvent('popup');await guide.getByRole('link',{name:'OpenAPI 3.0'}).click();
   const apiPage=await popup;await apiPage.waitForLoadState();expect(apiPage.url()).toContain('/api.json');await apiPage.close();
   const downloadPending=adminPage.waitForEvent('download');await guide.getByRole('link',{name:'下载当前接口清单'}).click();
   const download=await downloadPending;expect(download.suggestedFilename()).toBe('template-api-contracts.json');
   const exported=JSON.parse(readFileSync((await download.path())!,'utf8'));
   expect(exported.openapi).toBe('3.0.0');expect(Object.keys(exported.paths)).toEqual(['/x/idc-portal/api/v1/public/brand']);
   await adminPage.screenshot({path:path.resolve('../../temp/20261016/113100-website-development-guide.png'),fullPage:true});
   expect((await call(endpoint,'PUT',{...input,selectedPlugin:'website-html',config:{herotitle:'原生模板验收'}})).code).toBe(0);
   const native=(await call('/x/idc-portal/api/v1/public/brand')).data.brand;
   expect(native.templateEntry).toMatchObject({url:'/x-assets/website-html/v0.1.3/index.html',spa:false});
   const website=await adminPage.context().newPage();
   try {
    await website.goto('http://127.0.0.1:6680/products.html');
    await expect(website.locator('h1')).toHaveText('原生模板验收');
    await expect(website.getByRole('link',{name:'会员登录验收'})).toHaveAttribute('href','http://localhost:6032/auth/login');
    await expect(website.getByRole('link',{name:'会员注册验收'})).toHaveAttribute('href','http://localhost:6032/auth/register');
    await expect(website.getByRole('link',{name:'接口文档验收'})).toHaveAttribute('rel','noopener noreferrer');
    await expect(website.locator('#error')).toBeEmpty();
    await website.screenshot({path:path.resolve('../../temp/20261016/113200-native-website.png'),fullPage:true});
    expect((await website.request.get('http://127.0.0.1:6680'+native.templateEntry.assetBase+'app.js')).status()).toBe(200);
    expect((await website.request.get('http://127.0.0.1:6680/x-assets/website-html/v0.1.3/missing.js')).status()).toBe(404);
    expect((await call(endpoint,'PUT',input)).code).toBe(0);
    await website.goto('http://127.0.0.1:6680/');
    await expect(website.locator('body')).toContainText('导航验收首页');
    await expect(website.locator('#products')).toHaveCount(0);
   } finally {await website.close();}

  } finally {
   const restore={selectedPlugin:original.selectedPlugin||selected.pluginId,common:original.common,config:original.config??{},navigation:original.navigation};
   expect((await call(endpoint,'PUT',restore)).code).toBe(0);
  }
 });
});

test('TC-2b: 真实页面候选与无跳转分组，桌面、键盘和移动端展开',async({adminPage})=>{
 await adminPage.goto('/admin/system/website-configuration');
 const token=await adminPage.evaluate(()=>{const key=Object.keys(localStorage).find(k=>k.endsWith('-core-access'));return key?JSON.parse(localStorage.getItem(key)!).accessToken:'';});
 const headers={Authorization:'Bearer '+token};
 async function call(method='GET',data?:unknown){return(await adminPage.request.fetch(config.backendBaseURL+endpoint,{method,headers,data})).json();}
 const original=(await call()).data;
 const navigation={header:[{id:'group',group:true,nameZh:'产品分组验收',nameEn:'Products group',parentId:'',pageId:'',url:'',sortOrder:0},{id:'products',nameZh:'真实商品页面',nameEn:'Product list',parentId:'group',pageId:'products',url:'',sortOrder:0},{id:'empty',group:true,nameZh:'空分组不展示',nameEn:'Empty group',parentId:'',pageId:'',url:'',sortOrder:1}],footer:[{id:'footer-group',group:true,nameZh:'底部分类验收',nameEn:'Footer group',parentId:'',pageId:'',url:'',sortOrder:0},{id:'footer-products',nameZh:'底部商品页面',nameEn:'Footer products',parentId:'footer-group',pageId:'products',url:'',sortOrder:0}]};
 const input={selectedPlugin:'website-default',common:original.common,config:original.configurations?.['website-default']??{},navigation};
 try{
  expect((await call('PUT',input)).code).toBe(0);
  await adminPage.reload();
  const root=adminPage.locator('[data-plugin-frontend-root="idc-portal"]');
  await expect(root).toContainText('产品分组验收');await expect(root).toContainText('仅展开子菜单（不跳转）');
  await root.getByRole('row').filter({hasText:'产品分组验收'}).filter({hasText:'仅展开子菜单（不跳转）'}).getByRole('button',{name:/编.*辑/}).click();
  const dialog=adminPage.getByRole('dialog',{name:'编辑',exact:true});
  await dialog.getByLabel('跳转页面',{exact:true}).click();
  await expect(adminPage.locator('.ant-select-dropdown:visible').getByText('商品列表 (/products)',{exact:true})).toBeVisible();
  await adminPage.screenshot({path:path.resolve('../../temp/20261016/website-group-editor.png'),fullPage:true});
  await adminPage.keyboard.press('Escape');await expect(dialog).toBeHidden();
  const website=await adminPage.context().newPage();
  try{
   for(const viewport of [{width:1440,height:900},{width:390,height:844}]){
    await website.setViewportSize(viewport);await website.goto('http://127.0.0.1:6680/');await website.waitForLoadState('networkidle');
    const group=website.getByRole('button',{name:'产品分组验收'});const child=website.getByRole('link',{name:'真实商品页面'});
    await expect(group).toBeVisible();await expect(child).toBeHidden();await expect(website.getByText('空分组不展示',{exact:true})).toHaveCount(0);
    const current=website.url();await group.click();await expect(group).toHaveAttribute('aria-expanded','true');await expect(child).toBeVisible();expect(website.url()).toBe(current);
    await group.focus();await website.keyboard.press('Escape');await expect(child).toBeHidden();await website.keyboard.press('Enter');await expect(child).toBeVisible();expect(website.url()).toBe(current);
    await website.screenshot({path:path.resolve('../../temp/20261016/website-group-'+viewport.width+'.png'),fullPage:true});
    const footerGroup=website.getByRole('button',{name:'底部分类验收'});const footerChild=website.getByRole('link',{name:'底部商品页面'});
    await expect(footerChild).toBeHidden();await footerGroup.click();await expect(footerChild).toBeVisible();expect(website.url()).toBe(current);await footerGroup.click();await expect(footerChild).toBeHidden();
    await child.click();await expect(website).toHaveURL(/\/products$/);await expect(website.getByTestId('public-catalog')).toBeVisible();
   }
   await website.setViewportSize({width:1440,height:900});
   await website.goto('http://127.0.0.1:6684/products');await website.waitForLoadState('networkidle');
   await expect(website.getByTestId('public-catalog')).toBeVisible();await expect(website.getByRole('button',{name:'产品分组验收'})).toBeVisible();
   await website.getByRole('button',{name:'产品分组验收'}).click();await expect(website.getByRole('link',{name:'真实商品页面'})).toBeVisible();
   await website.screenshot({path:path.resolve('../../temp/20261016/website-static-independent.png'),fullPage:true});
   const htmlInput={...input,selectedPlugin:'website-html',config:original.configurations?.['website-html']??{}};
   expect((await call('PUT',htmlInput)).code).toBe(0);
   const htmlData=(await call()).data;expect(htmlData.plugins.find((item:any)=>item.pluginId==='website-html').pages.find((page:any)=>page.id==='products').path).toBe('/products.html');
   await website.goto('http://127.0.0.1:6680/');await website.getByRole('button',{name:'产品分组验收'}).click();await website.getByRole('link',{name:'真实商品页面'}).click();await expect(website).toHaveURL(/\/products\.html$/);await expect(website.locator('#error')).toBeEmpty();
   expect((await website.request.get('http://127.0.0.1:6680/not-implemented.html')).status()).toBe(404);
  }finally{await website.close();}
 }finally{expect((await call('PUT',{selectedPlugin:original.selectedPlugin,common:original.common,config:original.config??{},navigation:original.navigation})).code).toBe(0);}
});
