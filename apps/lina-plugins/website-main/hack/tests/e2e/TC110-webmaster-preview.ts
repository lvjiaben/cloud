import {test,expect} from '../support/source-site';
import {test as liveTest} from '@playwright/test';
import {WebmasterPage} from '../pages/WebmasterPage';
test.use({baseURL:'http://localhost:6680'});
test.describe('TC-110 百站赞助公益计划',()=>{
test('TC110a 桌面入口、刷新、原文案、联系入口与本地图片',async({page},info)=>{
 const site=new WebmasterPage(page),errors:string[]=[],writes:string[]=[];
 page.on('pageerror',error=>errors.push(error.message));page.on('request',request=>{if(!['GET','HEAD'].includes(request.method()))writes.push(request.url());});
 expect((await site.open())?.status()).toBe(200);
 await expect(page.locator('.wm-hero h1')).toHaveText('百站赞助公益计划');
 await expect(page.locator('.wm-hero p')).toHaveText('我们致力于通过赞助百个精选站点的服务器资源，推动科技前沿探索，促进知识共享和网络创新，共建一个更开放、多元的互联网生态。');
 await expect(page.locator('.wm-preview,.wm-disclaimer')).toHaveCount(0);
 await expect(page.locator('.wm-card h2')).toHaveText(['技术支持与协作','资源共享网络','创新激励计划']);
 await expect(page.locator('.wm-card p')).toHaveText([
  '为赞助站点提供全面的技术支持和优化建议，确保网站运行稳定，加速内容加载，促进网站与用户间的高效互动。',
  '建立资源共享平台，允许赞助站点之间互通有无，分享流量、内容与数据，激发网络社群的协同增长。',
  '通过定期的技术创新激励计划，鼓励站点开发新功能、探索新技术，不断提升网站价值和用户体验。',
 ]);
 await site.checkCardHover();
 await site.openNavigation();const entry=page.locator('.horizontal-menu-navbar-menu a[href$="/webmaster.html"]');await expect(entry).toHaveCount(1);await entry.click();const refreshed=await page.reload();expect(refreshed?.status()).toBe(200);await expect(page.locator('.wm-hero h1')).toHaveText('百站赞助公益计划');await expect(page.locator('.wm-voucher')).toHaveCount(2);
 const plans=page.locator('.wm-voucher');await expect(plans).toHaveCount(2);
 await expect(page.locator('.wm-application-header p')).toHaveText('确认申请的项目满足赞助要求，并准备项目内容说明和流量情况报表。');
 await expect(page.locator('.wm-requirement h3')).toHaveText(['资格条件','所需材料','申请流程']);
 await expect(page.locator('.wm-requirement p')).toHaveText([
  '注册账户并完成实名认证，确保申请者是网站或项目的正式拥有者。实名认证旨在验证身份，保障赞助流程的公正性和透明度。',
  '请通过官方渠道提交申请工单或联系客服。申请中需要包括网站详情、目标用户群以及预期使用赞助资源的计划。',
  '提交申请后，我们将进行审核。一旦审核通过，客服将与您确认具体的赞助详情，并将赞助资源直接分配到您的账户中。',
 ]);
 await expect(page.locator('.wm-plan-label')).toHaveCount(0);
 await expect(page.locator('.wm-plan-copy a')).toHaveCount(2);
 for(const link of await page.locator('.wm-plan-copy a').all())await expect(link).toHaveAttribute('href','/contact.html');
 const expectedPlans=[
  {title:'通用型服务器赞助',configuration:'2核2G/50G数据盘/3M带宽'},
  {title:'计算型服务器赞助',configuration:'4核4G/50G数据盘/5M带宽'},
 ];
 for(let index=0;index<expectedPlans.length;index++){
  const plan=plans.nth(index),expected=expectedPlans[index],details=plan.locator('.wm-plan-details p');
  await expect(plan.locator('.wm-plan-copy h3')).toHaveText(expected.title);
  await expect(plan.locator('.wm-plan-copy p')).toHaveText(expected.configuration);
  await expect(plan.locator('.wm-duration strong')).toHaveText('365 天');
  await expect(plan.locator('.wm-duration p')).toHaveText('到期需重新申请');
  await expect(details.nth(0).locator('span').nth(1)).toHaveText('海外服务器，国内服务器（特殊商品除外）');
  await expect(details.nth(1).locator('span').nth(1)).toHaveText('自领取之日起，1年内有效。');
 }
 await expect(page.locator('.wm-rules-panel h3')).toContainText('赞助公益计划规则');
 await expect(page.locator('.wm-rules-panel ol>li')).toHaveCount(7);
 await expect(page.locator('.wm-rules-panel ol>li').nth(3).locator('ul>li')).toHaveCount(4);
 await expect(page.locator('.wm-rules-panel ol>li').nth(0)).toContainText('申请的项目必须是互联网方面的公益或半公益项目，且不违反《中华人民共和国法律》及我司相关规定；');
 await expect(page.locator('.wm-rules-panel ol>li').nth(1)).toContainText('项目已正常运行半年或以上，具有一定网络流量，例如网站项目权重、GitHub项目的Star数量、同类项目中知名度较高；');
 await expect(page.locator('.wm-rules-panel ol>li').nth(2)).toContainText('申请方应仔细阅读并确保己完全理解并同意遵守我司服务的各项内容，包括但不限于协议、规则和使用规范等，否则请勿申请我司相关服务;');
 await expect(page.locator('.wm-rules-panel ol>li').nth(3)).toContainText('申请方可能存在无法申请到相应支持的情况，包括但不限于：');
 await expect(page.locator('.wm-rules-panel ol>li').nth(3).locator('ul>li')).toHaveText([
  '申请方未按要求完成注册和实名认证、或账户存在安全风险;',
  '申请方申请的赞助配置与项目所能提供的流量差距较大；',
  '在申请和使用服务过程中，有任何违反诚实信用、公平公正等原则;',
  '公益计划可供赞助资源不足;',
 ]);
 await expect(page.locator('.wm-rules-panel ol>li').nth(4)).toContainText('申请方如获得赞助，不得转赠他人或用于与赞助项目无关的业务（特殊情况请说明报备），否则我司将立即暂停该服务内容；');
 await expect(page.locator('.wm-rules-panel ol>li').nth(5)).toContainText('因不可抗力原因（包括但不限于重大自然灾害事件、政府主管部门指令停止或调整、平台遭受严重网络攻击或社会公共性系统故障、公司业务重大调整等）导致平台服务无法继续开展的，我司有权单方面决定暂停或终止服务。');
 await expect(page.locator('.wm-rules-panel ol>li').nth(6)).toContainText('因项目特殊性，需要取得相关资质证书的，需在申请材料中提交相关资质证明文件；');
 await expect(page.locator('#wm-cases>.wm-heading p')).toHaveText('为站长提供服务器赞助，支持内容创新和建设，促进信息共享和交流。');
 await expect(page.locator('.wm-case')).toHaveCount(0);
 expect(await site.images()).toBe(true);expect(await site.overflow()).toBe(false);
 await expect(page.locator('.sky-footer-container')).toBeVisible();
 await site.screenshot(info.outputPath('webmaster-desktop.png'));
 await expect(page.locator('.wm-application-header a')).toHaveAttribute('href','/contact.html');
 await site.switchLanguage();await expect(page.locator('.wm-hero h1')).toHaveText('Website sponsorship program');
 await expect(plans.nth(0).locator('.wm-plan-copy p')).toHaveText('2 vCPU/2 GB RAM/50 GB data disk/3 Mbps bandwidth');
 await expect(plans.nth(0).locator('.wm-duration strong')).toHaveText('365 days');
 await expect(plans.nth(0).locator('.wm-duration p')).toHaveText('Reapplication required upon expiration');
 await expect(page.locator('.wm-application-header a')).toHaveAttribute('href','/contact.html');
 expect(errors).toEqual([]);expect(writes).toEqual([]);
});
test('TC110b 手机与减少动态效果',async({page},info)=>{
 await page.setViewportSize({width:390,height:844});await page.emulateMedia({reducedMotion:'reduce'});const site=new WebmasterPage(page);await site.open();
 await expect(page.locator('.wm-hero h1')).toBeVisible();expect(await site.overflow()).toBe(false);expect(await site.images()).toBe(true);
 await expect(page.locator('.wm-voucher')).toHaveCount(2);await expect(page.locator('.wm-duration strong').first()).toHaveText('365 天');
 await expect(page.locator('.wm-section').first()).toHaveCSS('opacity','1');await expect(page.locator('.wm-section').first()).toHaveCSS('transition-duration','0s');
 await site.screenshot(info.outputPath('webmaster-mobile.png'));
 await expect(page.locator('.wm-plan-copy a').first()).toHaveAttribute('href','/contact.html');
});

liveTest('TC-110c: 申请说明立即申请进入现有联系页面',async({page},info)=>{
 const site=new WebmasterPage(page);const response=await page.goto('http://127.0.0.1:6680/webmaster.html');expect(response?.status()).toBe(200);const refreshed=await page.reload();expect(refreshed?.status()).toBe(200);
 await expect(page.locator('.wm-hero h1')).toHaveText('百站赞助公益计划');
 await expect(page.locator('.wm-hero p')).toHaveText('我们致力于通过赞助百个精选站点的服务器资源，推动科技前沿探索，促进知识共享和网络创新，共建一个更开放、多元的互联网生态。');
 await expect(page.locator('.wm-preview,.wm-disclaimer,.wm-plan-label')).toHaveCount(0);
 await expect(page.locator('.wm-card .wm-accent')).toHaveText(['支持','共享','创新']);
 await expect(page.locator('.wm-card h2')).toHaveText(['技术支持与协作','资源共享网络','创新激励计划']);
 await expect(page.locator('.wm-card p')).toHaveText([
  '为赞助站点提供全面的技术支持和优化建议，确保网站运行稳定，加速内容加载，促进网站与用户间的高效互动。',
  '建立资源共享平台，允许赞助站点之间互通有无，分享流量、内容与数据，激发网络社群的协同增长。',
  '通过定期的技术创新激励计划，鼓励站点开发新功能、探索新技术，不断提升网站价值和用户体验。',
 ]);
 await expect(page.locator('.wm-application-header h3')).toHaveText('如何申请百站赞助公益计划？');
 await expect(page.locator('.wm-application-header p')).toHaveText('确认申请的项目满足赞助要求，并准备项目内容说明和流量情况报表。');
 await expect(page.locator('.wm-requirement h3')).toHaveText(['资格条件','所需材料','申请流程']);
 await expect(page.locator('.wm-requirement p')).toHaveText([
  '注册账户并完成实名认证，确保申请者是网站或项目的正式拥有者。实名认证旨在验证身份，保障赞助流程的公正性和透明度。',
  '请通过官方渠道提交申请工单或联系客服。申请中需要包括网站详情、目标用户群以及预期使用赞助资源的计划。',
  '提交申请后，我们将进行审核。一旦审核通过，客服将与您确认具体的赞助详情，并将赞助资源直接分配到您的账户中。',
 ]);
 const plans=page.locator('.wm-voucher');await expect(plans).toHaveCount(2);
 await expect(plans.nth(0).locator('.wm-plan-copy p')).toHaveText('2核2G/50G数据盘/3M带宽');
 await expect(plans.nth(1).locator('.wm-plan-copy p')).toHaveText('4核4G/50G数据盘/5M带宽');
 for(const plan of await plans.all()){
  await expect(plan.locator('.wm-duration strong')).toHaveText('365 天');
  await expect(plan.locator('.wm-duration p')).toHaveText('到期需重新申请');
  await expect(plan.locator('.wm-plan-details')).toContainText('海外服务器，国内服务器（特殊商品除外）');
  await expect(plan.locator('.wm-plan-details')).toContainText('自领取之日起，1年内有效。');
 }
 await expect(page.locator('.wm-rules-panel h3')).toContainText('赞助公益计划规则');
 await expect(page.locator('.wm-rules-panel ol>li')).toHaveCount(7);
 await expect(page.locator('.wm-rules-panel ol>li').nth(0)).toHaveText('申请的项目必须是互联网方面的公益或半公益项目，且不违反《中华人民共和国法律》及我司相关规定；');
 await expect(page.locator('.wm-rules-panel ol>li').nth(1)).toHaveText('项目已正常运行半年或以上，具有一定网络流量，例如网站项目权重、GitHub项目的Star数量、同类项目中知名度较高；');
 await expect(page.locator('.wm-rules-panel ol>li').nth(2)).toHaveText('申请方应仔细阅读并确保己完全理解并同意遵守我司服务的各项内容，包括但不限于协议、规则和使用规范等，否则请勿申请我司相关服务;');
 await expect(page.locator('.wm-rules-panel ol>li').nth(3)).toContainText('申请方可能存在无法申请到相应支持的情况，包括但不限于：');
 await expect(page.locator('.wm-rules-panel ol>li').nth(3).locator('ul>li')).toHaveText([
  '申请方未按要求完成注册和实名认证、或账户存在安全风险;',
  '申请方申请的赞助配置与项目所能提供的流量差距较大；',
  '在申请和使用服务过程中，有任何违反诚实信用、公平公正等原则;',
  '公益计划可供赞助资源不足;',
 ]);
 await expect(page.locator('.wm-rules-panel ol>li').nth(4)).toHaveText('申请方如获得赞助，不得转赠他人或用于与赞助项目无关的业务（特殊情况请说明报备），否则我司将立即暂停该服务内容；');
 await expect(page.locator('.wm-rules-panel ol>li').nth(5)).toHaveText('因不可抗力原因（包括但不限于重大自然灾害事件、政府主管部门指令停止或调整、平台遭受严重网络攻击或社会公共性系统故障、公司业务重大调整等）导致平台服务无法继续开展的，我司有权单方面决定暂停或终止服务。');
 await expect(page.locator('.wm-rules-panel ol>li').nth(6)).toHaveText('因项目特殊性，需要取得相关资质证书的，需在申请材料中提交相关资质证明文件；');
 await expect(page.locator('#wm-cases>.wm-heading p')).toHaveText('为站长提供服务器赞助，支持内容创新和建设，促进信息共享和交流。');
 await expect(page.locator('.wm-case')).toHaveCount(0);
 await site.revealAll();await page.screenshot({path:info.outputPath('webmaster-live-6680.png'),fullPage:true,animations:'disabled'});
 await page.setViewportSize({width:390,height:844});await expect(page.locator('.wm-server-art').first()).toHaveCSS('display','none');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);await expect(page.locator('.wm-duration strong').first()).toHaveText('365 天');await page.screenshot({path:info.outputPath('webmaster-live-6680-mobile.png'),fullPage:true,animations:'disabled'});
 await page.setViewportSize({width:1440,height:1000});
 await expect(page.locator('.wm-application-header a')).toHaveText('立即申请');
 await expect(page.locator('.wm-application-header a')).toHaveAttribute('href','/contact.html');
 await site.contact();await expect(page).toHaveURL('http://127.0.0.1:6680/contact.html');
 await expect(page.locator('.currency-title-a').first()).toHaveText('联系我们');
 await expect(page.locator('[data-content-contact="email"]')).toBeVisible({timeout:15000});
 await expect(page.locator('[data-content-contact="email"] a')).toHaveAttribute('href',/^mailto:.+@.+/);
 await page.screenshot({path:info.outputPath('contact-from-webmaster.png')});
});

});
