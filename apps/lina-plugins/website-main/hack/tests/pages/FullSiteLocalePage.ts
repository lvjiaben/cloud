import {Page} from '@playwright/test';

const exactEnglishCopy:Record<string,string>={
 '公司名称':'Company name',
 '版权信息版权信息版权信息版权信息版权信息版权信息版权信息':'Copyright information',
 '地址地址地址':'Address',
 '电信增值业务许可证编号电信增值业务许可证编号':'Value-added telecommunications business license number',
 '公安备案编号公安备案编号公安备案编号公安备案编号':'Public security registration number',
 'ICP备案编号ICP备案编号ICP备案编号':'ICP registration number',
 '新闻标题':'News title',
 '知识库标题':'Knowledge base title',
 '知识库摘要知识库摘要知识库摘要知识库摘要知识库摘要':'Knowledge base summary',
 '公告标题':'Announcement title',
 '新闻摘要新闻摘要新闻摘要新闻摘要新闻摘要新闻摘要':'News summary',
 '公告摘要公告摘要公告摘要公告摘要':'Announcement summary',
};
function localizedExact(value:string|undefined,locale:string){
 if(!value||locale!=='en-US')return value;
 const trimmed=value.trim(),english=exactEnglishCopy[trimmed];
 return english?value.replace(trimmed,english):value;
}

