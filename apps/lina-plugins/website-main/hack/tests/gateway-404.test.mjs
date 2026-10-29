import assert from 'node:assert/strict';
import test from 'node:test';
import {mkdtemp,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createWebsiteHandler} from '../../gateway/site.mjs';

test('missing local routes render the website 404 document with HTTP 404',async()=>{
 const root=await mkdtemp(join(tmpdir(),'website-404-'));
 try{
  await writeFile(join(root,'404.html'),'<!doctype html><title>本地404</title>');
  const handle=createWebsiteHandler({publicRoot:root,backendUrl:'http://127.0.0.1:6030'});
  const missing=await handle(new Request('http://website.invalid/market.html'));
  assert.equal(missing.status,404);assert.match(await missing.text(),/本地404/);
  const document=await handle(new Request('http://website.invalid/404.html'));
  assert.equal(document.status,404);assert.match(await document.text(),/本地404/);
  const head=await handle(new Request('http://website.invalid/missing',{method:'HEAD'}));
  assert.equal(head.status,404);assert.equal(await head.text(),'');
  const traversal=await handle(new Request('http://website.invalid/%5c..%5cprivate'));
  assert.equal(traversal.status,404);assert.doesNotMatch(await traversal.text(),/本地404/);
 }finally{await rm(root,{recursive:true,force:true});}
});
