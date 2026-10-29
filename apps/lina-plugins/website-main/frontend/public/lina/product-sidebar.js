import {localized} from './locale.js';
import {bindCategoryToggle,categoryToggleLabel} from './catalog-page.js';
import {mergeCatalogCache,readCatalogCache} from './navigation-cache.js';

// Backend categories are groups; their servers are the selectable second level.
export function createProductSidebar({document,request,base,expanded}){
 const menu=document.querySelector('[data-categories]'),cache=new Map();let latestCategories=readCatalogCache()?.categories||[];
 const name=item=>localized(item,'name');
 const make=(tag,text,cls)=>{const el=document.createElement(tag);if(text!==undefined)el.textContent=text;if(cls)el.className=cls;return el;};
 let generation=0;
 const stateFor=key=>{let state=cache.get(key);if(!state){state={products:[],cursor:'',loaded:false,pending:null,error:false};cache.set(key,state);}return state;};
 async function loadCategory(key,state,append=false){
  if(state.pending){await state.pending;return;}
  if(state.loaded&&!append)return;
  state.pending=(async()=>{
   try{
    const query=new URLSearchParams({categoryNo:key,limit:'12'});if(append&&state.cursor)query.set('cursor',state.cursor);
    const data=await request(base+'/catalog/products?'+query),products=(data.products||[]).filter(product=>product.categoryNo===key&&product.publicSlug);
    const bySlug=new Map((append?state.products:[]).map(product=>[product.publicSlug,product]));for(const product of products)bySlug.set(product.publicSlug,product);
    state.products=[...bySlug.values()];state.cursor=data.nextCursor||'';state.loaded=true;state.error=false;mergeCatalogCache({categories:latestCategories,products:state.products});
   }catch{state.error=true;}
   finally{state.pending=null;}
  })();await state.pending;
 }
 function preload(categoryNo){const key=typeof categoryNo==='string'?categoryNo:'';if(key)void loadCategory(key,stateFor(key));}
 function preloadCategories(categories){
  const keys=categories.map(category=>category.categoryNo).filter(key=>typeof key==='string'&&key);
  void (async()=>{for(let index=0;index<keys.length;index+=3)await Promise.all(keys.slice(index,index+3).map(key=>loadCategory(key,stateFor(key))));})();
 }
 function render(categories,{categoryNo='',publicSlug='',currentProduct,initialProducts=[]}={}){
  latestCategories=categories;
  const revision=++generation;menu.replaceChildren();menu.dataset.selectedCategory=categoryNo;
  const first=categories[0]?.categoryNo;
  for(const category of categories){
   const key=category.categoryNo;if(!key)continue;
   const item=make('div',undefined,'sky-cart-menu-item'),title=make('h2',undefined,'sky-cart-menu-title');
   const toggle=make('button',name(category),'lina-catalog-toggle lina-catalog-group-heading');toggle.type='button';toggle.dataset.catalogGroup=key;toggle.dataset.catalogCategory=key;toggle.setAttribute('aria-label',categoryToggleLabel(category));
   const group=make('div',undefined,'sky-cart-submenu'),list=make('div',undefined,'submenu-item-row');group.id='server-group-'+key;toggle.setAttribute('aria-controls',group.id);
   title.append(toggle);group.append(list);item.append(title,group);menu.append(item);
   const state=stateFor(key);
   if(!state.loaded&&!state.pending&&state.products.length===0&&Array.isArray(initialProducts))state.products=initialProducts.filter(product=>product.categoryNo===key&&product.publicSlug);
   const feedback=make('p',undefined,'lina-server-menu-feedback'),more=make('button','加载更多','lina-server-menu-more');more.type='button';
   function paint(){
    if(revision!==generation)return;
    list.replaceChildren();
    const products=[...state.products];if(currentProduct?.categoryNo===key&&!products.some(product=>product.publicSlug===currentProduct.publicSlug))products.push(currentProduct);
    for(const product of products){
     const link=make('a',undefined,'sky-cart-submenu-item'),content=make('div',undefined,'submenu-item-u'),title=make('h5');title.append(make('span',name(product),'lina-product-name'));title.append(make('span','NEW','lina-product-new-badge'));content.append(title);link.append(content);
     link.href='/products.html?'+new URLSearchParams({categoryNo:key,product:product.publicSlug});link.dataset.catalogProduct=product.publicSlug;
     if(product.publicSlug===publicSlug){link.classList.add('active');link.setAttribute('aria-current','page');}
     list.append(link);
    }
    feedback.textContent=state.pending&&!products.length?'正在加载服务器…':state.error?'服务器列表加载失败，请重试。':state.loaded&&!products.length?'该分组暂无公开服务器。':'';
    more.textContent=state.error?'重新加载':'加载更多';more.hidden=!!state.pending||state.loaded&&!state.cursor&&!state.error;more.disabled=!!state.pending;list.append(feedback,more);
   }
   async function load(append=false){
    if(state.pending){paint();more.disabled=true;await state.pending;paint();return;}
    if(state.loaded&&!append){paint();return;}
    paint();more.disabled=true;await loadCategory(key,state,append);paint();
   }
   more.addEventListener('click',()=>load(state.loaded));paint();
   const open=expanded.has(key)?expanded.get(key):key===(categoryNo||first);
   bindCategoryToggle(menu,title,group,toggle,open,value=>{expanded.set(key,value);if(value)void load();});
  }
  preloadCategories(categories);
 }
 return {render,preload,preloadCategories};
}
