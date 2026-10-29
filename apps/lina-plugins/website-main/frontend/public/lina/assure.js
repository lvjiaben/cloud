import {assureHTML,assureShell} from './assure-content.js';
import {setPageTitle} from './locale.js';

const instances=new WeakMap();
export function initializeAssure(document){
 const root=document.querySelector('[data-assure]');
 if(!root)return null;
 if(instances.has(root))return instances.get(root);
 document.querySelector('[data-assure-header]').outerHTML=assureShell.headerHTML;
 document.querySelector('[data-assure-footer]').outerHTML=assureShell.footerHTML;
 root.innerHTML=assureHTML;

 const view=document.defaultView,bar=root.querySelector('.et-container');
 const links=[...bar.querySelectorAll('.et-hero-tab a')],sections=[...root.querySelectorAll('.et-slide')];
 const marker=document.createElement('div');bar.before(marker);
 function update(){
  root.style.setProperty('--assure-anchor-height',bar.offsetHeight+'px');
  const stuck=marker.getBoundingClientRect().top<=0;
  document.body.classList.toggle('assure-reading',stuck);
  bar.classList.toggle('is-stuck',stuck);
  let index=0;
  sections.forEach((section,i)=>{if(section.getBoundingClientRect().top<=bar.offsetHeight+16)index=i;});
  if(view.scrollY+view.innerHeight>=document.documentElement.scrollHeight-2)index=sections.length-1;
  links.forEach((a,i)=>{a.parentElement.classList.toggle('active',i===index);if(i===index)a.setAttribute('aria-current','location');else a.removeAttribute('aria-current');});
 }
 view.addEventListener('scroll',update,{passive:true});
 view.addEventListener('resize',update);view.requestAnimationFrame(update);
 const page={renderBrand(brand){
  setPageTitle(document,'服务保障',brand);
  const phone=typeof brand.contactPhone==='string'?brand.contactPhone.trim():'';
  const slot=root.querySelector('[data-assure-phone]');
  slot.textContent='24小时服务热线： '+phone;
  if(phone&&/^[+\d\s().-]+$/.test(phone)){slot.href='tel:'+encodeURIComponent(phone);slot.removeAttribute('aria-disabled');}
  else{slot.removeAttribute('href');slot.setAttribute('aria-disabled','true');}
  const support=document.querySelector('[data-lina-support-bar] a.consultation-button[href]');
  const consult=root.querySelector('[data-assure-consult]');
  consult.href=support?.href||'/contact.html';
 }};
 instances.set(root,page);return page;
}
