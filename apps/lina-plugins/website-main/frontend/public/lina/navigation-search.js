// Search destinations use the existing public catalog and content list adapters.
export function attachNavigationSearch(document,{searchProducts,navigate}){
 const wrapper=document.querySelector('.horizontal-menu-navbar-search');
 const trigger=wrapper?.querySelector('.search-input'),panel=wrapper?.querySelector('.nav-search'),form=wrapper?.querySelector('.lina-nav-search');
 if(!trigger||!panel||!form)return;
 const input=form.querySelector('input'),tabs=[...panel.querySelectorAll('[data-search-target]')];let target='all',restoring=false;
 const open=()=>{if(restoring)return;panel.hidden=false;trigger.setAttribute('aria-expanded','true');trigger.value=input.value;input.focus();};
 const close=restore=>{panel.hidden=true;trigger.setAttribute('aria-expanded','false');if(restore){restoring=true;trigger.focus();restoring=false;}};
 trigger.addEventListener('focus',open);trigger.addEventListener('click',open);trigger.addEventListener('input',()=>{input.value=trigger.value;open();});
 input.addEventListener('input',()=>{trigger.value=input.value;});
 panel.querySelector('.nav-search-close').addEventListener('click',()=>close(true));
 for(const tab of tabs)tab.addEventListener('click',()=>{target=tab.dataset.searchTarget;for(const item of tabs){const selected=item===tab;item.setAttribute('aria-pressed',String(selected));item.parentElement.classList.toggle('active',selected);}input.setAttribute('aria-label',{all:'搜索商品',news:'搜索新闻资讯',docs:'搜索帮助文档'}[target]);input.name=target==='all'?'namePrefix':'query';form.action={all:'/products.html',news:'/news.html',docs:'/help.html'}[target];input.focus();});
 form.addEventListener('submit',event=>{event.preventDefault();const value=input.value.trim();if(target==='all')searchProducts(value);else{const query=new URLSearchParams();if(value)query.set('query',value);navigate((target==='news'?'/news.html':'/help.html')+(query.size?'?'+query:''));}});
 wrapper.addEventListener('keydown',event=>{if(event.key==='Escape'&&!panel.hidden){event.preventDefault();close(true);}});
 document.addEventListener('pointerdown',event=>{if(!wrapper.contains(event.target))close(false);});
 wrapper.addEventListener('focusout',event=>{if(!wrapper.contains(event.relatedTarget))close(false);});
}
