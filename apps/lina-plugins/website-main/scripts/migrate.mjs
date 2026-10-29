// Offline source migration. Never evaluates source scripts or contacts the source site.
import {readFile,writeFile,readdir,mkdir,copyFile,unlink} from 'node:fs/promises';
import {resolve,dirname,relative,extname} from 'node:path';
import {createHash} from 'node:crypto';
import {parseHTML} from 'linkedom';
import {contentTemplate} from './content-template.mjs';
import {companyTemplate} from './company-template.mjs';
import {navigationToolsTemplate} from './navigation-template.mjs';
const root=resolve(import.meta.dirname,'..');
if(!process.argv[2])throw new Error('An explicitly authorized local page directory is required');
const source=resolve(process.argv[2]);
const output=resolve(root,'frontend/public');
await mkdir(output,{recursive:true});await mkdir(resolve(root,'docs'),{recursive:true});
const hash=b=>createHash('sha256').update(b).digest('hex');
// Reviewed against the local originals, not inferred from filenames. Other images
// include source-domain watermarks, commercial claims, logos or remain unreviewed.
const reviewedImages=new Set(['remoteL16850667456953.svg','remoteL16905328288110.svg','remoteL17054789546267.svg','Hotline.svg','remoteL17092453231715.svg','remoteL17092453245142.svg','remoteL17092453245429.svg']);
// User approved these three additional original footer SVG files.
for(const name of ["remoteL16905328285515.svg","remoteL16905328293623.svg","remoteL16905328292905.svg"])reviewedImages.add(name);
// Reviewed neutral contact channel glyphs; parent cards still require public contact data.
for(const name of ['telephone.svg','mailbox.svg'])reviewedImages.add(name);
// Licensed neutral sidebar replacements; original Lucide geometry is retained.
const sidebarAssets=['sidebar-newspaper.svg','sidebar-megaphone.svg','sidebar-book-open.svg','sidebar-download.svg','sidebar-icons-LICENSE.txt'];
for(const name of sidebarAssets)reviewedImages.add(name);
const resourceDecisions=[];
function sanitizeCSS(css,file){
 return css.replace(/--(?:waplogo|whitelogo|logo)\s*:[^;}]+;?/gi,'').replace(/url\(\s*(?:"([^"]*)"|'([^']*)'|([^)]*))\s*\)/gi,(all,a,b,c)=>{
  const url=(a??b??c).trim();
  if(url.startsWith('data:')||/^(?:\.\.\/)?(?:static\/)?font\//.test(url)||/\.(?:woff2?|ttf|otf|eot)(?:[?#]|$)/i.test(url)&&!/^https?:/i.test(url))return all;
  const name=url.split('/').pop();const keep=!/^https?:|^\/\//i.test(url)&&reviewedImages.has(name);
  resourceDecisions.push({file,url,decision:keep?'保留：人工检查为中性背景':'不发布：远程依赖、原品牌水印或未审计资源'});return keep?all:'none';
 });
}
// Preserve the source component tree for unavailable sections while removing
// source business content, links and identifiers. The renderer may hide the
// section until an authoritative public contract is available.
function scrubTemplate(node){
 const clearText=el=>{for(const child of [...el.childNodes]){if(child.nodeType===3)child.textContent='';else if(child.nodeType===1)clearText(child);}};
 for(const el of [node,...node.querySelectorAll('*')]){
  el.removeAttribute('id');
  for(const attr of [...el.attributes]){
   if(attr.name==='href'||attr.name==='src'||attr.name==='srcset'||attr.name.startsWith('on'))el.removeAttribute(attr.name);
  }
  if(el.matches('img,svg,video,iframe,source')){el.replaceChildren();el.removeAttribute('alt');el.removeAttribute('title');}
 }
 clearText(node);
}
// User confirmed source text and footer SVG use for these three static displays only.
const staticFeatures=[["业务运营优化","数据分析与挖掘","安全防护"],["高质量游戏服务","全流程解决方案","优化玩家体验"],["防火墙","DDoS防御","安全审计"],["稳定的托管服务","高效的网站性能","可扩展的解决方案"],["专业的金融服务","稳定、安全的云服务","数字化转型"],["数据安全保障","高性能稳定性","可扩展的服务能力","严格的数据隐私保护"],["大规模场景支撑","强安全、高可用、高性能","信创建设化、数据要素化、计算边缘化","云边保障、数据安全"]],staticTags=[["快速部署","成本优化","业务弹性"],["灵活选择","定制化服务","多系统支持"],["数据保护","网络隔离","访问管理","安全加固"],["一键操作","直观界面","管理便捷","高效运维","实时监控"]],staticFooter=[{"title":"全天候品质服务","image":"static/picture/remoteL16905328288110.svg"},{"title":"极速服务应答","image":"static/picture/remoteL16905328285515.svg"},{"title":"客户价值为先","image":"static/picture/remoteL16905328293623.svg"},{"title":"全方位安全保障","image":"static/picture/remoteL16905328292905.svg"}];
function restoreStaticDisplays(d){
 d.querySelectorAll('.solutionv-card-features').forEach((list,i)=>{list.querySelectorAll('li').forEach((n,j)=>{if(j>=staticFeatures[i].length){n.remove();return;}n.textContent=staticFeatures[i][j];});list.closest('.skills-experience').hidden=false;});
 d.querySelectorAll('.chooseUs-itme-label').forEach((list,i)=>{[...list.children].forEach((n,j)=>{n.textContent=staticTags[i][j];});list.hidden=false;});
 d.querySelectorAll('.sky-footer-block').forEach((card,i)=>{const item=staticFooter[i];card.removeAttribute('href');card.removeAttribute('onclick');const img=card.querySelector('img');img.src=item.image;img.alt='';img.hidden=false;card.querySelector('.sky-footer-title').textContent=item.title;});
 const top=d.querySelector('.sky-footer-top');if(top)top.hidden=false;
}

