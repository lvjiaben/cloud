import assert from 'node:assert/strict';
import test from 'node:test';
import {parseHTML} from 'linkedom';
import {catalogCacheKey} from '../../frontend/public/lina/navigation-cache.js';
import {createCatalogPage} from '../../frontend/public/lina/catalog-page.js';

test('catalog sidebar renders a fresh local snapshot before awaiting the public catalog',async()=>{
 const {document}=parseHTML('<main><div data-products></div><div data-categories></div><button data-more></button></main>');
 const category={categoryNo:'cat-fast',nameZhCN:'快速服务器'};
 const product={publicSlug:'node-fast',categoryNo:category.categoryNo,nameZhCN:'缓存的服务器'};
 const other={publicSlug:'other-category-server',categoryNo:'another-category',nameZhCN:'其他分类服务器'};
 const storage=new Map([[catalogCacheKey,JSON.stringify({savedAt:Date.now(),categories:[category],products:[product,other]})]]);
 const previous=globalThis.window;globalThis.window={localStorage:{getItem:key=>storage.get(key)||null}};
 let rendered;
 const page=createCatalogPage({document,request:()=>new Promise(()=>{}),base:'/catalog',status:()=>{},retry:()=>{},customerLink:()=>{},renderSidebar:(items,state)=>{rendered={items,state};},params:new URLSearchParams()});
 try{
  void page.load();
  assert.deepEqual(rendered.items,[category]);
  assert.deepEqual(rendered.state.initialProducts,[product]);
  assert.equal(rendered.state.categoryNo,category.categoryNo);
 }finally{if(previous===undefined)delete globalThis.window;else globalThis.window=previous;}
});

test('an empty category list never falls back to unscoped products',async t=>{
 const previous=globalThis.window;globalThis.window={localStorage:{getItem:()=>null}};
 t.after(()=>{if(previous===undefined)delete globalThis.window;else globalThis.window=previous;});
 const {document}=parseHTML('<main><div data-products></div><div data-categories></div><button data-more></button></main>');
 const requests=[],messages=[];
 const page=createCatalogPage({document,request:async url=>{requests.push(url);return {categories:[],products:[{publicSlug:'must-not-render'}]};},base:'/catalog',status:message=>messages.push(message),retry:()=>{},customerLink:()=>{},params:new URLSearchParams()});
 await page.load();
 assert.deepEqual(requests,['/catalog/catalog/products?limit=1']);
 assert.equal(document.querySelector('[data-products]').children.length,0);
 assert.equal(messages.at(-1),'暂无可订购商品。');
});
