import {btHTML,btShell,btContent} from './bt-content.js';
import {setPageTitle} from './locale.js';
const instances=new WeakMap();
export function initializeBt(document){
 const root=document.querySelector('[data-bt]');
 if(!root)return null;
 if(instances.has(root))return instances.get(root);
 document.querySelector('[data-bt-header]').outerHTML=btShell.headerHTML;
 const header=document.querySelector('body>.nav-layoutcontainer');header.classList.remove('black');header.classList.add('white','Force_white');
 document.querySelector('[data-bt-footer]').outerHTML=btShell.footerHTML;
 root.innerHTML=btHTML;
 const view=document.defaultView,bar=root.querySelector('.et-container');
 const marker=document.createElement('div');bar.before(marker);
 const links=[...bar.querySelectorAll('.et-hero-tab a')],sections=[...root.querySelectorAll('.et-slide')];
 const feedback=document.createElement('div');feedback.className='bt-copy-feedback';feedback.setAttribute('role','status');feedback.setAttribute('aria-live','polite');root.after(feedback);
 const buttons=[...root.querySelectorAll('.copyBtn')];
 buttons.forEach(button=>{button.type='button';button.previousElementSibling.tabIndex=0;});
 root.addEventListener('click',async event=>{
  const button=event.target.closest('.copyBtn');if(!button||button.disabled)return;
  const command=btContent.commands[buttons.indexOf(button)];if(!command)return;
  button.disabled=true;feedback.textContent='';
  try{
   if(!view.navigator.clipboard?.writeText)throw new Error('Clipboard unavailable');
   await view.navigator.clipboard.writeText(command.copyText);
   feedback.textContent=btContent.feedback.success;feedback.dataset.state='success';
  }catch{
   feedback.textContent=btContent.feedback.failure;feedback.dataset.state='failure';
   const code=button.previousElementSibling;code.focus({preventScroll:true});
   const range=document.createRange();range.selectNodeContents(code);const selection=view.getSelection();selection.removeAllRanges();selection.addRange(range);
  }finally{button.disabled=false;}
 });
 function update(){
  root.style.setProperty('--bt-anchor-height',bar.offsetHeight+'px');
  const stuck=marker.getBoundingClientRect().top<=0;document.body.classList.toggle('bt-reading',stuck);bar.classList.toggle('is-stuck',stuck);
  let index=0;sections.forEach((section,i)=>{if(section.getBoundingClientRect().top<=bar.offsetHeight+16)index=i;});
  if(view.scrollY+view.innerHeight>=document.documentElement.scrollHeight-2)index=sections.length-1;
  links.forEach((link,i)=>{link.parentElement.classList.toggle('active',i===index);if(i===index)link.setAttribute('aria-current','location');else link.removeAttribute('aria-current');});
 }
 view.addEventListener('scroll',update,{passive:true});view.addEventListener('resize',update);view.requestAnimationFrame(update);
 const page={renderBrand(brand){setPageTitle(document,btContent.copy[0].text,brand);}};
 instances.set(root,page);return page;
}
