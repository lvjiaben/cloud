// All solution recommendations are explicit template associations, read in one bounded catalog query.
import {localized,getLocale} from './locale.js';

export const solutionProductFields={electricity:'relatedproductselectricity','web-solution':'relatedproductsweb',game:'relatedproductsgame',goingsea:'relatedproductsgoingsea',financial:'relatedproductsfinancial',government:'relatedproductsgovernment',safety:'relatedproductssafety'};

export function relatedProductSlugs(value,origin){
 if(typeof value!=='string'||!value.trim())return [];
 const slugs=[];
 for(const line of value.split(/\r?\n/).map(s=>s.trim()).filter(Boolean)){
  let slug=line;
  if(line.includes('/')||line.includes('?')){
   const url=new URL(line,origin);
   if(url.origin!==origin||!['/products.html','/configure.html'].includes(url.pathname))throw new Error('Invalid product link');
   slug=url.searchParams.get('product')||'';
  }
  if(slug.length>128||!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug))throw new Error('Invalid product');
  if(!slugs.includes(slug))slugs.push(slug);
  if(slugs.length>24)throw new Error('Too many products');
 }
 return slugs;
}

export function initializeSolutionProducts(document,request){
 const key=solutionProductFields[document.body.dataset.page],slot=document.querySelector('.correlation-Content-row');
 if(!key||!slot)return null;
 const view=document.defaultView;
 let config,products=[],state='waiting',generation=0,running;
 const t=(zh,en)=>getLocale()==='en-US'?en:zh;
 const el=(tag,text,cls)=>{const node=document.createElement(tag);if(text!==undefined)node.textContent=text;if(cls)node.className=cls;return node;};
 slot.classList.add('solution-products');slot.setAttribute('aria-live','polite');
 // Shared styling stays scoped to these seven slots, including source pages with different card CSS.
 const style=el('style');style.textContent=`.solution-products{display:grid!important;grid-template-columns:repeat(3,minmax(0,1fr));gap:24px;width:100%;box-sizing:border-box}.solution-products .web-product{width:auto!important;min-width:0;margin:0!important;padding:28px!important;box-sizing:border-box;background:#fff;border:1px solid #e5e9ef;display:flex;flex-direction:column;gap:20px;overflow-wrap:anywhere}.solution-products h3{font-size:20px;line-height:1.5;margin:0}.solution-products .correlation-item-desc{flex:1;line-height:1.8;color:#596579}.solution-products p{margin:0}.solution-products .web-product-price{font-size:18px;color:#1767db}.solution-products .web-product-link,.solution-products .web-retry{align-self:flex-start;padding:10px 22px;border:1px solid #1767db;background:#fff;color:#1767db;border-radius:3px;text-decoration:none;cursor:pointer}.solution-products .web-product-state{grid-column:1/-1;text-align:center;padding:36px 12px;color:#596579;line-height:2}.solution-products .web-product-state button{margin-top:16px}@media(max-width:760px){.solution-products{grid-template-columns:minmax(0,1fr);gap:16px}}`;
 document.head.append(style);
 function render(){
  slot.replaceChildren();slot.setAttribute('aria-busy',String(state==='loading'||state==='waiting'));
  if(state!=='ready'||!products.length){
   const box=el('div',undefined,'web-product-state');
   const message=state==='invalid'?t('关联商品配置有误，请联系网站管理员。','The related product settings need attention. Please contact the site administrator.'):state==='error'?t('商品加载失败，请重试。','Products could not be loaded. Please try again.'):state==='loading'||state==='waiting'?t('正在加载商品…','Loading products…'):t('暂无关联商品。','No related products are available.');
   box.append(el('p',message));
   if(state==='error'){const retry=el('button',t('重新加载','Retry'),'web-retry');retry.type='button';retry.addEventListener('click',()=>load());box.append(retry);}slot.append(box);return;
  }
  for(const product of products){
   const card=el('article',undefined,'correlation-Content-item web-product'),heading=el('div',undefined,'correlation-item-title');
   heading.append(el('h3',localized(product,'name')));card.append(heading);
   const description=el('div',undefined,'correlation-item-desc');description.append(el('p',localized(product,'summary')));card.append(description);
   const price=product.defaultConfigurationMonthlyPrice;
   if(price?.currencyCode==='CNY'&&/^\d+$/.test(price.amountMinor??'')){
    const minor=BigInt(price.amountMinor),amount=(minor/100n).toString()+'.'+(minor%100n).toString().padStart(2,'0');
    card.append(el('p',t('默认配置月费','Default configuration per month')+' ¥'+amount,'web-product-price'));
   }
   if(product.availability!=='available')card.append(el('p',t('暂不可售','Currently unavailable')));
   const link=el('a',t('查看商品','View product'),'web-product-link');link.href='/products.html?product='+encodeURIComponent(product.publicSlug);card.append(link);slot.append(card);
  }
 }
 function load(){
  const current=++generation;let slugs;
  try{slugs=relatedProductSlugs(config,view.location.origin);}catch{products=[];state='invalid';render();return Promise.resolve();}
  if(!slugs.length){products=[];state='ready';render();return Promise.resolve();}
  products=[];state='loading';render();
  const query=new URLSearchParams({limit:'24'});slugs.forEach(slug=>query.append('publicSlugs[]',slug));
  const task=(async()=>{
   try{
    const data=await request('/x/idc-core/api/v1/public/catalog/products?'+query);
    if(!Array.isArray(data.products))throw new Error('Invalid catalog');
    if(current!==generation)return;
    const bySlug=new Map(data.products.map(product=>[product.publicSlug,product]));
    products=slugs.map(slug=>bySlug.get(slug)).filter(Boolean);state='ready';
   }catch{if(current!==generation)return;products=[];state='error';}
   render();
  })();running=task;return task.finally(()=>{if(running===task)running=undefined;});
 }
 document.addEventListener('lina:locale-changed',render);render();
 return {renderBrand(brand){const value=brand.templateConfig?.[key]??'';if(value!==config){config=value;return load();}if(!running)return load();return running;}};
}
