import {test,expect} from '../support/source-site';
import {PartnerPage} from '../pages/PartnerPage';
import {sourceHTML,sourceDocument} from '../support/partner-reference.mjs';
const live=process.env.PARTNER_LIVE==='1';
test.use({baseURL:live?'http://127.0.0.1:6680':'http://localhost:6680',websiteGateway:live});
test.describe('TC-130 合作伙伴原文与交互',()=>{
 for(const mobile of [false,true])test(mobile?'TC-130d 手机首页伙伴入口':'TC-130c 桌面首页伙伴入口',async({page},info)=>{
  if(mobile)await page.setViewportSize({width:390,height:844});
  await page.addInitScript(()=>localStorage.setItem('lina.website.locale','zh-CN'));
  const site=new PartnerPage(page);await site.openHomePartnerMenu();
  const entry=site.partnerMenuLink();
  await expect(entry).toHaveAttribute('href',/\/cooperation\/partner\.html$/);
  await expect(entry.locator('.Submenu-title')).toHaveText('合作伙伴');
  await expect(entry.locator('.Submenu-desc')).toHaveText('共同发展，实现互利共赢的合作伙伴关系');
  await page.screenshot({path:info.outputPath('partner-home-menu.png')});
  await entry.locator(mobile?'.Submenu-desc':'.Submenu-title').click();
  await expect(page).toHaveURL(/\/cooperation\/partner\.html$/);await site.ready();
  await expect(page.locator('[data-partner] .globalBanner')).toContainText('合作伙伴');
  await expect(page.locator('[data-partner] .introduce-section')).toHaveCount(5);
  await page.screenshot({path:info.outputPath('partner-home-arrival.png')});
 });
 for(const mobile of [false,true])test(mobile?'TC-130b 手机':'TC-130a 桌面',async({page},info)=>{
  test.setTimeout(90000);
  const errors:string[]=[],foreign:string[]=[],missing:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',r=>{if(/xiaobaiyun|weixin\.qq/.test(new URL(r.url()).hostname))foreign.push(r.url());});
  page.on('response',r=>{if(r.status()>=400&&/partner|static\//.test(new URL(r.url()).pathname))missing.push(r.url());});
  if(mobile)await page.setViewportSize({width:390,height:844});
  await page.addInitScript(()=>localStorage.setItem('lina.website.locale','zh-CN'));
  const site=new PartnerPage(page);expect((await site.open())?.status()).toBe(200);await site.ready();
  const comparison=await page.locator('[data-partner]').evaluate((root,source)=>{
   const extract=(root:Element|Document)=>{
   const records:any[]=[];
   for(const [index,selector] of ['.globalBanner','.et-container','main.et-main'].entries()){
    const clone=root.querySelector(selector)!.cloneNode(true) as Element;
    clone.querySelectorAll('.partner-table-viewport,.partner-requirements-viewport').forEach(e=>e.replaceWith(...Array.from(e.childNodes)));
    clone.querySelectorAll('[class]').forEach(e=>e.setAttribute('class',e.className.split(/\s+/).filter(c=>c&&c!=='active').join(' ')));
    const sections=['advantages','benefits','requirements','process','faq'];
    const visit=(e:Element,path:string)=>Array.from(e.childNodes).forEach((n,position)=>{
     if(n.nodeType===3&&n.textContent?.trim())records.push({section:index===2?sections[Array.from(clone.querySelectorAll(':scope > .et-slide')).indexOf(e.closest('.et-slide')!)]:index===0?'hero':'anchors',parent:path,tag:e.tagName.toLowerCase(),level:/^H[1-6]$/.test(e.tagName)?Number(e.tagName[1]):null,text:n.textContent});
     if(n.nodeType===1&&!['SCRIPT','STYLE'].includes((n as Element).tagName)){const c=n as Element;visit(c,path+'/'+c.tagName.toLowerCase()+(c.className?'.'+c.className.split(/\s+/).join('.'):'')+'['+position+']');}
    });visit(clone,index===0?'hero':index===1?'anchors':'body');
   }return records;
   };
   const expected=new DOMParser().parseFromString(source,'text/html');
   const header=expected.querySelector('.grade-table-header-cell:last-child')!;
   header.lastChild!.textContent=header.lastChild!.textContent!.replace('砖石代理','钻石代理');
   return {actual:extract(root),expected:extract(expected)};
  },sourceHTML);
  expect(comparison.actual).toEqual(comparison.expected);
  await expect(page.locator('.grade-table-header-cell').last()).toHaveText('钻石代理');
  await expect(page.locator('.partner_table .partner_row').last().locator('.partner_cell').first()).toHaveText('砖石代理');
  const expectedRows=Array.from(sourceDocument.querySelectorAll('.grade-table tr')).map((r:any)=>Array.from(r.children).map((c:any)=>({rowspan:c.getAttribute('rowspan'),colspan:c.getAttribute('colspan'),status:c.classList.contains('grade-status-yes')?'yes':c.classList.contains('grade-status-no')?'no':null})));
  expect(await page.locator('.grade-table tr').evaluateAll(rows=>rows.map(r=>Array.from(r.children).map(c=>({rowspan:c.getAttribute('rowspan'),colspan:c.getAttribute('colspan'),status:c.classList.contains('grade-status-yes')?'yes':c.classList.contains('grade-status-no')?'no':null}))))).toEqual(expectedRows);
  await expect(page.locator('.introduce-section.active')).toHaveCount(1);
  await expect(page.locator('.introduce-section').first()).toHaveClass(/active/);
  await expect(page.locator('.sky-footer-container')).toBeVisible();
  expect(await page.locator('.pagebanner-title').evaluate(e=>e.getBoundingClientRect().top>=0)).toBe(true);
  expect(await page.locator('.pagebanner-button a').first().evaluate(e=>getComputedStyle(e.querySelector('span')!).backgroundImage)).not.toBe('none');
  await site.screenshot(info.outputPath('partner-'+(mobile?'mobile':'desktop')+'-initial.png'));
  for(let i=0;i<5;i++){
   await site.advantage(i);await expect(page.locator('.introduce-section').nth(i)).toHaveClass(/active/);
   await expect(page.locator('.introduce-section').nth(i).locator('.introduce-section-desc')).toBeVisible();
   await expect.poll(()=>page.locator('.introduce-section').nth(i).evaluate(e=>{const d=e.querySelector('.introduce-section-desc')!;return d.getBoundingClientRect().bottom<=e.getBoundingClientRect().bottom+1;})).toBe(true);
  }
  for(let i=0;i<5;i++){await site.section(i);await expect.poll(()=>page.locator('#partner-'+i).evaluate(e=>{const top=e.getBoundingClientRect().top;return top>=0&&(top<110||Math.abs(scrollY-(document.documentElement.scrollHeight-innerHeight))<2);})).toBe(true);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);}
  await expect(page.locator('.problem-content-item-content')).toHaveCount(3);
  for(const answer of await page.locator('.problem-content-item-content').all())await expect(answer).toBeVisible();
  const assets=['remoteL17131030094606.jpeg','remoteL17130183791832.png','remoteL17069792867331.png','remoteL17075794763815.png','remoteL17075795114472.png','correct.svg','error.svg'];
  expect(await page.evaluate(async names=>Promise.all(names.map(async name=>{const image=new Image();image.src='/lina/partner-assets/'+name;try{await image.decode();return image.naturalWidth>0;}catch{return false;}})),assets)).toEqual(assets.map(()=>true));
  for(const [selector,name]of [['.globalBanner',assets[0]],['.introduce-section-img',assets[1]],['.problem_bg',assets[3]]])expect(await page.locator(selector).first().evaluate(e=>getComputedStyle(e).backgroundImage)).toContain(name);
  for(const state of ['yes','no'])expect(await page.locator('.grade-status-'+state).first().evaluate(e=>getComputedStyle(e,'::before').backgroundImage)).toContain(state==='yes'?'correct.svg':'error.svg');
  expect(await page.locator('[data-partner] img').evaluateAll(images=>images.every(i=>(i as HTMLImageElement).complete&&(i as HTMLImageElement).naturalWidth>0))).toBe(true);
  if(mobile){for(const selector of ['.partner-table-viewport','.partner-requirements-viewport']){const viewport=page.locator(selector);expect(await viewport.evaluate(e=>e.scrollWidth>e.clientWidth)).toBe(true);await site.scrollTable(selector,true);expect(await viewport.evaluate(e=>e.scrollLeft>0)).toBe(true);}await site.screenshot(info.outputPath('partner-mobile-tables-right.png'));for(const selector of ['.partner-table-viewport','.partner-requirements-viewport'])await site.scrollTable(selector,false);}
  await site.screenshot(info.outputPath('partner-'+(mobile?'mobile':'desktop')+'-complete.png'));
  const ctas=page.locator('[data-partner] a[href="/contact.html"]');await expect(ctas).toHaveCount(3);
  for(let i=0;i<3;i++){const contact=await site.contact(i);await expect(contact).toHaveURL(/\/contact.html$/);await expect(contact.locator('body')).toHaveAttribute('data-page','contact');await contact.close();}
  expect(errors).toEqual([]);expect(foreign).toEqual([]);expect(missing).toEqual([]);
 });
});
