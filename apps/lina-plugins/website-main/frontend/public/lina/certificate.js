import {financialShell} from './financial-content.js';
import {certificateHTML,certificateRecords} from './certificate-content.js';
import {setPageTitle} from './locale.js';
const instances=new WeakMap();
export function initializeCertificate(document){
 const root=document.querySelector('[data-certificate]');
 if(!root)return null;
 if(instances.has(root))return instances.get(root);
 document.querySelector('[data-certificate-header]').outerHTML=financialShell.headerHTML;
 document.querySelector('[data-certificate-footer]').outerHTML=financialShell.footerHTML;
 root.innerHTML=certificateHTML();
 // A hidden template reserves the card format without displaying invented qualifications.
 const template=document.createElement('template');template.dataset.certificateCardTemplate='';
 template.innerHTML='<article class="certificate-card"><button type="button" class="certificate-preview"><img alt=""></button><h3></h3><dl><div><dt>颁发机构</dt><dd data-certificate-issuer></dd></div><div><dt>有效期</dt><dd data-certificate-validity></dd></div></dl></article>';
 root.append(template);
 const grid=root.querySelector('[data-certificate-grid]');
 const imageURL=value=>{try{if(typeof value!=='string'||value.startsWith('//')||value.includes('\\'))return null;const u=new URL(value,document.location.origin);return ['http:','https:'].includes(u.protocol)&&!u.username&&!u.password?u.href:null;}catch{return null;}};
 for(const record of certificateRecords){
  const src=imageURL(record.image);if(!src||!record.name)continue;
  const card=template.content.firstElementChild.cloneNode(true),button=card.querySelector('button'),img=card.querySelector('img');img.src=src;img.alt=record.name;card.querySelector('h3').textContent=record.name;
  for(const [key,selector]of [['issuer','[data-certificate-issuer]'],['validity','[data-certificate-validity]']]){const slot=card.querySelector(selector);if(record[key])slot.textContent=record[key];else slot.parentElement.remove();}
  button.setAttribute('aria-label','查看'+record.name);
  button.addEventListener('click',()=>{
   const dialog=document.createElement('dialog'),close=document.createElement('button'),large=document.createElement('img');dialog.className='certificate-dialog';dialog.setAttribute('aria-label',record.name);close.textContent='关闭';large.src=src;large.alt=record.name;dialog.append(close,large);document.body.append(dialog);close.addEventListener('click',()=>dialog.close());dialog.addEventListener('close',()=>{dialog.remove();button.focus();},{once:true});dialog.showModal();
  });grid.append(card);
 }
 grid.dataset.empty=String(!grid.childElementCount);
 const page={renderBrand(brand){
  setPageTitle(document,'资质荣誉',brand);
  const support=document.querySelector('[data-lina-support-bar] a.consultation-button[href]');
  root.querySelector('[data-certificate-consult]').href=support?.href||'/contact.html';
 }};
 instances.set(root,page);return page;
}
