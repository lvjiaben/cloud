import {localized} from './locale.js';
import {renderHeaderLogo} from './header-logo.js';
import {renderFooterIdentity} from './footer.js';

// Public identity is a short-lived first-paint hint; every page still reads the authority.
export const brandIdentityKey='lina.public-brand-identity.v1';
const maxAge=30000;
const fields=['siteName','footer','logoUrl','companyName','icpNumber','icpUrl','policeNumber','policeUrl','telecomLicenseNumber','telecomLicenseUrl','headerLogoUrl','footerLogoUrl','copyright','businessQq','companyAddress','contactPhone','contactEmail','enterpriseWechatQrCodeUrl','wechatOfficialQrCodeUrl'];

export function readServerBrandIdentityEntry(view=window,now=Date.now()){
 const hint=view.__LINA_PUBLIC_IDENTITY__;
 return hint?.brand&&Number.isFinite(hint.savedAt)&&now>=hint.savedAt&&now-hint.savedAt<=maxAge?hint:null;
}

export function readServerBrandIdentity(view=window,now=Date.now()){
 return readServerBrandIdentityEntry(view,now)?.brand||null;
}

export function readBrandIdentityEntry(view=window,now=Date.now()){
 try{
  const entry=JSON.parse(view.localStorage.getItem(brandIdentityKey)||'null');
  if(!entry||!Number.isFinite(entry.savedAt)||now<entry.savedAt||now-entry.savedAt>maxAge||!entry.brand||typeof entry.brand!=='object'||Array.isArray(entry.brand))return null;
  return entry;
 }catch{return null;}
}

export function readBrandIdentity(view=window,now=Date.now()){
 return readBrandIdentityEntry(view,now)?.brand||null;
}

export function writeBrandIdentity(brand,view=window,now=Date.now()){
 try{view.localStorage.setItem(brandIdentityKey,JSON.stringify({savedAt:now,brand:Object.fromEntries(fields.map(key=>[key,brand[key]]))}));}catch{/* Storage is optional; the public response remains authoritative. */}
}

export function renderBrandIdentity(document,brand){
 const name=localized(brand.siteName);
 for(const slot of document.querySelectorAll('[data-brand-name]'))if(!slot.closest('.horizontal-menu-navbar-logo'))slot.textContent=name;
 renderHeaderLogo(document,brand,name);
 renderFooterIdentity(document,brand);
}
