import {test,expect} from '../support/source-site';
import {controlledBusiness} from '../support/controlled-business';
import {WebsitePage} from '../pages/SourceWebsitePage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},customerOrigin:'http://localhost:6032'});
test.describe('TC-28 非空首页导航资料与拒绝未授权写（模拟UI）',()=>{
 test('TC-28a: 首页资料卡、新闻悬停焦点、分组导航与跨页搜索',async({page})=>{
  const f=await controlledBusiness(page),site=new WebsitePage(page);
  f.state.brand={...f.state.brand,heroTitle:'测试专用首页标题',heroDescription:'测试专用首页说明',homepageBlocks:[{title:{zhCN:'测试业务卡'},description:{zhCN:'测试卡片说明'},href:'/products.html'}],contactEmail:'controlled@example.test',contactPhone:'000-TEST-ONLY',footer:{zhCN:'测试专用页脚'}};
  await site.open('/index.html');await expect(page.locator('[data-hero-title]')).toHaveText('测试专用首页标题');
  await expect(page.locator('[data-brand-contact] p').first()).toHaveCSS('color','rgb(255, 255, 255)');
  await expect(page.locator('[data-home-blocks]')).toContainText('测试业务卡');await expect(page.locator('[data-products]')).toContainText('测试专用甲');
  const news=page.locator('[data-home-news] > *');await expect(news).toHaveCount(2);await news.nth(1).hover();await expect(news.nth(1)).toHaveClass(/active/);await expect(news.first()).not.toHaveClass(/active/);
  await news.first().focus();await expect(news.first()).toHaveClass(/active/);await site.capture('controlled-home-nonempty');
  await page.locator('[data-categories] a').filter({hasText:'测试分类乙'}).click();
  await expect(page).toHaveURL(/index.html\?categoryNo=cat-b$/);
  await expect(page.locator('[data-products] > *')).toHaveCount(1);
  await expect(page.locator('[data-products]')).toContainText('测试专用缺货');
  await expect(page.locator('[data-categories] [aria-current=true]')).toHaveText('测试分类乙');
  const group=page.getByRole('button',{name:'测试产品',exact:true});await group.click();await expect(group).toHaveAttribute('aria-expanded','true');
  await page.getByRole('link',{name:'测试目录',exact:true}).click();await expect(page).toHaveURL(/products.html$/);
  await site.open('/contact.html');await expect(page.locator('[data-content-contact=email]')).toContainText('controlled@example.test');await expect(page.locator('[data-content-contact=email] a')).toHaveAttribute('href','mailto:controlled@example.test');
  await expect(page.locator('[data-content-contact=phone] a')).toHaveAttribute('href','tel:000-TEST-ONLY');await site.capture('controlled-contact-nonempty');
  await site.open('/about.html');await expect(page.locator('[data-content-body]')).toContainText('受控测试专用网站');await expect(page.locator('[data-status]')).toHaveText('公司介绍、资质和服务承诺尚无公开资料。');
  await site.searchNavigation('测试专用甲');await expect(page).toHaveURL(/products.html\?namePrefix=/);await expect(page.locator('[data-products] > *')).toHaveCount(1);
  expect(f.requests.every(r=>r.method==='GET')).toBe(true);
 });
 test('TC-28b: 支付、供应商及未声明业务写均被测试隔离层拒绝',async({page})=>{
  const f=await controlledBusiness(page);await page.goto('/products.html');
  for(const path of ['/x/idc-core/api/v1/account/commerce/payments','/x/idc-core/api/v1/account/servers/provision']){
   const result=await page.evaluate(async path=>{const response=await fetch(path,{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'});return {status:response.status,body:await response.json()};},path);
   expect(result.status).toBe(403);expect(result.body.code).toBe(403);
  }
  const denied=await page.evaluate(async()=>{try{await fetch('http://localhost:6032/x/idc-core/api/v1/account/commerce/orders',{method:'POST',body:'{}'});return false;}catch{return true;}});
  expect(denied).toBe(true);expect(f.requests.filter(r=>r.method==='POST')).toHaveLength(2);
 });
});
