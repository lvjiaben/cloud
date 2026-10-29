import {governmentHTML,governmentShell} from './government-content.js';
import {setPageTitle} from './locale.js';
import {renderGovernmentImages} from './government-assets.js';

const governmentPages=new WeakMap();

export function initializeGovernment(document) {
 const root=document.querySelector('[data-government]');
 if(!root)return null;
 if(governmentPages.has(root))return governmentPages.get(root);
 const header=document.querySelector('[data-government-header]');
 const footer=document.querySelector('[data-government-footer]');
 if(header)header.outerHTML=governmentShell.headerHTML;
 if(footer)footer.outerHTML=governmentShell.footerHTML;
 root.innerHTML=governmentHTML;
 renderGovernmentImages(root);
 const view=document.defaultView,bar=root.querySelector('.et-container');
 const links=[...bar.querySelectorAll('.et-hero-tab a')],sections=[...root.querySelectorAll('.et-slide')];
 const marker=document.createElement('div');bar.before(marker);
 function update(){
  root.style.setProperty('--government-anchor-height',bar.offsetHeight+'px');
  const stuck=marker.getBoundingClientRect().top<=0;
  document.body.classList.toggle('web-reading',stuck);bar.classList.toggle('is-stuck',stuck);
  let index=0;
  sections.forEach((s,i)=>{if(s.getBoundingClientRect().top<=bar.offsetHeight+16)index=i;});
  if(view.scrollY+view.innerHeight>=document.documentElement.scrollHeight-2)index=sections.length-1;
  links.forEach((a,i)=>{a.parentElement.classList.toggle('active',i===index);if(i===index)a.setAttribute('aria-current','location');else a.removeAttribute('aria-current');});
  const active=links[index],slider=bar.querySelector('.et-hero-tab-slider');
  slider.style.left=active.parentElement.offsetLeft+'px';slider.style.width=active.offsetWidth+'px';
 }
 links.forEach(a=>a.addEventListener('click',()=>{
  links.forEach(link=>link.removeAttribute('aria-current'));a.setAttribute('aria-current','location');
 }));
 view.addEventListener('scroll',update,{passive:true});view.addEventListener('resize',update);
 view.requestAnimationFrame(update);
 const page={renderBrand(brand){
  setPageTitle(document,root.querySelector('.pagebanner-title').textContent,brand);
  // The existing support adapter owns the configured customer-service destination.
  const target=document.querySelector('[data-lina-support-bar] a.consultation-button[href]');
  for(const a of root.querySelectorAll('[data-government-contact]')){
   if(target){a.href=target.href;a.removeAttribute('aria-disabled');}
   else {a.removeAttribute('href');a.setAttribute('aria-disabled','true');}
  }
 }};
 governmentPages.set(root,page);
 return page;
}
