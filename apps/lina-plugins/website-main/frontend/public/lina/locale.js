// Native-HTML adapter for the original website's zh-CN / en-US public copy.
import {englishCopy,contentPlaceholderEnglishCopy} from './locale-copy.js';
export const localeStorageKey='lina.website.locale';
export function readLocale(view=globalThis.window){
 try{return view?.localStorage.getItem(localeStorageKey)==='en-US'?'en-US':'zh-CN';}catch{return 'zh-CN';}
}
let current=readLocale();
const pairs=new Map(Object.entries(englishCopy));
const reverse=new Map([...pairs].map(([zh,en])=>[en,zh]));
export const getLocale=()=>current;
export function localizeAssetImage(image,zhCN,enUS){
 if(!image)return image;
 image.dataset.localeSrcZhCn=zhCN;image.dataset.localeSrcEnUs=enUS;
 const source=current==='en-US'?enUS:zhCN;
 if(image.getAttribute('src')!==source)image.setAttribute('src',source);
 return image;
}
const pageTitles=new WeakMap();
// Keep a page's fixed title separate from its bilingual merchant name.
export function setPageTitle(document,title,brand){
 let state=pageTitles.get(document);
 if(!state){
  state={title:'',brand:{}};pageTitles.set(document,state);
  document.addEventListener('lina:locale-changed',()=>{
   const name=localized(state.brand.siteName);
   document.title=translate(state.title)+(name?' - '+name:'');
  });
 }
 state.title=reverse.get(title)||title;state.brand=brand;
 const name=localized(brand.siteName);
 document.title=translate(state.title)+(name?' - '+name:'');
}
export function localized(value,stem){
 if(!value)return '';
 if(typeof value==='string')return value;
 const zh=stem?value[stem+'ZhCN']:value.zhCN,en=stem?value[stem+'EnUS']:value.enUS;
 if(zh&&en&&zh!==en){pairs.set(zh,en);reverse.set(en,zh);}
 return current==='en-US'?(en||zh||''):(zh||en||'');
}
export function localizedName(value){return localized(value,'label')||localized(value,'name');}
// Attribute labels compose a localized field name with a controlled UI suffix.
export function localizedControlLabel(label,suffix){
 const zh=reverse.get(label)||label,en=pairs.get(zh)||label;
 return localized({zhCN:zh+suffix,enUS:en+' '+(pairs.get(suffix)||suffix).toLowerCase()});
}
export function localizedDate(value,time=false){
 const date=new Date(value);if(Number.isNaN(date.getTime()))return '';
 const format=locale=>time?date.toLocaleString(locale):date.toLocaleDateString(locale);
 const zh=format('zh-CN'),en=format('en-US');pairs.set(zh,en);reverse.set(en,zh);return current==='en-US'?en:zh;
}
export function translate(text){
 if(current!=='en-US'||typeof text!=='string')return text;
 const trimmed=text.trim(),exact=pairs.get(trimmed);
 if(exact)return text.replace(trimmed,exact);
 const slide=trimmed.match(/^第(\d+)张$/);if(slide)return 'Slide '+slide[1];
 const stock=trimmed.match(/^库存(\d+)$/);if(stock)return 'Stock: '+stock[1];
 const search=trimmed.match(/^在当前分类中搜索以“(.*)”开头的商品$/);if(search)return 'Search this category for products starting with “'+search[1]+'”';
 // Only controlled UI compositions, never arbitrary merchant prose.
 const prefixes=['商品分类：','发布时间：','24小时服务热线：','下载 ','关闭 ','展开 '];
 for(const prefix of prefixes)if(text.startsWith(prefix))return (pairs.get(prefix.trim())||prefix)+text.slice(prefix.length);
 return text;
}
export function initializeLocale(document){
 const view=document.defaultView,originals=new WeakMap(),attributes=new WeakMap();
 const articleOriginals=new WeakMap();
 const articleReverse=new Map(Object.entries(contentPlaceholderEnglishCopy).map(([zh,en])=>[en,zh]));
 function applyArticle(root){
  if(root.nodeType===3){
   if(root.parentElement?.closest('script,style,code,pre,textarea,[data-locale-original]'))return;
   const value=root.nodeValue,previous=articleOriginals.get(root);
   const original=previous?.rendered===value?previous.original:articleReverse.get(value.trim())||value;
   const english=contentPlaceholderEnglishCopy[original.trim()];
   if(!english)return;
   const rendered=current==='en-US'?original.replace(original.trim(),english):original;
   articleOriginals.set(root,{original,rendered});if(value!==rendered)root.nodeValue=rendered;
  }else for(const child of root.childNodes)applyArticle(child);
 }
 const excluded=node=>node.parentElement?.closest('script,style,code,pre,textarea,[data-content-body],[data-locale-original]');
 const renderValue=(target,value,states,key)=>{
  let state=states.get(target);if(!state){state=new Map();states.set(target,state);}
  const previous=state.get(key);
  const original=previous&&previous.rendered===value?previous.original:reverse.get(value)||value;
  const rendered=translate(original);state.set(key,{original,rendered});return rendered;
 };
 function apply(root){
  if(root.nodeType===3){if(root.parentElement?.closest('[data-content-body]'))applyArticle(root);else if(!excluded(root)){const value=renderValue(root,root.nodeValue,originals,'text');if(root.nodeValue!==value)root.nodeValue=value;}return;}
  if(root.nodeType!==1&&root.nodeType!==9&&root.nodeType!==11)return;
  if(root.nodeType===1){
   if(root.matches('[data-content-body]')){applyArticle(root);return;}
   if(root.matches('script,style,code,pre,textarea,[data-locale-original]'))return;
   const zhCN=root.getAttribute('data-locale-src-zh-cn'),enUS=root.getAttribute('data-locale-src-en-us');
   if(zhCN&&enUS){const source=current==='en-US'?enUS:zhCN;if(root.getAttribute('src')!==source)root.setAttribute('src',source);}
   for(const attr of ['title','placeholder','aria-label','alt'])if(root.hasAttribute(attr)){const value=renderValue(root,root.getAttribute(attr),attributes,attr);if(value!==root.getAttribute(attr))root.setAttribute(attr,value);}
  }
  for(const child of root.childNodes)apply(child);
 }
 const container=document.querySelector('.horizontal-menu-right-container')||document.querySelector('[data-locale-actions]');
 let button=container?.querySelector('[data-public-locale-switch]');
 if(container&&!button){button=document.createElement('button');button.type='button';button.className='lina-locale-switch';button.dataset.publicLocaleSwitch='';button.dataset.localeOriginal='';container.append(button);}
 function render(){
  document.documentElement.lang=current;
  for(const heading of document.querySelectorAll('[data-locale-heading-title]')){
   const title=heading.dataset.localeHeadingTitle;if(!title)continue;
   const text=[...heading.childNodes].find(node=>node.nodeType===3);if(text)text.nodeValue=current==='en-US'?'Region ('+translate(title)+')':'区域（'+(reverse.get(title)||title)+'）';
  }
  if(button){button.textContent=current==='en-US'?'中文':'English';button.setAttribute('aria-label',current==='en-US'?'切换到中文':'Switch to English');}
  apply(document.documentElement);

 }
 const setLocale=value=>{current=value==='en-US'?'en-US':'zh-CN';try{view.localStorage.setItem(localeStorageKey,current);}catch{}render();document.dispatchEvent(new view.CustomEvent('lina:locale-changed',{detail:{locale:current}}));};
 button?.addEventListener('click',()=>setLocale(current==='en-US'?'zh-CN':'en-US'));
 view.addEventListener('storage',event=>{if(event.key===localeStorageKey)setLocale(event.newValue);});
 // Render legacy HTML and asynchronous UI using the same explicit copy catalog.
 const observer=new view.MutationObserver(records=>{for(const record of records){if(record.type==='childList'){for(const added of record.addedNodes)apply(added);}else apply(record.target);}});
 render();observer.observe(document.documentElement,{childList:true,subtree:true,characterData:true,attributes:true,attributeFilter:['title','placeholder','aria-label','alt']});
 return {setLocale,render};
}