// Reads the running website; only actual API string values exempt merchant copy.
export class FullSiteLocalePage {
 readonly businessCopy=new Set<string>();
 readonly errors:string[]=[];
 readonly failedResources:string[]=[];
 readonly writes:string[]=[];
 private brand:Record<string,any>|undefined;
 private pending:Promise<void>[]=[];
 constructor(readonly page:Page){
  page.on('pageerror',error=>this.errors.push(error.message));
  page.on('request',request=>{if(!['GET','HEAD'].includes(request.method())&&!/\/purchase-configuration\/(evaluate|quote)$/.test(new URL(request.url()).pathname))this.writes.push(request.method()+' '+request.url());});
  page.on('response',response=>{
   const url=new URL(response.url());
   if(url.pathname.startsWith('/x/')&&response.headers()['content-type']?.includes('json')){
    this.pending.push((async()=>{try{
     const payload=await response.json();
     if(url.pathname==='/x/idc-portal/api/v1/public/brand'&&payload?.data?.brand)this.brand=payload.data.brand;
     const collect=(value:unknown)=>{if(!value||typeof value!=='object')return;
      const object=value as Record<string,unknown>;
      for(const [key,item] of Object.entries(object)){
       if(typeof item!=='string'){collect(item);continue;}
       if(!/[\u3400-\u9fff]/.test(item))continue;
       const counterpart=key==='zhCN'?object.enUS:key.endsWith('ZhCN')?object[key.slice(0,-4)+'EnUS']:undefined;
       if(typeof counterpart==='string'&&counterpart.trim()&&counterpart!==item)continue;
       this.businessCopy.add(item.trim());
       for(const fragment of item.split(/[\r\n；;]+/))if(fragment.trim())this.businessCopy.add(fragment.trim());
      }
     };collect(payload);
    }catch{/* Non-JSON errors are recorded by HTTP status below. */}})());
   }
   if(response.status()>=400&&['script','stylesheet','image','font'].includes(response.request().resourceType())&&url.hostname==='localhost')this.failedResources.push(response.status()+' '+url.pathname);
  });
 }
 async open(path:string){
  const response=await this.page.goto(path);await this.page.bringToFront();
  await this.page.locator('[data-public-locale-switch]').waitFor({state:'attached'});
  await this.settle();return response?.status();
 }
 async settle(){await Promise.all(this.pending);await this.page.waitForTimeout(120);}
 async waitForBrand(){
  const phone=this.page.locator('[data-footer-detail="qq"] a');if(!await phone.count())return;
  const deadline=Date.now()+30000;while(!this.brand&&Date.now()<deadline)await this.page.waitForTimeout(100);
  if(!this.brand)throw new Error('The authoritative public brand response did not arrive');
  const expected=this.brand,locale=await this.page.locator('html').getAttribute('lang')||'zh-CN';
  const raw=(value:any)=>typeof value==='string'?value:locale==='en-US'?(value?.enUS||value?.zhCN||''):(value?.zhCN||value?.enUS||'');
  const footer=localizedExact(raw(expected.footer),locale),copyright=localizedExact(expected.copyright||'',locale),address=localizedExact(expected.companyAddress||'',locale);
  while(Date.now()<deadline){
   const state=await this.snapshot();
   const filings=state.filings.map(row=>row.text);
   const expectedFilings=['telecomLicense','publicSecurity','icp'].map(key=>localizedExact(typeof expected[key+'Number']==='string'?expected[key+'Number'].trim():'',locale));
   if(state.company===footer&&state.copyright===copyright&&state.address===address&&state.businessQq===(expected.businessQq||'')&&JSON.stringify(filings)===JSON.stringify(expectedFilings))return;
   await this.page.waitForTimeout(100);
  }
  throw new Error('The official brand response arrived, but its footer projection never settled');
 }
 async waitForPageData(path:string){
  if(path==='/cooperation/partner.html')await this.page.locator('[data-partner] .pagebanner-title').waitFor({state:'visible',timeout:20000});
  if(path.startsWith('/products.html?product=')){
   if(new URL(path,'http://localhost').searchParams.get('action')==='configureproduct'){
    await this.page.locator('[data-catalog-title]').waitFor({state:'visible',timeout:20000});
    await this.page.waitForFunction(()=>{const title=document.querySelector('[data-catalog-title]')?.textContent?.trim();return Boolean(title&&title!=='商品选购');},null,{timeout:20000});
   }else await this.page.locator('[data-product]').first().waitFor({state:'visible',timeout:20000});
  }
  if(path.startsWith('/configure.html?product='))await this.page.locator('#addCartForm input,#addCartForm select,#addCartForm textarea').first().waitFor({state:'attached',timeout:20000});
  if(path.startsWith('/content.html?id=cnt_'))await this.page.locator('[data-content-body]').waitFor({state:'visible',timeout:20000});
 }
 async language(locale:'zh-CN'|'en-US'){
  if(await this.page.locator('html').getAttribute('lang')===locale)return;
  const toggle=this.page.locator('[data-public-locale-switch]');
  if(!await toggle.isVisible())await this.page.locator('.lina-menu-toggle').click();
  await toggle.click();await this.settle();
  const menu=this.page.locator('.lina-menu-toggle');
  if(await menu.isVisible()&&await menu.getAttribute('aria-expanded')==='true')await menu.click();
 }
 async reload(){await this.page.reload();await this.settle();}
 async sibling(path:string){
  const page=await this.page.context().newPage();
  await page.route('**/*',route=>['GET','HEAD'].includes(route.request().method())?route.continue():route.abort('blockedbyclient'));
  const site=new FullSiteLocalePage(page);await site.open(path);return site;
 }
 async desktopMenuFits(){return this.page.locator('.horizontal-menu-navbar-menu').evaluate(menu=>{
  const right=menu.parentElement!.getBoundingClientRect().right;
  return [...menu.querySelectorAll(':scope>li>a')].every(link=>link.getBoundingClientRect().right<=right+1);
 });}
 async mobileHomeFits(){return this.page.evaluate(()=>{
  const bounds=(selector:string)=>{const el=document.querySelector(selector);if(!el)return null;const rect=el.getBoundingClientRect();return {left:rect.left,right:rect.right,top:rect.top,bottom:rect.bottom,width:rect.width,height:rect.height,lineHeight:Number.parseFloat(getComputedStyle(el).lineHeight)||0,text:el.textContent?.trim()||''};};
  return {title:bounds('.lina-home-carousel .swiper-text.active .Title'),description:bounds('.lina-home-carousel .swiper-text.active .Describe'),tabs:bounds('.lina-home-carousel .swipervertical-container'),arrows:[...document.querySelectorAll('.lina-home-carousel .home-button-prev,.lina-home-carousel .home-button-next')].filter(el=>el.getClientRects().length&&getComputedStyle(el).visibility!=='hidden').length,footer:[...document.querySelectorAll('.sky-footer-top .sky-footer-title')].map(el=>{const rect=el.getBoundingClientRect();return {width:rect.width,height:rect.height,lineHeight:Number.parseFloat(getComputedStyle(el).lineHeight)||0,text:el.textContent?.trim()||''};})};
 });}
 async homeSolutionTabsFit(){return this.page.locator('.solution-main-content .main-option-name').evaluateAll(items=>items.every(el=>{
  const range=document.createRange();range.selectNodeContents(el);
  const lines=new Set([...range.getClientRects()].filter(rect=>rect.width>0&&rect.height>0).map(rect=>Math.round(rect.top)));
  return lines.size>0&&lines.size<=2&&el.scrollWidth<=el.clientWidth+1;
 }));}
 async mobileFooterFits(){return this.page.locator('.sky-footer-nav-column').evaluateAll(items=>items.filter(el=>el.getClientRects().length).every(el=>el.getBoundingClientRect().width>=120));}
 async mobileTranslatedControlsFit(){return this.page.evaluate(()=>{
  const ctas=[...document.querySelectorAll('.architecture-Content-btn a')].every(el=>el.getBoundingClientRect().height<=64&&el.scrollWidth<=el.clientWidth+1);
  const titles=[...document.querySelectorAll('.introduce-section:not(.active) .introduce-section-title')].every(el=>el.getBoundingClientRect().bottom<=el.closest('.introduce-section')!.getBoundingClientRect().bottom);
  const hero=document.querySelector('[data-electricity] .globalBanner'),buttons=hero?.querySelector('.pagebanner-button');
  return ctas&&titles&&(!hero||!buttons||buttons.getBoundingClientRect().bottom<=hero.getBoundingClientRect().bottom+1);
 });}
 async snapshot(){return this.page.evaluate(()=>({
  language:document.documentElement.lang,title:document.title,width:innerWidth,scrollWidth:Math.max(document.body.scrollWidth,document.documentElement.scrollWidth),
  footer:document.querySelector('.sky-footer-container')?.textContent?.replace(/\s+/g,' ').trim()||'',
  company:document.querySelector('[data-brand-footer]')?.textContent,
  copyright:document.querySelector('[data-footer-copyright]')?.textContent,
  address:document.querySelector('[data-footer-detail="address"] a')?.textContent,
  businessQq:document.querySelector('[data-footer-detail="qq"] a')?.textContent,
  filings:[...document.querySelectorAll('[data-filing-kind]')].map(row=>({kind:row.getAttribute('data-filing-kind'),text:row.querySelector('.filing-title')?.textContent,href:row.getAttribute('href'),icon:row.querySelector('img')?.getAttribute('src')})),
  partnerTitle:document.querySelector('.partner-Content .currency-title h2')?.textContent?.trim(),
  partnerSubtitle:document.querySelector('.partner-Content .currency-title p')?.textContent?.trim(),
  partnerLogos:[...document.querySelectorAll('.partner-Content-logo img')].map(image=>image.getAttribute('src')),
   partnerPageTitle:document.querySelector('[data-partner] .pagebanner-title')?.textContent?.trim(),
   partnerPageSubtitle:document.querySelector('[data-partner] .pagebanner-description')?.textContent?.trim(),
  architectureImage:document.querySelector('.architecture-Content-right img')?.getAttribute('src'),
  news:[...document.querySelectorAll('.news-item')].map(item=>({title:item.querySelector('.news-title h1')?.textContent?.trim()||'',summary:item.querySelector('.news-description')?.textContent?.trim()||''})),
 }))}
 async untranslated(){
  return this.page.evaluate(allowed=>{
   const results:{text:string;location:string}[]=[];
   const visible=(el:Element)=>!!el.getClientRects().length&&getComputedStyle(el).visibility!=='hidden';
   const check=(text:string,location:string)=>{text=text.trim();if(!/[\u3400-\u9fff]/.test(text)||text==='中文')return;
    let unmatched=text;for(const business of allowed)unmatched=unmatched.replaceAll(business,'');
    if(!/[\u3400-\u9fff]/.test(unmatched))return;
    results.push({text,location});};
   const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);let node:Node|null;
   while((node=walker.nextNode())){const el=node.parentElement;if(!el||el.closest('script,style,code,pre,textarea,[data-content-body]')||!visible(el))continue;check(node.textContent||'',el.tagName+'.'+el.className);}
   for(const el of document.querySelectorAll('[title],[placeholder],[aria-label],[alt]'))if(visible(el))for(const key of ['title','placeholder','aria-label','alt']){
    if(key==='aria-label'&&el.hasAttribute('data-public-locale-switch'))continue; // The language chooser names its target language.
    check(el.getAttribute(key)||'',el.tagName+'@'+key);
   }
   return [...new Map(results.map(item=>[item.text+'|'+item.location,item])).values()];
  },[...this.businessCopy].sort((a,b)=>b.length-a.length));
 }
 async capture(path:string){await this.page.screenshot({path,fullPage:true,animations:'disabled'});}
}
