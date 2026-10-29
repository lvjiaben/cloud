// Mount the shipped customer page in a browser without requiring a live account.
// The real API adapter and translations remain in use; only transport and layout
// are isolated, so retired writes cannot be hidden by a server rejection.
import { test, expect } from '@host-tests/fixtures/auth';
import { createRequire } from 'node:module';
import { readFile, mkdir } from 'node:fs/promises';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const pluginRoot = fileURLToPath(new URL('../../..', import.meta.url));
const appRoot = path.join(pluginRoot, 'frontend/customer');
const repoRoot = path.resolve(pluginRoot, '../../..');
let server: { close(): Promise<void> };
let origin = '';

// The app's locked workspace supplies its compiler; no external network or host
// frontend source is needed to execute this template-owned regression.
test.beforeAll(async () => {
  const appRequire = createRequire(path.join(appRoot, 'package.json'));
  const compilerRequire = createRequire(path.join(appRoot, 'vendor/vben/internal/vite-config/package.json'));
  const { createServer } = await import(pathToFileURL(appRequire.resolve('vite')).href);
  const { default: vue } = await import(pathToFileURL(compilerRequire.resolve('@vitejs/plugin-vue')).href);
  const locales = Object.fromEntries(await Promise.all(['zh-CN', 'en-US'].map(async locale => [locale,
    Object.fromEntries(await Promise.all(['purchaseConfiguration', 'commerce'].map(async name => [name,
      JSON.parse(await readFile(path.join(appRoot, 'src/locales/langs', locale, name + '.json'), 'utf8')),
    ]))),
  ])));
  const instance = await createServer({
    root: appRoot,
    configFile: false,
    logLevel: 'error',
    optimizeDeps: { noDiscovery: true, include: ['vue', 'vue-router', 'ant-design-vue', 'dayjs'] },
    resolve: { alias: [{find:'#/api/request',replacement:'\0fixture-request'},{find:'#/locales',replacement:'\0fixture-locales'},{find:'#',replacement:path.join(appRoot,'src')}] },
    plugins: [vue(), {
      name: 'purchase-browser-fixture',
      enforce: 'pre',
      resolveId(id: string) {
        if (id.startsWith("\0")) return id;
        if (['virtual:purchase-entry', '#/api/request', '#/locales', '@vben/common-ui', '@vben/preferences', '@vben/stores'].includes(id)) return '\0' + id.replace('#/api/request', 'fixture-request').replace('#/locales', 'fixture-locales');
        return null;
      },
      load(id: string) {
        if (id === '\0virtual:purchase-entry') return `
          import { createApp,h } from 'vue';
          import { createRouter,createWebHistory,RouterView } from 'vue-router';
          import Redirect from '/src/views/account/website-purchase-redirect.vue';
          import Bridge from '/src/views/account/website-purchase.vue';
          import Session from '/src/views/account/website-session.vue';
          import 'ant-design-vue/dist/reset.css';
          const router=createRouter({history:createWebHistory(),routes:[
            {path:'/account/products/:publicSlug/configure',component:Redirect},
            {path:'/account/cart',name:'CustomerCart',component:Redirect},
            {path:'/account/website-purchase',component:Bridge},
            {path:'/auth/website-session',component:Session},
            {path:'/auth/login',component:{render:()=>h('h1','登录客户中心')}},
            {path:'/account/test-parent',component:{render:()=>h('h1','官网结算登录')}},
          ]});createApp(RouterView).use(router).mount('#app');`;
        if (id === '\0@vben/stores') return `const store={accessToken:'fixture-session-token'};export const useAccessStore=()=>store;`;
        if (id === '\0@vben/preferences') return `export const preferences={app:{locale:'zh-CN'}};`;
        if (id === '\0@vben/common-ui') return `import {h} from 'vue'; export const Page={setup:(_, {slots})=>()=>h('main',slots.default?.())};`;
        if (id === '\0fixture-request') return `
          export const pluginApiPath=(plugin,path)=>'/x/'+plugin+'/api/v1/'+path;
          import {useAccessStore} from '@vben/stores';
          async function request(path,method='GET',body){const response=await fetch(path==='/user/info'?'/api/v1/user/info':path,{method,headers:{'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});if(response.status===401){useAccessStore().accessToken='';throw new Error('session expired');}const result=await response.json();if(!response.ok||result.code!==0)throw new Error('request failed');return result.data;}
          export const requestClient={get:path=>request(path),post:(path,body)=>request(path,'POST',body),delete:(path)=>request(path,'DELETE')};`;
        if (id === '\0fixture-locales') return `
          const messages=${JSON.stringify(locales)};
          export const $t=key=>key.split('.').reduce((value,part)=>value?.[part],messages[new URL(location.href).searchParams.get('lang')||'zh-CN'])||key;`;
        return null;
      },
      configureServer(vite: { middlewares: { use(handler: unknown): void } }) {
        vite.middlewares.use((req: { url?: string }, res: { setHeader(key: string, value: string): void; end(value: string): void }, next: () => void) => {
          if(req.url==='/api/v1/user/info'){res.setHeader('Content-Type','application/json');res.end(JSON.stringify({code:0,data:{username:'fixture'}}));return;}
          if (!req.url?.startsWith('/account/') && !req.url?.startsWith('/auth/')) return next();
          res.setHeader('Content-Type', 'text/html');
          res.end('<div id="app"></div><script type="module" src="/@id/__x00__virtual:purchase-entry"></script>');
        });
      },
    }],
    server: { host: '127.0.0.1', port: 0 },
  });
  await instance.listen();
  server = instance;
  const address = instance.httpServer.address();
  origin = `http://127.0.0.1:${address.port}`;
});
test.afterAll(async () => { await server?.close(); });


