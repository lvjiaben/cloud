import {goingseaShell} from './goingsea-content.js';
import {partnerHTML} from './partner-content.js';

const pages=new WeakMap();
export function initializePartner(document){
 const root=document.querySelector('[data-partner]');
 if(!root)return null;
 if(pages.has(root))return pages.get(root);
 const header=document.querySelector('[data-partner-header]'),footer=document.querySelector('[data-partner-footer]');
 if(header)header.outerHTML=goingseaShell.headerHTML;
 if(footer)footer.outerHTML=goingseaShell.footerHTML;
 root.innerHTML=partnerHTML;

 root.querySelector('.globalBanner').style.backgroundImage='url(/lina/partner-assets/remoteL17131030094606.jpeg)';
 const bar=root.querySelector('.et-hero-tabs-container'),links=[...bar.querySelectorAll('.et-hero-tab a')],sections=[...root.querySelectorAll('main.et-main>.et-slide')];
 links.forEach((link,index)=>{link.href='#partner-'+index;sections[index].id='partner-'+index;});
 const table= root.querySelector('.grade-table');
 table.querySelectorAll('.grade-table-header-cell').forEach(cell=>{if(cell.textContent.trim()==='钻石代理')cell.setAttribute('data-corrected-copy','true');});
 const viewport=document.createElement('div');viewport.className='partner-table-viewport';table.before(viewport);viewport.append(table);
 const requirements=root.querySelector('.partner_table'),requirementsViewport=document.createElement('div');requirementsViewport.className='partner-requirements-viewport';requirements.before(requirementsViewport);requirementsViewport.append(requirements);
 const update=()=>{let active=0;sections.forEach((section,index)=>{if(section.getBoundingClientRect().top<=bar.offsetHeight+16)active=index;});if(document.defaultView.scrollY+document.defaultView.innerHeight>=document.documentElement.scrollHeight-2)active=sections.length-1;links.forEach((link,index)=>{link.parentElement.classList.toggle('active',index===active);if(index===active)link.setAttribute('aria-current','location');else link.removeAttribute('aria-current');});};
 links.forEach(link=>link.addEventListener('click',event=>{event.preventDefault();root.querySelector(link.getAttribute('href')).scrollIntoView({behavior:'smooth'});}));
 const advantages=[...root.querySelectorAll('.introduce-section')];
 advantages[0]?.classList.add('active');
 advantages.forEach(item=>item.addEventListener('click',()=>{if(item.classList.contains('active'))return;advantages.forEach(other=>other.classList.toggle('active',other===item));}));
 document.defaultView.addEventListener('scroll',update,{passive:true});document.defaultView.addEventListener('resize',update);update();
 const page={renderBrand(){}};pages.set(root,page);return page;
}
