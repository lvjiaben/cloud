import {prependServerFlag} from './server-country.js';
import {getLocale} from './locale.js';
export function renderCommerceHeading(document,{title,description='',back='/products.html'}){
 const root=document.querySelector('.nav-top-page-title');if(!root)return;
 const link=root.querySelector('.return');link.href=back;
 if(!link.querySelector('i')){const icon=document.createElement('i');icon.className='far fa-angle-left';icon.setAttribute('aria-hidden','true');link.prepend(icon);}
 link.querySelector('span').textContent='返回';
 const heading=root.querySelector('[data-catalog-title]');
 heading.textContent=title?(getLocale()==='en-US'?'Region (':'区域（')+title+(getLocale()==='en-US'?')':'）'):'商品选购';
 heading.dataset.localeHeadingTitle=title;
 prependServerFlag(root.querySelector('[data-catalog-title]'),title);
 let note=root.querySelector('[data-catalog-context-note]');
 if(!note){note=document.createElement('p');note.setAttribute('data-catalog-context-note','');root.append(note);}
 note.textContent=description;note.title=description;note.hidden=!description;
}
