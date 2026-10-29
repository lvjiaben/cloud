import {WebsitePage} from './SourceWebsitePage';

export class ContactMediaPage extends WebsitePage {
 async mediaViolations(kind:'about'|'contact'){
  return this.page.evaluate(kind=>{
   const images=Array.from(document.querySelectorAll('.et-main [src]'));
   if(kind==='about')return images.length?['about-src']:[];
   const allowed=[['phone','static/picture/telephone.svg'],['email','static/picture/mailbox.svg']];
   const violations:string[]=[];
   for(const [key,src] of allowed){
    const nodes=document.querySelectorAll('[data-content-contact="'+key+'"] .currency-content-item-img > img');
    if(nodes.length!==1||nodes[0].getAttribute('src')!==src)violations.push(key);
   }
   for(const image of images){
    if(!allowed.some(([key,src])=>image.matches('[data-content-contact="'+key+'"] .currency-content-item-img > img')&&image.getAttribute('src')===src))violations.push('unexpected-src');
   }
   if(images.length!==2)violations.push('src-count');
   return violations;
  },kind);
 }
 async imageStates(){return this.page.locator('[data-content-contact] .currency-content-item-img > img').evaluateAll(nodes=>nodes.map(node=>{const image=node as HTMLImageElement;return {src:image.getAttribute('src'),alt:image.getAttribute('alt'),ariaHidden:image.getAttribute('aria-hidden'),decoded:image.complete&&image.naturalWidth>0};}));}
 async probeRejectedMedia(probe:string){
  const original=await this.page.locator('.et-main').innerHTML();
  try{
   await this.page.evaluate(probe=>{
    const phone=document.querySelector('[data-content-contact="phone"] .currency-content-item-img > img')!;
    const email=document.querySelector('[data-content-contact="email"] .currency-content-item-img > img')!;
    if(probe==='extra')phone.parentElement!.appendChild(phone.cloneNode(true));
    else if(probe==='wrong-card')email.setAttribute('src','static/picture/telephone.svg');
    else phone.setAttribute('src',probe);
   },probe);
   return await this.mediaViolations('contact');
  }finally{await this.page.locator('.et-main').evaluate((node,html)=>{node.innerHTML=html;},original);}
 }
}
