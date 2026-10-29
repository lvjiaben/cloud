import {translate} from './locale.js';
import {initializeServerHero} from './server-hero.js';
// Static page content never consumes retired template configuration.
const observedIntros=new WeakSet();
const observedScrollEffects=new WeakSet();
function initializeHomeScrollEffects(document){
 if(observedScrollEffects.has(document))return;observedScrollEffects.add(document);
 const view=document.defaultView,effects=[
  ['.sk-title-container.wow','pulse'],
  ['.systemproduct-right-container.wow','slideInRight'],
  ['.chooseUs-min-container.lina-benefit-pulse','pulse'],
  ['.comprehensive-news-right.wow','slideInRight'],
 ];
 const targets=effects.flatMap(([selector,name])=>[...document.querySelectorAll(selector)].map(target=>({target,name})));
 if(!targets.length)return;
 if(view.matchMedia('(prefers-reduced-motion: reduce)').matches||!view.IntersectionObserver){
  targets.forEach(({target})=>target.classList.add('lina-scroll-static'));
  return;
 }
 targets.forEach(({target})=>target.classList.add('lina-scroll-pending'));
 const observer=new view.IntersectionObserver(entries=>{for(const entry of entries)if(entry.isIntersecting){
  const effect=targets.find(({target})=>target===entry.target);if(!effect)continue;
  entry.target.classList.remove('lina-scroll-pending');entry.target.classList.add('lina-scroll-revealed','animate__animated','animate__'+effect.name);observer.unobserve(entry.target);
 }},{threshold:0.12});
 targets.forEach(({target})=>observer.observe(target));
}

function observeProductIntro(document,intro){
 if(observedIntros.has(intro))return;observedIntros.add(intro);
 const view=document.defaultView;if(!view.IntersectionObserver||view.matchMedia('(prefers-reduced-motion: reduce)').matches)return;
 const observer=new view.IntersectionObserver(entries=>{for(const entry of entries)if(entry.isIntersecting){entry.target.classList.add(entry.target===intro?'lina-product-intro-entered':'lina-product-cards-entered');observer.unobserve(entry.target);}},{threshold:0.1});observer.observe(intro);
 const cards=intro.parentElement.querySelector('.systemproduct-right-container');if(cards)observer.observe(cards);
}

const partnerObservers=new WeakMap();
function fitPartnerTracks(section){
 const fit=()=>{for(const row of section.querySelectorAll('.partners-item-container')){
  if(row.hidden)continue;
  const track=row.querySelector('.partners-item-row'),page=track.firstElementChild;
  if(!page)continue;
  page.querySelectorAll('[data-partner-repeat]').forEach(node=>node.remove());
  const cards=[...page.children],width=cards[0]?.getBoundingClientRect().width;
  if(!width||!cards.length)continue;
  const target=Math.max(13,Math.ceil(row.clientWidth/width));
  for(let i=cards.length;i<Math.max(cards.length,target);i++){const card=cards[i%cards.length];const copy=card.cloneNode(true);copy.dataset.partnerRepeat='';copy.setAttribute('aria-hidden','true');copy.inert=true;page.append(copy);}
  const repeat=track.children[1];repeat.replaceChildren(...[...page.children].map(card=>card.cloneNode(true)));
 }};
 fit();const observer=new ResizeObserver(fit);observer.observe(section);partnerObservers.set(section.ownerDocument,observer);
}


const initializedHomes=new WeakSet();
const homeBlockTargets=[
 {path:'/newestactivity.html',customer:false,get title(){return translate('查看最新活动');}},
 {product:true},
 {path:'/account/referrals',customer:true,get title(){return translate('进入推广中心');}},
 {path:'/account/servers',customer:true,get title(){return translate('进入云服务器控制台');}},
];
export function initializeHomeBlocks(document,{safeURL,customerLink}={}){
 const row=document.querySelector('[data-home-blocks]');if(!row)return;
 const cards=[...row.querySelectorAll(':scope > .bannerbelow-list')];
 for(const [index,card] of cards.entries()){
  const target=homeBlockTargets[index];if(!target)continue;
  if(target.product)continue;
  card.dataset.homeTarget=target.path;
  card.title=target.title;
  if(target.customer){
   if(typeof customerLink==='function')customerLink(card,target.path);
   continue;
  }
  const url=typeof safeURL==='function'?safeURL(target.path):null;
  if(url){card.href=url.href;card.removeAttribute('aria-disabled');}
  else{card.removeAttribute('href');card.setAttribute('aria-disabled','true');}
 }
}
// Bind fixed HTML controls without consulting retired public configuration.
export function initializeStaticHome(document){
 if(initializedHomes.has(document))return;initializedHomes.add(document);
 initializeHomeScrollEffects(document);
 const partnerSection=document.querySelector(".container_bg30");if(partnerSection){fitPartnerTracks(partnerSection);document.defaultView.addEventListener("pagehide",()=>partnerObservers.get(document)?.disconnect(),{once:true});}
 initializeServerHero(document);
 for(const track of document.querySelectorAll('.partners-item-row')){const repeat=track.children[1];if(repeat){repeat.setAttribute('aria-hidden','true');repeat.inert=true;}}
}
export function renderTemplateHome(document,links){
 initializeStaticHome(document);
 const intro=document.querySelector("[data-home-product-intro]");
 if(intro)observeProductIntro(document,intro);
}

// Footer display is static HTML, independent of promise/link configuration.
export function renderTemplateFooter(document){
 const footer=document.querySelector('.sky-footer-top');if(!footer)return;
 let count=0;
 for(const card of footer.children){
  card.removeAttribute('href');card.removeAttribute('onclick');card.removeAttribute('tabindex');card.onclick=null;
  const image=card.querySelector('img');image.hidden=!image.getAttribute('src');
  const title=card.querySelector('.sky-footer-title').textContent.trim();
  card.hidden=!title;count+=Number(!!title);
 }
 footer.hidden=!count;
}

// These two sections are explicitly approved static source displays.
export function initializeStaticSolutions(document){
 const section=document.querySelector(".container_bg15");if(!section?.querySelector(".main-option"))return;
 const tabs=[...section.querySelectorAll(".main-option-name")],profiles=[...section.querySelectorAll(".sidebar-profile")],details=[...section.querySelectorAll(".main-option-line")];
 section.querySelector(".main-option").setAttribute("role","tablist");
 const select=index=>{for(const group of [profiles,details])group.forEach((n,i)=>{n.hidden=i!==index;n.classList.toggle("active",i===index);});tabs.forEach((n,i)=>{n.classList.toggle("active",i===index);n.setAttribute("aria-selected",String(i===index));});};
 tabs.forEach((tab,i)=>{tab.setAttribute("role","tab");tab.tabIndex=0;tab.onclick=()=>select(i);tab.onfocus=()=>tab.scrollIntoView({block:"nearest",inline:"nearest"});tab.onkeydown=e=>{if(["Enter"," "].includes(e.key)){e.preventDefault();select(i);}};});select(0);
}

