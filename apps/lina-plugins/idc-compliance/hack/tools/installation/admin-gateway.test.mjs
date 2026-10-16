import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createAdminServer} from './admin-gateway.mjs';
const listen=server=>new Promise(resolve=>server.listen(0,'127.0.0.1',()=>resolve('http://127.0.0.1:'+server.address().port)));
const close=server=>new Promise(resolve=>{server.closeAllConnections();server.close(resolve);});
test('admin delivery serves SPA refresh and proxies exact backend requests',async()=>{
 const root=await mkdtemp(join(tmpdir(),'admin-delivery-'));await writeFile(join(root,'index.html'),'<main>admin</main>');await writeFile(join(root,'_app.config.js'),'window.config={}');
 const backend=createServer((req,res)=>{res.writeHead(200,{'Content-Type':'application/json','Set-Cookie':['one=1','two=2']});res.end(JSON.stringify({path:req.url,auth:req.headers.authorization}));});
 const backendOrigin=await listen(backend);const gateway=createAdminServer({assetRoot:root,backendOrigin});const origin=await listen(gateway);
 try{
  const deep=await fetch(origin+'/admin/system/file');assert.equal(deep.status,200);assert.equal(await deep.text(),'<main>admin</main>');
  assert.equal((await fetch(origin+'/missing.js')).status,404);assert.equal(await(await fetch(origin+'/admin/_app.config.js')).text(),'window.config={}');
  const head=await fetch(origin+'/auth/login',{method:'HEAD'});assert.equal(head.status,200);assert.equal(await head.text(),'');
  assert.equal((await fetch(origin+'/',{method:'POST'})).status,405);
  const response=await fetch(origin+'/api/v1/test?q=1',{headers:{Authorization:'Bearer fixture-only'}});assert.deepEqual(await response.json(),{path:'/api/v1/test?q=1',auth:'Bearer fixture-only'});assert.equal(response.headers.getSetCookie().length,2);
  await close(backend);assert.equal((await fetch(origin+'/api/v1/test')).status,503);
 }finally{await close(gateway);if(backend.listening)await close(backend);await rm(root,{recursive:true,force:true});}
});
