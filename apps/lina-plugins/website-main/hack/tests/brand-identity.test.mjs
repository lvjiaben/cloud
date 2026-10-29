import assert from 'node:assert/strict';
import test from 'node:test';
import {brandIdentityKey,readBrandIdentity,readBrandIdentityEntry,readServerBrandIdentityEntry,writeBrandIdentity} from '../../frontend/public/lina/brand-identity.js';

function storage(){const values=new Map();return {localStorage:{getItem:key=>values.get(key)||null,setItem:(key,value)=>values.set(key,value)}};}

test('only public identity is retained for at most thirty seconds without extending on read',()=>{
 const view=storage();writeBrandIdentity({siteName:{zhCN:'站名'},headerLogoUrl:'/logo.png',contactPhone:'01012345678',templateConfig:{private:'excluded'},navigation:[{href:'/old'}]},view,1000);
 const cached=readBrandIdentity(view,1001);
 assert.equal(cached.headerLogoUrl,'/logo.png');assert.equal(cached.contactPhone,'01012345678');assert.equal(cached.templateConfig,undefined);assert.equal(cached.navigation,undefined);
 assert.ok(readBrandIdentity(view,31000));assert.equal(readBrandIdentity(view,31001),null);
 assert.equal(readBrandIdentity(view,999),null);
});

test('malformed and inaccessible storage never blocks authoritative loading',()=>{
 const view=storage();
 for(const value of ['{bad','null',JSON.stringify({savedAt:1000,brand:[]}),JSON.stringify({savedAt:1000,brand:'bad'})]){
  view.localStorage.setItem(brandIdentityKey,value);assert.equal(readBrandIdentity(view,1001),null);
 }
 const denied={localStorage:{getItem(){throw new Error('denied');},setItem(){throw new Error('denied');}}};
 assert.equal(readBrandIdentity(denied),null);assert.doesNotThrow(()=>writeBrandIdentity({},denied));
});

test('the newer short-lived browser source wins when the shared hint is older',()=>{
 const view=storage();writeBrandIdentity({footer:{zhCN:'最新公司'}},view,2000);
 view.__LINA_PUBLIC_IDENTITY__={savedAt:1000,brand:{footer:{zhCN:'旧公司'}}};
 assert.equal(readBrandIdentityEntry(view,2001).brand.footer.zhCN,'最新公司');
 assert.equal(readServerBrandIdentityEntry(view,2001).brand.footer.zhCN,'旧公司');
});
