import {localized,translate} from './locale.js';
// Public website fields only; missing identity or qualification data stays hidden.
const defaultFooterNavigation=new WeakMap();
const footerNavigationSignatures=new WeakMap();
// Capture untranslated template nodes before the locale adapter renders English.
export function initializeFooter(document){
 const root=document.querySelector('[data-footer-navigation]');
 if(root&&!defaultFooterNavigation.has(root))defaultFooterNavigation.set(root,root.cloneNode(true));
 for(const [key,label] of [['qq','商务QQ：'],['address','公司地址：']]){
  const row=[...document.querySelectorAll('.sky-footer-left-itme')].find(item=>item.querySelector('p')?.textContent.trim()===label);
  if(row)row.dataset.footerDetail=key;
 }
}
export function bindCertificateFooter(document){
 for(const a of document.querySelectorAll('.sky-footer-nav-item a'))if(['荣誉资质','资质荣誉','Certifications'].includes(a.textContent.trim())){a.href='/certificate.html';a.removeAttribute('aria-disabled');a.removeAttribute('title');a.removeAttribute('target');a.removeAttribute('rel');}
}
export function bindAboutFooter(document){
 for(const a of document.querySelectorAll('.sky-footer-nav-item a'))if(['公司简介','Company Profile'].includes(a.textContent.trim())){a.href='/about.html';a.removeAttribute('aria-disabled');a.removeAttribute('title');a.removeAttribute('target');a.removeAttribute('rel');}
}
// The independent website common settings own footer identity and contact details.
function renderFooterDetails(document,footer,brand){
 const left=footer.querySelector('.sky-footer-left');
 const text=value=>typeof value==='string'&&!/[\x00-\x1f\x7f]/.test(value)?value.trim():'';
 const imageURL=value=>{const raw=text(value);if(!raw||raw.length>2048||raw.startsWith('//')||raw.includes('\\'))return null;try{const url=new URL(raw,document.location.origin);return ['http:','https:'].includes(url.protocol)&&!url.username&&!url.password&&(raw.startsWith('/')||/^https?:\/\//i.test(raw))?url.href:null;}catch{return null;}};
 let copy=left.querySelector('.sky-footer-copy');
 if(!copy){copy=document.createElement('div');copy.className='sky-footer-copy';copy.dataset.footerCopyright='';left.querySelector('.sky-footer-companyInformation').append(copy);}
 copy.dataset.footerCopyright='';copy.textContent=translate(text(brand.copyright));copy.hidden=!copy.textContent;
 let logo=left.querySelector('[data-footer-identity-logo]');const src=imageURL(brand.footerLogoUrl||brand.logoUrl);
 if(src){if(!logo){logo=document.createElement('div');logo.className='sky-footer-left-logo';logo.dataset.footerIdentityLogo='';logo.append(document.createElement('img'));left.prepend(logo);}const img=logo.firstElementChild;if(img.getAttribute('src')!==src)img.src=src;img.alt=localized(brand.footer);}else logo?.remove();
 for(const [key,label,value] of [['qq','商务QQ：',brand.businessQq],['address','公司地址：',brand.companyAddress]]){
  let row=left.querySelector('[data-footer-detail='+key+']')||[...left.querySelectorAll('.sky-footer-left-itme')].find(row=>row.querySelector('p')?.textContent.trim()===label);
  if(!row){row=document.createElement('div');row.className='sky-footer-left-itme';row.dataset.footerDetail=key;const p=document.createElement('p');p.textContent=label;row.append(p,document.createElement('a'));left.append(row);}
  row.dataset.footerDetail=key;const a=row.querySelector('a'),valueText=text(value),valid=!!valueText&&(key!=='qq'||/^\d{1,20}$/.test(valueText));
  row.hidden=!valid;a.textContent=valid?translate(valueText):'';for(const attr of ['href','target','rel'])a.removeAttribute(attr);
  if(valid&&key==='qq'){a.href='https://wpa.qq.com/msgrd?v=3&uin='+encodeURIComponent(valueText)+'&site=qq&menu=yes';a.target='_blank';a.rel='noopener noreferrer';}
 }
 let socials=left.querySelector('.sky-footer-socials');
 if(!socials){socials=document.createElement('div');socials.className='sky-footer-socials';socials.dataset.footerSocials='';left.append(socials);}
 socials.dataset.footerSocials='';socials.replaceChildren();
 for(const [value,label]of [[brand.enterpriseWechatQrCodeUrl,'企业微信'],[brand.wechatOfficialQrCodeUrl,'微信公众号']]){const url=imageURL(value);if(!url)continue;const card=document.createElement('div'),img=document.createElement('img'),p=document.createElement('p');card.className='sky-footer-ewm';img.src=url;img.alt=label;p.className='sky-footer-ewm-title';p.textContent=label;card.append(img,p);socials.append(card);}
 socials.hidden=!socials.childElementCount;
}
export function renderFooterIdentity(document,brand){
 const local=localized;
 const footer=document.querySelector(".sky-footer-container"),company=footer.querySelector("[data-brand-footer]");
 company.textContent=translate(local(brand.footer));company.hidden=!company.textContent;
 renderFooterDetails(document,footer,brand);
 for(const [key,value,protocol] of [["phone",brand.contactPhone,"tel:"],["email",brand.contactEmail,"mailto:"]]){
  const row=footer.querySelector("[data-footer-contact="+key+"]"),a=row.querySelector("a");
  const valid=typeof value==="string"&&value.trim()&&!/[\r\n\x00-\x1f]/.test(value);row.hidden=!valid;a.textContent=valid?value:"";a.removeAttribute("href");if(valid)a.href=protocol+encodeURIComponent(value).replaceAll("%40","@").replaceAll("%2B","+");
 }
}
export function renderBrandFooter(document,brand,safeURL){
 renderFooterIdentity(document,brand);
 const local=localized,footer=document.querySelector('.sky-footer-container');
 const root=footer.querySelector("[data-footer-navigation]");
 initializeFooter(document);
 const navigationSignature=JSON.stringify(brand.footerNavigation||[]);
 if(footerNavigationSignatures.get(root)!==navigationSignature){
 if(brand.footerNavigation?.length&&document.body.dataset.page!=='safety'){
  root.replaceChildren();
  const append=(parent,item)=>{const a=document.createElement("a"),row=document.createElement("div");row.className="sky-footer-nav-item";a.textContent=local(item.label);row.append(a);parent.append(row);const url=safeURL(item.href,item.customer);if(url&&!item.group){a.href=url.href;if(item.external){a.target="_blank";a.rel="noopener noreferrer";}}for(const child of item.children||[])append(parent,child);};
  for(const item of brand.footerNavigation){const column=document.createElement("div");column.className="sky-footer-nav-column";if(item.group||item.children?.length){const heading=document.createElement("div");heading.className="sky-footer-nav-heading";heading.textContent=local(item.label);column.append(heading);for(const child of item.children||[])append(column,child);}else append(column,item);root.append(column);}
 }
 else root.replaceChildren(...[...defaultFooterNavigation.get(root).childNodes].map(child=>child.cloneNode(true)));
 footerNavigationSignatures.set(root,navigationSignature);
 }
 // Both historic labels refer to the same local qualification page.
 bindCertificateFooter(document);
 bindAboutFooter(document);
 const terms=safeURL(brand.termsUrl);if(terms){const a=document.querySelector(".comprehensive-news-left-inner-button a");if(a){a.href=terms.href;a.removeAttribute("aria-disabled");a.removeAttribute("title");}}
 // Only the independent website common registrations are public; legacy template fields stay inert.
 for(const row of footer.querySelectorAll('[data-filing-kind]')){
  const kind=row.dataset.filingKind,key=kind==='icp'?'icp':kind==='public-security'?'police':kind==='telecom-license'?'telecomLicense':null;
  const number=key&&typeof brand[key+'Number']==='string'?brand[key+'Number'].trim():'';
  row.hidden=!number;row.querySelector('.filing-title').textContent=translate(number);
  // Registration numbers share the same local, non-brand pictograms.
  const iconSource=key==='police'||key==='telecomLicense'?'/static/filing/gongan.png':key==='icp'?'/static/filing/local657930f11d1cd.png':null;
  let icon=row.querySelector('.filing-icon');
  if(number&&iconSource){if(icon?.tagName!=='IMG'){const img=document.createElement('img');img.className='filing-icon';img.alt='';img.setAttribute('aria-hidden','true');if(icon)icon.replaceWith(img);else row.prepend(img);icon=img;}if(icon.getAttribute('src')!==iconSource)icon.setAttribute('src',iconSource);}
  else if(icon)icon.remove();
  for(const attr of ['href','target','rel'])row.removeAttribute(attr);
  const value=key&&brand[key+'Url'];let url;
  try{url=typeof value==='string'&&/^https?:\/\//i.test(value)&&!/[\x00-\x20\\]/.test(value)?new URL(value):null;}catch{url=null;}
  if(number&&url&&!url.username&&!url.password){row.href=url.href;row.target='_blank';row.rel='noopener noreferrer';}
 }
}
