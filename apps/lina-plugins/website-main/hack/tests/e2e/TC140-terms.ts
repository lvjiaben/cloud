import {test,expect} from '../support/source-site';
import {NavigationArrowsPage} from '../pages/NavigationArrowsPage';
test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,headless:true});
const headings=['适用范围','账号与信息安全','服务开通与使用','费用与续费','用户内容与合规责任','服务维护与中断','暂停与终止','数据备份与删除','知识产权','责任边界','条款变更','联系方式'];
test.describe('TC-140 服务条款',()=>{
 for(const mobile of [false,true])test(mobile?'TC-140b 手机说明入口、锚点与刷新':'TC-140a 桌面标题入口、锚点与刷新',async({page},info)=>{
  test.setTimeout(90000);if(mobile)await page.setViewportSize({width:390,height:844});
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand:{siteName:{zhCN:'本方云服务'},navigation:[{label:'支持与服务',children:[{label:'服务条款',href:'/404.html'}]}]}}}}));
  const nav=new NavigationArrowsPage(page);await nav.open('/');await nav.toggle('支持与服务');
  const entry=page.locator('[data-menu="支持与服务"] a.Submenu-item').filter({hasText:'服务条款'});
  await expect(entry).toHaveAttribute('href',/\/terms\.html$/);
  await expect(entry.locator('.Submenu-desc')).toHaveText('查看服务范围、使用规则与相关条款。');
  await entry.locator(mobile?'.Submenu-desc':'.Submenu-title').click();await expect(page).toHaveURL(/\/terms\.html$/);
  await expect(page.locator('.terms-hero h1')).toHaveText('服务条款');
  await expect(page).toHaveTitle('服务条款 - 本方云服务');
  await expect(page.locator('.nav-layoutcontainer')).toHaveCount(1);
  await expect(page.locator('.sky-footer-container')).toHaveCount(1);
  expect(await page.locator('.terms-page').evaluate(e=>Boolean(e.compareDocumentPosition(document.querySelector('.sky-footer-container')!)&Node.DOCUMENT_POSITION_FOLLOWING))).toBe(true);
  await expect(page.locator('.terms-document h2')).toHaveText(headings.map((h,i)=>`${i+1}. ${h}`));
  await expect(page.locator('.terms-document')).toContainText('本页不另行规定统一退款比例');
  await expect(page.locator('.terms-document')).toContainText('本页不作统一保留期限或数据可恢复承诺');
  await expect(page.locator('.terms-contact')).toHaveAttribute('href','/contact.html');
  await page.screenshot({path:info.outputPath('terms-initial.png')});
  const links=page.locator('.terms-directory a');
  for(let i=0;i<12;i++){
   const hash=await links.nth(i).getAttribute('href');await links.nth(i).click();await expect(page).toHaveURL(new RegExp(hash+'$'));
   const section=page.locator(hash!);await expect(section).toBeInViewport();
   expect(await section.evaluate(e=>e.getBoundingClientRect().top>=60)).toBe(true);
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  }
  await page.screenshot({path:info.outputPath('terms-contact.png')});
  expect((await page.reload())?.status()).toBe(200);await expect(page.locator('.terms-document section')).toHaveCount(12);
  await page.locator('.terms-contact').click();await expect(page).toHaveURL(/\/contact\.html$/);
  expect(errors).toEqual([]);
 });
 test('TC-140c 公开资料失败不抹去条款正文',async({page},info)=>{
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({status:503,json:{code:50,data:null}}));
  expect((await page.goto('/terms.html'))?.status()).toBe(200);
  await expect(page.locator('[data-brand-retry]')).toBeVisible();
  await expect(page.locator('.terms-document h2')).toHaveText(headings.map((h,i)=>`${i+1}. ${h}`));
  await expect(page.locator('.terms-contact')).toBeVisible();
  await page.screenshot({path:info.outputPath('terms-brand-failure.png')});
 });
});
