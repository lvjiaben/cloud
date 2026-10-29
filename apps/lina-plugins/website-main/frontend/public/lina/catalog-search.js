import {localized} from './locale.js';
// Public catalog category suggestions and server prefix search only.
export function matchingCategories(items, value) {
 const key=String(value).trim().toLocaleLowerCase();
 return key?items.filter(c=>[c.nameZhCN,c.nameEnUS].some(n=>String(n||'').toLocaleLowerCase().includes(key))):[];
}
export function searchURL(value, categoryNo='') {
 const query=new URLSearchParams();
 if(categoryNo)query.set('categoryNo',categoryNo);
 if(String(value).trim())query.set('namePrefix',String(value).trim());
 return '/products.html'+(query.size?'?'+query:'');
}
export function attachCatalogSearch({input,navigationInput,searchProducts,document}) {
 let categories=[];
 const panel=document.createElement('div');panel.className='lina-catalog-search-panel';
 const clear=document.createElement('button');clear.type='button';clear.className='lina-catalog-search-clear';clear.textContent='×';clear.setAttribute('aria-label','清空商品搜索');
 const results=document.createElement('div');results.className='lina-catalog-search-results';
 results.id='lina-catalog-search-results';input.setAttribute('aria-controls',results.id);
 input.closest('.sky-cart-search').append(panel);panel.append(clear,results);
 function render(){
  const value=input.value.trim();clear.hidden=!value;results.replaceChildren();results.hidden=!value;
  if(!value)return;
  const matches=matchingCategories(categories,value);
  if(!matches.length){const empty=document.createElement('p');empty.textContent='商品分类中未找到结果';results.append(empty);}
  for(const category of matches){const link=document.createElement('a');link.textContent=localized(category,'name');link.href=searchURL('',category.categoryNo);results.append(link);}
  const current=document.querySelector('[data-categories]')?.dataset.selectedCategory;
  if(current){const search=document.createElement('a');search.textContent='在当前分类中搜索以“'+value+'”开头的商品';search.href=searchURL(value,current);results.append(search);}
 }
 input.addEventListener('input',()=>{if(navigationInput)navigationInput.value=input.value;render();if(!input.value.trim())searchProducts('');});
 input.addEventListener('focus',render);
 navigationInput?.addEventListener('input',()=>{if(navigationInput.name==='namePrefix'){input.value=navigationInput.value;render();}});
 input.addEventListener('keydown',event=>{if(event.key==='Escape')results.hidden=true;});
 clear.addEventListener('click',()=>{input.value='';if(navigationInput)navigationInput.value='';render();searchProducts('');input.focus();});
 document.addEventListener('click',event=>{if(event.target!==input&&!panel.contains(event.target))results.hidden=true;});
 render();
 return items=>{categories=items;render();};
}
