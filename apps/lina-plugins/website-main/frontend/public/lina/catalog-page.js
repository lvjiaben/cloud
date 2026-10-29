import {localized,translate} from './locale.js';
// Original cart DOM and typography; all commercial values come from the public catalog.
import {renderCommerceHeading} from './commerce-heading.js';
import {prependServerFlag} from './server-country.js';
import {readCatalogCache,mergeCatalogCache} from './navigation-cache.js';
const name=p=>localized(p,'name');
export const categoryToggleLabel=category=>localized({
 zhCN:(category.nameZhCN||category.nameEnUS||'')+'子分类',
 enUS:(category.nameEnUS||category.nameZhCN||'')+' subcategories',
});
const element=(tag,text,cls)=>{const e=document.createElement(tag);if(text!==undefined)e.textContent=text;if(cls)e.className=cls;return e;};
export function renderCategoryNotices(document,category){
 for(const [selector,key] of [['[data-catalog-purchase-notice]','purchaseNotice'],['[data-catalog-usage-notice]','usageNotice']]){
  const block=document.querySelector(selector);if(!block)continue;
  const text=typeof category?.[key]==='string'?category[key].trim():'';
  block.querySelector('p').textContent=text;block.hidden=!text;
 }
}
function renderCatalogDescription(card,p){
 const description=card.querySelector('.product-description');description.replaceChildren();
 // Preserve the existing presentation while selecting the full bilingual source first.
 for(const line of localized(p,'summary').split(/[\r\n；;]+/).map(s=>s.trim()).filter(s=>s&&!/^(?:月付|月租|月价|价格|起价|安装费)\s*[:：]?/.test(s)))description.append(element('li',line));
}
export function renderCatalogCard(document,p){
 const card=document.querySelector('#product-card').content.firstElementChild.cloneNode(true);
 card.dataset.product=p.publicSlug;
 card.querySelector('h5').textContent=name(p);
 const badge=element('span','NEW','lina-product-new-badge badge badge-danger');card.querySelector('h5').append(badge);
 prependServerFlag(card.querySelector('h5'),p);
 const stock=card.querySelector('.stock-info');
 stock.textContent=translate(p.availability==='available'?'可订购':'暂不可订购');
 renderCatalogDescription(card,p);
 const price=card.querySelector('.price');price.replaceChildren();price.textContent=translate('配置后报价');price.removeAttribute('title');
 const buy=card.querySelector('.buy-now-button');buy.textContent=translate('选择配置');
 if(p.availability==='available')buy.href='/configure.html?product='+encodeURIComponent(p.publicSlug);
 else {buy.removeAttribute('href');buy.setAttribute('aria-disabled','true');buy.textContent=translate('暂不可订购');}
 return card;
}
export function createCatalogPage({document,request,base,status,retry,customerLink,updateCategories,renderSidebar,preloadSidebar,getCategories,params=new URLSearchParams(location.search),expanded=new Map()}){
 const slot=document.querySelector('[data-products]'),more=document.querySelector('[data-more]'),menu=document.querySelector('[data-categories]');
 let sequence=0;const cards=new Map();
 document.addEventListener('lina:locale-changed',()=>{for(const [card,product] of cards){if(slot.contains(card))renderCatalogDescription(card,product);else cards.delete(card);}});
 function renderMenu(items,selected,currentProduct,initialProducts=[]){if(renderSidebar)renderSidebar(items,{categoryNo:selected,publicSlug:params.get('product')||'',currentProduct,initialProducts});else renderCategoryMenu(document,menu,items,{selected,prefix:params.get('namePrefix')||'',expanded});updateCategories?.(items);}
 const login=document.querySelector('[data-catalog-login]');if(login)customerLink(login,'/auth/login');
 async function load(append=false){
  retry();const seq=++sequence;more.disabled=true;if(!append){cards.clear();slot.replaceChildren();renderCategoryNotices(document,null);}status('正在加载商品…');
  try{
   if(!append&&!params.get('product')){
    const cached=readCatalogCache();
    if(cached){
     const categoryNo=params.get('categoryNo')||cached.categories.find(item=>item.categoryNo)?.categoryNo||'',prefix=(params.get('namePrefix')||'').toLocaleLowerCase();
     const products=cached.products.filter(product=>categoryNo&&product.categoryNo===categoryNo&&(!prefix||name(product).toLocaleLowerCase().includes(prefix)));
     renderMenu(cached.categories,categoryNo,undefined,products);
    }
   }
   const slug=params.get('product'),knownCategories=getCategories?.();
   if(!slug&&!params.get('categoryNo')){
    const metadata=await request(base+'/catalog/products?limit=1');if(seq!==sequence)return;
    const items=metadata.categories||[],first=items.find(item=>item.categoryNo);
    if(!first){renderMenu(items,'');status('暂无可订购商品。');more.hidden=true;return;}
    params.set('categoryNo',first.categoryNo);
    document.defaultView.history?.replaceState(null,'',document.defaultView.location.pathname+'?'+params);
   }
   const q=new URLSearchParams({limit:'12'});for(const key of ['categoryNo','namePrefix'])if(params.get(key))q.set(key,params.get(key));if(append&&more.dataset.cursor)q.set('cursor',more.dataset.cursor);
   // Start auxiliary metadata alongside the detail, but never gate the product on it.
   const metadata=slug?knownCategories?Promise.resolve({categories:knownCategories}):request(base+'/catalog/products?limit=1').catch(()=>null):null;
   const data=await request(base+'/catalog/products'+(slug?'/'+encodeURIComponent(slug):'?'+q));if(seq!==sequence)return;
   const products=slug?[data.product].filter(Boolean):(data.products||[]).filter(product=>product.categoryNo===params.get('categoryNo'));
   if(!append&&!slug&&!params.get('categoryNo')&&!params.get('namePrefix'))mergeCatalogCache(data);
   if(slug&&products[0]?.categoryNo)preloadSidebar?.(products[0].categoryNo);
   for(const p of products){const card=renderCatalogCard(document,p);cards.set(card,p);slot.append(card);}
   function heading(category){renderCommerceHeading(document,{title:slug&&products[0]?name(products[0]):name(category||{}),description:slug?(localized(products[0],'summary')):(category?.purchaseNotice||''),back:'/products.html'+(slug&&category?'?categoryNo='+encodeURIComponent(category.categoryNo):'')});}
   function applyCategories(items){
    if(seq!==sequence)return;
    const selected=slug?products[0]?.categoryNo||'':params.get('categoryNo')||'';
    renderMenu(items,selected,slug?products[0]:undefined,products);
    const category=items.find(c=>c.categoryNo===selected);renderCategoryNotices(document,category);heading(category);
   }
   async function loadMetadata(pending){
    const catalog=await pending;if(seq!==sequence)return;
    if(catalog){applyCategories(catalog.categories||[]);return;}
    const button=element('button','重新加载商品分类','btn sky-config-btn');button.type='button';
    button.addEventListener('click',()=>{button.disabled=true;void loadMetadata(request(base+'/catalog/products?limit=1').catch(()=>null));});
    menu.replaceChildren(element('p','商品分类加载失败，请重试。'),button);
   }
   if(slug){heading();if(knownCategories)applyCategories(knownCategories);else{menu.replaceChildren(element('p','正在加载商品分类…'));void loadMetadata(metadata);}}
   else applyCategories(data.categories||[]);
   status(slot.children.length?'':'暂无可订购商品。');more.dataset.cursor=slug?'':data.nextCursor||'';more.hidden=!more.dataset.cursor;
  }catch{if(seq===sequence){status('商品加载失败，请重试。');more.hidden=true;retry(()=>load(append));}}
  finally{if(seq===sequence)more.disabled=false;}
 }
 return {load,cancel(){sequence++;},search(value){params.delete('product');if(value)params.set('namePrefix',value);else params.delete('namePrefix');history.replaceState(null,'',location.pathname+(params.size?'?'+params:''));return load();}};
}

