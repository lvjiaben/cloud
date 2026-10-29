import assert from 'node:assert/strict';
import test from 'node:test';
import {startBrandRefresh} from '../../frontend/public/lina/brand-refresh.js';

test('initial pageshow does not repeat loading; visible polling and restored pages still refresh',async()=>{
 const view=new EventTarget(),document=new EventTarget(),timers=new Map();let id=0,calls=0;
 view.setTimeout=(callback,delay)=>{assert.equal(delay,30000);timers.set(++id,callback);return id;};
 view.clearTimeout=id=>timers.delete(id);document.defaultView=view;document.hidden=false;
 const settle=()=>new Promise(resolve=>setImmediate(resolve));
 startBrandRefresh(document,async()=>{calls++;});await settle();assert.equal(calls,1);
 view.dispatchEvent(new Event('pageshow'));await settle();assert.equal(calls,1);
 await [...timers.values()][0]();assert.equal(calls,2);
 document.hidden=true;document.dispatchEvent(new Event('visibilitychange'));assert.equal(timers.size,0);
 view.dispatchEvent(new Event('focus'));await settle();assert.equal(calls,2);
 document.hidden=false;document.dispatchEvent(new Event('visibilitychange'));await settle();assert.equal(calls,3);
 view.dispatchEvent(new Event('pagehide'));assert.equal(timers.size,0);
 const restored=new Event('pageshow');Object.defineProperty(restored,'persisted',{value:true});view.dispatchEvent(restored);await settle();assert.equal(calls,4);
 view.dispatchEvent(new Event('pagehide'));assert.equal(timers.size,0);
});
