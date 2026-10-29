import {gameHTML,gameShell} from './game-content.js';
import {localizeAssetImage,setPageTitle} from './locale.js';

const gamePages=new WeakMap();

export function initializeGame(document) {
 const root=document.querySelector('[data-game]');
 if(!root)return null;
 if(gamePages.has(root))return gamePages.get(root);
 const header=document.querySelector('[data-game-header]');
 const footer=document.querySelector('[data-game-footer]');
 if(header)header.outerHTML=gameShell.headerHTML;
 if(footer)footer.outerHTML=gameShell.footerHTML;
 root.innerHTML=gameHTML;
 localizeAssetImage(root.querySelector('.architecture-Content-right img'),'/lina/game-assets/remoteL17139291334123.png','/lina/game-assets/remoteL17139291334123.en.svg');
 const view=document.defaultView,bar=root.querySelector('.et-container');
 const links=[...bar.querySelectorAll('.et-hero-tab a')],sections=[...root.querySelectorAll('.et-slide')];
 const marker=document.createElement('div');bar.before(marker);
 function update(){
  root.style.setProperty('--game-anchor-height',bar.offsetHeight+'px');
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
  const logo=document.querySelector('.horizontal-menu-navbar-logo [data-brand-name]');
  const image=logo?.querySelector('img');
  if(logo&&!image)logo.replaceChildren();
  image?.addEventListener('error',()=>logo.replaceChildren(),{once:true});
  // The existing support adapter owns the configured customer-service destination.
  const target=document.querySelector('[data-lina-support-bar] a.consultation-button[href]');
  for(const a of root.querySelectorAll('[data-game-contact]')){
   if(target){a.href=target.href;a.removeAttribute('aria-disabled');}
   else {a.removeAttribute('href');a.setAttribute('aria-disabled','true');}
  }
 }};
 gamePages.set(root,page);
 return page;
}