const menuGroups=new WeakMap();
export function bindCategoryToggle(menu,title,group,toggle,open,onChange=()=>{}){
 const groups=menuGroups.get(menu)||new Map();menuGroups.set(menu,groups);
 for(const other of groups.keys())if(!menu.contains(other))groups.delete(other);
 let animation;
 function setOpen(value,animate=true){
  const height=group.hidden?0:group.getBoundingClientRect().height;animation?.cancel();
  toggle.setAttribute('aria-expanded',String(value));title.classList.toggle('active',value);onChange(value);
  const reduced=menu.ownerDocument.defaultView.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if(!animate||reduced||!group.animate){group.hidden=!value;return;}
  group.hidden=false;group.style.overflow='hidden';
  const next=group.scrollHeight;
  animation=group.animate([{height:height+'px'},{height:(value?next:0)+'px'}],{duration:180,easing:'ease-in-out'});
  animation.onfinish=()=>{group.hidden=!value;group.style.removeProperty('overflow');animation=null;};
 }
 groups.set(toggle,setOpen);
 toggle.addEventListener('click',()=>{
  const value=toggle.getAttribute('aria-expanded')!=='true';
  if(value)for(const [other,close]of groups)if(other!==toggle&&!other.closest('.sky-cart-submenu')?.contains(toggle)&&!group.contains(other))close(false);
  setOpen(value);
 });
 setOpen(open,false);
}

