export function collectCertificateSource(document=globalThis.document){
 const root=document.querySelector('.globalBanner');
 return [...root.querySelectorAll('h1,p')].map((e,i)=>({section:'hero',order:i,parent:'section.globalBanner/div.comp_container/div.pagebanner-relative/div.pagebanner/div.pagebanner-text',tag:e.tagName.toLowerCase(),headingLevel:e.tagName==='H1'?1:null,text:e.textContent,interaction:'visible'}));
}
