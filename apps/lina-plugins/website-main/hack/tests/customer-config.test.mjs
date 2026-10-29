import {test} from 'node:test';
import assert from 'node:assert/strict';
import {loadCustomerConfiguration} from '../../frontend/public/lina/customer-config.js';

test('gateway supplied address takes precedence without fetching fallback',async()=>{
 const win={__LINA_TEMPLATE__:{customerUrl:'https://customer.example.test'}};
 await loadCustomerConfiguration(win,()=>{throw new Error('fallback must not run');});
 assert.equal(win.__LINA_TEMPLATE__.customerUrl,'https://customer.example.test');
});
test('public deployment configuration preserves other injected settings',async()=>{
 const win={__LINA_TEMPLATE__:{template:'website-main',customerUrl:''}};
 await loadCustomerConfiguration(win,async(path,options)=>{
  assert.equal(path,'/lina/runtime-config.json');assert.equal(options.credentials,'omit');
  return Response.json({customerUrl:'http://127.0.0.1:6032'});
 });
 assert.deepEqual(win.__LINA_TEMPLATE__,{template:'website-main',customerUrl:'http://127.0.0.1:6032/'});
});
test('missing, failed and unsafe deployment configuration remains unavailable',async()=>{
 const responses=[new Response('',{status:404}),Response.json({}),Response.json({customerUrl:'javascript:alert(1)'}),Response.json({customerUrl:'https://user:password@example.test'}),new Response('invalid json')];
 for(const response of responses){const win={};await loadCustomerConfiguration(win,async()=>response);assert.equal(win.__LINA_TEMPLATE__,undefined);}
 const win={};await loadCustomerConfiguration(win,async()=>{throw new Error('offline');});assert.equal(win.__LINA_TEMPLATE__,undefined);
});
