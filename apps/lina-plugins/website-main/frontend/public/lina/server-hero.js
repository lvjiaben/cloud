import {localized} from './locale.js';
import {startBrandRefresh} from './brand-refresh.js';

const heroes=new WeakMap();
export function initializeServerHero(document){
 if(heroes.has(document))return heroes.get(document);
 const hero=document.querySelector('.Rotation-layoutcontainer');if(!hero)return;
 const shortcut=document.querySelector('[data-home-blocks] > .bannerbelow-list:nth-child(2)');
 function updateShortcut(product){
  if(!shortcut)return;
  shortcut.title='查看产品特惠';
  if(product?.availability==='available'){
   const path='/products.html?product='+encodeURIComponent(product.slug);
   shortcut.href=path;shortcut.dataset.homeTarget=path;shortcut.removeAttribute('aria-disabled');
  }else{shortcut.removeAttribute('href');delete shortcut.dataset.homeTarget;shortcut.setAttribute('aria-disabled','true');}
 }
 updateShortcut();
 const view=document.defaultView,media=view.matchMedia('(prefers-reduced-motion: reduce)');
 const slidesRoot=hero.querySelector('.swiper-wrapper'),textRoot=hero.querySelector('.swipervertical-textcontainer'),titlesRoot=hero.querySelector('.swipervertical-titles'),dots=hero.querySelector('.home-pagination');
 const templates={slides:[...slidesRoot.children].map(n=>n.cloneNode(true)),text:textRoot.firstElementChild.cloneNode(true),title:titlesRoot.querySelector('.swipervertical-title').cloneNode(true)};
 const line=titlesRoot.querySelector('.active-title-line');
 hero.style.setProperty('--slidePagination','5000ms');
 const status=document.createElement('div');status.className='lina-server-status';status.setAttribute('role','status');
 const message=document.createElement('span'),retry=document.createElement('button');retry.type='button';retry.textContent='重试';retry.hidden=true;status.append(message,retry);hero.append(status);
 let slides=[],texts=[],titles=[],items=[],active=0,timer,enabled=true,signature,configuration={},lastProducts=[];
 const configuredURL=value=>{try{if(typeof value!=="string"||!value.trim()||value.startsWith("//")||/[\\\r\n\0]/.test(value))return "";const url=new URL(value,document.location.origin);return ["http:","https:"].includes(url.protocol)&&!url.username&&!url.password?url.href:"";}catch{return "";}};
 const bind=(node,action)=>{node.setAttribute('role','button');node.tabIndex=0;node.onclick=action;node.onkeydown=e=>{if(['Enter',' '].includes(e.key)){e.preventDefault();action();}};};
 const position=()=>{const title=titles[active];line.hidden=!title;if(title){line.style.top=title.offsetTop+'px';line.style.height=title.offsetHeight+'px';}};
 const stop=()=>{view.clearTimeout(timer);timer=undefined;hero.dataset.autoplay='paused';};
 const schedule=()=>{
  stop();
  if(enabled&&!document.hidden&&items.length>1){
   // Restart the CSS progress together with the five-second timer after a selection or resume.
   void hero.offsetWidth;hero.dataset.autoplay='running';
   timer=view.setTimeout(()=>{if(enabled&&!document.hidden)select(active+1);},5000);
  }
 };
 function select(index){
  if(!items.length)return;
  active=(index+items.length)%items.length;
  updateShortcut(items[active]);
  slides.forEach((n,i)=>{n.classList.toggle('active',i===active);n.setAttribute('aria-hidden',String(i!==active));n.inert=i!==active;});
  texts.forEach((n,i)=>{n.hidden=i!==active;n.classList.toggle('active',i===active);n.setAttribute('aria-hidden',String(i!==active));});
  titles.forEach((n,i)=>{n.classList.toggle('active',i===active);n.setAttribute('aria-pressed',String(i===active));});
  [...dots.children].forEach((n,i)=>{n.classList.toggle('pagination-bullet-active',i===active);n.setAttribute('aria-pressed',String(i===active));});
  position();schedule();
 }
 const controls=['.home-button-prev','.home-button-next'].map(selector=>hero.querySelector(selector));
 controls.forEach((n,i)=>{n.setAttribute('aria-label',i?'下一张':'上一张');bind(n,()=>select(active+(i?1:-1)));});
 function update(products){
  lastProducts=products;
  const next=products.slice(0,4).map(p=>({slug:p.publicSlug,name:localized(p,'name'),summary:localized(p,'summary'),availability:p.availability}));
  if(next[0]){if(configuration.herotitle)next[0].name=configuration.herotitle;if(configuration.herodescription)next[0].summary=configuration.herodescription;}
  const key=JSON.stringify([next,configuration.heroimage,configuration.hero1href]);message.textContent=next.length?'':'暂无已发布的服务器。';retry.hidden=true;
  if(key===signature)return;
  signature=key;const previous=items[active]?.slug;items=next;
  slidesRoot.replaceChildren();textRoot.replaceChildren();titlesRoot.replaceChildren(line);dots.replaceChildren();slides=[];texts=[];titles=[];
  next.forEach((p,i)=>{
   const slide=templates.slides[i%templates.slides.length].cloneNode(true),text=templates.text.cloneNode(true),title=templates.title.cloneNode(true),dot=document.createElement('button');
   for(const n of [slide,text,title]){n.dataset.index=String(i);n.removeAttribute('hidden');n.classList.remove('active');}
   slide.removeAttribute('href');slide.querySelector('img').alt='';
   if(i===0&&configuredURL(configuration.heroimage))slide.querySelector('img').src=configuredURL(configuration.heroimage);
   title.textContent=p.name;text.title=p.name;text.querySelector('.Title').textContent=p.name;text.querySelector('.Describe').textContent=p.summary;
   const buy=text.querySelector('.Button');buy.removeAttribute('title');buy.removeAttribute('href');buy.querySelector('span').textContent=p.availability==='available'?'立即购买':p.availability==='manual_confirmation'?'需人工确认':'暂不可订购';
   buy.setAttribute('aria-disabled',String(p.availability!=='available'));
   if(p.availability==='available')buy.href=i===0&&configuredURL(configuration.hero1href)||'/products.html?product='+encodeURIComponent(p.slug);
   dot.type='button';dot.className='pagination-bullet';dot.setAttribute('aria-label','第'+(i+1)+'张');dot.onclick=()=>select(i);bind(title,()=>select(i));
   slidesRoot.append(slide);textRoot.append(text);titlesRoot.append(title);dots.append(dot);slides.push(slide);texts.push(text);titles.push(title);
  });
  controls.forEach(n=>{n.hidden=next.length<2;});dots.hidden=next.length<2;
  select(Math.max(0,next.findIndex(p=>p.slug===previous)));if(!next.length){updateShortcut();stop();position();}
 }
 // Clear reference-site sales copy before any asynchronous public read.
 slidesRoot.replaceChildren();textRoot.replaceChildren();titlesRoot.replaceChildren(line);dots.replaceChildren();line.hidden=true;controls.forEach(n=>{n.hidden=true;});message.textContent='正在加载服务器…';
 view.addEventListener('resize',position);
 media.addEventListener('change',schedule);
 document.addEventListener('visibilitychange',schedule);
 const observer=new view.ResizeObserver(position);observer.observe(titlesRoot);
 view.addEventListener('pagehide',()=>{enabled=false;stop();observer.disconnect();});
 view.addEventListener('pageshow',()=>{enabled=true;observer.observe(titlesRoot);position();schedule();});
 const controller={update,configure(value){const next=value||{};if(JSON.stringify(configuration)===JSON.stringify(next))return;configuration=next;update(lastProducts);},fail(){message.textContent=items.length?'服务器资料更新失败，暂时显示上次内容。':'服务器加载失败，请重试。';retry.hidden=false;},bindRetry(action){retry.onclick=action;}};
 heroes.set(document,controller);return controller;
}

export function startServerHero(document,request){
 const hero=initializeServerHero(document);if(!hero)return;
 const refresh=startBrandRefresh(document,async()=>{try{const data=await request('/x/idc-core/api/v1/public/catalog/products?limit=4');if(!Array.isArray(data.products))throw new Error('Invalid catalog');hero.update(data.products);}catch{hero.fail();}});
 hero.bindRetry(refresh);
}

export function renderServerHeroConfiguration(document,configuration){initializeServerHero(document)?.configure(configuration);}
