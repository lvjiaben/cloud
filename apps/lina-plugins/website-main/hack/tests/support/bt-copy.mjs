export function collectBtCopy(document=globalThis.document){
 const scope=document.querySelector('[data-bt]')||document;
 const roots=scope.querySelector('[data-bt-section]')?[...scope.querySelectorAll('[data-bt-section]')]:[document.querySelector('.globalBanner'),document.querySelector('.et-container'),...document.querySelectorAll('main.et-main>.et-slide')];
 const names=['hero','anchors','advantages','systems','install','mount','notice'],records=[];
 function sourcePath(element){if(element.tagName==='BODY')return 'body';return sourcePath(element.parentElement)+'/'+element.tagName.toLowerCase()+'['+([...element.parentElement.children].indexOf(element)+1)+']';}
 roots.forEach((root,index)=>{
  root.normalize();
  function visit(element,path){
   const direct=[...element.childNodes].filter(node=>node.nodeType===3).map(node=>node.textContent).join('');if(direct.trim())records.push({section:names[index],order:records.length,parent:path,tag:element.tagName.toLowerCase(),headingLevel:/^H[1-6]$/.test(element.tagName)?Number(element.tagName[1]):null,text:direct.replace(/\r\n?/g,'\n'),interaction:index===1?'anchor-navigation':'visible'});
   [...element.children].forEach((child,i)=>visit(child,path+'/'+child.tagName.toLowerCase()+'['+(i+1)+']'));
  }
  visit(root,root.getAttribute('data-bt-source-root')||sourcePath(root));
 });
 return records;
}

export const canonicalBtCopy=records=>records.map(item=>{let text=item.text;const lf=String.fromCharCode(10);if(text.includes(lf)){const lines=text.split(lf);if(!lines[0].trim()){lines.shift();lines[0]=lines[0].trimStart();}if(!lines.at(-1).trim())lines.pop();text=lines.join(lf);}return {...item,text};});