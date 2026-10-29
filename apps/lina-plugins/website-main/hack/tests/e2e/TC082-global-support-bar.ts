import {expect,test} from '../support/source-site';
import {SupportBarPage} from '../pages/SupportBarPage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},headless:true,reducedMotion:'reduce'});
const brand={contactPhone:'4001234567',businessQq:'12345678',enterpriseWechatQrCodeUrl:'/static/picture/21.svg',wechatOfficialQrCodeUrl:'/static/picture/22.svg'};
test.describe('TC-82 官网全局客服栏',()=>{
 test.beforeEach(async({page})=>{await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand}}}));});
 test('TC-82a: 全局悬停、完整工单入口、联系方式、二维码与回顶',async({page},info)=>{
  const site=new SupportBarPage(page);await site.open();await expect(site.bar).toBeVisible();await site.capture(info.outputPath('initial.png'));
  await site.hover('工单与反馈');for(const name of ['提交工单','建议反馈','违法举报'])await expect(site.bar.getByRole('link',{name:new RegExp(name)})).toHaveAttribute('href','http://localhost:6032/account/support/new');
  await site.capture(info.outputPath('work.png'));await site.leave();await expect(site.trigger('工单与反馈')).toHaveAttribute('aria-expanded','false');
  await site.hover('联系客服');await expect(site.bar.getByRole('link',{name:/服务热线/})).toHaveAttribute('href','tel:4001234567');await expect(site.bar.getByRole('link',{name:/商务 QQ/})).toHaveAttribute('href',/uin=12345678/);
  await site.hover('微信二维码');await expect(site.bar.getByAltText('企业微信二维码')).toBeVisible();await expect(site.bar.getByAltText('微信公众号二维码')).toBeVisible();await site.capture(info.outputPath('qr.png'));
  await site.open('/help.html');await expect(site.bar).toBeVisible();await site.open();await expect(site.bar).toBeVisible();await site.bottom();await expect(site.bar.getByRole('button',{name:'返回顶部'})).toBeVisible();await site.top();await expect.poll(()=>page.evaluate(()=>window.scrollY)).toBe(0);
  await site.keyboard('工单与反馈','Escape');await expect(site.trigger('工单与反馈')).toHaveAttribute('aria-expanded','false');
  await site.mobile();await site.click('工单与反馈');await expect(site.bar.getByRole('link',{name:/违法举报/})).toBeVisible();const rect=await site.bar.locator('.hoverWindow:visible').boundingBox();expect(rect!.x).toBeGreaterThanOrEqual(0);await site.capture(info.outputPath('mobile.png'));
 });
 test('TC-82b: 缺少客户中心及非法二维码不生成错误入口，资料重读不重复装配',async({page},info)=>{
  const site=new SupportBarPage(page);await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand:{enterpriseWechatQrCodeUrl:'javascript:alert(1)',businessQq:'bad'}}}}));
  await site.open('/contact.html');await expect(site.bar).toBeVisible();await page.evaluate(()=>{(window as any).__LINA_TEMPLATE__.customerUrl='';});
  await page.evaluate(async()=>{const {renderSupportBar}=await import('/lina/support-bar.js');renderSupportBar(document,{enterpriseWechatQrCodeUrl:'javascript:alert(1)'},()=> '');renderSupportBar(document,{enterpriseWechatQrCodeUrl:'javascript:alert(1)'},()=> '');});
  await expect(site.trigger('工单与反馈')).toHaveCount(0);await expect(site.bar).toHaveCount(1);await site.hover('微信二维码');await expect(site.bar.getByRole('link',{name:/暂未配置二维码/})).toHaveAttribute('href','/contact.html');await expect(site.bar.locator('img')).toHaveCount(0);await site.capture(info.outputPath('unconfigured.png'));
 });
});
