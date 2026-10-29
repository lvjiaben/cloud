import assert from 'node:assert/strict';
import test from 'node:test';
import {catalogCacheKey,navigationCacheKey,mergeCatalogCache,readCatalogCache,readNavigationCache,writeCatalogCache,writeNavigationCache} from '../../frontend/public/lina/navigation-cache.js';

function browserStorage(){
 const values=new Map();
 return {localStorage:{getItem:key=>values.get(key)||null,setItem:(key,value)=>values.set(key,value)},values};
}

test('public navigation is cached locally and remains fresh for the next page entry',()=>{
 const win=browserStorage(),items=[{label:'产品与服务',href:'/products.html'}];
 writeNavigationCache(items,win,1000);
 assert.deepEqual(JSON.parse(win.localStorage.getItem(navigationCacheKey)),{savedAt:1000,items});
 assert.deepEqual(readNavigationCache(win,1001),items);
});

test('catalog categories and products are available as a short-lived menu snapshot',()=>{
 const win=browserStorage(),data={categories:[{categoryNo:'cat-a'}],products:Array.from({length:105},(_,index)=>({publicSlug:'product-'+index}))};
 writeCatalogCache(data,win,1000);
 const cached=readCatalogCache(win,1001);
 assert.equal(cached.categories[0].categoryNo,'cat-a');assert.equal(cached.products.length,100);
 assert.equal(readCatalogCache(win,301001),null);
 win.localStorage.setItem(catalogCacheKey,JSON.stringify({savedAt:1000,categories:[],products:'invalid'}));
 assert.equal(readCatalogCache(win,1001),null);
});

test('catalog refresh merges category products instead of discarding products from other groups',()=>{
 const win=browserStorage();writeCatalogCache({categories:[{categoryNo:'cat-old'}],products:[{publicSlug:'server-a',categoryNo:'cat-a'}]},win,1000);
 mergeCatalogCache({categories:[{categoryNo:'cat-fresh'}],products:[{publicSlug:'server-b',categoryNo:'cat-b'},{publicSlug:'server-a',categoryNo:'cat-a',nameZhCN:'更新名称'}]},win,2000);
 assert.deepEqual(readCatalogCache(win,2001),{categories:[{categoryNo:'cat-fresh'}],products:[{publicSlug:'server-a',categoryNo:'cat-a',nameZhCN:'更新名称'},{publicSlug:'server-b',categoryNo:'cat-b'}]});
});

test('expired, malformed, and future-dated navigation snapshots are ignored',()=>{
 const win=browserStorage();
 win.localStorage.setItem(navigationCacheKey,JSON.stringify({savedAt:1,items:[]}));
 assert.equal(readNavigationCache(win,86400002),null);
 win.localStorage.setItem(navigationCacheKey,'{broken');
 assert.equal(readNavigationCache(win,2),null);
 win.localStorage.setItem(navigationCacheKey,JSON.stringify({savedAt:3,items:[]}));
 assert.equal(readNavigationCache(win,2),null);
});