const inventory=[];
for(const file of (await readdir(source)).filter(x=>x.endsWith('.html')).sort()){
 const bytes=await readFile(resolve(source,file));const {document:d}=parseHTML(bytes.toString());
 inventory.push({file,sha256:hash(bytes),title:d.title,forms:[...d.querySelectorAll('form')].map(e=>({id:e.id,action:e.getAttribute('action'),fields:[...e.querySelectorAll('input,select,textarea')].map(n=>({name:n.getAttribute('name'),type:n.getAttribute('type')}))})),actions:[...d.querySelectorAll('a,button,[onclick]')].map(e=>({text:e.textContent.trim().replace(/\s+/g,' ').slice(0,100),href:e.getAttribute('href'),onclick:e.getAttribute('onclick')})),dialogs:[...d.querySelectorAll('[role=dialog],.modal,[class*=popup]')].map(e=>({id:e.id,class:e.className})),scripts:[...d.querySelectorAll('script')].map(e=>({src:e.getAttribute('src'),inlineSha256:e.getAttribute('src')?undefined:hash(e.textContent)}))});
}
await writeFile(resolve(root,'docs/source-inventory.json'),JSON.stringify({source,generatedAt:new Date().toISOString(),pages:inventory},null,2));
const load=async f=>parseHTML(await readFile(resolve(source,f),'utf8')).document;
const catalog=await load('cart-2_7.html');
const homeSource=await load('index.html');
const homeCard=homeSource.querySelector('.systemproduct-item').cloneNode(true);
scrubTemplate(homeCard);
homeCard.removeAttribute('href');
homeCard.querySelector('.systemproduct-tagline img')?.setAttribute('src','static/picture/remoteL16850667456953.svg');
homeCard.querySelector('.systemproduct-tagline img')?.setAttribute('alt','');
homeCard.querySelector('.systemproduct-tag')?.replaceChildren();
homeCard.querySelector('.systemproduct-name').textContent='';homeCard.querySelector('.systemproduct-desc').textContent='';
const card=catalog.querySelector('.product-card').cloneNode(true);
scrubTemplate(card);
card.querySelector('h5').textContent='';card.querySelector('.stock-info').textContent='';card.querySelector('.product-description').textContent='';card.querySelector('.price').textContent='配置后报价';card.querySelector('a').removeAttribute('href');
const navigation=(await load('index.html')).querySelector('.nav-layoutcontainer').cloneNode(true);
scrubTemplate(navigation);
navigation.querySelector('.horizontal-menu-navbar-logo').innerHTML='<a href="/index.html" data-brand-name></a>';
navigation.querySelector('.horizontal-menu-navbar-menu').innerHTML='';
const navSearch=navigation.querySelector('.horizontal-menu-navbar-search');
if(navSearch)navSearch.innerHTML=navigationToolsTemplate();
navigation.insertAdjacentHTML('beforeend','<button class="lina-menu-toggle" type="button" aria-label="展开导航" aria-expanded="false">☰</button>');
const footer=(await load('index.html')).querySelector('.sky-footer-container').cloneNode(true);
scrubTemplate(footer);
// Keep the original footer columns. Source promises/QR codes/qualifications have
// no authoritative equivalent and are unavailable until supplied by the merchant.
footer.querySelector('.sky-footer-top').hidden=true;
footer.querySelectorAll('.sky-footer-icon').forEach(image=>{image.hidden=true;image.removeAttribute('src');image.removeAttribute('srcset');});
footer.querySelectorAll('.sky-footer-block').forEach(card=>{card.removeAttribute('href');card.removeAttribute('onclick');});
footer.querySelector('.sky-footer-left-logo')?.remove();
footer.querySelector('.sky-footer-companyInformation').innerHTML='<div class="sky-footer-address" data-brand-footer></div>';
footer.querySelectorAll('.sky-footer-left-itme,.sky-footer-socials').forEach(e=>e.remove());
footer.querySelector('.sky-footer-left').insertAdjacentHTML('beforeend','<div data-brand-contact></div>');
footer.querySelector('.sky-footer-nav-row').innerHTML='';footer.querySelector('.sky-footer-nav-row').setAttribute('data-footer-navigation','');
footer.querySelector('.sky-footer-bottom').innerHTML='';
restoreStaticDisplays(footer);
const mappings=[['index.html','index.html','home'],['cart-2_7.html','products.html','products'],['cart-configureproduct_128.html','configure.html','configure'],['news.html','news.html','news'],['news.html','announcements.html','announcement'],['help.html','help.html','knowledge'],['109.html','content.html','content'],['downloads.html','downloads.html','download'],['contact.html','contact.html','contact'],['Introduction.html','about.html','about']];
const provenance=[];
for(const [origin,target,kind] of mappings){
 const d=await load(origin);const original=Buffer.from(d.toString());
 // Remove all executable source logic, SEO, old links and body sections before adding data slots.
 const inlineStyles=[...d.querySelectorAll('style')].map(e=>sanitizeCSS(e.textContent,origin));
 d.querySelectorAll('script,meta:not([charset]):not([name=viewport]),base,style,link:not([rel=stylesheet])').forEach(e=>e.remove());
 d.querySelectorAll('link[rel=stylesheet]').forEach(e=>{const href=e.getAttribute('href');if(!href?.startsWith('static/css/'))e.remove();});
 const styles=[...new Set([...d.querySelectorAll('link[rel=stylesheet]')].map(e=>e.getAttribute('href')))];
 // Downloads originated in the source customer theme. The migrated website
 // header/footer retain their original dependencies instead of replacement UI.
 if(kind==='download'){
  for(const href of ['static/css/typo.css','static/css/universalComponents.css','static/css/global.css','static/css/font_4334155_zodnrrjlblp.css','static/css/nav.css','static/css/footer.css','static/css/module.css'])styles.push(href);
  const theme=[...homeSource.querySelectorAll('style')].find(e=>e.textContent.trim().startsWith(':root'));
  if(!theme)throw new Error('Source website theme variables are missing');
  inlineStyles.unshift(sanitizeCSS(theme.textContent,'index.html shared download theme'));
 }
 const keep=kind==='home'?['.Rotation-layoutcontainer','.bannerbelow-layoutcontainer','.systemproduct-layoutcontainer','.container_bg15','section.container_bg1:has(.chooseUs-container)','.container_bg27','.container_bg6','.container_bg30']:kind==='products'||kind==='configure'?['.shopping-cart-page']:kind==='download'?['.sky-container']:['.globalBanner','.et-main'];
 const candidates=keep.map(s=>d.querySelector(s)).filter(Boolean);
 const parts=candidates.filter(part=>!candidates.some(parent=>parent!==part&&parent.contains(part)));
 d.body.replaceChildren(navigation.cloneNode(true),...parts,footer.cloneNode(true));
 d.body.setAttribute('data-page',kind);d.title='官网';
 for(const content of inlineStyles){const style=d.createElement('style');style.textContent=content;d.head.append(style);}
 for(const href of styles)if(!d.head.querySelector(`link[href="${href}"]`)){const link=d.createElement('link');link.rel='stylesheet';link.href=href;d.head.append(link);}
 if(kind==='home'){
  const slides=d.querySelector('.swiper-wrapper');
  const sourceSlides=[...slides.querySelectorAll('.swiper-slide')];
  if(!sourceSlides.length)throw new Error('Source homepage slide structure is missing');
  sourceSlides.forEach((slide,index)=>{scrubTemplate(slide);slide.hidden=index>0;});
  const first=sourceSlides[0];
  first.querySelector('.comp_swiper_img')?.setAttribute('data-hero-image','');
  first.insertAdjacentHTML('beforeend','<div class="comp_container comp_container_auto lina-hero-copy"><h1 data-hero-title></h1><p data-hero-description></p></div>');
  // Keep the original controls and pagination template. A single backend hero
  // hides them; multi-slide data remains an explicit integration gap.
  d.querySelectorAll('.swiper-leftAndRightLayout,.home-pagination-container,.home-button-prev,.home-button-next').forEach(e=>{scrubTemplate(e);e.hidden=true;e.setAttribute('data-hero-control','');});
  const section=d.querySelector('.systemproduct-layoutcontainer');
  section.querySelector('.sk-title').textContent='产品与服务';section.querySelector('.sk-desc').innerHTML='<a href="/products.html">查看全部</a>';
  const cats=section.querySelector('.systemproduct-categories');cats.replaceChildren();cats.setAttribute('data-categories','');
  const productContainer=section.querySelector('.systemproduct-container');
  const sourceGroup=productContainer?.querySelector('.systemproducts');
  if(!sourceGroup)throw new Error('Source homepage product structure is missing');
  const group=sourceGroup.cloneNode(true);
  productContainer.replaceChildren(group);
  const intro=group.querySelector('.systemproduct-left-container');if(intro){scrubTemplate(intro);intro.classList.remove('wow','animate__animated','animate__slideInLeft');intro.classList.add('lina-product-intro-entry');intro.querySelector('.systemproduct-left-top')?.removeAttribute('style');intro.querySelector('.systemproduct-right-bottom')?.remove();intro.insertAdjacentHTML('beforeend','<div class="systemproduct-right-bottom"><div class="systemproduct-right-bottom-title"></div><div class="systemproduct-right-bottom-desc">更多定制机型，为您提供个性化高性能服务器解决方案。</div><div class="systemproduct-right-bottom-button"><a href="/contact.html" class="systemproduct-right-gradient"><span>定制咨询</span></a></div></div>');intro.hidden=false;intro.setAttribute('data-home-product-intro','');}
  const cards=group.querySelector('.systemproduct-right-product');
  cards.replaceChildren();cards.setAttribute('data-products','');
  group.insertAdjacentHTML('beforeend','<p data-status role="status">正在加载商品…</p><button type="button" data-more hidden>加载更多</button>');
   const homeBlocks=[
    ['.bannerbelow-layoutcontainer','首页快捷入口'],
    ['.container_bg15','行业解决方案'],
    ['section.container_bg1:has(.chooseUs-container)','服务能力'],
    ['.container_bg27','资源地域'],
    ['.container_bg6','新闻资讯'],
    ['.container_bg30','合作伙伴']
   ];
   // The public HomepageBlock contract has title/description/href only. Reuse
   // source quick-link cards for that projection, never invent solution/region data.
   const shortcuts=d.querySelector('.bannerbelow-layoutcontainer');
   const shortcutRow=shortcuts.querySelector('.bannerbelow-row');
   const shortcut=shortcutRow.querySelector('.bannerbelow-list').cloneNode(true);
   shortcut.removeAttribute('href');const shortcutIcon=shortcut.querySelector('.bannerbelow-icon-container');scrubTemplate(shortcutIcon);shortcutIcon.hidden=true;shortcutIcon.querySelector('.bannerbelow-item-icon').removeAttribute('style');
   shortcut.querySelector('.bannerbelow-title').textContent='';shortcut.querySelector('.bannerbelow-description').textContent='';
   const shortcutTemplate=d.createElement('template');shortcutTemplate.id='home-block-card';shortcutTemplate.innerHTML=shortcut.outerHTML;
   shortcutRow.replaceChildren();shortcutRow.setAttribute('data-home-blocks','');shortcuts.hidden=true;d.body.append(shortcutTemplate);
   const news=d.querySelector('.container_bg6');
   news.querySelector('.sk-title').textContent='新闻与公告';news.querySelector('.sk-desc').textContent='';
   const featured=news.querySelector('.comprehensive-news-left-inner');
   featured.removeAttribute('style');featured.querySelector('.comprehensive-news-left-inner-title').textContent='';featured.querySelector('.comprehensive-news-left-inner-describe').textContent='';featured.querySelector('a').removeAttribute('href');featured.querySelector('a span').textContent='阅读全文';
   const article=news.querySelector('.news-article-item').cloneNode(true);article.removeAttribute('href');article.classList.remove('active');
   article.querySelectorAll('p,span,.news-article-item-date').forEach(n=>n.textContent='');
   const newsTemplate=d.createElement('template');newsTemplate.id='home-news-card';newsTemplate.innerHTML=article.outerHTML;d.body.append(newsTemplate);
   const newsRows=news.querySelector('.news-right-row');newsRows.replaceChildren();newsRows.setAttribute('data-home-news','');
   featured.closest('.comprehensive-news-left').hidden=true;
   newsRows.insertAdjacentHTML('afterend','<p data-home-news-status role="status">正在加载新闻与公告…</p><button type="button" data-home-news-retry hidden>重试</button>');
   for(const [selector,title] of homeBlocks.filter(([selector])=>!['.bannerbelow-layoutcontainer','.container_bg6'].includes(selector))){
    const block=parts.find(part=>part.matches(selector));if(!block)continue;
    // No public contract supplies industry/region/partner semantics. Do not
    // publish their source claims or a tall placeholder masquerading as content.
    scrubTemplate(block);
    // Preserve cleaned source display slots, hidden until an existing contract supplies them.
    block.querySelectorAll('.skills-experience,.chooseUs-itme-label,.network_footer,.network_canvas,.network_right').forEach(node=>{node.hidden=true;});
    block.hidden=true;block.setAttribute('data-unavailable-section',title);
   }
 restoreStaticDisplays(d);
 const newsLeft=parseHTML(await readFile(resolve(root,"scripts/assets/sections/news-left.html"),"utf8")).document.querySelector(".comprehensive-news-left");d.querySelector(".comprehensive-news-left").replaceWith(newsLeft);d.querySelector(".container_bg6 .sk-title").textContent="了解最新新闻资讯、网站公告";d.querySelector(".container_bg6 .sk-desc").textContent="资讯公告、产品发布，汇聚前沿的云计算技术";
 for(const [name,selector] of [["solutions",".container_bg15"],["benefits","section.container_bg1:has(.chooseUs-container)"]]){const section=parseHTML(await readFile(resolve(root,"scripts/assets/sections/"+name+".html"),"utf8")).document.querySelector("section");d.querySelector(selector).replaceWith(section);}
  // Fixed presentation is independent of retired template fields.
  for(const [selector,name]of [['.bannerbelow-layoutcontainer','fixed-shortcuts'],['.Rotation-layoutcontainer','fixed-hero'],['.systemproduct-left-top','fixed-product-intro'],['.container_bg30','fixed-partners'],['.container_bg27','fixed-region']]){const fragment=parseHTML(await readFile(resolve(root,'scripts/assets/sections/'+name+'.html'),'utf8')).document;d.querySelector(selector).replaceWith(fragment.querySelector(selector));}
 }else if(kind==='products'||kind==='configure'){
  const sidebar=d.querySelector('.sky-cart-sidebar');sidebar.querySelector('.sky-cart-menu').innerHTML='';sidebar.querySelector('.sky-cart-menu').setAttribute('data-categories','');sidebar.querySelector('.sky-cart-search-results')?.remove();sidebar.querySelector('.sky-cart-search-clear')?.remove();
  const product=d.querySelector('.shopping-cart-product');
  if(kind==='products'){
   product.replaceWith(parseHTML(await readFile(resolve(root,'scripts/assets/sections/catalog-product.html'),'utf8')).document.querySelector('.shopping-cart-product'));
   d.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="lina/catalog-page.css">');
  }
  else {
   d.head.insertAdjacentHTML('beforeend','<link rel="stylesheet" href="lina/catalog-page.css">');
   const allocation=product.querySelector('.allocation-card');
   const referenceNotices=parseHTML('<div class="configure-reference-notices" data-configure-reference-notices><div class="bgw" data-configure-login-notice><div class="layui-elem-quote blue-theme alert-icon icon-info"><p>已有账号？<a data-catalog-login data-customer="/auth/login">登录</a>登录后根据您的账号情况展示适用价格。</p></div></div><div class="bgw" data-catalog-purchase-notice hidden><div class="layui-elem-quote grey-theme"><p></p></div></div></div>').document.querySelector('.configure-reference-notices');
   product.replaceChildren(referenceNotices,allocation);allocation.querySelector('.bgw')?.remove();allocation.querySelector('h1').textContent='商品配置';
   const categoryContext=parseHTML('<p class="lina-configure-context" data-configure-category hidden></p>').document.querySelector('[data-configure-category]');allocation.querySelector('.allocation-header-title')?.prepend(categoryContext);
   allocation.querySelector('.allocation-header-shoppingCart').innerHTML='购物车';allocation.querySelector('.allocation-header-shoppingCart').removeAttribute('href');allocation.querySelector('.allocation-header-shoppingCart').setAttribute('data-customer','/account/cart');
   allocation.querySelector('.allocation-mian').innerHTML='<form id="addCartForm" class="needs-validation configoption_form sky-config-form"><div data-models></div><div class="bgw lina-configure-notice" data-catalog-usage-notice hidden><div class="layui-elem-quote red-theme alert-icon icon-error"><p></p></div></div><p data-product-summary></p><p class="form-group-text lina-configure-feedback" data-status role="status">正在读取商品配置…</p><button class="btn btn-light lina-configure-refresh" type="button" data-refresh>重新读取配置</button><div data-fields></div><div data-purchase-controls></div><div class="sky-form-group sky-config-row configureproduct"><label class="sky-md sky-form-label"><div class="label-title"><span>付款周期</span></div></label><div class="sky-md-100-p"><div class="sky-btn-group" data-quote></div></div></div></form>';
   allocation.querySelector('.configoption_total').removeAttribute('val');allocation.querySelector('.configoption_total').innerHTML='<div class="lina-selected-price" data-selected-price>请选择配置与周期</div><p data-status role="status">正在读取配置…</p><button class="buy-now-button btn btn-primary" type="submit" form="addCartForm" data-add disabled>加入购物车</button><button class="btn btn-light" type="button" data-refresh>重新读取配置</button><a class="btn btn-light" data-customer-configure hidden>前往用户中心选配置</a>';
  }
 }else if(kind==='download'){
  d.querySelector('.sky-sidebar')?.remove();d.querySelector('.sky-navbar')?.remove();
  const content=d.querySelector('.sky-content');
  content.querySelector('.filemanager-sidebar .mb-4').innerHTML='<form data-content-search><input class="form-control" name="query" aria-label="搜索下载" placeholder="搜索下载"><button class="btn btn-primary">搜索</button></form>';
  const panel=content.querySelector('.w-100 .card-body'),table=panel.querySelector('table');
  if(!table)throw new Error('Source download table is missing');
  const headings=table.querySelectorAll('th');headings[0].textContent='资源名称';headings[1].textContent='发布时间';
  // The public content list exposes title and publication time, not a download counter.
  for(const heading of [...headings].slice(2))heading.remove();
  table.querySelector('tbody').replaceChildren();table.querySelector('tbody').setAttribute('data-content-list','');
  panel.insertAdjacentHTML('beforeend','<p data-status role="status">正在加载…</p><button class="btn btn-primary" data-more hidden>加载更多</button>');
 }else if(kind==='contact'||kind==='about'){
  const {banner,tabs,main}=companyTemplate(await load(origin),kind);
  d.querySelector('.globalBanner').replaceWith(banner);
  d.querySelector('.et-main').replaceWith(tabs,main);
 }else if(kind==='content'){
  d.querySelector('.globalBanner')?.replaceChildren();
  d.querySelector('.et-main').replaceWith(contentTemplate(await load(origin)));
 }else{
  const banner=d.querySelector('.globalBanner');if(banner)banner.innerHTML='<div class="comp_container comp_container_auto"><h1 data-page-title></h1></div>';
  const main=d.querySelector('.et-main');const row=main.querySelector('.main-row')||main;const left=main.querySelector('.et-left')||row;
  const right=main.querySelector('.et-right');if(right){const group=right.querySelector('.news_righ_item');if(group){right.querySelector('.news-right-row').replaceChildren(group);group.querySelector('.create-article').innerHTML='<a href="/news.html"><img src="static/picture/sidebar-newspaper.svg" alt="" width="38" height="38"><span>新闻动态</span></a><a href="/help.html"><img src="static/picture/sidebar-book-open.svg" alt="" width="38" height="38"><span>帮助文档</span></a><a href="/downloads.html"><img src="static/picture/sidebar-download.svg" alt="" width="38" height="38"><span>资源下载</span></a>';}else right.replaceChildren();}
  // Reuse the original news header, search form, category bar and nested list.
  if(left.querySelector('.unique_search_container')){
   left.querySelector('.pagebanner')?.classList.remove('white');
   left.querySelector('.pagebanner-title').textContent='';left.querySelector('.pagebanner-title').setAttribute('data-page-title','');left.querySelector('.searchTerms')?.remove();
   const form=left.querySelector('.unique_search_container form');form.setAttribute('data-content-search','');form.removeAttribute('action');form.innerHTML='<input class="unique_search_input" name="query" aria-label="搜索内容" placeholder="搜索内容"><button class="unique_search_button">搜索</button>';
   const tabs=left.querySelector('.newsPage_left_list ul');tabs.innerHTML='<li class="newsPage_left_list_item"><a href="/news.html">新闻资讯</a></li><li class="newsPage_left_list_item"><a href="/announcements.html">公告</a></li><li class="newsPage_left_list_item"><a href="/help.html">帮助文档</a></li>';
   const list=left.querySelector('.news-list');list.replaceChildren();list.setAttribute('data-content-list','');left.querySelector('.news-page-container')?.remove();list.insertAdjacentHTML('afterend','<article data-content-body></article><p data-status role="status">正在加载…</p><button type="button" class="btn btn-primary" data-more hidden>加载更多</button>');
  }else left.innerHTML='<form data-content-search><input name="query" aria-label="搜索内容" placeholder="搜索内容"><button>搜索</button></form><div class="newsPage_row container_bg1"><ul class="news-list" data-content-list></ul><article data-content-body></article><p data-status role="status">正在加载…</p><button type="button" class="btn btn-primary" data-more hidden>加载更多</button></div>';
 }
 d.querySelector('.sky-footer-container').replaceWith(parseHTML(await readFile(resolve(root,'scripts/assets/sections/footer.html'),'utf8')).document.querySelector('.sky-footer-container'));
 // No inline data, handlers or old URLs may survive migration.
 d.querySelectorAll('*').forEach(e=>{for(const a of [...e.attributes]){if(a.name==='style')e.setAttribute('style',sanitizeCSS(a.value,origin));else if(a.name.startsWith('on')||a.name.startsWith('data-')&&!/^(data-wow-|data-page|data-brand-|data-footer-|data-filing-|data-hero-|data-home-|data-unavailable-|data-categories|data-catalog-|data-products|data-status|data-more|data-customer|data-product-summary|data-fields|data-purchase-controls|data-quote|data-add|data-refresh|data-content-|data-page-title)/.test(a.name))e.removeAttribute(a.name);}});
 const template=d.createElement('template');template.id='product-card';template.innerHTML=(kind==='home'?homeCard:card).outerHTML;d.body.append(template);
 const css=d.createElement('link');css.rel='stylesheet';css.href='lina/site.css?v=20261026-category-opening';d.head.append(css);
 const script=d.createElement('script');script.type='module';script.src='lina/site.js';d.body.append(script);
 // Comments can contain stale branding and deleted business code.
 const clearComments=n=>{for(const c of [...n.childNodes])if(c.nodeType===8)c.remove();else clearComments(c);};clearComments(d);
 // The downloaded file includes a link after </html>. Preserve its attribution
 // separately in source documentation; do not publish a second document tail.
 await writeFile(resolve(output,target),'<!DOCTYPE html>\n'+d.documentElement.outerHTML);provenance.push({source:origin,target,kind,sourceSha256:hash(await readFile(resolve(source,origin))),purpose:'保留页面容器、类名与样式，替换原业务内容和脚本'});
}
// Only CSS and fonts are copied initially. Images need explicit review before entering the public bundle.
const assets=[];
async function copyTree(dir){for(const ent of await readdir(resolve(source,dir),{withFileTypes:true})){const name=dir+'/'+ent.name;if(ent.isDirectory())await copyTree(name);else{const b=await readFile(resolve(source,name));await mkdir(dirname(resolve(output,name)),{recursive:true});await copyFile(resolve(source,name),resolve(output,name));assets.push({source:name,target:name,sha256:hash(b)});}}}
await copyTree('static/css');await copyTree('static/font');
// Strip source-site branding URLs and image references pending asset review, while keeping layout declarations.
for(const a of assets.filter(x=>extname(x.target)==='.css')){const css=sanitizeCSS(await readFile(resolve(output,a.target),'utf8'),a.source);await writeFile(resolve(output,a.target),css);a.purpose='保留完整样式及许可；逐项过滤远程资源与未通过图片审计的引用';}
for(const dir of ['static/image','static/picture']){
 await mkdir(resolve(output,dir),{recursive:true});
 // Remove only this generator's stale asset copies from its owned output directory.
 for(const name of await readdir(resolve(output,dir)))if(!reviewedImages.has(name))await unlink(resolve(output,dir,name));
 for(const name of await readdir(resolve(source,dir))){if(!reviewedImages.has(name))continue;const file=dir+'/'+name;await copyFile(resolve(source,file),resolve(output,file));assets.push({source:file,target:file,sha256:hash(await readFile(resolve(source,file))),purpose:'人工源码审计：中性矢量图标，无文字、脚本、外部资源及嵌入位图'});}
}
for(const name of sidebarAssets){const bytes=await readFile(resolve(root,'scripts/assets/sidebar',name));await writeFile(resolve(output,'static/picture',name),bytes);assets.push({source:'scripts/assets/sidebar/'+name,target:'static/picture/'+name,sha256:hash(bytes),purpose:'Lucide 0.577.0 neutral sidebar replacement; ISC and Feather MIT notices retained'});}
// Keep the reviewed filing icons independent of offline watermarked assets.
for(const name of ['gongan.png','local657930f11d1cd.png']){await mkdir(resolve(output,'static/filing'),{recursive:true});await copyFile(resolve(root,'scripts/assets/filing',name),resolve(output,'static/filing',name));}
// Approved public image bytes are bundled with the generator; never recopy watermarked offline variants.
for(const name of ['remoteL17139316863477.jpg','remoteL17090945938503.jpg','remoteL17090946214147.jpg','remoteL17090946561037.jpg','remoteL17025342246554.jpeg','remoteL17139356757263.png','remoteL17139361014962.jpeg','remoteL17139356213316.png','remoteL17176540578767.jpg','remoteL17140335196703.png','remoteL17140335436547.png','remoteL17140335582829.png','remoteL17140335823614.png','remoteL17075739033142.png','remoteL17137759208918.png','remoteL17137759367861.png','remoteL17137759611360.png','remoteL17137759901567.png']){const bytes=await readFile(resolve(root,'scripts/assets/homepage',name));await mkdir(resolve(output,'static/homepage'),{recursive:true});await writeFile(resolve(output,'static/homepage',name),bytes);assets.push({source:'scripts/assets/homepage/'+name,target:'static/homepage/'+name,sha256:hash(bytes),purpose:'Unmodified public image download reviewed on 2026-10-20; source and license limits documented'});}
// Reviewed globe dependencies are preserved independently of the offline script tree.
for(const [dir,names]of [['region',['globe.png','local670fd72bd88ad.svg','local670fcdccc5355.svg','local670fd04222a92.svg']],['vendor',['three.min.js']]])for(const name of names){const bytes=await readFile(resolve(root,'scripts/assets',dir,name));await mkdir(resolve(output,'static',dir),{recursive:true});await writeFile(resolve(output,'static',dir,name),bytes);assets.push({source:'scripts/assets/'+dir+'/'+name,target:'static/'+dir+'/'+name,sha256:hash(bytes),purpose:'Reviewed original globe dependency; license and provenance in region-globe-20261020.md'});}
await writeFile(resolve(root,'docs/resource-decisions.json'),JSON.stringify(resourceDecisions,null,2));
await writeFile(resolve(root,'docs/migration-map.json'),JSON.stringify({pages:provenance,assets,excludedScripts:'全部原脚本未复制或执行，包括带混淆与授权检查的 common.js；无绕过操作',unreviewedImages:'未发布，不能用包含原品牌或套餐的图片填充页面'},null,2));
console.log(`Migrated ${mappings.length} pages; inventoried ${inventory.length} source pages; copied ${assets.length} CSS/font files`);
