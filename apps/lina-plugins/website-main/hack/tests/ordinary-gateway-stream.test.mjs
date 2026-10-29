import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer,get} from 'node:http';
import {once} from 'node:events';
import {spawn} from 'node:child_process';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
const listen=async server=>{server.listen(0,'127.0.0.1');await once(server,'listening');return 'http://127.0.0.1:'+server.address().port;};

test('ordinary gateway survives upstream body timeout/interruption and releases cancelled streams',{timeout:10000},async t=>{
 let cancelled,arrived;
 const seen=new Promise(resolve=>{arrived=resolve;});
 const upstream=createServer((req,res)=>{
  res.setHeader('content-type','application/json');
  if(req.url==='/x/idc-portal/api/v1/public/brand')return res.end(JSON.stringify({code:0,data:{brand:null}}));
  const mode=new URL(req.url,'http://upstream.invalid').searchParams.get('case');
  if(mode==='early'){
   cancelled=once(res,'close');arrived();
   const timer=setTimeout(()=>{res.writeHead(200);res.write('partial');},40);timer.unref();return;
  }
  if(mode==='cancel')cancelled=once(res,'close');
  res.writeHead(200);res.write('{"code":0,"data":');
  if(mode==='interrupt'){const timer=setTimeout(()=>res.destroy(),30);timer.unref();}
 });
 const backend=await listen(upstream),probe=createServer(),origin=await listen(probe);await new Promise(resolve=>probe.close(resolve));
 const cache=await mkdtemp(join(tmpdir(),'website-stream-cache-'));t.after(()=>rm(cache,{recursive:true,force:true}));
 // Shorten only the child process's timeout to reproduce a real fetch body TimeoutError.
 const preload='const original=AbortSignal.timeout;AbortSignal.timeout=()=>original(250);';
 const child=spawn(process.execPath,['--import','data:text/javascript,'+encodeURIComponent(preload),fileURLToPath(new URL('../../gateway/server.mjs',import.meta.url))],{env:{...process.env,HOST:'127.0.0.1',PORT:new URL(origin).port,WEBSITE_API_ORIGIN:backend,WEBSITE_CUSTOMER_URL:backend,WEBSITE_BRAND_CACHE_DIR:cache},stdio:['ignore','ignore','pipe']});
 let stderr='';child.stderr.on('data',chunk=>{stderr+=chunk;});
 try{
  let ready=false;
  for(let n=0;n<80;n++){try{if((await fetch(origin+'/__website/configuration')).ok){ready=true;break;}}catch{}await new Promise(resolve=>setTimeout(resolve,25));}
  assert.ok(ready,'gateway did not start: '+stderr);
  const path='/x/idc-core/api/v1/public/catalog/products';
  const early=get(origin+path+'?case=early');early.on('error',()=>{});await seen;early.destroy();
  await Promise.race([cancelled,new Promise((_,reject)=>{const timer=setTimeout(()=>reject(new Error('upstream request was not released before its timeout')),150);timer.unref();})]);
  await new Promise(resolve=>setTimeout(resolve,400));
  assert.equal(child.exitCode,null,'disconnect before upstream headers killed gateway: '+stderr);
  await cancelled;
  for(const mode of ['timeout','interrupt']){
   const response=await fetch(origin+path+'?case='+mode,{signal:AbortSignal.timeout(2000)});assert.equal(response.status,200);
   await assert.rejects(response.text());
   assert.equal(child.exitCode,null,'body '+mode+' killed gateway: '+stderr);
   assert.equal((await fetch(origin+'/__website/configuration')).status,200,'gateway must handle the next request');
  }
  const controller=new AbortController();
  const response=await fetch(origin+path+'?case=cancel',{signal:controller.signal});assert.equal(response.status,200);controller.abort();await assert.rejects(response.text());
  await Promise.race([cancelled,new Promise((_,reject)=>{const timer=setTimeout(()=>reject(new Error('upstream body leaked after client disconnect')),1000);timer.unref();})]);
  assert.equal(child.exitCode,null);assert.equal((await fetch(origin+'/__website/configuration')).status,200);
  assert.doesNotMatch(stderr,/Unhandled 'error' event/);
 }finally{
  if(child.exitCode===null&&child.signalCode===null){const ended=once(child,'exit');child.kill();await ended;}
  upstream.closeAllConnections();await new Promise(resolve=>upstream.close(resolve));
 }
});
