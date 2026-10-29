import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile,readdir} from 'node:fs/promises';
import {resolve} from 'node:path';
import {createHash} from 'node:crypto';
import {createWebsiteHandler} from '../../gateway/site.mjs';
const root=resolve(import.meta.dirname,'../..');
const output=resolve(process.env.WEBSITE_SITE_OUTPUT||resolve(root,'dist/site'));
const request=(path,method='GET')=>new Request('http://website.local'+path,{method});

test('built ordinary pages and fonts load with backend unavailable and no template lookup',async()=>{
 const handle=createWebsiteHandler({publicRoot:resolve(output,'public'),backendUrl:'http://127.0.0.1:1',customerUrl:'http://localhost:6032'});
 const checksums=JSON.parse(await readFile(resolve(output,'checksums.json'),'utf8'));
 for(const name of ['index','products','configure','cart','news','announcements','help','downloads','content','contact','about']){
  const response=await handle(request('/'+name+'.html'));assert.equal(response.status,200,name);
  const html=await response.text();assert.match(html,/__LINA_TEMPLATE__/);assert.doesNotMatch(html,/<base[^>]*x-assets|\/x-assets\/website-main/);
  const source=await readFile(resolve(root,'frontend/public',name+'.html'));
  assert.equal(createHash('sha256').update(source).digest('hex'),checksums['public/'+name+'.html']);
 }
 const font=Object.keys(checksums).find(name=>/\.(?:woff2?|ttf)$/.test(name));assert.ok(font);
 const response=await handle(request('/'+font.slice('public/'.length)));assert.equal(response.status,200);
 assert.equal(createHash('sha256').update(Buffer.from(await response.arrayBuffer())).digest('hex'),checksums[font]);
 assert.equal((await handle(request('/products.html','HEAD'))).body,null);
});

test('ordinary gateway rejects private APIs, invalid paths and unsupported writes',async()=>{
 const handle=createWebsiteHandler({publicRoot:resolve(output,'public'),backendUrl:'http://127.0.0.1:1'});
 for(const path of ['/x/idc-core/api/v1/admin/catalog/products','/x/idc-core/api/v1/public/purchase-configuration-handoffs','/missing.html','/%zz','/%5csecret','/%2e%2e%2fpackage.json'])assert.equal((await handle(request(path))).status,404,path);
 assert.equal((await handle(request('/x/idc-core/api/v1/account/commerce/orders'))).status,405,'order lookup is not a website contract');
 assert.equal((await handle(request('/x/idc-core/api/v1/account/commerce/cart'))).status,401,'current-customer cart requires a bearer session');
 assert.equal((await handle(request('/x/idc-core/api/v1/account/commerce/orders','POST'))).status,401,'order creation requires a bearer session');
 for(const path of ['/products.html','/x/idc-core/api/v1/public/catalog/products'])assert.equal((await handle(request(path,'DELETE'))).status,405,path);
 assert.equal((await handle(request('/x/idc-core/api/v1/public/catalog/products'))).status,502);
});

test('deployment output excludes dynamic plugin payloads and all QA data',async()=>{
 const checksums=JSON.parse(await readFile(resolve(output,'checksums.json'),'utf8'));
 const names=Object.keys(checksums);
 assert.ok(names.length>100);
 for(const name of ['gateway/server.mjs','gateway/site.mjs','gateway/commerce.mjs'])assert.deepEqual(await readFile(resolve(output,name)),await readFile(resolve(root,name)),name+' must contain current source');
 for(const name of names){
  assert.doesNotMatch(name,/(?:\.wasm$|plugin\.yaml$|^manifest\/|^backend\/|^hack\/|^node_modules\/)/);
  const bytes=await readFile(resolve(output,name));
  assert.equal(createHash('sha256').update(bytes).digest('hex'),checksums[name]);
  for(const marker of ['isolated-integration-only','隔离联调商品','website-qa@project1.test','test-customer-private.json','local-download-test.png'])assert.equal(bytes.includes(Buffer.from(marker)),false,name+': '+marker);
 }
 // The historical file artifact excludes metadata; the canonical source is now
 // the existing managed plugin again and must contain its own plugin.yaml.
 assert.equal((await readdir(output)).includes('plugin.yaml'),false);
});

test('ordinary public delivery includes current non-blocking navigation cache',async()=>{
 const source=resolve(root,'frontend/public'),deployed=resolve(process.env.WEBSITE_PUBLIC_OUTPUT||resolve(root,'../../../temp/output/websites/website-default/public'));
 for(const name of ['lina/site.js','lina/navigation.js','lina/navigation-cache.js']){
  const expected=await readFile(resolve(source,name));
  assert.deepEqual(await readFile(resolve(deployed,name)),expected,name+' must be deployed from current source');
 }
});

test('ordinary API handler reads actual product plugin without template-selected assets',async()=>{
 const backend=process.env.WEBSITE_API_ORIGIN;
 assert.ok(backend,'explicit real local backend is required');assert.ok(['127.0.0.1','localhost'].includes(new URL(backend).hostname));
 const handle=createWebsiteHandler({publicRoot:resolve(output,'public'),backendUrl:backend});
 const path='/x/idc-core/api/v1/public/catalog/products';
 const direct=await (await fetch(new URL(path,backend))).json();
 const response=await handle(request(path));assert.equal(response.status,200);
 const via=await response.json();assert.equal(via.code,0);
 // Each response issues its own opaque pagination token; compare business data.
 assert.equal(typeof via.data.nextCursor,typeof direct.data.nextCursor);
 const {nextCursor:viaCursor,...viaData}=via.data,{nextCursor:directCursor,...directData}=direct.data;
 assert.deepEqual(viaData,directData);
});
