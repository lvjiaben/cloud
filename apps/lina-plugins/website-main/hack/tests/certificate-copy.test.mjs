import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {parseGameSource} from './support/game-copy.mjs';
import {collectCertificateSource} from './support/certificate-copy.mjs';
import {certificateSource,certificateAdditions,certificateRecords,certificateHTML} from '../../frontend/public/lina/certificate-content.js';
import {composeNavigation} from '../../frontend/public/lina/navigation-content.js';
import {financialContent} from '../../frontend/public/lina/financial-content.js';
test('附件两项原文与完整首屏层级一致，新增四栏目独立归属',async()=>{
 const source=parseGameSource(await readFile(new URL('./support/certificate-source.html',import.meta.url),'utf8'));
 const reference=JSON.parse(await readFile(new URL('./support/certificate-copy-reference.json',import.meta.url),'utf8'));
 assert.deepEqual(collectCertificateSource(source),reference.copy);assert.deepEqual(certificateSource,reference);
 const d=parseGameSource(certificateHTML());assert.deepEqual(collectCertificateSource(d),reference.copy);
 const structure=e=>[e.tagName,[...e.children].map(structure)];assert.deepEqual(structure(d.querySelector('.globalBanner')),structure(source.querySelector('.globalBanner')));
 assert.equal(source.querySelector('main').textContent.trim(),'');assert.equal(d.querySelectorAll('main>section').length,4);
 assert.deepEqual([...d.querySelectorAll('main h2')].map(e=>e.textContent),certificateAdditions.sections.map(s=>s.title));
 assert.equal(d.querySelectorAll('main h3').length,3);assert.deepEqual(certificateRecords,[]);assert.equal(d.querySelectorAll('[data-certificate-grid]>*').length,0);
 assert.doesNotMatch(d.toString(),/江苏白猿|小白云|400-8876675|ISO|行业第一|国家级/);
});
test('旧资质名称及404目标归并到唯一荣誉资质入口',()=>{
 for(const label of ['资质荣誉','荣誉资质']){
  const group=composeNavigation([{label:'了解我们',children:[{label,href:'/404.html'}]}]).find(i=>i.label==='了解我们');
  const entries=group.children.filter(i=>['资质荣誉','荣誉资质'].includes(i.label));assert.equal(entries.length,1);assert.equal(entries[0].label,'荣誉资质');assert.equal(entries[0].href,'/certificate.html');
 }
 const group=composeNavigation([],[],financialContent.navigation).find(i=>i.label==='了解我们');
 assert.equal(group.children.find(i=>i.label==='资质荣誉').href,'/certificate.html');
});