import { WebsitePurchaseEntryPage } from '../pages/WebsitePurchaseEntryPage';
test('TC-2a: 客户购买入口跳转官网，失败可重试并保留商品路径',async({page})=>{
 const ui=new WebsitePurchaseEntryPage(page,origin);let fail=true;
 await page.route('**/__website/configuration',route=>route.fulfill({status:fail?503:200,json:{origin:'http://127.0.0.1:6680'}}));
 await page.route('http://127.0.0.1:6680/products/test-product',route=>route.fulfill({contentType:'text/html; charset=utf-8',body:'<h1>官网商品选配</h1>'}));
 await ui.openProduct();await expect(page.getByText('官网暂时不可用，请稍后重试。',{exact:true})).toBeVisible();await ui.screenshot('redirect-error');
 fail=false;await page.getByRole('button',{name:/重\s*试/}).click();await expect(page.getByRole('heading',{name:'官网商品选配'})).toBeVisible();
});
test('TC-2b: 登录交接仅向同源打开者交付会话，令牌不进入URL',async({page})=>{
 const ui=new WebsitePurchaseEntryPage(page,origin);await page.goto(origin+'/account/test-parent');
 await page.evaluate(()=>{window.addEventListener('message',event=>{if(event.origin===location.origin)(window as any).handoff=event.data;});});
 const nonce='00000000-1111-2222-3333-444444444444';
 await page.evaluate(({nonce})=>window.open('/account/website-purchase','lina-website-login-'+nonce),{nonce});
 await expect.poll(()=>page.evaluate(()=>(window as any).handoff?.type)).toBe('lina-website-session');
 expect(await page.evaluate(()=>(window as any).handoff)).toEqual({type:'lina-website-session',nonce,accessToken:'fixture-session-token'});
 await ui.screenshot('login-return');
});

test('TC-2c: 拒绝向不同源的打开者交付登录会话',async({page})=>{
 await page.goto(origin.replace('127.0.0.1','localhost')+'/account/test-parent');
 await page.evaluate(()=>{window.addEventListener('message',event=>(window as any).foreignHandoff=event.data);});
 const popupPromise=page.waitForEvent('popup');
 await page.evaluate(({url})=>window.open(url,'lina-website-login-00000000-1111-2222-3333-444444444444'),{url:origin+'/account/website-purchase'});
 const popup=await popupPromise;
 await expect(popup.getByText('已登录，正在返回官网结算。',{exact:true})).toBeVisible();
 expect(await page.evaluate(()=>(window as any).foreignHandoff)).toBeUndefined();expect(popup.isClosed()).toBe(false);
 await popup.close();
});

test('TC-2d: 客户中心已有会话通过隐藏页面交接，不弹窗、不重新登录',async({page})=>{
 await page.route('**/__website/configuration',route=>route.fulfill({json:{origin}}));
 await page.goto(origin+'/account/test-parent');
 await page.evaluate(()=>{
  window.addEventListener('message',event=>{if(event.origin===location.origin)(window as any).handoff=event.data;});
  const frame=document.createElement('iframe');frame.hidden=true;frame.src='/auth/website-session?nonce=12345678-1234-1234-1234-123456789012';document.body.append(frame);
 });
 await expect.poll(()=>page.evaluate(()=>(window as any).handoff?.accessToken)).toBe('fixture-session-token');
 await expect(page).toHaveURL(origin+'/account/test-parent');expect(page.context().pages()).toHaveLength(1);
});

test('TC-2e: 隐藏认证页拒绝向配置外的网站交付会话',async({page})=>{
 await page.route('**/__website/configuration',route=>route.fulfill({json:{origin:'https://other.invalid'}}));
 await page.goto(origin+'/account/test-parent');
 await page.evaluate(()=>{
  (window as any).handoffCount=0;window.addEventListener('message',()=>{(window as any).handoffCount++;});
  const frame=document.createElement('iframe');frame.src='/auth/website-session?nonce=12345678-1234-1234-1234-123456789012';document.body.append(frame);
 });
 await expect(page.locator('iframe')).toBeVisible();
 await page.waitForTimeout(500);
 expect(await page.evaluate(()=>(window as any).handoffCount)).toBe(0);
});

