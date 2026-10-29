export function collectAssureCopy(document=globalThis.document){
 const names=['hero','navigation','support','refund','report'];
 const scope=document.querySelector('[data-assure]')||document;
 const roots=[scope.querySelector('.globalBanner'),scope.querySelector('.et-container'),...scope.querySelectorAll('main.et-main>.et-slide')];
 const records=[];
 roots.forEach((root,index)=>{
  function visit(element,path){
   [...element.childNodes].forEach((node,position)=>{
    if(node.nodeType===3&&node.textContent.trim())records.push({section:names[index],order:records.length,parent:path,tag:element.tagName.toLowerCase(),headingLevel:/^H[1-6]$/.test(element.tagName)?Number(element.tagName[1]):null,text:node.textContent.replace(/\r\n?/g,'\n'),interaction:index===1?'scroll-navigation':'visible',...(element.matches('a[href^="tel:"],[data-assure-phone]')?{binding:'contactPhone'}:{})});
    if(node.nodeType===1&&!['IMG','SCRIPT','STYLE'].includes(node.tagName))visit(node,path+'/'+node.tagName.toLowerCase()+(node.className?'.'+node.className.split(/\s+/).join('.'):'')+'['+position+']');
   });
  }
  visit(root,names[index]);
 });
 return records;
}
export const canonicalAssureCopy=records=>records.map(item=>({...item,text:item.binding?'{{contactPhone}}':item.text}));
