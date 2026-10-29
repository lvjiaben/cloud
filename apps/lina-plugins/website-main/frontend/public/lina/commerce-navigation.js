// Keep the public catalog shell alive; only the right-hand view is exchanged.
export function createCommerceNavigation({document,activate,isBusy,status,retry}){
 const views=new Map([[document.body.dataset.page,document.querySelector('.shopping-cart-product')]]),pending=new Map();
 let revision=0,activeURL=location.pathname+location.search;
 function target(value){
  const url=new URL(value,location.href);
  if(url.origin!==location.origin||!['/products.html','/configure.html'].includes(url.pathname))return null;
  const view=url.pathname==='/configure.html'||url.searchParams.get('action')==='configureproduct'?'configure':'products';
  url.pathname='/products.html';if(view==='configure')url.searchParams.set('action','configureproduct');else url.searchParams.delete('action');
  return {url,view};
 }
 async function panel(view){
  if(views.has(view))return views.get(view);
  if(!pending.has(view))pending.set(view,(async()=>{
   const response=await fetch('/lina/'+view+'-panel.html',{credentials:'same-origin'});
   if(!response.ok)throw new Error('Missing commerce view');
   const parsed=new DOMParser().parseFromString(await response.text(),'text/html'),content=parsed.querySelector('.shopping-cart-product');
   if(!content)throw new Error('Invalid commerce view');
   const template=parsed.querySelector('#product-card');if(template&&!document.querySelector('#product-card'))document.body.append(document.importNode(template,true));
   const result=document.importNode(content,true);views.set(view,result);return result;
  })().finally(()=>pending.delete(view)));
  return pending.get(view);
 }
 async function navigate(value,{replace=false,pop=false}={}){
  const next=target(value);if(!next)return false;
  if(isBusy()){if(pop)history.pushState(null,'',activeURL);return true;}
  const seq=++revision;
  try{
   const content=await panel(next.view);if(seq!==revision)return true;
   if(next.view==='configure'&&!document.querySelector('link[href$="lina/configuration-controls.css"]')){
    const css=document.createElement('link');css.rel='stylesheet';css.href='/lina/configuration-controls.css';css.dataset.commerceControls='';document.head.append(css);
   }
   const current=document.querySelector('.shopping-cart-product');if(content!==current)current.replaceWith(content);
   document.body.dataset.page=next.view;
   document.querySelector('.shopping-cart-page').classList.toggle('type_configureproduct',next.view==='configure');
   document.querySelector('.shopping-cart-content').classList.toggle('style_nav',next.view==='products');
   if(!pop)history[replace?'replaceState':'pushState'](null,'',next.url.pathname+next.url.search);
   activeURL=next.url.pathname+next.url.search;
   retry();await activate(next.view,next.url);
  }catch{if(seq===revision){status('页面暂时无法加载，请重试。');retry(()=>navigate(value,{replace,pop}));}}
  return true;
 }
 document.addEventListener('click',event=>{
  if(event.defaultPrevented||event.button!==0||event.ctrlKey||event.metaKey||event.shiftKey||event.altKey)return;
  const link=event.target.closest('a[href]');if(!link||link.hasAttribute('download')||link.target&&link.target!=='_self'||!target(link.href))return;
  event.preventDefault();void navigate(link.href);
 });
 window.addEventListener('popstate',()=>{void navigate(location.href,{pop:true});});
 return {navigate,start:()=>navigate(location.href,{replace:true})};
}
