import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {parseHTML} from 'linkedom';

export const sourceHTML=await readFile(new URL('./partner-source.html',import.meta.url),'utf8');
export const reference=JSON.parse(await readFile(new URL('./partner-copy-reference.json',import.meta.url),'utf8'));
export const sourceDocument=parseHTML(sourceHTML).document;
export const shape=node=>node.nodeType===3?node.textContent:node.nodeType===1?{tag:node.tagName.toLowerCase(),class:node.className,attrs:[...node.attributes].filter(a=>a.name!=='href'&&a.name!=='src'&&a.name!=='style').map(a=>[a.name,a.value]),children:[...node.childNodes].filter(n=>n.nodeType!==8).map(shape)}:null;
export const copy=doc=>{
 const records=[];
 for(const [index,selector]of ['.globalBanner','.et-container','main.et-main'].entries()){
  const root=doc.querySelector(selector),mainSections=['advantages','benefits','requirements','process','faq'];
  const visit=(element,path)=>[...element.childNodes].forEach((node,position)=>{
   if(node.nodeType===3&&node.textContent.trim())records.push({section:index===2?mainSections[[...root.querySelectorAll(':scope > .et-slide')].indexOf(element.closest('.et-slide'))]:index===0?'hero':'anchors',parent:path,tag:element.tagName.toLowerCase(),level:/^H[1-6]$/.test(element.tagName)?Number(element.tagName[1]):null,text:node.textContent});
   if(node.nodeType===1&&!['SCRIPT','STYLE'].includes(node.tagName))visit(node,path+'/'+node.tagName.toLowerCase()+(node.className?'.'+node.className.split(/\s+/).join('.'):'')+'['+position+']');
  });
  visit(root,index===0?'hero':index===1?'anchors':'body');
 }
 return records;
};
export const sourceHash=createHash('sha256').update(sourceHTML).digest('hex');