export function renderCategoryMenu(document,menu,items,{selected='',prefix='',expanded=new Map()}={}){
  menu.dataset.selectedCategory=selected;
  menu.replaceChildren();
  menuGroups.set(menu,new Map());
  const byNo=new Map(items.filter(c=>c.categoryNo).map(c=>[c.categoryNo,c])),children=new Map(),visited=new Set(),ancestors=new Set();
  for(const c of byNo.values()){const parent=byNo.has(c.parentCategoryNo)&&c.parentCategoryNo!==c.categoryNo?c.parentCategoryNo:'';if(!children.has(parent))children.set(parent,[]);children.get(parent).push(c);}
  for(let c=byNo.get(selected);c&&!ancestors.has(c.categoryNo);c=byNo.get(c.parentCategoryNo))ancestors.add(c.categoryNo);
  function categoryLink(c){
   const link=element('a',name(c)),query=new URLSearchParams();
   if(c.categoryNo)query.set('categoryNo',c.categoryNo);
   if(prefix)query.set('namePrefix',prefix);
   link.href='/products.html'+(query.size?'?'+query:'');link.dataset.catalogCategory=c.categoryNo;
   if(selected===c.categoryNo)link.setAttribute('aria-current','page');
   return link;
  }
  function render(c,container,depth){
   if(visited.has(c.categoryNo))return;visited.add(c.categoryNo);
   const item=element('div',undefined,depth?'lina-catalog-child':'sky-cart-menu-item'),title=element(depth?'div':'h2',undefined,depth?'lina-catalog-subtitle':'sky-cart-menu-title');
   const link=categoryLink(c);if(depth){link.className='sky-cart-submenu-item';const text=element('div',undefined,'submenu-item-u');text.append(element('h5',name(c)));link.replaceChildren(text);}
   if(ancestors.has(c.categoryNo)){title.classList.add('active');if(selected===c.categoryNo)link.classList.add('active');}
   title.append(link);item.append(title);container.append(item);
   const nested=(children.get(c.categoryNo)||[]).filter(child=>!visited.has(child.categoryNo));
   if(!nested.length)return;
   const group=element('div',undefined,'sky-cart-submenu'),row=element('div',undefined,'submenu-item-row'),toggle=element('button',name(c),'lina-catalog-toggle');
   group.id='catalog-group-'+visited.size;toggle.type='button';toggle.setAttribute('aria-label',categoryToggleLabel(c));toggle.setAttribute('aria-controls',group.id);
   if(depth===0)toggle.dataset.catalogCategory=c.categoryNo;
   const open=expanded.has(c.categoryNo)?expanded.get(c.categoryNo):ancestors.has(c.categoryNo)||(!selected&&container===menu&&menu.children.length===1);
   if(depth===0){title.replaceChildren(toggle);toggle.classList.add('lina-catalog-group-heading');}else{toggle.textContent='';title.append(toggle);}
   group.append(row);item.append(group);bindCategoryToggle(menu,title,group,toggle,open,value=>expanded.set(c.categoryNo,value));
   for(const child of nested)render(child,row,depth+1);
  }
  for(const c of children.get('')||[])render(c,menu,0);
  // A missing/disabled parent stays absent; keep public children discoverable.
  // Defensive fallback also avoids hiding nodes if an inconsistent cycle arrives.
  for(const c of byNo.values())if(!visited.has(c.categoryNo))render(c,menu,0);

 }
