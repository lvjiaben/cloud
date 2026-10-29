import{test}from'node:test';import assert from'node:assert/strict';import{readFile}from'node:fs/promises';import{createHash}from'node:crypto';
test('all original page, style, image and font bytes match the selected branch',async()=>{
 const baseline=JSON.parse(await readFile(new URL('./support/original-visual-resources.json',import.meta.url),'utf8'));
 assert.equal(baseline.sourceCommit,'240aec4ab452da1cac33c5c2eed9aa2a7ebbb3ba');
 for(const[path,expected]of Object.entries(baseline.sha256))assert.equal(createHash('sha256').update(await readFile(new URL('../../'+path,import.meta.url))).digest('hex'),expected,path);
});
test('new cart reuses original styles and existing injected recommendation styles stay intact',async()=>{
 const baseline=JSON.parse(await readFile(new URL('./support/original-visual-resources.json',import.meta.url),'utf8'));
 for(const[path,expected]of Object.entries(baseline.injectedCSS)){const source=await readFile(new URL('../../'+path,import.meta.url),'utf8');const css=source.split('style.textContent=`')[1].split('`')[0];assert.equal(createHash('sha256').update(css).digest('hex'),expected,path);}
 const root=new URL('../../frontend/public/',import.meta.url),original=await readFile(new URL('configure.html',root),'utf8'),cart=await readFile(new URL('cart.html',root),'utf8');
 const styles=html=>[...html.matchAll(/<link\b[^>]*rel=["']stylesheet["'][^>]*>|<link\b[^>]*href=["'][^"']+\.css[^>]*>/gu)].map(m=>m[0]);
 assert.deepEqual(styles(cart),styles(original));assert.doesNotMatch(cart,/cart-commerce\.css/);assert.equal(cart.match(/<style>[\s\S]*?<\/style>/u)?.[0],original.match(/<style>[\s\S]*?<\/style>/u)?.[0]);
});
