import assert from 'node:assert/strict';
import test from 'node:test';
import {parseHTML} from 'linkedom';
import {readPublicContentList,readPublicContentDetail} from '../../frontend/public/lina/public-content-client.js';
import {renderHeaderLogo} from '../../frontend/public/lina/header-logo.js';
import {renderBrandFooter} from '../../frontend/public/lina/footer.js';
import {writeBrandIdentity,readBrandIdentity} from '../../frontend/public/lina/brand-identity.js';
import {initializeLocale} from '../../frontend/public/lina/locale.js';
import {renderCatalogCard} from '../../frontend/public/lina/catalog-page.js';
const announcement={id:23,title:'维护公告',content:'<p>维护说明</p><img src="/images/notice.png">',createdAt:1793203200000,updatedAt:1793289600000};
test('announcement list uses its owner and bounded page contract without detail waterfalls',async()=>{
 const calls=[],request=async path=>{calls.push(new URL(path,'https://shop.test'));return {list:[announcement],total:3};};
 const first=await readPublicContentList(request,{type:'announcement',limit:2});
 assert.equal(calls.length,1);assert.equal(calls[0].pathname,'/x/content-notice/api/v1/public/announcements');assert.equal(calls[0].searchParams.get('pageNum'),'1');assert.equal(calls[0].searchParams.get('pageSize'),'2');assert.equal(first.items[0].contentNo,'announcement:23');assert.equal(first.items[0].publishedAt,announcement.createdAt);assert.equal(first.nextCursor,'2');
 const last=await readPublicContentList(request,{type:'announcement',cursor:'2',limit:2});assert.equal(calls[1].searchParams.get('pageNum'),'2');assert.equal(last.nextCursor,'');
 await assert.rejects(()=>readPublicContentList(request,{type:'announcement',query:'unsupported'}),/announcement_search_unavailable/);assert.equal(calls.length,2);
});
test('announcement detail retains inline images and failure is not disguised as an empty article',async()=>{
 let path;const result=await readPublicContentDetail(async url=>{path=url;return announcement;},'announcement:23');assert.equal(path,'/x/content-notice/api/v1/public/announcements/23');assert.equal(result.item.title,announcement.title);assert.equal(result.item.body,announcement.content);assert.equal(result.item.file,undefined);
 await assert.rejects(()=>readPublicContentDetail(()=>{throw new Error('unexpected request');},'announcement:23/extra'),/invalid_public_announcement/);
 await assert.rejects(()=>readPublicContentDetail(async()=>{throw new Error('unavailable');},'announcement:23'),/unavailable/);
});
test('news, knowledge and download use current contenthub query, cursor and detail metadata',async()=>{
 for(const type of ['news','knowledge','download']){let path;const expected={items:[{contentNo:'CT_current',type,title:'Current article'}],nextCursor:'opaque-next'};assert.deepEqual(await readPublicContentList(async url=>{path=new URL(url,'https://shop.test');return expected;},{type,query:'资料',cursor:'opaque-cursor',limit:20}),expected);assert.equal(path.pathname,'/x/idc-core/api/v1/public/content/items');assert.equal(path.searchParams.get('type'),type);assert.equal(path.searchParams.get('query'),'资料');assert.equal(path.searchParams.get('cursor'),'opaque-cursor');}
 const expected={item:{contentNo:'CT_current',type:'download',body:'<p>说明</p>',file:{fileName:'manual.pdf',sizeBytes:1024}}};let path;assert.deepEqual(await readPublicContentDetail(async url=>{path=url;return expected;},'CT_current'),expected);assert.equal(path,'/x/idc-core/api/v1/public/content/items/CT_current');
});
test('current logoUrl and policeNumber render header/footer and survive the public identity cache',()=>{
 const {document}=parseHTML('<header class="horizontal-menu-navbar-logo"><a data-brand-name></a></header><footer class="sky-footer-container"><div class="sky-footer-left"><div class="sky-footer-companyInformation"><p data-brand-footer></p></div><div data-footer-contact="phone"><a></a></div><div data-footer-contact="email"><a></a></div></div><div data-footer-navigation></div><a data-filing-kind="icp"><span class="filing-title"></span></a><a data-filing-kind="public-security"><span class="filing-title"></span></a><a data-filing-kind="telecom-license"><span class="filing-title"></span></a></footer>');document.location={origin:'https://shop.test'};
 const brand={siteName:{zhCN:'当前网站',enUS:'Current site'},footer:{zhCN:'当前公司'},logoUrl:'/images/current-logo.svg',policeNumber:'公安备案123',policeUrl:'https://beian.example.test/123',icpNumber:'ICP备案456',footerNavigation:[],contactPhone:'123',contactEmail:'support@example.test'};
 renderHeaderLogo(document,brand,'当前网站');renderBrandFooter(document,brand,value=>{try{return new URL(value,'https://shop.test');}catch{return null;}});assert.equal(document.querySelector('.horizontal-menu-navbar-logo img').getAttribute('src'),'https://shop.test/images/current-logo.svg');assert.equal(document.querySelector('[data-footer-identity-logo] img').getAttribute('src'),'https://shop.test/images/current-logo.svg');
 const police=document.querySelector('[data-filing-kind="public-security"]');assert.equal(police.hidden,false);assert.equal(police.querySelector('.filing-title').textContent,'公安备案123');assert.equal(police.getAttribute('href'),'https://beian.example.test/123');assert.equal(document.querySelector('[data-filing-kind="telecom-license"]').hidden,true);
 const values=new Map(),view={localStorage:{setItem:(key,value)=>values.set(key,value),getItem:key=>values.get(key)}};writeBrandIdentity({...brand,templateConfig:{private:'excluded'}},view,1000);assert.equal(readBrandIdentity(view,1001).logoUrl,brand.logoUrl);assert.equal(readBrandIdentity(view,1001).policeNumber,brand.policeNumber);assert.equal(readBrandIdentity(view,1001).templateConfig,undefined);
 renderHeaderLogo(document,{logoUrl:'javascript:alert(1)'},'当前网站');assert.equal(document.querySelector('.horizontal-menu-navbar-logo img'),null);
});
test('current catalog availability remains buyable without invented inventory or monthly prices',t=>{
 const {document}=parseHTML('<template id="product-card"><article><h5></h5><p class="stock-info"></p><div class="product-description"></div><div class="price"></div><a class="buy-now-button"></a></article></template>');
 const previous=Object.getOwnPropertyDescriptor(globalThis,'document');Object.defineProperty(globalThis,'document',{value:document,configurable:true});t.after(()=>{if(previous)Object.defineProperty(globalThis,'document',previous);else delete globalThis.document;});
 const item={publicSlug:'server-current',nameZhCN:'当前服务器',nameEnUS:'Current server',summaryZhCN:'内存：2 GiB',summaryEnUS:'Memory: 2 GiB',currencies:['CNY'],availability:'available',pricingPresentation:'base_price'};
 const card=renderCatalogCard(document,item);assert.ok(card.querySelector('h5').textContent.includes(item.nameZhCN));assert.equal(card.querySelector('.lina-product-new-badge').textContent,'NEW');assert.equal(card.querySelector('.stock-info').textContent,'可订购');assert.equal(card.querySelector('.price').textContent,'配置后报价');assert.equal(card.querySelector('.buy-now-button').getAttribute('href'),'/configure.html?product=server-current');assert.equal(card.querySelector('.price').textContent.includes('0.00'),false);
 const locale=initializeLocale(document);locale.setLocale('en-US');t.after(()=>locale.setLocale('zh-CN'));const english=renderCatalogCard(document,item);assert.ok(english.querySelector('h5').textContent.includes(item.nameEnUS));assert.equal(english.querySelector('.lina-product-new-badge').textContent,'NEW');assert.equal(english.querySelector('.price').textContent,'Configure for a quote');assert.equal(english.querySelector('.stock-info').textContent,'Available');assert.equal(english.querySelector('.buy-now-button').textContent,'Configure');locale.setLocale('zh-CN');
 const unavailable=renderCatalogCard(document,{...item,availability:'unavailable'});assert.equal(unavailable.querySelector('.buy-now-button').hasAttribute('href'),false);assert.equal(unavailable.querySelector('.buy-now-button').getAttribute('aria-disabled'),'true');
});
