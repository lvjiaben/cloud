// No framework required: consume the same public APIs as any Vue, React or jQuery template.
const envelope = async path => { const response=await fetch(path); const result=await response.json(); if(!response.ok || result.code!==0) throw new Error('页面暂时不可用，请刷新重试。'); return result.data; };
const text = value => value?.zhCN || value?.enUS || '';
function renderNavigation(container, items) {
 const ul=document.createElement('ul');
 for(const item of items ?? []) {
  const li=document.createElement('li'); const control=document.createElement(item.group?'button':'a');control.textContent=text(item.label);
  if(item.group){control.type='button';control.setAttribute('aria-expanded','false');}
  else { control.href=item.customer ? new URL(item.href, window.__LINA_TEMPLATE__.customerUrl).href : item.href;if(item.external){control.target='_blank';control.rel='noopener noreferrer';} }
  li.append(control);if(item.children?.length){renderNavigation(li,item.children);if(item.group){const submenu=li.querySelector('ul');submenu.hidden=true;control.addEventListener('click',()=>{submenu.hidden=!submenu.hidden;control.setAttribute('aria-expanded',String(!submenu.hidden));});li.addEventListener('keydown',event=>{if(event.key==='Escape'){submenu.hidden=true;control.setAttribute('aria-expanded','false');control.focus();}});}}
  ul.append(li);
 } container.append(ul);
}
try { const {brand}=await envelope('/x/idc-portal/api/v1/public/brand'); if(!brand)throw new Error('官网尚未启用'); document.title=text(brand.seoTitle)||text(brand.siteName);document.querySelector('#brand').textContent=text(brand.siteName);document.querySelector('h1').textContent=text(brand.heroTitle)||text(brand.siteName);document.querySelector('#description').textContent=text(brand.seoDescription);document.querySelector('#company').textContent=text(brand.footer);renderNavigation(document.querySelector('#header-nav'),brand.navigation);renderNavigation(document.querySelector('#footer-nav'),brand.footerNavigation);const data=await envelope('/x/idc-core/api/v1/public/catalog/products?limit=12');for(const product of data.products ?? []) { const card=document.createElement('article');const name=document.createElement('h2');name.textContent=product.nameZhCN||product.nameEnUS;const description=document.createElement('p');description.textContent=product.summaryZhCN||product.summaryEnUS;const link=document.createElement('a');link.textContent='查看商品';link.href=new URL('/account/products/'+encodeURIComponent(product.publicSlug)+'/configure',window.__LINA_TEMPLATE__.customerUrl).href;card.append(name,description,link);document.querySelector('#products').append(card); } } catch(error) {document.querySelector('#error').textContent=error.message;}
