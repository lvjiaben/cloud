import {test,expect} from '../support/source-site';
import {ServerHeroPage} from '../pages/ServerHeroPage';
import {previewResponse,previewProducts} from '../support/home-preview-data.mjs';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-60 服务器轮播与公开目录同步',()=>{
 test('TC-60a: 前四项、切换、位置线、购买权限、定时更新、失败保留与下架',async({page},info)=>{
  let rows=structuredClone(previewProducts),failed=false,reads=0;
  rows[0].nameZhCN='华东高性能云服务器 E5-2683 v4 双路大内存型';rows[1].nameZhCN='香港·轻量型';
  rows[0].summaryZhCN='模拟服务器，仅供流程演示，不提供真实资源。\nCPU：Intel Xeon E5-2683 v4，4 vCPU；内存：8GB DDR4\n系统盘：80GB SSD；带宽：20Mbps；说明仅供了解。';
  await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());
   if(u.pathname.endsWith('/catalog/products')&&u.searchParams.get('limit')==='4'){reads++;if(failed)return route.fulfill({status:503,json:{code:503}});r.data={products:rows.slice(0,4)};}
   await route.fulfill({status:r.status,json:{code:0,data:r.data}});
  });
  const site=new ServerHeroPage(page);await site.installClock();await site.open('/');
  const titles=page.locator('.swipervertical-title'),active=page.locator('.swiper-text:visible .Title'),hero=page.locator('.Rotation-layoutcontainer');
  await expect(titles).toHaveText(rows.slice(0,4).map(p=>p.nameZhCN));await expect(hero).not.toContainText('成都西信');
  expect(await titles.evaluateAll(nodes=>new Set(nodes.map(n=>n.getBoundingClientRect().height)).size)).toBe(1);
  await page.setViewportSize({width:390,height:844});
  expect(await titles.evaluateAll(nodes=>new Set(nodes.map(n=>n.getBoundingClientRect().height)).size)).toBe(1);
  await expect(hero.locator('.swiper-text:visible .Describe')).toHaveText(rows[0].summaryZhCN);
  await page.setViewportSize({width:1440,height:900});
  await expect(hero.locator('.swiper-text:visible .Describe')).toHaveText(rows[0].summaryZhCN);
  await expect(hero.locator('.swiper-text:visible .Describe > *')).toHaveCount(0);
  expect(await hero.locator('.swiper-text:visible .Describe').textContent()).toBe(rows[0].summaryZhCN);
  await expect(hero.locator('.swiper-text:visible .Describe')).toHaveCSS('white-space','pre-wrap');
  await expect(hero.locator('.swiper-slide')).toHaveCount(4);await site.nextHero();await expect(active).toHaveText(rows[1].nameZhCN);
  const line=hero.locator('.active-title-line');expect(await line.evaluate(n=>parseFloat((n as HTMLElement).style.top))).toBe(await titles.nth(1).evaluate(n=>(n as HTMLElement).offsetTop));
  await expect(hero.locator('.swiper-text:visible .Button')).toHaveAttribute('href','/products.html?product='+rows[1].publicSlug);
  await site.heroByKeyboard(3);await site.nextHero();await expect(active).toHaveText(rows[0].nameZhCN);
  await site.previousHero();await expect(active).toHaveText(rows[3].nameZhCN);
  await hero.locator('.pagination-bullet').nth(1).click();await expect(active).toHaveText(rows[1].nameZhCN);await site.heroByKeyboard(3);
  await site.captureSection('.Rotation-layoutcontainer',info.outputPath('servers-loaded.png'));
  rows[3].nameZhCN='后台更新后的服务器';rows[3].summaryZhCN='后台更新后的说明';rows[3].availability='unavailable';
  await site.advance(30001);await expect(titles.nth(3)).toHaveText('后台更新后的服务器');await site.heroByKeyboard(3);await expect(active).toHaveText('后台更新后的服务器');await expect(hero.locator('.swiper-text:visible .Describe')).toHaveText('后台更新后的说明');
  await expect(hero.locator('.swiper-text:visible .Button')).not.toHaveAttribute('href');await expect(hero.locator('.swiper-text:visible .Button')).toHaveText('暂不可订购');
  rows=[rows[3],rows[1],rows[0],rows[2]];await site.refreshServers();await expect(titles).toHaveText(rows.map(p=>p.nameZhCN));await expect(active).toHaveText('后台更新后的服务器');
  failed=true;await site.refreshServers();await expect(hero.locator('.lina-server-status')).toContainText('暂时显示上次内容');await expect(active).toHaveText('后台更新后的服务器');
  await site.captureSection('.Rotation-layoutcontainer',info.outputPath('servers-retained.png'));
  failed=false;rows=[];await site.retryServers();await expect(hero).toContainText('暂无已发布的服务器。');await expect(titles).toHaveCount(0);await expect(hero.locator('a[href]')).toHaveCount(0);
  const before=reads;await site.hidePage(true);await site.advance(60000);expect(reads).toBe(before);await site.hidePage(false);await expect.poll(()=>reads).toBeGreaterThan(before);
 });
 test('TC-60b: 首次失败不展示原站商品、重试、真实淡入动画与五秒自动切换',async({page},info)=>{
  let failed=true;await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/catalog/products')&&u.searchParams.get('limit')==='4'){if(failed)return route.fulfill({status:503,json:{code:503}});r.data={products:previewProducts.slice(0,4)};}await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new ServerHeroPage(page);await site.installClock();await site.open('/');
  await expect(page.locator('.lina-server-status')).toContainText('服务器加载失败');await expect(page.locator('.swipervertical-title')).toHaveCount(0);
  failed=false;await site.retryServers();await expect(page.locator('.swipervertical-title')).toHaveCount(4);
  await site.motion('no-preference');await site.advance(5001);await expect(page.locator('.swiper-text:visible .Title')).toHaveText(previewProducts[1].nameZhCN);
  await expect(page.locator('.swiper-slide.active')).toHaveCSS('transition-duration','0.3s');await expect(page.locator('.swiper-text.active .swiper-text-container')).toHaveCSS('animation-name','bounce');
  await site.motion('reduce');await site.advance(5001);await expect(page.locator('.swiper-text:visible .Title')).toHaveText(previewProducts[2].nameZhCN);await expect(page.locator('.swiper-text.active .swiper-text-container')).toHaveCSS('animation-name','none');
  const hero=page.locator('.Rotation-layoutcontainer'),progress=hero.locator('.pagination-bullet-active');
  await expect(hero).toHaveAttribute('data-autoplay','running');
  expect(await progress.evaluate(n=>getComputedStyle(n,'::after').animationName)).toBe('bulletProgress');
  await site.hidePage(true);await expect(hero).toHaveAttribute('data-autoplay','paused');
  expect(await progress.evaluate(n=>getComputedStyle(n,'::after').animationName)).toBe('none');
  await site.advance(15000);await expect(page.locator('.swiper-text:visible .Title')).toHaveText(previewProducts[2].nameZhCN);
  await site.hidePage(false);await site.advance(4999);await expect(page.locator('.swiper-text:visible .Title')).toHaveText(previewProducts[2].nameZhCN);
  await site.advance(2);await expect(page.locator('.swiper-text:visible .Title')).toHaveText(previewProducts[3].nameZhCN);
  await site.nextHero();await expect(page.locator('.swiper-text:visible .Title')).toHaveText(previewProducts[0].nameZhCN);
  await site.advance(5001);await expect(page.locator('.swiper-text:visible .Title')).toHaveText(previewProducts[1].nameZhCN);
  await site.captureSection('.Rotation-layoutcontainer',info.outputPath('servers-motion.png'));
 });
 test.describe('真实官网网关',()=>{
 test.use({websiteGateway:true,baseURL:'http://127.0.0.1:6680'});
 test('TC-60c: 当前后台公开资料逐字映射，不写后台',async({page},info)=>{
  const response=await page.request.get('http://127.0.0.1:6030/x/idc-core/api/v1/public/catalog/products?limit=4');expect(response.ok()).toBe(true);
  const envelope=await response.json();expect(envelope.code).toBe(0);const rows=envelope.data.products;
  const site=new ServerHeroPage(page);await site.open('/');await expect(page.locator('.swipervertical-title')).toHaveText(rows.map(p=>p.nameZhCN||p.nameEnUS));
  if(rows.length){const summary=rows[0].summaryZhCN||rows[0].summaryEnUS||'';expect(await page.locator('.swiper-text:visible .Describe').textContent()).toBe(summary);await expect(page.locator('.swiper-text:visible .Describe')).toHaveCSS('white-space','pre-wrap');}
  await site.captureSection('.Rotation-layoutcontainer',info.outputPath('servers-real-backend.png'));
 });
 });
});
