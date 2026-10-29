// The UI fixture exercises real forms without changing the running site's public or historical settings.
import {test,expect} from '../../../../../../hack/tests/fixtures/auth';
import {MainLayout} from '../../../../../../hack/tests/pages/MainLayout';
import {PortalBrandPage} from '../pages/PortalBrandPage';
import {installWebsiteMessages} from '../support/website-i18n';

const commonFields=[
 ['siteName','网站名称','Website name','Fixture website'],['siteTitle','网站标题','Website title','Fixture title'],['companyName','公司名称','Company name','Fixture company'],['contactPhone','联系电话','Contact phone','4000000000'],['contactEmail','联系邮箱','Contact email','fixture@example.test'],['description','网站简介','Website description','Fixture description'],['logoUrl','默认 Logo 图片地址','Default logo image URL','/legacy.png'],['headerLogoUrl','顶部 Logo 图片地址','Header logo image URL','/header.png'],['footerLogoUrl','底部 Logo 图片地址','Footer logo image URL','/footer.png'],['copyright','版权信息','Copyright text','© Fixture'],['businessQq','商务 QQ','Business QQ','123456'],['companyAddress','公司地址','Company address','Fixture park'],['enterpriseWechatQrCodeUrl','企业微信二维码图片地址','Enterprise WeChat QR code image URL','/enterprise.png'],['wechatOfficialQrCodeUrl','微信公众号二维码图片地址','WeChat official account QR code image URL','/official.png'],['icpNumber','网站备案号','Website ICP registration number','ICP fixture'],['icpUrl','网站备案查询链接','Website ICP registration link','https://records.example/icp'],['policeNumber','公安备案号','Public security registration number','Police fixture'],['policeUrl','公安备案查询链接','Public security registration link','https://records.example/police'],['telecomLicenseNumber','增值电信业务经营许可证号','Value-added telecom license number','Telecom fixture'],['telecomLicenseUrl','增值电信业务经营许可证查询链接','Value-added telecom license lookup URL','https://records.example/telecom'],
] as const;
const templateFields=[['herotitle','首页标题','Home title'],['herodescription','首页说明','Home description'],['heroimage','首页图片地址','Home image URL'],['hero1href','首页第一张按钮地址','First home slide button URL'],['relatedproductselectricity','电商方案关联商品','Related products for E-commerce solution'],['relatedproductsweb','网站方案关联商品','Related products for Website solution'],['relatedproductsgame','游戏方案关联商品','Related products for Gaming solution'],['relatedproductsgoingsea','出海方案关联商品','Related products for Going global solution'],['relatedproductsfinancial','金融方案关联商品','Related products for Finance solution'],['relatedproductsgovernment','政企方案关联商品','Related products for Government and enterprise solution'],['relatedproductssafety','安全方案关联商品','Related products for Security solution']] as const;
async function fixture(page:any,stale=false){
 await installWebsiteMessages(page);
 const fields=templateFields.map(([key,zhCN,enUS])=>({key,type:key.startsWith('related')?'multi_line':'single_line',maximumLength:8192,label:{zhCN,enUS}}));
 const state:any={selectedPlugin:stale?'retired-template':'website-main',common:Object.fromEntries(commonFields.map(([key,,,value])=>[key,value])),config:{},configurations:{'website-old':{herotitle:'Old independent template'},'website-main':{}},plugins:[{pluginId:'website-main',name:'官网主模板',pages:[{id:'home',nameZh:'首页',nameEn:'Home',path:'/'},{id:'products',nameZh:'商品列表',nameEn:'Products',path:'/products.html'}],schema:{version:'v1',fields}},{pluginId:'website-old',name:'旧独立模板',pages:[],schema:{version:'v1',fields:[fields[0]]}}],navigation:{header:[{id:'home',parentId:'',nameZh:'已有首页',nameEn:'Existing home',pageId:'home',url:'',sortOrder:0}],footer:[]}};
 const writes:any[]=[];
 await page.route('**/x/idc-portal/api/v1/admin/site/configuration',async(route:any)=>{
  if(route.request().method()==='PUT'){const input=route.request().postDataJSON();writes.push(input);Object.assign(state,input);state.configurations[input.selectedPlugin]=structuredClone(input.config);}
  await route.fulfill({json:{code:0,data:state}});
 });
 return {state,writes};
}
test.describe('TC-4 官网公共配置完整接入',()=>{
 test('TC-4a: 陈旧模板恢复、全部公共信息与11项模板配置保存回读、头尾导航增改删',async({adminPage},info)=>{
  const {state,writes}=await fixture(adminPage,true),portal=new PortalBrandPage(adminPage);await portal.goto();
  await expect(portal.navigation).toBeVisible();await expect(portal.navigation).toContainText('已有首页');await expect(portal.navigation.getByRole('button',{name:/添\s*加\s*导\s*航/})).toBeDisabled();
  await portal.selectPlugin('官网主模板');await expect(portal.navigation.getByRole('button',{name:/添\s*加\s*导\s*航/})).toBeEnabled();await portal.screenshot(info.outputPath(Date.now()+'-website-current-template.png'));
  for(const [,zh,,value] of commonFields)await expect(portal.field(zh)).toHaveValue(value);
  for(const [key,zh] of templateFields)await portal.field(zh).fill(key==='heroimage'?'/fixture-hero.png':key==='hero1href'?'/products.html':key.startsWith('related')?'/configure.html?product=fixture-'+key:'Fixture '+key);
  await portal.field('顶部 Logo 图片地址').fill('/updated-header.png');
  await portal.addNavigation();await portal.editor.getByLabel('中文名称',{exact:true}).fill('临时产品');await portal.editor.getByLabel('英文名称',{exact:true}).fill('Temporary products');await portal.destination('跳转页面','商品列表 (/products.html)');await portal.editor.getByRole('spinbutton').fill('7');await portal.confirmNavigation();
  await expect(portal.navigation).toContainText('临时产品');await portal.editNavigation('临时产品');await portal.editor.getByLabel('中文名称',{exact:true}).fill('修改后产品');await portal.confirmNavigation();
  await portal.navigationPosition('底部导航');await portal.addNavigation();await portal.editor.getByLabel('中文名称',{exact:true}).fill('临时底部');await portal.editor.getByLabel('英文名称',{exact:true}).fill('Temporary footer');await portal.confirmNavigation();
  await portal.save();expect(writes).toHaveLength(1);expect(writes[0].common.headerLogoUrl).toBe('/updated-header.png');expect(writes[0].common.logoUrl).toBe('/legacy.png');expect(Object.keys(writes[0].common)).toHaveLength(20);expect(Object.keys(writes[0].config)).toHaveLength(11);expect(writes[0].navigation.header.find((r:any)=>r.nameZh==='修改后产品').sortOrder).toBe(7);expect(writes[0].navigation.footer).toHaveLength(1);expect(state.configurations['website-old']).toEqual({herotitle:'Old independent template'});
  await portal.button('刷新').click();await expect(portal.field('顶部 Logo 图片地址')).toHaveValue('/updated-header.png');await expect(portal.field('安全方案关联商品')).toHaveValue('/configure.html?product=fixture-relatedproductssafety');await expect(portal.root).not.toContainText('plugin.idc-portal.');await portal.field('顶部 Logo 图片地址').scrollIntoViewIfNeeded();await portal.screenshot(info.outputPath(Date.now()+'-website-complete-saved.png'));
  await portal.navigationPosition('底部导航');await portal.removeNavigation('临时底部');await portal.save();expect(writes.at(-1).navigation.footer).toEqual([]);
  await portal.selectPlugin('旧独立模板');await expect(portal.field('首页标题')).toHaveValue('Old independent template');await expect(portal.field('顶部 Logo 图片地址')).toHaveValue('/updated-header.png');await portal.selectPlugin('官网主模板');await expect(portal.field('首页标题')).toHaveValue('Fixture herotitle');
 });
 test('TC-4b: 英文公共字段、模板字段与导航页面候选正确翻译，窄屏表单可操作',async({adminPage},info)=>{
  await fixture(adminPage);const portal=new PortalBrandPage(adminPage);await portal.goto();await new MainLayout(adminPage).switchLanguage('English');await portal.goto();
  await expect(portal.root.getByRole('heading',{name:'Website navigation',exact:true})).toBeVisible();
  for(const [,,en,value] of commonFields)await expect(portal.field(en)).toHaveValue(value);
  for(const [,,en] of templateFields)await expect(portal.field(en)).toBeVisible();
  await expect(portal.navigation).toContainText('Existing home');await portal.addNavigation('Add link');await expect(portal.editor.getByLabel('Chinese name',{exact:true})).toBeVisible();await portal.destination('Destination','Products (/products.html)');await expect(portal.editor).not.toContainText('plugin.idc-portal.');await portal.screenshot(info.outputPath(Date.now()+'-website-navigation-en.png'));
  await portal.editor.getByLabel('Chinese name',{exact:true}).fill('英文导航');await portal.editor.getByLabel('English name',{exact:true}).fill('English navigation');await portal.confirmNavigation('Confirm');
  await adminPage.setViewportSize({width:390,height:844});await portal.field('Header logo image URL').fill('/mobile-header.png');await expect(portal.field('Header logo image URL')).toHaveValue('/mobile-header.png');await portal.field('Header logo image URL').scrollIntoViewIfNeeded();await expect(portal.field('Header logo image URL')).toBeInViewport();await portal.screenshot(info.outputPath(Date.now()+'-website-common-mobile-en.png'));
 });
});