test('TC-2f: 已配置本地官网别名复用同一个客户会话',async({page})=>{
 const alias=origin.replace('127.0.0.1','localhost');
 await page.route('**/__website/configuration',route=>route.fulfill({json:{origin,origins:[origin,alias]}}));
 await page.goto(alias+'/account/test-parent');
 await page.evaluate(({customer,website})=>{
  window.addEventListener('message',event=>{if(event.origin===customer)(window as any).handoff=event.data;});
  const frame=document.createElement('iframe');frame.hidden=true;frame.src=customer+'/auth/website-session?nonce=12345678-1234-1234-1234-123456789012&websiteOrigin='+encodeURIComponent(website);document.body.append(frame);
 },{customer:origin,website:alias});
 await expect.poll(()=>page.evaluate(()=>(window as any).handoff?.accessToken)).toBe('fixture-session-token');
 await expect(page).toHaveURL(alias+'/account/test-parent');expect(page.context().pages()).toHaveLength(1);
});

test('TC-2g: 浏览器隔离嵌入式会话时在当前页交接，令牌只在表单正文',async({page})=>{
 await page.route('**/__website/configuration',route=>route.fulfill({json:{origin}}));let posts=0;
 await page.route('**/__website/session',async route=>{
  expect(route.request().method()).toBe('POST');expect(route.request().url()).not.toContain('fixture-session-token');
  const body=new URLSearchParams(route.request().postData()||'');expect(body.get('accessToken')).toBe('fixture-session-token');expect(body.get('returnTo')).toBe('/checkout');posts++;
  await route.fulfill({contentType:'text/html; charset=utf-8',body:'<h1>已返回官网确认订单</h1>'});
 });
 await page.goto(origin+'/auth/website-session?probe=1&returnTo=%2Fcheckout&websiteOrigin='+encodeURIComponent(origin));
 await expect(page.getByRole('heading',{name:'已返回官网确认订单',exact:true})).toBeVisible();expect(posts).toBe(1);expect(page.context().pages()).toHaveLength(1);
});


test('TC-2h: 过期客户会话以游客返回官网，不交接失效令牌',async({page})=>{
 const ui=new WebsitePurchaseEntryPage(page,origin);
 await page.route('**/__website/configuration',r=>r.fulfill({json:{origin}}));
 await page.route('**/api/v1/user/info',r=>r.fulfill({status:401,contentType:'text/plain',body:'Unauthorized'}));
 let posts=0;
 await page.route('**/__website/session',async r=>{
  expect(r.request().method()).toBe('POST');
  const form=new URLSearchParams(r.request().postData()||'');expect(form.get('accessToken')).toBe('');expect(form.get('returnTo')).toBe('/checkout');posts++;
  await r.fulfill({contentType:'text/html; charset=utf-8',body:'<h1>请先登录后结算</h1>'});
 });
 await ui.openSession('probe');
 await expect(page.getByRole('heading',{name:'请先登录后结算',exact:true})).toBeVisible();expect(posts).toBe(1);expect(page.context().pages()).toHaveLength(1);
 await ui.currentScreenshot('expired-probe');
});

test('TC-2i: 过期会话主动登录进入认证页并保留官网返回地址',async({page})=>{
 const ui=new WebsitePurchaseEntryPage(page,origin);
 await page.route('**/__website/configuration',r=>r.fulfill({json:{origin}}));
 await page.route('**/api/v1/user/info',r=>r.fulfill({status:401,contentType:'text/plain',body:'Unauthorized'}));
 await ui.openSession('login');await expect(page.getByRole('heading',{name:'登录客户中心',exact:true})).toBeVisible();
 expect(await page.evaluate(()=>sessionStorage.getItem('lina.website.return'))).toBe(origin+'/checkout');
 expect(new URL(page.url()).searchParams.get('redirect')).toBe(encodeURIComponent('/account/website-purchase'));
 expect(page.context().pages()).toHaveLength(1);await ui.currentScreenshot('expired-login');
});

test('TC-2j: 会话检查网络失败保留登录状态，重试成功才交接',async({page})=>{
 const ui=new WebsitePurchaseEntryPage(page,origin);let failed=true,posts=0;
 await page.route('**/__website/configuration',r=>r.fulfill({json:{origin}}));
 await page.route('**/api/v1/user/info',r=>r.fulfill({status:failed?503:200,json:failed?{code:50}:{code:0,data:{username:'fixture'}}}));
 await page.route('**/__website/session',async r=>{expect(new URLSearchParams(r.request().postData()||'').get('accessToken')).toBe('fixture-session-token');posts++;await r.fulfill({contentType:'text/html; charset=utf-8',body:'<h1>已返回官网确认订单</h1>'});});
 await ui.openSession('probe');await expect(page.getByText('官网暂时不可用，请稍后重试。',{exact:true})).toBeVisible();expect(posts).toBe(0);await ui.currentScreenshot('session-unavailable');
 failed=false;await ui.retry();await expect(page.getByRole('heading',{name:'已返回官网确认订单',exact:true})).toBeVisible();expect(posts).toBe(1);
});
