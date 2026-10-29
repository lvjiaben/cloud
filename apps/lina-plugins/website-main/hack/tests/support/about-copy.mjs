// Independent attachment reader: only indentation and the explicitly approved empty fields are excluded.
export function collectAboutSource(documentRoot=document){
 const roots=[documentRoot.querySelector('.globalBanner'),documentRoot.querySelector('.et-container'),documentRoot.querySelector('main.et-main')],rows=[];
 roots.forEach((root,index)=>{
  function visit(el,path){
   if(el.matches('.companyIntroduction-right-title,.companyIntroduction-right-content,.slowNumber'))return;
   for(const node of el.childNodes){
    if(node.nodeType===3&&node.textContent.trim())rows.push({section:index===0?'hero':index===1?'anchors':el.closest('section[id]')?.id||'main',order:rows.length,parent:path,tag:el.localName,headingLevel:/^h[1-6]$/.test(el.localName)?+el.localName[1]:null,text:node.textContent.trim(),interaction:el.closest('.programme-content-item')?'card-'+[...documentRoot.querySelectorAll('.programme-content-item')].indexOf(el.closest('.programme-content-item')):index===1?'scroll-navigation':'visible'});
    if(node.nodeType===1&&!['script','style','img'].includes(node.localName)){const classes=[...node.classList].filter(c=>c!=='active');visit(node,path+'/'+node.localName+(classes.length?'.'+classes.join('.'):''));}
   }
  }
  visit(root,['hero','anchors','main'][index]);
 });return rows;
}
