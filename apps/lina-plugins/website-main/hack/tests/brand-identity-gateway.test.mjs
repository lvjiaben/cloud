import assert from 'node:assert/strict';
import test from 'node:test';
import {mkdtemp,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {createServer} from 'node:http';
import {createBrandIdentityCache} from '../../gateway/brand-identity.mjs';
import {createWebsiteHandler} from '../../gateway/site.mjs';

const brand={siteName:{zhCN:'测试网站',enUS:'Test site'},footer:{zhCN:'测试公司',enUS:'Test company'},headerLogoUrl:'https://images.example.test/logo.png',footerLogoUrl:'https://images.example.test/logo.png',copyright:'版权',contactPhone:'123'};
const folder=async()=>mkdtemp(join(tmpdir(),'website-brand-'));

test('shared identity is public-only, visible to gateway peers, expires and clears authoritatively',async()=>{
 const directory=await folder();let now=1000;
 try{
  const first=createBrandIdentityCache({directory,backendUrl:'http://127.0.0.1:6030',now:()=>now,report:()=>{}});
  const peer=createBrandIdentityCache({directory,backendUrl:'http://127.0.0.1:6030',now:()=>now,report:()=>{}});
  await first.publish({...brand,adminSecret:'never persist',config:{template:'private'},plugins:['private']},now);
  assert.deepEqual((await peer.read()).brand.siteName,brand.siteName);
  const saved=await readFile(resolve(directory,(await import('node:crypto')).createHash('sha256').update('http://127.0.0.1:6030').digest('hex').slice(0,20)+'.json'),'utf8');
  assert.doesNotMatch(saved,/adminSecret|never persist|template|plugins|private/);
  now+=30001;assert.equal(await peer.read(),null);
  await first.publish(null,now);assert.equal(await peer.read(),null);
 }finally{await rm(directory,{recursive:true,force:true});}
});

test('gateway injects only a fresh public hint and starts escaped configured image preloads',async()=>{
 const directory=await folder();let now=Date.now();
 try{
  const cache=createBrandIdentityCache({directory,backendUrl:'http://127.0.0.1:6030',now:()=>now,report:()=>{}});
  await cache.publish(brand,now);
  const handle=createWebsiteHandler({publicRoot:resolve(import.meta.dirname,'../../frontend/public'),backendUrl:'http://127.0.0.1:6030',brandCache:cache});
  const response=await handle(new Request('http://website.invalid/'));
  const html=await response.text();
  assert.match(html,/__LINA_PUBLIC_IDENTITY__/);assert.match(html,/测试公司/);assert.match(html,/fetchpriority="high"/);assert.match(html,/brand-bootstrap\.js/);
  assert.equal((html.match(/rel="preload" as="image"/g)||[]).length,1);
  await cache.publish({...brand,headerLogoUrl:'https://images.example.test/logo.png\"><script>bad()<\/script>',footerLogoUrl:''},now+1);now++;
  const unsafe=await(await handle(new Request('http://website.invalid/'))).text();assert.doesNotMatch(unsafe,/<link rel="preload"/);
  now+=30001;
  const expired=await(await handle(new Request('http://website.invalid/'))).text();assert.doesNotMatch(expired,/__LINA_PUBLIC_IDENTITY__|brand-bootstrap\.js|rel="preload" as="image"/);
 }finally{await rm(directory,{recursive:true,force:true});}
});

test('authoritative brand reads refresh the shared hint without changing the response',async()=>{
 const directory=await folder(),backend=createServer((_request,response)=>{response.writeHead(200,{'content-type':'application/json'});response.end(JSON.stringify({code:0,data:{brand}}));});
 await new Promise(resolve=>backend.listen(0,'127.0.0.1',resolve));
 try{
  const backendUrl='http://127.0.0.1:'+backend.address().port;
  const cache=createBrandIdentityCache({directory,backendUrl,report:()=>{}});
  const handle=createWebsiteHandler({publicRoot:resolve(import.meta.dirname,'../../frontend/public'),backendUrl,brandCache:cache});
  const response=await handle(new Request('http://website.invalid/x/idc-portal/api/v1/public/brand'));
  assert.deepEqual(await response.json(),{code:0,data:{brand}});
  for(let attempt=0;attempt<20&&!await cache.read();attempt++)await new Promise(resolve=>setTimeout(resolve,10));
  assert.equal((await cache.read()).brand.footer.zhCN,'测试公司');
 }finally{await new Promise(resolve=>backend.close(resolve));await rm(directory,{recursive:true,force:true});}
});
