import {syncSession,signIn,commerce,getAccessToken,rememberCart} from './commerce-session.js';
import {createServerLogin} from './server-login.js';
import {readPublicContentList,readPublicContentDetail} from './public-content-client.js';
import {initializeAbout} from './about.js';
import {initializeSolutionProducts} from './solution-products.js';
import {initializeAffiliate} from './affiliate.js';
import {localized,localizedName,localizedDate,localizedControlLabel,getLocale,initializeLocale,translate,setPageTitle} from './locale.js';
import {initializeContact,renderContactBrand} from './contact.js';
import {createCatalogPage,renderCategoryMenu,renderCategoryNotices,bindCategoryToggle} from './catalog-page.js';
import {createCommerceNavigation} from './commerce-navigation.js';
import {startServerHero,renderServerHeroConfiguration} from './server-hero.js';
import {initializeRegionGlobe} from './region-globe.js';
import {startBrandRefresh} from './brand-refresh.js';
// Browser adapter for the project's public contracts. No source-site data or credentials.
import {renderBrandFooter,bindCertificateFooter,bindAboutFooter,initializeFooter} from './footer.js';
import {renderHeaderLogo} from './header-logo.js';
import {brandIdentityKey,readBrandIdentity,readBrandIdentityEntry,readServerBrandIdentityEntry,writeBrandIdentity,renderBrandIdentity} from './brand-identity.js';
import {renderNavigation} from './navigation.js?v=20261025-bt';
import {catalogCacheKey,navigationCacheKey,readCatalogCache,readNavigationCache,mergeCatalogCache,writeNavigationCache} from './navigation-cache.js';
import {attachNavigationSearch} from './navigation-search.js';
import {attachCatalogSearch} from './catalog-search.js';
import {quoteStateText} from './quote-state.js';
import {applyConfigurationDefaults} from './configuration-defaults.js';
import {renderHomeBrand,loadHomeNews} from './home.js';
import {renderTemplateFooter,initializeStaticSolutions,initializeStaticHome,initializeHomeBlocks} from './home-template.js';
import {initializeActivityPage} from './activity.js';
import {enhanceOperatingSystem} from './configuration-controls.js';
import {renderSupportBar} from './support-bar.js';
import {loadCustomerConfiguration} from './customer-config.js';
import {createProductSidebar} from './product-sidebar.js';
import {serverSpecificationRows,serverSpecificationOptions,defaultUsageNotice} from './server-specifications.js';
import {renderCommerceHeading} from './commerce-heading.js';
import {prependServerFlag} from './server-country.js';
import {initializeWebmaster} from './webmaster.js';
import {initializeElectricity} from './electricity.js';
import {initializeWebSolution} from './web-solution.js';
import {initializeGame} from './game.js';
import {initializeFinancial} from './financial.js';
import {initializeSafety} from './safety.js';
import {initializeGoingsea} from './goingsea.js';
import {initializeGovernment} from './government.js';
import {initializePartner} from './partner.js';
import {initializeAssure} from './assure.js';
import {initializeCertificate} from './certificate.js';
import {initializeBt} from './bt.js';
const goingseaPage=initializeGoingsea(document);
const gamePage=initializeGame(document);
const financialPage=initializeFinancial(document);
const safetyPage=document.body.dataset.page==='safety'?initializeSafety(document):null;
const governmentPage=document.body.dataset.page==='government'?initializeGovernment(document):null;
const partnerPage=document.body.dataset.page==='partner'?initializePartner(document):null;
const assurePage=initializeAssure(document);
const certificatePage=initializeCertificate(document);
const aboutPage=initializeAbout(document);
const btPage=initializeBt(document);
const webSolutionPage=document.body.dataset.page==='web-solution'?initializeWebSolution(document,request):null;
if(document.body.dataset.page==='electricity')initializeElectricity(document);
if(document.body.dataset.page==='webmaster')initializeWebmaster(document);
const affiliatePage=document.body.dataset.page==='affiliate'?initializeAffiliate(document):null;
bindCertificateFooter(document);
bindAboutFooter(document);
initializeFooter(document);
initializeLocale(document);
const solutionProducts=initializeSolutionProducts(document,request);
// Start the authoritative read before the remaining page initialization.
let initialBrandRequest=document.hidden?null:request('/x/idc-portal/api/v1/public/brand');
initialBrandRequest?.catch(()=>{});
const serverIdentity=readServerBrandIdentityEntry(),browserIdentity=readBrandIdentityEntry();
const firstIdentity=serverIdentity&&(!browserIdentity||serverIdentity.savedAt>=browserIdentity.savedAt)?serverIdentity.brand:browserIdentity?.brand;
if(firstIdentity)renderBrandIdentity(document,firstIdentity);
window.addEventListener('storage',event=>{if(event.key===brandIdentityKey){const identity=readBrandIdentity();if(identity)renderBrandIdentity(document,identity);}});
if(document.body.dataset.page==='contact')initializeContact(document,request);
const customerConfiguration=loadCustomerConfiguration();
customerConfiguration.then(()=>{
 affiliatePage?.bindCustomerLinks();
 if(webSolutionPage||assurePage)for(const a of document.querySelectorAll('[data-customer]')){
  const unavailable=a.getAttribute('aria-disabled')==='true';customerLink(a,a.dataset.customer);
  if(unavailable&&a.hasAttribute('href'))a.removeAttribute('title');
 }
});
initializeStaticSolutions(document);
initializeStaticHome(document);
initializeRegionGlobe(document);
window.addEventListener('pageshow',()=>initializeRegionGlobe(document));
const $=(s,r=document)=>r.querySelector(s);
const base='/x/idc-core/api/v1/public';
let page=document.body.dataset.page;
const params=new URLSearchParams(location.search);
// Home catalog selection is page state, not part of the home address.
if(page==='home'){
 const address=new URL(location.href);address.pathname='/';address.searchParams.delete('categoryNo');
 if(address.href!==location.href)history.replaceState(history.state,'',address.pathname+address.search+address.hash);
 params.delete('categoryNo');
}
let contentMetadataReady=false;
const label=localizedName;
const billingPeriod=term=>{
 const text=label(term).trim().toLowerCase();
 if(/^(1\s*(月|个月|month|mo)|月付|monthly)$/.test(text))return {rank:1,text:'月'};
 if(/^(3\s*(月|个月|month|months|mo)|季付|quarterly)$/.test(text))return {rank:2,text:'季'};
 if(/^(6\s*(月|个月|month|months|mo)|半年付|semi[- ]?annual(?:ly)?)$/.test(text))return {rank:3,text:'半年'};
 if(/^(1\s*(年|年期|year|yr)|年付|annually|annual)$/.test(text))return {rank:4,text:'年'};
 return {rank:99,text:label(term)};
};
const local=localized;
let statusRevision=0;
const status=(text,articleAction=false)=>{statusRevision++;const slot=$('[data-status]');if(slot){slot.textContent=text;slot.toggleAttribute('data-content-action-feedback',articleAction);}};
let retryAction;
function retry(action){retryAction=action;let button=$('[data-retry]');if(!button){button=page==='content'?node('button',undefined,'fancy-button bg-gradient1'):node('button','重试','btn sky-config-btn');if(page==='content')button.append(node('span','重试'));button.type='button';button.setAttribute('data-retry','');$('[data-status]')?.after(button);button.addEventListener('click',()=>{button.hidden=true;retryAction?.();});}button.hidden=!action;}
function node(tag,text,className){const n=document.createElement(tag);if(text!==undefined)n.textContent=text;if(className)n.className=className;return n;}
function saveCatalogCache(data){mergeCatalogCache(data);document.dispatchEvent(new CustomEvent('lina:catalog-cache-updated'));}
function renderConfigurationCategory(items,categoryNo){
 const selected=items.find(item=>item.categoryNo===categoryNo),byNo=new Map(items.filter(item=>item.categoryNo).map(item=>[item.categoryNo,item]));
 renderCommerceHeading(document,{title:product?label(product):selected?label(selected):'',description:localized(product,'summary'),back:'/products.html'+(categoryNo?'?categoryNo='+encodeURIComponent(categoryNo):'')});
 const back=$('.nav-top-page .return');if(back)back.href='/products.html'+(categoryNo?'?categoryNo='+encodeURIComponent(categoryNo):'');
 const context=$('[data-configure-category]');
 if(context){
  const path=[],seen=new Set();
  for(let item=selected;item&&!seen.has(item.categoryNo);item=byNo.get(item.parentCategoryNo)){seen.add(item.categoryNo);const title=label(item);if(title)path.unshift(title);}
  context.textContent=path.length?'商品分类：'+path.join(' / '):'';context.hidden=!path.length;
 }
 renderCategoryNotices(document,selected);
 const usage=$('[data-catalog-usage-notice]');
 if(usage){usage.hidden=false;const text=usage.querySelector('p');if(text)text.textContent=defaultUsageNotice;let merchant=usage.querySelector('[data-merchant-usage-notice]');if(!merchant){merchant=node('p');merchant.dataset.merchantUsageNotice='';(usage.querySelector('.form-group-text-content')||usage).append(merchant);}merchant.textContent=selected?.usageNotice&&selected.usageNotice!==defaultUsageNotice?selected.usageNotice:'';merchant.hidden=!merchant.textContent;}
}
export async function request(path,body){
 if(path.startsWith('/x/idc-core/api/v1/account/'))return commerce(path.slice('/x/idc-core/api/v1/'.length),body===undefined?'GET':'POST',body);
 const res=await fetch(path,{method:body===undefined?'GET':'POST',credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json','Accept-Language':getLocale(),...(body===undefined?{}:{'Content-Type':'application/json'})},body:body===undefined?undefined:JSON.stringify(body),signal:AbortSignal.timeout(15000)});
 const envelope=await res.json();if(!res.ok||envelope.code!==undefined&&String(envelope.code)!=='0'||!envelope.data)throw new Error('服务暂时不可用，请稍后重试。');return envelope.data;
}
function safeURL(value,customer=false){
 if(typeof value!=='string'||!value||/^(?:javascript|data):/i.test(value))return null;
 const origin=customer?window.__LINA_TEMPLATE__?.customerUrl:location.origin;if(customer&&!origin)return null;
 try{const u=new URL(value,origin);return ['http:','https:'].includes(u.protocol)?u:null;}catch{return null;}
}
function customerLink(a,path){if(path==='/account/cart'){a.href='/cart.html';a.removeAttribute('aria-disabled');a.removeAttribute('target');return;}const u=safeURL(path,true);if(u){a.href=u.href;a.removeAttribute('aria-disabled');}else{a.removeAttribute('href');a.setAttribute('aria-disabled','true');a.title='用户中心地址尚未配置';}}
let brandLoaded=false;
async function brand(){
 const pending=initialBrandRequest;initialBrandRequest=null;
 let data;
 try{data=await(pending||request('/x/idc-portal/api/v1/public/brand'));}
 catch(error){
  // A transient first read must not leave the company page failed until the next poll.
  if(page!=='about'||brandLoaded)throw error;
  await new Promise(resolve=>setTimeout(resolve,500));
  data=await request('/x/idc-portal/api/v1/public/brand');
 }
 const {brand:rawBrand}=data;
 const b=rawBrand;
 if(!b){try{localStorage.removeItem(brandIdentityKey);}catch{}renderBrandIdentity(document,{});throw new Error('brand unavailable');}
 writeBrandIdentity(rawBrand);
 if(/^#[0-9a-f]{6}$/i.test(b.primaryColor||''))document.documentElement.style.setProperty('--Original',b.primaryColor);
 for(const slot of document.querySelectorAll('[data-brand-name]'))if(!slot.closest('.horizontal-menu-navbar-logo'))slot.textContent=local(b.siteName);
 renderHeaderLogo(document,b,local(b.siteName));
 if(!contentMetadataReady){
  if(page==='terms'||page==='electricity')setPageTitle(document,page==='terms'?'服务条款':'电商云解决方案',b);
  else document.title=local(b.seoTitle)||local(b.siteName)||'官网';
 }
 if(page==='home')renderServerHeroConfiguration(document,b.templateConfig);
 webSolutionPage?.renderBrand(b);
 solutionProducts?.renderBrand(b);
 renderNavigation(document,b.navigation||[],{safeURL,customerLink,request});
 writeNavigationCache(b.navigation||[]);
 void customerConfiguration.then(()=>renderSupportBar(document,b,path=>safeURL(path,true)?.href||''));
 renderBrandFooter(document,b,safeURL);
 renderSupportBar(document,b,path=>safeURL(path,true)?.href||'');
 gamePage?.renderBrand(b);
 financialPage?.renderBrand(b);
 safetyPage?.renderBrand(b);
 governmentPage?.renderBrand(b);
 assurePage?.renderBrand(b);
 certificatePage?.renderBrand(b);
 aboutPage?.renderBrand(b);
 btPage?.renderBrand(b);
 goingseaPage?.renderBrand(b);
 if(!brandLoaded)renderTemplateFooter(document);
 if(!contentMetadataReady){let meta=$('meta[name=description]');if(!meta){meta=node('meta');meta.name='description';document.head.append(meta);}meta.content=local(b.seoDescription);}
 if(page==='home'&&!brandLoaded){
  renderHomeBrand(document,b,safeURL,customerLink);
 }
 if(page==='contact'||page==='about'){
  if(page==='contact'){
   renderContactBrand(document,b,path=>safeURL(path,true)?.href||'');
   status('');
  }
 }
 brandLoaded=true;
}
let catalogSequence=0;
let catalogPage,commerceNavigation;
const categoryExpanded=new Map();
const productSidebar=['products','configure'].includes(page)?createProductSidebar({document,request,base,expanded:categoryExpanded}):null;
function makeCatalogPage(){return createCatalogPage({document,request,base,status,retry,customerLink,params,expanded:categoryExpanded,getCategories:()=>configurationCategoryItems,renderSidebar:(items,selected)=>productSidebar.render(items,selected),preloadSidebar:categoryNo=>productSidebar.preload(categoryNo),updateCategories:items=>{configurationCategoryItems=items;updateSearchCategories?.(items);}});}
function categories(items,selectedCategory=params.get('categoryNo')||''){
 if(page==='configure'){
  const currentProduct=product?.publicSlug===params.get('product')?product:configurationCatalogProducts.find(item=>item.publicSlug===params.get('product'));
  const category=selectedCategory||currentProduct?.categoryNo||'';
  updateSearchCategories?.(items);productSidebar.render(items,{categoryNo:category,publicSlug:params.get('product')||'',currentProduct,initialProducts:configurationCatalogProducts});
  renderConfigurationCategory(items,category);if(product)configurationModels(product);return;
 }
 updateSearchCategories?.(items);
 if(page==='home'){
  const current=items.find(item=>item.categoryNo===selectedCategory);
  $('.systemproduct-left-title').textContent=current?label(current):'';
  const description=$('.systemproduct-left-desc');description.textContent='各项配置均衡，支持按套餐配置进行升级或单独升级硬盘、带宽等附加配置。';description.hidden=false;
  const intro=$('[data-home-product-intro]'),previous=intro.dataset.category;
  intro.dataset.category=selectedCategory;
  if(previous!==undefined&&previous!==selectedCategory&&!matchMedia('(prefers-reduced-motion: reduce)').matches){
   const cards=$('.systemproduct-right-container');
   intro.classList.remove('lina-product-intro-entered');cards.classList.remove('lina-product-cards-entered');
   void intro.offsetWidth;
   intro.classList.add('lina-product-intro-entered');cards.classList.add('lina-product-cards-entered');
  }
 }
 const slot=$('[data-categories]');if(!slot)return;slot.replaceChildren();
 for(const c of items){
  const a=node('a',undefined,page==='home'?'systemproduct-category':'sky-cart-submenu-item');
  a.append(node(page==='home'?'p':'span',label(c),page==='home'?'systemproduct-category-name':''));
  const selected=selectedCategory===c.categoryNo;
  a.classList.toggle('active',selected);if(selected)a.setAttribute('aria-current','true');
  const query=new URLSearchParams();if(c.categoryNo)query.set('categoryNo',c.categoryNo);if(params.get('namePrefix'))query.set('namePrefix',params.get('namePrefix'));
  a.href=(page==='home'?'/index.html':'/products.html')+(query.size?'?'+query:'');
  if(page==='home')a.addEventListener('click',event=>{
   if(event.button!==0||event.ctrlKey||event.metaKey||event.shiftKey||event.altKey)return;
   event.preventDefault();clearTimeout(searchTimer);
   if(c.categoryNo)params.set('categoryNo',c.categoryNo);else params.delete('categoryNo');
   catalog();
  });
  slot.append(a);
 }
}
// Per-page display snapshots; every selection revalidates against the public API.
// Never used for purchase configuration, inventory checks or pricing decisions.
const homeCatalogSnapshots=new Map(),homeCatalogPending=new Map();
let homeCategoryItems=[];
function renderCatalogProducts(data,append=false){
 const slot=$('[data-products]'),more=$('[data-more]');
 if(!append)slot.replaceChildren();
  for(const p of data.products||[]){const card=$('#product-card').content.firstElementChild.cloneNode(true);const url='/products.html?product='+encodeURIComponent(p.publicSlug);
   if(page==='home'){$('.systemproduct-name',card).textContent=label(p);$('.systemproduct-name',card).title=label(p);$('.systemproduct-desc',card).textContent=localized(p,'summary');if(p.availability==='available')card.href=url;else card.append(node('p',p.availability==='manual_confirmation'?'需人工确认':'暂不可订购','lina-product-availability'));}
   else{$('h5',card).textContent=label(p);$('.stock-info',card).textContent=p.availability==='available'?'可订购':p.availability==='manual_confirmation'?'需人工确认':'暂不可订购';$('.product-description',card).textContent=localized(p,'summary');const buy=$('.buy-now-button',card);buy.textContent='选择配置';if(p.availability==='available')buy.href=url;else buy.remove();}slot.append(card);}
  status(slot.children.length?'':'暂无可订购商品。');more.dataset.cursor=data.nextCursor||'';more.hidden=!data.nextCursor;
}
async function catalog(append=false){
 if(catalogPage)return catalogPage.load(append);
 if(page==='home')$('.systemproducts').classList.add('active');
 retry();const seq=++catalogSequence,slot=$('[data-products]'),more=$('[data-more]');more.disabled=true;
 const query=new URLSearchParams({limit:page==='home'&&!params.get('categoryNo')?'1':'12'});for(const key of ['categoryNo','namePrefix'])if(params.get(key))query.set(key,params.get(key));if(append&&more.dataset.cursor)query.set('cursor',more.dataset.cursor);
 const key=query.toString(),isHome=page==='home';
 const snapshot=isHome&&!append?homeCatalogSnapshots.get(key):null;
 if(isHome&&homeCategoryItems.length)categories(homeCategoryItems);
 if(snapshot&&Date.now()-snapshot.savedAt<30000)renderCatalogProducts(snapshot.data);
 else{if(!append)slot.replaceChildren();more.hidden=true;status('正在加载商品…');}
 try{
  let pending=isHome?homeCatalogPending.get(key):null;
  if(!pending){
   pending=request(base+'/catalog/products?'+query);
   if(isHome){homeCatalogPending.set(key,pending);pending.finally(()=>{if(homeCatalogPending.get(key)===pending)homeCatalogPending.delete(key);}).catch(()=>{});}
  }
  const data=await pending;
  if(isHome&&!append)homeCatalogSnapshots.set(key,{savedAt:Date.now(),data});
  if(seq!==catalogSequence)return;
  if(!params.get('categoryNo')&&!params.get('namePrefix')&&!append)saveCatalogCache(data);
  if(isHome){
   homeCategoryItems=data.categories||[];
   if(homeCategoryItems.length&&!homeCategoryItems.some(c=>c.categoryNo===params.get('categoryNo'))){
    params.set('categoryNo',homeCategoryItems[0].categoryNo);return catalog();
   }
  }
  categories(data.categories||[]);
  renderCatalogProducts(data,append);
 }catch{if(seq===catalogSequence){status('商品加载失败，请刷新重试。');more.hidden=true;retry(()=>catalog(append));}}finally{if(seq===catalogSequence)more.disabled=false;}
}

export function money(value,currency){
 const digits=new Intl.NumberFormat('zh-CN',{style:'currency',currency}).resolvedOptions().maximumFractionDigits;
 const amount=BigInt(value);const sign=amount<0n?'-':'';const absolute=amount<0n?-amount:amount;const factor=10n**BigInt(digits);
 return `${currency} ${sign}${absolute/factor}${digits?'.'+String(absolute%factor).padStart(digits,'0'):''}`;
}
let configuration,product,quote,chosenTerm,currency='',sequence=0,timer,busy=false,valid=false,quoteIdentity='',loginControls,editingItem,editingCartVersion=0,cartAttempt,preferredTerm,needsExplicitTerm=false,configurationNotice='';
const purchaseCounts={quantity:'1',cycles:'1'};
const productPath=()=>'/x/idc-core/api/v1/'+(getAccessToken()?'account':'public')+'/catalog/products/'+encodeURIComponent(params.get('product')||'');
const invalidate=()=>{quote=undefined;chosenTerm=undefined;valid=false;$('[data-add]').disabled=true;$('[data-quote]').replaceChildren();renderBreakdown(null);const breakdown=$('[data-breakdown]');if(breakdown)breakdown.hidden=true;$('[data-breakdown-toggle]')?.setAttribute('aria-expanded','false');const price=$('[data-selected-price]');if(price)price.textContent='请选择配置与周期';};
function row(title){const r=node('div',undefined,'sky-form-group sky-config-row configureproduct');const l=node('label',undefined,'sky-md sky-form-label');const t=node('div',undefined,'label-title');t.append(node('span',title));l.append(t);const body=node('div',undefined,'sky-md-100-p');r.append(l,body);return {r,l,body};}
const arm={integer:'integerValue',decimal:'decimalValue',boolean:'booleanValue',date:'dateValue',datetime:'dateTimeValue',single_select:'optionTokens',multi_select:'optionTokens',tree_select:'treeOptionTokens',address:'addressOptionTokens',file:'fileTokens'};
function setValue(field,value){const v={fieldToken:field.fieldToken,type:field.type,[arm[field.type]||'textValue']:value};configuration.values=configuration.values.filter(x=>x.fieldToken!==field.fieldToken);if(value!==''&&(!Array.isArray(value)||value.length))configuration.values.push(v);sequence++;invalidate();clearTimeout(timer);timer=setTimeout(evaluate,300);}
// Visual projection only: the original select remains the authoritative value/constraint control.
function optionButtons(input,field,body){
 if((!['single_select','multi_select'].includes(field.type)&&field.type!=='integer')||!input.options?.length||field.options?.some(o=>o.children?.length))return;
 const group=node('div',undefined,'sky-btn-group lina-option-buttons');group.setAttribute('role','group');group.setAttribute('aria-label',localizedControlLabel(label(field),'选项'));
 input.classList.add('lina-option-source');
 const sync=()=>{for(const button of group.children){const selected=[...input.selectedOptions].some(o=>o.value===button.dataset.option);button.classList.toggle('active',selected);button.setAttribute('aria-pressed',String(selected));}};
 for(const [index,option]of [...input.options].entries()){const button=node('button',option.textContent,'btn sky-config-btn');button.id=input.id+'-choice-'+index;button.dataset.configEnhanced='true';button.type='button';button.dataset.option=option.value;button.disabled=input.disabled||option.disabled;button.addEventListener('click',()=>{if(input.multiple)option.selected=!option.selected;else input.value=option.value;sync();input.dispatchEvent(new Event('change',{bubbles:true}));});group.append(button);}
 if(body.closest('[data-server-specification="region"]'))for(const button of group.children)prependServerFlag(button,button.textContent);
 input.addEventListener('change',sync);input.addEventListener('focus',()=>{([...group.children].find(b=>b.getAttribute('aria-pressed')==='true')||group.firstElementChild)?.focus();});
 sync();body.append(group);
}
function integerSlider(input,field,body,key){
 if(field.type!=='integer'||!['','auto','slider'].includes(field.displayControl||'')||!/^[-]?\d+$/.test(field.minimumInteger??'')||!/^[-]?\d+$/.test(field.maximumInteger??''))return;
 const minimum=BigInt(input.min),increment=BigInt(input.step||'1');if(increment<=0n)return;
 const maximum=minimum+(BigInt(input.max)-minimum)/increment*increment;if(maximum<=minimum)return;input.max=String(maximum);
 const container=node('div',undefined,'sky-config-slider-container lina-integer-slider'),progress=node('div',undefined,'progress-content'),range=node('input');range.id=input.id+'-range';range.dataset.configEnhanced='true';range.type='range';range.min=input.min;range.max=input.max;range.step=input.step;range.disabled=input.disabled;range.setAttribute('aria-label',localizedControlLabel(label(field),'滑块'));
 const info=node('div',undefined,'sky-config-value-info');for(const value of [input.min,input.max])info.append(node('span',value+(field.unit?' '+field.unit:'')));progress.append(range,info);
 const toolbar=node('div',undefined,'progress-toolbar'),buttons=[];
 const sync=()=>{range.value=input.value||input.min;range.style.setProperty('--range-fill',((Number(range.value)-Number(minimum))/(Number(maximum)-Number(minimum))*100)+'%');for(const [button,boundary]of buttons)button.disabled=input.disabled||input.value===String(boundary);};
 range.addEventListener('input',()=>{input.value=range.value;sync();input.dispatchEvent(new Event('input',{bubbles:true}));});input.addEventListener('input',sync);
 for(const [text,direction]of [['−',-1n],['+',1n]]){const button=node('button',text,direction<0n?'sky-config-decrease':'sky-config-increase');button.type='button';button.setAttribute('aria-label',localizedControlLabel(label(field),direction<0n?'减少':'增加'));button.addEventListener('click',()=>{try{const current=BigInt(input.value||input.min),offset=current-minimum,index=direction>0n?offset/increment+1n:(offset+increment-1n)/increment-1n,next=minimum+index*increment;input.value=String(next<minimum?minimum:next>maximum?maximum:next);sync();input.dispatchEvent(new Event('input',{bubbles:true}));}catch{input.reportValidity();}});buttons.push([button,direction<0n?minimum:maximum]);if(direction<0n)toolbar.append(button,input);else toolbar.append(button);}
 sync();container.append(progress,toolbar);body.append(container);
}
function renderFields(){
 applyConfigurationDefaults(configuration);
 const focusId=document.activeElement?.dataset.configEnhanced?document.activeElement.id:'';
 const slot=$('[data-fields]');slot.replaceChildren();
 for(const specification of serverSpecificationRows(configuration.fields||[])){
  const {r,l,body}=row(specification.title);r.dataset.serverSpecification=specification.key;
  const f=specification.field;const v=configuration.values.find(x=>x.fieldToken===f.fieldToken)?.[arm[f.type]||'textValue'];let input;
  if(f.type==='file'){const input=node('input');input.type='file';input.id='field-'+f.fieldToken;l.htmlFor=input.id;input.multiple=(f.uploadPolicy?.maximumFiles||1)>1;input.accept=(f.uploadPolicy?.allowedExtensions||[]).join(',');input.disabled=f.disabled||busy;input.required=Boolean(f.required)&&!(v||[]).length;input.addEventListener('change',()=>uploadFiles(f,input));body.append(input);if((v||[]).length)body.append(node('p',getLocale()==='en-US'?'Files uploaded. Select again to replace.':'文件已上传，重新选择可替换。','form-group-text'));slot.append(r);continue;}
  if(f.type==='address'){
   const selected=Array.isArray(v)?[...v]:[];let options=f.options||[];
   for(let depth=0;options.length;depth++){
    const select=node('select');for(const o of options){const option=new Option(label(o),o.optionToken);option.disabled=!!o.disabled;select.append(option);}select.value=selected[depth]||'';select.disabled=f.disabled||busy;select.required=f.required;select.className='form-control';select.setAttribute('aria-label',label(f)+' 第'+(depth+1)+'级');
    const level=depth;select.addEventListener('change',()=>{setValue(f,[...selected.slice(0,level),select.value].filter(Boolean));renderFields();});body.append(select);options=options.find(o=>o.optionToken===selected[depth])?.children||[];
   }slot.append(r);continue;
  }
  const numeric=f.type==='integer'&&['radio','select'].includes(f.displayControl);
  if(numeric){input=node('select');for(const value of f.numericChoices||[]){const names=f.numericLabels?.[value];const option=new Option(names?localized(names):value,value);option.selected=value===v;input.append(option);}input.addEventListener('change',()=>setValue(f,input.value));
  }else if(['single_select','multi_select','tree_select'].includes(f.type)){
   input=node('select');input.multiple=f.type==='multi_select';
   const options=(items,depth=0)=>{for(const o of items||[]){const opt=new Option('　'.repeat(depth)+label(o),o.optionToken);opt.disabled=!!o.disabled;opt.selected=(v||[]).includes(o.optionToken);input.append(opt);options(o.children,depth+1);}};options(serverSpecificationOptions(specification));
   input.addEventListener('change',()=>setValue(f,[...input.selectedOptions].map(x=>x.value).filter(Boolean)));
  }else{
   input=node(f.type==='multi_line'?'textarea':'input');if(f.type!=='multi_line')input.type={integer:'number',decimal:'number',boolean:'checkbox',date:'date',datetime:'datetime-local'}[f.type]||'text';
   if(f.type==='boolean')input.checked=!!v;else if(f.type==='datetime'&&v){const date=new Date(v);input.value=Number.isNaN(date.getTime())?'':new Date(date.getTime()-date.getTimezoneOffset()*60000).toISOString().slice(0,16);body.append(node('p','按您设备的本地时区输入。','form-group-text'));}else input.value=v??'';
   if(['integer','decimal'].includes(f.type)){input.step=f.type==='integer'?(f.quantityStep||'1'):'any';const min=f.minimumInteger??f.minimumDecimal,max=f.maximumInteger??f.maximumDecimal;if(min!==undefined)input.min=min;if(max!==undefined)input.max=max;}
   if(f.maximumLength!==undefined)input.maxLength=f.maximumLength;if(f.minimumLength!==undefined)input.minLength=f.minimumLength;
   input.addEventListener('input',()=>setValue(f,f.type==='boolean'?input.checked:f.type==='datetime'&&input.value?new Date(input.value).toISOString():input.value));
  }
  input.id='field-'+f.fieldToken;l.htmlFor=input.id;input.disabled=f.disabled||busy;input.required=f.required&&f.type!=='boolean';input.className=f.type==='boolean'?'lina-config-checkbox':'form-control';body.append(input);
  let enhanced=false;if(f.type==='single_select'){try{enhanced=enhanceOperatingSystem(input,f,label);}catch(error){enhanced=false;}}
  if(!enhanced&&(f.presentation==='region'||f.displayControl==='radio'||(!f.displayControl||f.displayControl==='auto')&&['single_select','multi_select'].includes(f.type)&&specification.key!=='network'))optionButtons(input,f,body);integerSlider(input,f,body,specification.key);if(f.descriptionZhCN||f.descriptionEnUS)body.append(node('p',localized(f,'description'),'form-group-text'));slot.append(r);
 }
 if(focusId){const target=document.getElementById(focusId);if(target&&!target.disabled)target.focus({preventScroll:true});}
}
function customerConfigurationLink(){
 const link=$('[data-customer-configure]');if(!link)return;
 link.hidden=true;link.removeAttribute('href');
 if(!product?.publicSlug||product.availability!=='available'||!currency)return;
 const target=safeURL('/account/products/'+encodeURIComponent(product.publicSlug)+'/configure?'+new URLSearchParams({currency}),true);
 if(target&&target.hostname===location.hostname&&target.protocol===location.protocol){link.href=target.href;link.target='_blank';link.rel='noopener';link.hidden=false;}
}
function purchaseControls(counts={}){
 const slot=$('[data-purchase-controls]');slot.replaceChildren();
 customerConfigurationLink();
 for(const [name,value]of [['quantity','1'],['cycles',counts.cycles||'1']]){const input=node('input');input.type='hidden';input.name=name;input.id='purchase-'+name;input.value=value;slot.append(input);}
}
function validPurchaseCounts(){const quantity=Number($('[name=quantity]')?.value),cycles=Number($('[name=cycles]')?.value);return Number.isInteger(quantity)&&quantity>=1&&quantity<=1000&&Number.isInteger(cycles)&&cycles>=1&&cycles<=120&&cycles>=(chosenTerm?.minimumPurchaseCycles||1);}
function updateAdd(){$('[data-add]').disabled=busy||!valid||!quote||!chosenTerm||chosenTerm.status!=='final'||!validPurchaseCounts();}
function renderBreakdown(term){
 const summary=$('[data-total-summary]'),amount=$('[data-total-amount]'),discount=$('[data-total-discount]'),breakdown=$('[data-breakdown]');
 if(!summary||!amount||!breakdown)return;
 const toggle=$('[data-breakdown-toggle]');
 summary.hidden=false;
 if(!term||term.totalAmount===undefined||term.totalAmount===null){amount.textContent='待报价';discount.textContent='请先完成配置并选择付款周期';breakdown.replaceChildren();breakdown.hidden=true;if(toggle){toggle.disabled=true;toggle.setAttribute('aria-expanded','false');}return;}
 if(toggle)toggle.disabled=false;
 summary.hidden=false;amount.textContent=money(term.totalAmount,quote.currencyCode).replace(/^CNY /,'¥');
 const setup=BigInt(term.setupAmount||0),subtotal=BigInt(term.subtotalAmount||0),tax=BigInt(term.taxAmount||0),total=BigInt(term.totalAmount||0);
 discount.textContent=setup===0n?'无折扣':'含安装费';breakdown.replaceChildren();
 for(const [labelText,value] of [['商品小计',subtotal],['安装费',setup],['税费',tax],['应付合计',total]]){const row=node('div',undefined,'lina-cost-row');row.append(node('span',labelText),node('strong',money(value.toString(),quote.currencyCode).replace(/^CNY /,'¥')));breakdown.append(row);}
}
function renderQuote(){
 const slot=$('[data-quote]');slot.replaceChildren();
 const periodTitle=slot.closest('.sky-config-row')?.querySelector('.label-title span');if(periodTitle)periodTitle.textContent='周期';
 const terms=(quote.terms||[]).map((term,index)=>({term,index,period:billingPeriod(term)})).sort((a,b)=>a.period.rank-b.period.rank||a.index-b.index).map(item=>item.term);
 for(const term of terms){
  const period=billingPeriod(term),text=period.text+(term.status==='final'?'':'：'+quoteStateText(term.status));
  if(term.status==='requires_customer_context'){
   const login=node('button',text,'btn sky-config-btn');login.type='button';login.addEventListener('click',async()=>{rememberPurchase();try{if(await signIn())await evaluate();}catch(error){status(error.message);}});slot.append(login);continue;
  }
  const b=node('button',text,'btn sky-config-btn');b.type='button';b.dataset.billingTermToken=term.billingTermToken;b.setAttribute('aria-pressed','false');b.disabled=busy||term.status!=='final';b.addEventListener('click',()=>{chosenTerm=term;preferredTerm=term;needsExplicitTerm=false;for(const x of slot.children){x.classList.remove('active');if(x.tagName==='BUTTON')x.setAttribute('aria-pressed','false');}b.classList.add('active');b.setAttribute('aria-pressed','true');const cycles=$('[name=cycles]');cycles.min=String(term.minimumPurchaseCycles);if(Number(cycles.value)<term.minimumPurchaseCycles)cycles.value=String(term.minimumPurchaseCycles);const price=$('[data-selected-price]');if(price)price.textContent=money(term.totalAmount,quote.currencyCode)+' / '+billingPeriod(term).text+'（单件）';renderBreakdown(term);updateAdd();});slot.append(b);
 }
 const preferred=preferredTerm&&terms.filter(term=>term.billingTermToken===preferredTerm.billingTermToken||(term.labelZhCN===preferredTerm.labelZhCN&&term.labelEnUS===preferredTerm.labelEnUS));
 const first=needsExplicitTerm?undefined:preferredTerm?(preferred?.length===1?preferred[0]:undefined):terms.find(term=>term.status==='final');
 if(first?.status==='final'){
  chosenTerm=first;preferredTerm=first;const button=[...slot.children].find(item=>item.dataset.billingTermToken===first.billingTermToken);
  if(button){button.classList.add('active');button.setAttribute('aria-pressed','true');}
  const cycles=$('[name=cycles]');if(cycles){cycles.min=String(first.minimumPurchaseCycles);if(Number(cycles.value)<first.minimumPurchaseCycles)cycles.value=String(first.minimumPurchaseCycles);}
  renderBreakdown(first);const price=$('[data-selected-price]');if(price)price.textContent=money(first.totalAmount,quote.currencyCode)+' / '+billingPeriod(first).text+'（单件）';updateAdd();
 }
 status(quote.terms?.length?(configurationNotice||(!chosenTerm?(getLocale()==='en-US'?'Select a billing period before saving.':'请选择付款周期后保存。'):'')):'当前没有可用的结算周期。');
}
async function evaluate(){
 const seq=++sequence,identity=getAccessToken();invalidate();status('正在检查配置并报价…');
 try{applyConfigurationDefaults(configuration);const result=await request(productPath()+'/purchase-configuration/evaluate',{configurationRevision:configuration.configurationRevision,values:configuration.values});if(seq!==sequence)return;configuration=result;renderFields();
  if(!result.valid){status('配置尚未完整或不符合要求，请检查选择。');return;}
  const priced=await request(productPath()+'/purchase-configuration/quote',{configurationRevision:result.configurationRevision,currencyCode:currency,values:result.values});if(seq!==sequence)return;
  if(priced.configurationRevision!==result.configurationRevision||priced.currencyCode!==currency)throw new Error('stale');if(identity!==getAccessToken())return evaluate();quote=priced;quoteIdentity=identity;valid=true;renderQuote();
 }catch{if(seq===sequence){invalidate();status('配置或报价不可用，商品可能已变更。请重新读取配置。');}}
}
const cachedCatalog=readCatalogCache();
let configurationCatalogProducts=cachedCatalog?.products||[];
let configurationCategoryItems=cachedCatalog?.categories||null;
function configurationModels(current){
 const group=$('[data-models]');if(!group)return;
 const notice=$('[data-catalog-usage-notice]');
 group.replaceChildren();
 const modelRow=row('产品型号'),list=modelRow.body;list.classList.add('lina-configure-model-list','sky-btn-group','basicConfigurationItems');list.setAttribute('aria-label','当前商品型号');
 group.classList.add('sky-form-group','sky-config-row','configureproduct');group.append(...modelRow.r.children);
 const selected=node('a',label(current),'btn sky-config-btn-three active lina-configure-model');selected.href='/configure.html?product='+encodeURIComponent(current.publicSlug);selected.setAttribute('aria-current','page');list.append(selected);
 prependServerFlag(selected,current);
 if(notice){const texts=[...notice.querySelectorAll('p')],content=node('span',undefined,'form-group-text-content');content.append(...texts);notice.className='form-group-text lina-configure-notice';const icon=node('span','!','lina-usage-icon');icon.setAttribute('aria-hidden','true');notice.replaceChildren(icon,content);list.append(notice);}
}
async function loadConfigurationCategories(){
 const slot=$('[data-categories]');if(!slot)return;
 if(configurationCategoryItems)categories(configurationCategoryItems,product?.categoryNo||'');else slot.replaceChildren(node('p','正在加载商品分类…'));
  try{const data=await request(base+'/catalog/products?limit=1');configurationCategoryItems=data.categories||[];mergeCatalogCache({categories:configurationCategoryItems,products:[]});if(page==='configure')categories(configurationCategoryItems,product?.categoryNo||'');}
  catch{if(page!=='configure'||configurationCategoryItems)return;renderConfigurationCategory([], '');slot.replaceChildren(node('p','商品分类加载失败，请重试。'));const button=node('button','重新加载商品分类','btn sky-config-btn');button.type='button';button.addEventListener('click',loadConfigurationCategories);slot.append(button);}
}
async function configure(){
 for(const price of document.querySelectorAll('.shopping-cart-product [data-selected-price]'))price.remove();
 
 const customerConfiguration=$('[data-customer-configure]');if(customerConfiguration){customerConfiguration.hidden=true;customerConfiguration.removeAttribute('href');}
 for(const name of ['quantity','cycles']){const input=$('[name='+name+']');if(input)purchaseCounts[name]=input.value;}
 const seq=++sequence;clearTimeout(timer);invalidate();status('正在读取商品配置…');
  // The notice is moved into the model list; preserve it before resetting that list.
  const models=$('[data-models]'),usageNotice=$('[data-catalog-usage-notice]');
  if(models?.contains(usageNotice))models.before(usageNotice);
  $('[data-fields]').replaceChildren();$('[data-models]')?.replaceChildren();$('[data-purchase-controls]').replaceChildren();$('[data-product-summary]').textContent='';renderConfigurationCategory(configurationCategoryItems||[],'');
 try{const [detail,view]=await Promise.all([request(base+'/catalog/products/'+encodeURIComponent(params.get('product')||'')),request(base+'/catalog/products/'+encodeURIComponent(params.get('product')||'')+'/purchase-configuration')]);if(seq!==sequence)return;product=detail.product;if(configurationCategoryItems)categories(configurationCategoryItems,product.categoryNo||'');if(product.availability!=='available'){status('商品当前不可订购。');return;}const previous=configuration;if(previous?.configurationRevision===view.configurationRevision){configuration=view;configuration.values=structuredClone(previous.values);}else{configuration=view;}currency=product.currencies?.[0]||'';const title=$('.allocation-header-title h1');title.textContent=label(product);prependServerFlag(title,product);$('[data-product-summary]').textContent=localized(product,'summary');restorePurchaseDraft();renderFields();purchaseControls(purchaseCounts);await initializeLogin();if(!currency){status('此商品尚未提供可用币种。');return;}await evaluate();}catch{if(seq===sequence)status('无法读取商品配置，请重试。');}
}
async function add(event){
 event.preventDefault();if(busy||!valid||!quote||!chosenTerm||chosenTerm.status!=='final'||!validPurchaseCounts()||!$('#addCartForm').reportValidity())return;
 busy=true;clearTimeout(timer);updateAdd();status(getLocale()==='en-US'?'Adding to cart…':'正在加入购物车…');
 try{
  rememberPurchase();if(!(await signIn())){loginControls?.clearPassword();return;}
  if(quoteIdentity!==getAccessToken()){busy=false;await evaluate();status(getLocale()==='en-US'?'Review the current price, then add to cart.':'已更新账号价格，请核对后加入购物车。');return;}
  if(editingItem?.quantity>1&&loginControls?.hasInput())throw new Error(getLocale()==='en-US'?'Set this item to one server in the cart, then configure the others separately.':'请先在购物车把该项数量改为一台，其余服务器分别配置。');
  const serverLoginToken=await loginControls?.persist();rememberPurchase();
  const selected={quantity:1,purchaseCycles:Math.max(chosenTerm.minimumPurchaseCycles||1,Number($('[name=cycles]').value)||1),publicSlug:product.publicSlug,configurationRevision:configuration.configurationRevision,billingTermToken:chosenTerm.billingTermToken,currencyCode:quote.currencyCode,values:structuredClone(configuration.values),...(serverLoginToken?{serverLoginToken}:{})};
  if(params.get('quoteItemNo')){
   await commerce('account/commerce/quote-items/'+encodeURIComponent(params.get('quoteItemNo'))+'/purchase-configuration','POST',{publicSlug:selected.publicSlug,configurationRevision:selected.configurationRevision,values:selected.values,...(serverLoginToken?{serverLoginToken}:{})});
  }else{
   let result;
   if(params.get('cartItemNo')){if(!editingItem)throw new Error(getLocale()==='en-US'?'Cart item is no longer available.':'购物车项目已变更，请重新打开。');result=await commerce('account/commerce/cart-items/'+encodeURIComponent(editingItem.cartItemNo),'PUT',{expectedCartVersion:editingCartVersion,expectedItemVersion:editingItem.version,quantity:editingItem.quantity,purchase:selected});}
   else{const fingerprint=JSON.stringify([getAccessToken(),selected]);if(cartAttempt?.fingerprint!==fingerprint){const {cart}=await commerce('account/commerce/carts','POST',{currencyCode:selected.currencyCode});cartAttempt={fingerprint,expectedCartVersion:cart.version};}result=await commerce('account/commerce/cart-items','POST',{...selected,expectedCartVersion:cartAttempt.expectedCartVersion});}
   const item=result.cart?.items?.find(item=>item.serverLoginToken&&item.serverLoginToken===serverLoginToken)||(params.get('cartItemNo')?result.cart?.items?.find(item=>item.cartItemNo===params.get('cartItemNo')):result.cart?.items?.at(-1));
   await rememberCart(result.cart);
   if(item?.configurationReference)savePurchaseDraft(item.configurationReference,{...selected,...loginControls?.snapshot(),summary:configurationSummary(),termLabelZhCN:chosenTerm.labelZhCN,termLabelEnUS:chosenTerm.labelEnUS,locale:getLocale()});
  }
  sessionStorage.removeItem(restorePurchaseKey());location.assign('/cart.html?lang='+getLocale());
 }catch(error){status(error.message||(getLocale()==='en-US'?'Could not add the item. Check the cart before retrying.':'操作未完成，请检查购物车后重试。'));}
 finally{busy=false;updateAdd();}
}
const restorePurchaseKey=()=> 'website-main-configure:'+String(params.get('product')||'')+':'+String(params.get('cartItemNo')||params.get('quoteItemNo')||'');
function purchaseDrafts(){try{return JSON.parse(sessionStorage.getItem('website-main-purchase-drafts')||'{}');}catch{return {};}}
function savePurchaseDraft(reference,draft){try{const drafts=purchaseDrafts();drafts[reference]=draft;sessionStorage.setItem('website-main-purchase-drafts',JSON.stringify(Object.fromEntries(Object.entries(drafts).slice(-100))));}catch{}}
function configurationSummary(){return (configuration?.fields||[]).filter(field=>field.visible).map(field=>{const value=configuration.values.find(value=>value.fieldToken===field.fieldToken),tokens=value?.optionTokens||value?.treeOptionTokens||value?.addressOptionTokens,all=[];const visit=options=>{for(const option of options||[]){all.push(option);visit(option.children);}};visit(field.options);const selected=all.filter(option=>tokens?.includes(option.optionToken));const show=lang=>tokens?selected.map(option=>option['label'+lang]||option.labelZhCN||option.labelEnUS).join(', '):String(value?.integerValue??value?.decimalValue??value?.textValue??value?.booleanValue??value?.dateValue??value?.dateTimeValue??'');return {labelZhCN:field.labelZhCN,labelEnUS:field.labelEnUS,valueZhCN:show('ZhCN'),valueEnUS:show('EnUS')};});}
function rememberPurchase(){try{sessionStorage.setItem(restorePurchaseKey(),JSON.stringify({configurationRevision:configuration.configurationRevision,values:configuration.values,...loginControls?.snapshot(),passwordNeedsReentry:loginControls?.passwordNeedsReentry()}));}catch{}}
function restorePurchaseDraft(){try{const draft=JSON.parse(sessionStorage.getItem(restorePurchaseKey())||'null');if(draft?.configurationRevision===configuration.configurationRevision)configuration.values=structuredClone(draft.values);}catch{}}
async function initializeLogin(){
 let container=$('[data-server-login]');if(!container||!loginControls){container?.remove();container=node('section');container.dataset.serverLogin='';container.className='lina-server-login';$('[data-fields]').after(container);loginControls=createServerLogin(container,input=>commerce('account/catalog/products/'+encodeURIComponent(product.publicSlug)+'/purchase-configuration/server-login','POST',{configurationRevision:configuration.configurationRevision,...input}));}
 let draft={};try{draft=JSON.parse(sessionStorage.getItem(restorePurchaseKey())||'{}');}catch{}
 if(params.get('cartItemNo')||params.get('quoteItemNo')){
  if(await syncSession()){
   const {cart}=await commerce('account/commerce/cart');editingCartVersion=cart?.version;
   const quoteItem=cart?.latestQuote?.items?.find(item=>item.quoteItemNo===params.get('quoteItemNo'));
   editingItem=cart?.items?.find(item=>item.cartItemNo===(params.get('cartItemNo')||quoteItem?.cartItemNo));
   if(editingItem){const saved=purchaseDrafts()[editingItem.configurationReference];currency=cart.currencyCode;purchaseCounts.cycles=String(editingItem.purchaseCycles||1);$('[name=cycles]').value=purchaseCounts.cycles;preferredTerm=undefined;needsExplicitTerm=true;configurationNotice='';if(saved?.configurationRevision===configuration.configurationRevision){configuration.values=structuredClone(saved.values);preferredTerm={billingTermToken:saved.billingTermToken,labelZhCN:saved.termLabelZhCN,labelEnUS:saved.termLabelEnUS};needsExplicitTerm=false;renderFields();}else{configurationNotice=getLocale()==='en-US'?'Previous choices could not be restored. Review all options and select a billing period before saving.':'原选项无法恢复，请重新核对所有配置并选择付款周期后保存。';}draft={...editingItem,...draft};}
  }
 }
 loginControls.render(configuration.serverLoginPolicy,draft,String(product.publicSlug)+':'+configuration.configurationRevision+':'+String(params.get('cartItemNo')||params.get('quoteItemNo')||''));if(params.get('cartItemNo'))$('[data-add]').textContent=getLocale()==='en-US'?'Save changes':'保存修改';
}
async function uploadFiles(field,input){
 if(busy)return;busy=true;invalidate();rememberPurchase();
 try{
  if(!(await signIn()))return;const files=[...input.files],policy=field.uploadPolicy;
  if(!policy||files.length>policy.maximumFiles||files.some(file=>file.size>policy.maximumFileBytes)||files.reduce((sum,file)=>sum+file.size,0)>policy.maximumTotalBytes)throw new Error(getLocale()==='en-US'?'Files exceed the allowed size or count.':'文件大小或数量超出允许范围。');
  const fileTokens=[];for(const file of files){const reserved=await commerce('account/catalog/products/'+encodeURIComponent(product.publicSlug)+'/purchase-configuration/file-uploads','POST',{configurationRevision:configuration.configurationRevision,fieldToken:field.fieldToken,fileName:file.name,sizeBytes:file.size});const result=await commerce('account/catalog/purchase-configuration/file-uploads/'+encodeURIComponent(reserved.uploadToken)+'/content','PUT',file);if(result.status!=='ready')throw new Error(getLocale()==='en-US'?'File upload is not ready. Please retry.':'文件尚未就绪，请重试。');fileTokens.push(result.fileToken);}setValue(field,fileTokens);
 }catch(error){status(error.message);input.value='';}finally{busy=false;updateAdd();}
}
let contentSequence=0;
function contentListReturn(path,value){
 if(typeof value!=='string'||!value.startsWith('/')||value.startsWith('//')||/[\\\x00-\x20\x7f]/.test(value))return path;
 try{const u=new URL(value,location.origin);if(u.origin!==location.origin||u.pathname!==path)return path;const query=new URLSearchParams();for(const key of ['query','categoryNo','category'])if(u.searchParams.get(key))query.set(key,u.searchParams.get(key));return path+(query.size?'?'+query:'');}catch{return path;}
}
function articleNavigation(item,article){
 const sections={news:['/news.html','新闻资讯'],announcement:['/announcements.html','公告'],knowledge:['/help.html','帮助文档'],download:['/downloads.html','资源下载']};
 const section=sections[item.type];
 const parent=$('[data-content-parent]'),back=$('[data-content-tool=back]');
 if(parent){parent.hidden=!section;if(section){parent.href=section[0];parent.textContent=section[1];}}
 if(back){back.hidden=!section;if(section)back.href=contentListReturn(section[0],params.get('returnTo'));}
 const breadcrumb=$('[data-content-breadcrumb]');if(breadcrumb)breadcrumb.textContent=item.title;
 for(const el of document.querySelectorAll('[data-content-category]')){el.textContent=label(item.category);el.hidden=!el.textContent;}
 const time=$('[data-content-time]');if(time){time.hidden=!item.publishedAt;time.textContent=item.publishedAt?'发布时间：'+localizedDate(item.publishedAt,true):'';time.parentElement.hidden=time.hidden&&!label(item.category);}
 const directory=$('[data-content-directory]');if(!directory)return;
 directory.replaceChildren();const headings=[...article.querySelectorAll('h1,h2,h3,h4,h5,h6')].filter(el=>el.textContent.trim());
 if(!headings.length){directory.append(node('span','全文','nothing'));return;}
 const list=node('ul',undefined,'catalogbox');
 for(const [index,heading] of headings.entries()){
  heading.id='content-heading-'+index;
  const li=node('li',undefined,'level-'+Math.min(5,Number(heading.tagName[1]))),link=node('a',heading.textContent);
  link.href=location.pathname+location.search+'#'+heading.id;link.addEventListener('click',()=>{for(const other of list.children)other.classList.toggle('current-catalog',other===li);});li.append(link);list.append(li);
 }directory.append(list);
}
async function content(append=false){
 if(page==='announcement'){document.querySelector('[data-content-search]')?.remove();params.delete('query');}
 retry();const seq=++contentSequence,slot=$('[data-content-list]'),more=$('[data-more]');more.disabled=true;if(!append)slot.replaceChildren();if(page==='content')$('[data-content-body]').replaceChildren();status('正在加载内容…');
 try{
  if(page==='content'){
   $('[data-content-search]')?.remove();more.hidden=true;const {item}=await readPublicContentDetail(request,params.get('id')||'');if(seq!==contentSequence)return;for(const title of document.querySelectorAll('[data-page-title]'))title.textContent=item.title;
   contentMetadataReady=true;document.title=item.seoTitle||item.title;const article=$('[data-content-body]');article.replaceChildren();let meta=$('meta[name=description]');if(!meta){meta=node('meta');meta.name='description';document.head.append(meta);}meta.content=item.seoDescription||item.summary||'';
   // Public content may contain HTML. Parse inertly and allow only presentation nodes.
   const parsed=new DOMParser().parseFromString(item.body||'','text/html');const allowed=new Set(['P','BR','STRONG','EM','UL','OL','LI','H1','H2','H3','H4','H5','H6','BLOCKQUOTE','PRE','CODE','TABLE','THEAD','TBODY','TR','TH','TD','A','IMG','HR']);
   const clean=(src,target)=>{for(const c of src.childNodes){if(c.nodeType===3)target.append(document.createTextNode(c.textContent));else if(c.nodeType===1&&!['SCRIPT','STYLE','IFRAME','OBJECT','SVG','FORM'].includes(c.tagName)){if(allowed.has(c.tagName)){const el=node(c.tagName.toLowerCase());if(['TH','TD'].includes(c.tagName)){for(const key of ['colspan','rowspan']){const value=c.getAttribute(key);if(value!==null&&/^(0|[1-9]\d{0,4})$/.test(value)){const n=Number(value),max=key==='colspan'?1000:65534;if(n<=max&&(n>0||key==='rowspan'))el.setAttribute(key,value);}}}if(c.tagName==='A'){const u=safeURL(c.getAttribute('href'));if(u){el.href=u.href;el.rel='noopener noreferrer';}}if(c.tagName==='IMG'){const u=safeURL(c.getAttribute('src'));if(!u)continue;el.src=u.href;el.alt=c.getAttribute('alt')||'';el.loading='lazy';}clean(c,el);target.append(el);}else clean(c,target);}}};clean(parsed.body,article);
   articleNavigation(item,article);
   if(item.file){const b=node('button',undefined,'fancy-button bg-gradient1');b.append(node('span','下载 '+item.file.fileName));b.type='button';b.addEventListener('click',async()=>{
    if(b.disabled)return;b.disabled=true;status('正在准备下载…');
    try{const ticket=await request(base+'/content/items/'+encodeURIComponent(item.contentNo)+'/download-tickets',{});
     if(typeof ticket.token!=='string'||!ticket.token.trim()||!Number.isFinite(ticket.expiresAt)||ticket.expiresAt<=Date.now())throw new Error('invalid download ticket');
     location.assign(base+'/content/downloads/'+encodeURIComponent(ticket.token));status('已请求下载。若未开始，请重试。');
    }catch{status('下载暂不可用，请重试。');}finally{b.disabled=false;}
   });article.append(b);}status('');return;
  }
  const query=new URLSearchParams({type:page==='download'?'download':page,limit:'20'});if(params.get('query'))query.set('query',params.get('query'));if(append&&more.dataset.cursor)query.set('cursor',more.dataset.cursor);
  const data=await readPublicContentList(request,{type:query.get('type'),query:query.get('query')||'',cursor:query.get('cursor')||'',limit:Number(query.get('limit'))});if(seq!==contentSequence)return;
  for(const item of data.items||[]){
   const a=node('a',undefined,page==='download'?'':'news-link');const returnTo=contentListReturn(location.pathname,location.pathname+location.search);a.href='/content.html?'+new URLSearchParams({id:item.contentNo,returnTo});
   if(page==='download'){
    const tr=node('tr'),title=node('td'),published=node('td',item.publishedAt?localizedDate(item.publishedAt):'');
    a.textContent=item.title;title.append(a);const category=label(item.category);if(category)title.append(node('span',category,'lina-content-category'));tr.append(title,published);slot.append(tr);continue;
   }
   const li=node('li',undefined,'news-item'),wrap=node('div',undefined,'news-item-container'),text=node('div',undefined,'news-item-text'),title=node('div',undefined,'news-title');title.append(node('h1',item.title));text.append(title,node('p',item.summary||'','news-description'));const category=label(item.category);if(category)text.append(node('span',category,'lina-content-category'));wrap.append(text);if(item.publishedAt){const meta=node('div',undefined,'news-articleInformation');meta.append(node('p',localizedDate(item.publishedAt),'news-create-time'));wrap.append(meta);}a.append(wrap);li.append(a);slot.append(li);
  }status(slot.children.length?'':'暂无已发布内容。');more.dataset.cursor=data.nextCursor||'';more.hidden=!data.nextCursor;
 }catch{if(seq===contentSequence){status('内容加载失败，请重试。');more.hidden=true;retry(()=>content(append));}}finally{if(seq===contentSequence)more.disabled=false;}
}
for(const tool of document.querySelectorAll('[data-content-tool]')){
 const action=tool.dataset.contentTool;if(action==='back')continue;
 const run=async()=>{if(action==='top'){window.scrollTo({top:0,behavior:matchMedia('(prefers-reduced-motion: reduce)').matches?'instant':'smooth'});return;}
  const copyRevision=++statusRevision;
  try{await navigator.clipboard.writeText(location.href);if(copyRevision===statusRevision)status('链接已复制，可以粘贴分享。',true);}catch{if(copyRevision===statusRevision)status('未能复制链接，请复制浏览器地址栏中的网址。',true);}};
 tool.addEventListener('click',run);tool.addEventListener('keydown',event=>{if(['Enter',' '].includes(event.key)){event.preventDefault();run();}});
}
for(const a of document.querySelectorAll('[data-customer]'))customerLink(a,a.dataset.customer);
if(page==='home')initializeHomeBlocks(document,{safeURL,customerLink});
if(!['products','configure'].includes(page))$('[data-more]')?.addEventListener('click',()=>page==='home'?catalog(true):content(true));
const contentSearch=$('[data-content-search]');if(contentSearch){$('[name=query]',contentSearch).value=params.get('query')||'';contentSearch.addEventListener('submit',e=>{e.preventDefault();const value=String(new FormData(e.currentTarget).get('query')||'').trim();if(value)params.set('query',value);else params.delete('query');const search=params.toString();history.replaceState(null,'',location.pathname+(search?'?'+search:''));content();});}
$('.lina-menu-toggle')?.addEventListener('click',e=>{const open=$('.nav-layoutcontainer').classList.toggle('lina-mobile-open');e.currentTarget.setAttribute('aria-expanded',String(open));});
const initializedCommercePanels=new WeakSet();
function initializeCommercePanel(){
 const panel=$('.shopping-cart-product');if(initializedCommercePanels.has(panel))return;initializedCommercePanels.add(panel);
 const login=$('[data-catalog-login]',panel);if(login){login.closest('.layui-elem-quote').hidden=Boolean(getAccessToken());const paragraph=login.closest('p');paragraph.replaceChildren(document.createTextNode('您可点此，'),login,document.createTextNode('，登录后根据您的账号情况展示适用价格。'));login.textContent='登录';customerLink(login,'/auth/login');}
 for(const a of panel.querySelectorAll('[data-customer]'))customerLink(a,a.dataset.customer);
 $('[data-refresh]')?.addEventListener('click',configure);$('#addCartForm')?.addEventListener('submit',add);
 const refresh=$('[data-refresh]');if(refresh){const feedback=$('[data-status]');const sync=()=>{refresh.hidden=!feedback.textContent||!/(失败|不可用|无法读取|已变更|unavailable|failed|could not|reload)/i.test(feedback.textContent);};new MutationObserver(sync).observe(feedback,{childList:true,characterData:true,subtree:true});sync();}
 $('[data-more]')?.addEventListener('click',()=>catalog(true));
 $('[data-breakdown-toggle]')?.addEventListener('click',e=>{const breakdown=$('[data-breakdown]');if(!breakdown)return;const open=breakdown.hidden;breakdown.hidden=!open;e.currentTarget.setAttribute('aria-expanded',String(open));});
 const card=$('.allocation-card'),footer=$('.allocation-footer');
 if(card&&footer){
  const align=()=>{if(!card.isConnected)return;const r=card.getBoundingClientRect(),side=$('.shopping-cart-side .sky-cart-menu'),mobile=innerWidth<768,left=mobile?0:Math.max(0,side?side.getBoundingClientRect().right:r.left);footer.style.setProperty('--lina-settlement-left',left+'px');footer.style.setProperty('--lina-settlement-width',(mobile?document.documentElement.clientWidth:Math.max(0,document.documentElement.clientWidth-left))+'px');card.style.setProperty('--lina-settlement-height',footer.getBoundingClientRect().height+'px');};
  new ResizeObserver(align).observe(footer);new ResizeObserver(align).observe(card);window.addEventListener('resize',align);align();
 }
}
async function activateCommerce(view,url){
 const previousSlug=params.get('product');catalogPage?.cancel();sequence++;clearTimeout(timer);product=undefined;
 for(const key of [...params.keys()])params.delete(key);for(const [key,value]of url.searchParams)params.append(key,value);
 page=view;preferredTerm=undefined;needsExplicitTerm=false;configurationNotice='';cartAttempt=undefined;editingItem=undefined;initializeCommercePanel();
 if(productSearch)productSearch.value=params.get('namePrefix')||'';if(navigationInput)navigationInput.value=params.get('namePrefix')||'';
 if(view==='products'){
  catalogPage??=makeCatalogPage();await catalog();
 }else{
  if(previousSlug!==params.get('product')){loginControls=undefined;editingItem=undefined;configuration=undefined;purchaseCounts.quantity='1';purchaseCounts.cycles='1';for(const name of ['quantity','cycles']){const input=$('[name='+name+']');if(input)input.value='1';}}
  const back=$('.nav-top-page .return');if(back)back.href='/products.html'+(params.get('categoryNo')?'?categoryNo='+encodeURIComponent(params.get('categoryNo')):'');
  void loadConfigurationCategories();await configure();
 }
}
let searchTimer;
const productSearch=$('#search-input');
const navigationSearch=$('.lina-nav-search');
const navigationInput=$('[name=namePrefix]',navigationSearch||document);
function searchProducts(value){
 clearTimeout(searchTimer);value=value.trim();
 const query=new URLSearchParams();if(params.get('categoryNo'))query.set('categoryNo',params.get('categoryNo'));if(value)query.set('namePrefix',value);
 const suffix=query.size?'?'+query:'';
 if(commerceNavigation){void commerceNavigation.navigate('/products.html'+suffix);return;}
 if(page!=='products'){location.assign('/products.html'+suffix);return;}
 if(productSearch)productSearch.value=value;if(navigationInput)navigationInput.value=value;
 if(catalogPage){catalogPage.search(value);return;}
 params.delete('namePrefix');if(value)params.set('namePrefix',value);
 history.replaceState(null,'',location.pathname+suffix);catalog();
}
if(navigationInput)navigationInput.value=params.get('namePrefix')||'';
attachNavigationSearch(document,{searchProducts,navigate:path=>location.assign(path)});
if(productSearch){
 productSearch.value=params.get('namePrefix')||'';
 productSearch.addEventListener('keydown',e=>{if(e.key==='Enter'){e.preventDefault();searchProducts(productSearch.value);}});
}
const updateSearchCategories=productSearch?attachCatalogSearch({input:productSearch,navigationInput,searchProducts,document}):null;
async function loadBrand(){
 try{await brand();$('.lina-brand-error')?.remove();}
 catch{
  if(brandLoaded)return;
  let message=$('.lina-brand-error');if(!message){message=node('div',undefined,'lina-brand-error');$('.nav-layoutcontainer').after(message);}
  const text='网站资料暂时无法加载，请重试。';message.replaceChildren(node('p',text));
  const button=node('button','重新加载网站资料','btn sky-config-btn');button.type='button';button.setAttribute('data-brand-retry','');button.addEventListener('click',()=>refreshBrand());message.append(button);
  if(['contact','about'].includes(page))status(text);
 }
}
async function refreshProductPresentation(){
 const slug=params.get('product'),currentPage=page;if(!slug||!['products','configure'].includes(page))return;
 try{
  const {product:current}=await request(base+'/catalog/products/'+encodeURIComponent(slug));if(!current||current.publicSlug!==slug||params.get('product')!==slug||page!==currentPage)return;
  if(page==='configure')product=current;
  const updateName=element=>{if(!element)return;element.textContent=label(current);prependServerFlag(element,current);};
  updateName($('[data-products] [data-product="'+CSS.escape(slug)+'"] h5'));
  const menuName=$('[data-catalog-product="'+CSS.escape(slug)+'"] .lina-product-name');if(menuName)menuName.textContent=label(current);
  const heading=$('[data-catalog-title]');if(heading)renderCommerceHeading(document,{title:label(current),description:localized(current,'summary'),back:heading.closest('.nav-top-page-title')?.querySelector('.return')?.href||'/products.html'});
  if(page==='configure'){
   updateName($('.allocation-header-title h1'));
   updateName($('[data-models] .lina-configure-model'));
   $('[data-product-summary]').textContent=localized(current,'summary');
  }
 }catch{}
}
async function refreshWebsitePresentation(){await Promise.all([loadBrand(),refreshProductPresentation()]);}
const refreshBrand=startBrandRefresh(document,refreshWebsitePresentation);
const cachedNavigation=readNavigationCache();
if(cachedNavigation){
 renderNavigation(document,cachedNavigation,{safeURL,customerLink,request});
}
window.addEventListener('storage',event=>{if(event.key!==navigationCacheKey)return;const items=readNavigationCache();if(items)renderNavigation(document,items,{safeURL,customerLink,request});});
window.addEventListener('storage',event=>{if(event.key===catalogCacheKey)document.dispatchEvent(new CustomEvent('lina:catalog-cache-updated'));});
if(page==='home'){const news=()=>loadHomeNews(document,request,safeURL);$('[data-home-news-retry]').addEventListener('click',news);news();}
if(page==='activity')initializeActivityPage(document);
if(page==='home')startServerHero(document,request);
if(['products','configure'].includes(page)){commerceNavigation=createCommerceNavigation({document,activate:activateCommerce,isBusy:()=>busy,status,retry});void commerceNavigation.start();}
else if(page==='home')catalog();else if(!['cart','terms','contact','about','activity','webmaster','affiliate','electricity','web-solution','game','safety','goingsea','financial','government','partner','assure','certificate','bt'].includes(page)){for(const title of document.querySelectorAll('[data-page-title]'))title.textContent={news:'新闻资讯',announcement:'公告',knowledge:'帮助文档',content:'内容详情'}[page]||'';content();}





if(['configure','products'].includes(page))void syncSession().then(()=>{const banner=$('[data-catalog-login]')?.closest('.layui-elem-quote');if(banner)banner.hidden=Boolean(getAccessToken());if(page==='configure'&&configuration&&quoteIdentity!==getAccessToken())void evaluate();}).catch(()=>{});
