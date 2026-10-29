import {localized,translate} from './locale.js';
// Present the published navigation tree; never infer source-site business links.
import {readCatalogCache} from './navigation-cache.js';
import {bindMovingIndicator} from './moving-indicator.js';
import {composeNavigation,navigationLabel,navigationDisplayLabel} from './navigation-content.js?v=20261025-partner-entry';
import {gameContent} from './game-content.js';
import {financialContent} from './financial-content.js';
import {safetyContent} from './safety-content.js';
import {goingseaContent} from './goingsea-content.js';
import {governmentContent} from './government-content.js';
const navigationStates=new WeakMap();
export function renderNavigation(document,items,{safeURL,customerLink,request}){
 const game=document.body.dataset.page==='game';
 items=composeNavigation(items,[document.defaultView.location.origin,safeURL('/',true)?.origin].filter(Boolean),game?gameContent.navigation:document.body.dataset.page==='financial'?financialContent.navigation:document.body.dataset.page==='safety'?safetyContent.navigation:document.body.dataset.page==='goingsea'?goingseaContent.navigation:document.body.dataset.page==='government'?governmentContent.navigation:undefined);
 const root=document.querySelector('.horizontal-menu-navbar-menu');
 let state=navigationStates.get(root);
 if(!state){
  state={signature:null,pending:null};navigationStates.set(root,state);
  const nav=root.closest('.nav-layoutcontainer'),tools=nav?.querySelector('.horizontal-menu-navbar-search');
  if(tools){const observer=new document.defaultView.ResizeObserver(()=>{const height=tools.getBoundingClientRect().height;if(height)nav.style.setProperty('--lina-mobile-tools-height',height+'px');});observer.observe(tools);}
  // Page-local sticky navigation takes over the viewport; close every global layer.
  const observer=new document.defaultView.MutationObserver(()=>{if(document.querySelector('.et-container.is-stuck'))state.collapse?.();});
  observer.observe(document.body,{attributes:true,attributeFilter:['class']});
  const flush=()=>queueMicrotask(()=>{if(state.pending&&!root.querySelector('.lina-open')&&!root.contains(document.activeElement)){const next=state.pending;state.pending=null;renderNavigation(...next);}});
  state.flush=flush;
  document.addEventListener('keydown',event=>{if(event.key==='Escape')state.close?.();});
  for(const event of ['focusout','mouseleave','click','keydown'])root.addEventListener(event,flush);
 }
 const signature=JSON.stringify(items);
 if(signature===state.signature){
  state.pending=null;
  items.forEach((item,index)=>{if(!item.customer)return;const anchor=root.children[index]?.querySelector('a');if(anchor)customerLink(anchor,item.href);});
  return;
 }
 if(state.signature!==null&&(root.querySelector('.lina-open')||root.contains(document.activeElement))){state.pending=[document,items,{safeURL,customerLink,request}];return;}
 state.dispose?.();state.pending=null;state.signature=signature;root.replaceChildren();
 const el=(tag,text,cls)=>{const n=document.createElement(tag);if(text!==undefined)n.textContent=text;if(cls)n.className=cls;return n;};
 const label=navigationLabel;
 let active,restoring=false,hoverTimer,closeTimer;
 const motions=new Map();
 const cancelHover=()=>{clearTimeout(hoverTimer);hoverTimer=undefined;};
 const cancelClose=()=>{clearTimeout(closeTimer);closeTimer=undefined;};
 const reduced=()=>document.defaultView.matchMedia('(prefers-reduced-motion:reduce)').matches;
 const slide=(panel,show)=>{
  const previous=motions.get(panel),from=previous?panel.getBoundingClientRect().height:show?0:panel.getBoundingClientRect().height;
  previous?.cancel();motions.delete(panel);
  panel.style.display='block';panel.inert=!show;
  if(reduced()||!panel.animate){panel.style.removeProperty('display');return;}
  const height=panel.getBoundingClientRect().height;
  const motion=panel.animate([{height:from+'px',overflow:'hidden'},{height:(show?height:0)+'px',overflow:'hidden'}],{duration:150,easing:'cubic-bezier(.445,.05,.55,.95)'});
  motions.set(panel,motion);
  motion.onfinish=()=>{if(motions.get(panel)!==motion)return;motions.delete(panel);panel.style.removeProperty('display');};
 };
 state.dispose=()=>{cancelHover();cancelClose();for(const motion of motions.values())motion.cancel();motions.clear();};
 const close=restore=>{cancelHover();cancelClose();if(!active)return;const {li,toggle,panel}=active;slide(panel,false);li.classList.remove('lina-open');toggle.setAttribute('aria-expanded','false');active=undefined;if(restore){restoring=true;toggle.focus();restoring=false;}state.flush();};
 state.close=()=>close(true);
 state.collapse=()=>{
  close(false);
  for(const motion of motions.values())motion.cancel();motions.clear();
  root.querySelectorAll('.lina-nav-panel').forEach(panel=>panel.style.removeProperty('display'));
  const nav=root.closest('.nav-layoutcontainer');
  if(nav?.classList.contains('lina-mobile-open'))nav.classList.remove('lina-mobile-open');
  nav?.querySelector('.lina-menu-toggle')?.setAttribute('aria-expanded','false');
  if(nav?.contains(document.activeElement))document.activeElement.blur();
 };
 const link=item=>{const a=el('a',localized(navigationDisplayLabel(item)),'Submenu-item');const u=!item.group&&safeURL(item.href,item.customer);if(u){a.href=u.href;if(item.external){a.target='_blank';a.rel='noopener noreferrer';}}else{a.setAttribute('aria-disabled','true');}return a;};
 items.forEach((item,index)=>{
  const li=el('li'),a=link(item);a.className='';li.append(a);root.append(li);
  a.dataset.key=String(index);
  const productMenu=!item.customer&&item.href==='/products.html';
  a.dataset.childtype=productMenu||item.group||item.children?.length?'nav':'url';
  if(!productMenu&&!item.group&&!item.children?.length){li.addEventListener('mouseenter',()=>close(false));return;}
  const toggle=item.group||productMenu?a:el('button','展开 '+label(item),'lina-submenu-toggle');toggle.classList.add('nav-arrow');if(toggle!==a){toggle.type='button';li.append(toggle);}
  toggle.removeAttribute('aria-disabled');toggle.setAttribute('role','button');toggle.tabIndex=0;toggle.setAttribute('aria-expanded','false');toggle.dataset.key=String(index);toggle.dataset.childtype='nav';
  const panel=el('div',undefined,'lina-nav-panel Submenu-background black'),layout=el('div',undefined,'Submenu-container'),left=el('div',undefined,'Submenu-left'),content=el('div',undefined,'Submenu-middle-right'),groups=el('div',undefined,'Submenu-middle-content');
  panel.id='lina-nav-panel-'+index;toggle.setAttribute('aria-controls',panel.id);panel.setAttribute('aria-label',label(item));
  panel.dataset.menu=label(item);panel.inert=true;
  layout.classList.add(productMenu?'product_services':(item.children||[]).some(child=>child.group||child.children?.length)?'page_services':'nav_services');
  const leftContainer=el('div',undefined,'Submenu-left-container'),info=el('div',undefined,'Submenu-menuInfo-container'),copy=el('div',undefined,'Submenu-menuInfo');
  copy.append(el(item.headingTag||'h2',localized(navigationDisplayLabel(item)),'Submenu-menuInfo-title'));
  if(item.description&&!productMenu)copy.append(el('p',item.description,'Submenu-desc'));
  info.append(copy);leftContainer.append(info);left.append(leftContainer);layout.append(left,content);content.append(groups);panel.append(layout);
  const card=child=>{const a=link(child);a.replaceChildren();const title=el('div',undefined,'Submenu-item-title');title.append(el('span',localized(navigationDisplayLabel(child)),'Submenu-title'));if(child.badge)title.append(el('span',child.badge,'Submenu-tag'));a.append(title);if(child.description)a.append(el('span',child.description,'Submenu-desc'));return a;};
  const tabs=el('div',undefined,'Submenu-page-tab');
  const tabList=el('div',undefined,'Submenu-page-list');tabs.append(tabList);
  // Opening replays the default line; category interaction hands control to hover/focus.
  for(const event of ['pointerover','focusin','click'])tabList.addEventListener(event,event=>{if(event.target.closest('.lina-nav-category'))panel.classList.remove('lina-category-opening');});
  const displayTabs=sections=>{info.append(tabs);const buttons=[];const views=[];sections.forEach((section,index)=>{const button=el('button',section.title,'lina-nav-category');button.type='button';button.setAttribute('aria-pressed',String(index===0));tabList.append(button);const view=el('div',undefined,'lina-nav-category-view');view.hidden=index!==0;for(const child of section.items)view.append(card(child));groups.append(view);buttons.push(button);views.push(view);const select=()=>{buttons.forEach((b,i)=>b.setAttribute('aria-pressed',String(i===index)));views.forEach((v,i)=>v.hidden=i!==index);};button.addEventListener('mouseenter',select);button.addEventListener('focus',select);button.addEventListener('click',select);});};
  if(!productMenu){
   const children=item.children||[];
   if(children.some(child=>child.group||child.children?.length)){displayTabs(children.map(child=>({title:label(child),items:child.children||[child]})));children.forEach((child,index)=>{if(child.description)groups.children[index].prepend(el('p',child.description,'lina-nav-group-description'));});}
   else for(const child of children)groups.append(card(child));
   if(!groups.children.length)groups.append(el('p','暂无已发布导航'));
   if(item.promotion||item.promotionCopy){
    content.classList.add('lina-nav-with-promotion');const aside=el('aside',undefined,'lina-nav-promotion');
    const referral=children.find(child=>label(child)==='推广返利'),target=link({...referral,label:translate('了解推广计划')});target.className='advertisement-content black';target.replaceChildren();
    target.setAttribute('aria-label','了解推广计划');
    const image=el(item.promotionCopy?'div':'img');
     if(item.promotionCopy){image.className=document.body.dataset.page==='goingsea'?'goingsea-promotion-image-slot':'game-promotion-image-slot';image.setAttribute('aria-hidden','true');}
    else {image.src='/static/picture/local6624afb6bf492.png';image.alt='';image.width=1020;image.height=522;image.decoding='async';}
    const position=el('div',undefined,'advertisement-position');position.append(el('div',item.promotionCopy?.title??'推广合作','advertisement-title'),el('div',item.promotionCopy?.description??'查看本方推广计划与参与规则。','advertisement-desc'));
    target.append(image,position);aside.append(target);content.append(aside);
   }
  }
  let loadProducts;
  if(productMenu){
   info.append(tabs);const search=el('input',undefined,'lina-nav-product-search');search.type='search';search.placeholder='请输入搜索内容';search.setAttribute('aria-label','搜索产品与服务');
   const all=link({label:translate('查看此分类'),href:'/products.html'});all.className='lina-nav-all';content.prepend(search);content.append(all);
   search.addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();const query=new URLSearchParams();if(selected)query.set('categoryNo',selected);if(search.value.trim())query.set('namePrefix',search.value.trim());document.defaultView.location.assign('/products.html'+(query.size?'?'+query:''));}});
   let loaded=false,sequence=0,selected='',buttons=[],categorySignature='';
   // Shared catalogue entries may be partial samples. Only scoped menu reads
   // establish a complete displayed page, including an authoritative empty page.
   const categoryPages=new Map();
   const renderCategories=(data,refresh=false)=>{
    if(loaded&&!refresh)return;
    const categories=(data.categories||[]).filter(category=>category.categoryNo);
    const signature=JSON.stringify(categories.map(c=>[c.categoryNo,localized(c,'name')]));
    if(signature===categorySignature&&loaded)return;
    categorySignature=signature;tabList.replaceChildren();buttons=[];
    for(const c of categories){const name=localized(c,'name'),b=el('button',name,'lina-nav-category');b.type='button';b.setAttribute('aria-pressed',String(c.categoryNo===selected));b.addEventListener('mouseenter',()=>{if(selected!==c.categoryNo)loadProducts(c.categoryNo,true);});b.addEventListener('focus',()=>{if(selected!==c.categoryNo)loadProducts(c.categoryNo,true);});b.addEventListener('click',()=>loadProducts(c.categoryNo,true));tabList.append(b);buttons.push([c.categoryNo,b]);}
    loaded=true;
   };
   const renderProducts=(products,category)=>{
    groups.replaceChildren();for(const product of products){const name=localized(product,'name'),a=link({label:name,href:'/products.html?product='+encodeURIComponent(product.publicSlug)}),title=el('div',undefined,'Submenu-item-title');title.append(el('span',name,'Submenu-title'));title.append(el('span','NEW','Submenu-tag'));a.replaceChildren(title,el('span',localized(product,'summary'),'Submenu-desc'));groups.append(a);}
    if(!groups.children.length)groups.append(el('p','暂无可展示商品。'));all.href='/products.html'+(category?'?categoryNo='+encodeURIComponent(category):'');
   };
   const showCached=(category)=>{
    const metadata=readCatalogCache();
    category=category||metadata?.categories.find(item=>item.categoryNo)?.categoryNo||'';
    selected=category;if(metadata)renderCategories(metadata);
    const snapshot=categoryPages.get(category);
    if(!snapshot||Date.now()-snapshot.savedAt>=30000)return false;
    renderProducts(snapshot.products,category);return true;
   };
   loadProducts=async(category='',refresh=false)=>{
    if(!request)return;if(!refresh&&loaded)category=selected;const seq=++sequence;selected=category;if(groups.contains(document.activeElement)){restoring=true;toggle.focus();restoring=false;}const hasCache=showCached(category);category=category||selected;selected=category;if(!hasCache)groups.replaceChildren(el('p','正在加载商品…'));
    for(const [id,b]of buttons)b.setAttribute('aria-pressed',String(id===category));
    let deadline;
    try{const data=await Promise.race([(async()=>{
      category=category||selected;
      if(!category){const metadata=await request('/x/idc-core/api/v1/public/catalog/products?limit=1');if(seq!==sequence)return null;renderCategories(metadata,true);category=(metadata.categories||[]).find(item=>item.categoryNo)?.categoryNo||'';}
      if(!category)return {categories:[],products:[]};
      return request('/x/idc-core/api/v1/public/catalog/products?'+new URLSearchParams({limit:'12',categoryNo:category}));
     })(),new Promise((_,reject)=>{deadline=setTimeout(()=>reject(new Error('catalog request timed out')),8000);})]);if(seq!==sequence||!data)return;
     selected=category;renderCategories(data,true);for(const [id,b]of buttons)b.setAttribute('aria-pressed',String(id===category));const products=(data.products||[]).filter(product=>category&&product.categoryNo===category);categoryPages.set(category,{savedAt:Date.now(),products});renderProducts(products,category);
    }catch{if(seq!==sequence)return;if(!hasCache)groups.replaceChildren();groups.append(el('p','商品加载失败，请重试。'));const retry=el('button','重试','lina-nav-retry');retry.type='button';retry.addEventListener('click',()=>loadProducts(category,true));groups.append(retry);}finally{clearTimeout(deadline);}
   };
   // Unscoped cache events must never replace an already loaded category page.
  }
  const button=el('button','×','lina-nav-close');button.type='button';button.setAttribute('aria-label','关闭 '+label(item));panel.append(button);li.append(panel);
  const open=()=>{cancelHover();if(document.querySelector('.et-container.is-stuck'))return;if(active?.li===li)return;close(false);active={li,toggle,panel};panel.classList.add('lina-category-opening');li.classList.add('lina-open');toggle.setAttribute('aria-expanded','true');slide(panel,true);loadProducts?.();};
  // Bind to the item and its visible title so moving onto the text itself
  // opens the panel just like the source navigation. Pointer events cover
  // mouse, pen and touch-capable desktop browsers while mouse events keep
  // compatibility with the existing stylesheet and test harness.
  const canHover=()=>document.defaultView.matchMedia('(min-width:768px)').matches;
  const openOnPointer=()=>{cancelClose();if(!canHover()||active?.li===li||hoverTimer!==undefined)return;hoverTimer=setTimeout(()=>{hoverTimer=undefined;if(li.matches(':hover'))open();},200);};
  const closeOnPointer=()=>{if(canHover()){cancelHover();cancelClose();if(active?.li===li)closeTimer=setTimeout(()=>{closeTimer=undefined;if(active?.li===li&&!li.matches(':hover'))close(false);},300);}};
  panel.addEventListener('pointerenter',cancelClose);
  panel.addEventListener('mouseenter',cancelClose);
  li.addEventListener('mouseenter',openOnPointer);
  li.addEventListener('pointerenter',openOnPointer);
  li.addEventListener('mouseleave',closeOnPointer);
  li.addEventListener('pointerleave',closeOnPointer);
  toggle.addEventListener('focus',()=>{if(!restoring)open();});toggle.addEventListener('click',e=>{e.preventDefault();open();});
  toggle.addEventListener('keydown',e=>{if(toggle===a&&['Enter',' '].includes(e.key)){e.preventDefault();open();}});
  li.addEventListener('focusout',e=>{if(!li.contains(e.relatedTarget)&&(e.relatedTarget||!li.matches(':hover')))close(false);});button.addEventListener('click',()=>close(true));
 });
 root.onkeydown=e=>{if(e.key==='Escape'){e.preventDefault();close(true);}};
 bindMovingIndicator(root);
}
