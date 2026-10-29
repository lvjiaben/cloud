import {aboutContentHTML} from './about-content.js';
import {localized,setPageTitle} from './locale.js';
const instances=new WeakMap();
export function initializeAbout(document){
 const root=document.querySelector('[data-about]');if(!root)return null;
 if(instances.has(root))return instances.get(root);
 root.querySelector('[data-about-slot]').outerHTML=aboutContentHTML;
 const view=document.defaultView;
 const tabs=[...root.querySelectorAll('.et-hero-tab a')],sections=tabs.map(a=>root.querySelector(a.hash));
 const cards=[...root.querySelectorAll('.programme-content-item')];
 function activate(index){cards.forEach((card,i)=>{const active=i===index;card.classList.toggle('active',active);card.setAttribute('aria-expanded',String(active));const link=card.querySelector('.programme-content-item-button a');link.tabIndex=active?0:-1;});}
 cards.forEach((card,i)=>{card.tabIndex=0;card.setAttribute('role','button');card.setAttribute('aria-label',card.querySelector('.programme-content-item-title').textContent.trim());card.addEventListener('mouseenter',()=>{if(view.matchMedia('(hover:hover)').matches)activate(i);});card.addEventListener('focus',()=>activate(i));card.addEventListener('click',()=>activate(i));card.addEventListener('keydown',e=>{if(e.target===card&&['Enter',' '].includes(e.key)){e.preventDefault();activate(i);}});});activate(0);
 const nav=root.querySelector('.et-container');
 function highlight(){const line=nav.getBoundingClientRect().bottom+16;let active=0;sections.forEach((section,i)=>{if(section.getBoundingClientRect().top<=line)active=i;});tabs.forEach((a,i)=>{a.parentElement.classList.toggle('active',i===active);if(i===active)a.setAttribute('aria-current','location');else a.removeAttribute('aria-current');});}
 let frame=0;view.addEventListener('scroll',()=>{if(!frame)frame=view.requestAnimationFrame(()=>{frame=0;highlight();});},{passive:true});view.addEventListener('resize',highlight);highlight();
 const page={renderBrand(brand){root.querySelector('[data-about-company]').textContent=localized(brand.footer);setPageTitle(document,'公司简介',brand);root.querySelector('[data-about-consult]').href=document.querySelector('[data-lina-support-bar] a.consultation-button[href]')?.href||'/contact.html';}};
 instances.set(root,page);return page;
}
