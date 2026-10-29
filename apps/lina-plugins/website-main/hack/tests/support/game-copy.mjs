import {parseHTML} from 'linkedom';

export const gameSectionNames=['hero','anchors','advantages','problems','architecture','products','partners','more'];

// Keep raw text, including meaningful spaces; whitespace-only indentation is not copy.
export function collectGameCopy(document){
 const roots=[document.querySelector('.globalBanner'),document.querySelector('.et-container'),...document.querySelectorAll('main.et-main>.et-slide')];
 const records=[];
 roots.forEach((root,index)=>{
  const visit=(element,path)=>{
   for(const [position,node]of [...element.childNodes].entries()){
    if(node.nodeType===3&&node.textContent.trim())records.push({section:gameSectionNames[index],order:records.length,parent:path,tag:element.tagName.toLowerCase(),headingLevel:/^H[1-6]$/.test(element.tagName)?Number(element.tagName[1]):null,text:node.textContent,interaction:index===1?'anchor-navigation':'visible'});
    if(node.nodeType===1&&!['IMG','SCRIPT','STYLE'].includes(node.tagName))visit(node,path+'/'+node.tagName.toLowerCase()+(node.className?'.'+node.className.split(/\s+/).join('.'):'')+'['+position+']');
   }
  };
  visit(root,gameSectionNames[index]);
 });
 return records;
}

export function parseGameSource(html){return parseHTML(html).document;}

export function extractGameNavigation(document,destinations){
 const known=new Map();
 const remember=items=>items.forEach(item=>{known.set(item.label,item);remember(item.children||[]);});
 remember(destinations);
 const card=element=>{
  const label=element.querySelector('.Submenu-title').textContent;
  const existing=known.get(label);
  return {label,description:element.querySelector('.Submenu-desc')?.textContent||'',href:existing?.href||'/404.html',...(element.querySelector('.Submenu-tag')?{badge:element.querySelector('.Submenu-tag').textContent}:{})};
 };
 return [...document.querySelectorAll('.horizontal-menu-navbar-menu>li')].map(li=>{
  const anchor=li.querySelector('a'),key=anchor.dataset.key;
  const group=document.querySelector('.nav-data>[data-key="'+key+'"]');
  const label=anchor.textContent.trim();
  if(!group)return {label,href:known.get(label)?.href||'/404.html'};
  const result={label,description:group.querySelector('.Submenu-menuInfo>.Submenu-desc')?.textContent||'',headingTag:group.querySelector('.Submenu-menuInfo-title')?.tagName.toLowerCase()||'h1'};
  if(label==='产品与服务')return {...result,href:'/products.html'};
  const views=[...group.querySelectorAll('.page-item')],tabs=[...group.querySelectorAll('.Submenu-page-name')];
  result.group=true;
  result.children=views.length?views.map((view,index)=>({label:tabs[index].textContent.trim(),group:true,children:[...view.querySelectorAll('.Submenu-item')].map(card)})):[...group.querySelectorAll('.Submenu-item')].map(card);
  const ad=group.querySelector('.advertisement-content');
  if(ad)result.promotionCopy={title:ad.querySelector('.advertisement-title').textContent,description:ad.querySelector('.advertisement-desc').textContent};
  return result;
 });
}
