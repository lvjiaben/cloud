export const navigationCacheKey='lina.public-navigation.v1';
export const catalogCacheKey='lina.public-catalog.v1';
const maxAge=24*60*60*1000;
const catalogMaxAge=5*60*1000;

export function readNavigationCache(win=window,now=Date.now()){
 try{
  const entry=JSON.parse(win.localStorage.getItem(navigationCacheKey)||'null');
  if(!entry||!Number.isFinite(entry.savedAt)||now-entry.savedAt<0||now-entry.savedAt>maxAge||!Array.isArray(entry.items))return null;
  return entry.items;
 }catch{return null;}
}

export function writeNavigationCache(items,win=window,now=Date.now()){
 if(!Array.isArray(items))return;
 try{win.localStorage.setItem(navigationCacheKey,JSON.stringify({savedAt:now,items}));}catch{/* Private browsing or a full storage quota must not block navigation. */}
}

export function readCatalogCache(win=window,now=Date.now()){
 try{
  const entry=JSON.parse(win.localStorage.getItem(catalogCacheKey)||'null');
  if(!entry||!Number.isFinite(entry.savedAt)||now-entry.savedAt<0||now-entry.savedAt>catalogMaxAge||!Array.isArray(entry.categories)||!Array.isArray(entry.products))return null;
  return {categories:entry.categories,products:entry.products};
 }catch{return null;}
}

export function writeCatalogCache(data,win=window,now=Date.now()){
 if(!Array.isArray(data?.categories)||!Array.isArray(data?.products))return;
 try{win.localStorage.setItem(catalogCacheKey,JSON.stringify({savedAt:now,categories:data.categories,products:data.products.slice(0,100)}));}catch{/* Local storage is an optional acceleration only. */}
}

export function mergeCatalogCache(data,win=window,now=Date.now()){
 if(!Array.isArray(data?.categories)||!Array.isArray(data?.products))return;
 const cached=readCatalogCache(win,now),products=new Map((cached?.products||[]).map(product=>[product.publicSlug,product]));
 for(const product of data.products)if(product?.publicSlug&&product?.categoryNo)products.set(product.publicSlug,product);
 try{win.localStorage.setItem(catalogCacheKey,JSON.stringify({savedAt:now,categories:data.categories,products:[...products.values()].slice(-100)}));}catch{/* Local storage is an optional acceleration only. */}
}
