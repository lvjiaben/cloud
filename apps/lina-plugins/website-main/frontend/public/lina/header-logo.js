// Render the independent public header logo; empty or failed images use the site name.
export function renderHeaderLogo(document,brand,siteName){
 const raw=typeof (brand.headerLogoUrl||brand.logoUrl)==='string'?(brand.headerLogoUrl||brand.logoUrl).trim():'';
 let source='';
 if(raw&&raw.length<=2048&&!raw.startsWith('//')&&!/[\\\r\n\0]/.test(raw)){
  try{
   const url=new URL(raw,document.location.origin);
   if(['http:','https:'].includes(url.protocol)&&!url.username&&!url.password&&(raw.startsWith('/')||/^https?:\/\//i.test(raw)))source=url.href;
  }catch{/* Invalid public image values fall back to the website name. */}
 }
 for(const slot of document.querySelectorAll('.horizontal-menu-navbar-logo [data-brand-name]')){
  const existing=slot.querySelector('img');
  slot.setAttribute('aria-label',siteName);
  if(!source){if(slot.textContent!==siteName||existing)slot.textContent=siteName;delete slot.dataset.headerLogoSource;continue;}
  // Preserve loaded images and failed-image fallback until the configured URL changes.
  if(slot.dataset.headerLogoSource===source){if(existing)existing.alt=siteName;else slot.textContent=siteName;continue;}
  slot.dataset.headerLogoSource=source;
  const image=document.createElement('img');
  image.className='horizontal-logosize';image.alt=siteName;
  image.addEventListener('error',()=>{if(image.parentNode===slot)slot.textContent=slot.getAttribute('aria-label')||'';},{once:true});
  image.src=source;slot.replaceChildren(image);
 }
}
