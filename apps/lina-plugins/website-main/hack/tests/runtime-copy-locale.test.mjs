import assert from 'node:assert/strict';
import test from 'node:test';
import {execFileSync} from 'node:child_process';
import {readFile} from 'node:fs/promises';
import {parseHTML} from 'linkedom';
import {initializeLocale,translate} from '../../frontend/public/lina/locale.js';
import {composeNavigation,navigationLabel,navigationDisplayLabel} from '../../frontend/public/lina/navigation-content.js';
import {certificateHTML} from '../../frontend/public/lina/certificate-content.js';
import {initializeHomeBlocks} from '../../frontend/public/lina/home-template.js';
import {renderSupportBar} from '../../frontend/public/lina/support-bar.js';

const shape=node=>[node.tagName,node.getAttribute('class'),[...node.children].map(shape)];
const destinations=items=>items.map(item=>[navigationLabel(item),item.href,!!item.customer,!!item.external,destinations(item.children||[])]);

test('translated default menu labels retain canonical identities and destinations across languages',()=>{
 const {document}=parseHTML('<html><body><div data-locale-actions></div></body></html>');
 const locale=initializeLocale(document);
 const published=[{label:'生态合作',children:[{label:'合作伙伴',href:'/404.html'},{label:'推广返利',href:'/404.html'}]},{label:'了解我们',children:[{label:'资质荣誉',href:'/404.html'}]}];
 locale.setLocale('zh-CN');
 const original=composeNavigation(published),canonical=destinations(original);
 assert.equal(original[2].label,'生态合作');
 locale.setLocale('en-US');
 const english=composeNavigation(published);
 assert.equal(composeNavigation()[2].label,'Partners');
 const labels=document.createElement('nav');
 for(const item of [composeNavigation()[2],composeNavigation()[2].children.find(item=>navigationLabel(item)==='合作伙伴')]){const link=document.createElement('a');link.textContent=navigationDisplayLabel(item);labels.append(link);}
 document.body.append(labels);locale.render();
 assert.deepEqual([...labels.children].map(link=>link.textContent),['Partners','Partners']);
 assert.equal(english[2].children.find(item=>navigationLabel(item)==='推广返利').href,'/aff.html');
 assert.deepEqual(destinations(english),canonical);
 locale.setLocale('zh-CN');
 assert.deepEqual([...labels.children].map(link=>link.textContent),['生态合作','合作伙伴']);
 assert.deepEqual(composeNavigation(published),original);
});

test('initial English import cannot confuse ecosystem and partner menu identities',()=>{
 const moduleURL=new URL('../../frontend/public/lina/navigation-content.js',import.meta.url).href;
 execFileSync(process.execPath,['--input-type=module','-e',`
  import assert from 'node:assert/strict';
  globalThis.window={localStorage:{getItem:()=> 'en-US'}};
  const {composeNavigation,navigationLabel,navigationDisplayLabel}=await import(${JSON.stringify(moduleURL)});
  const menus=composeNavigation([{label:'生态合作',children:[{label:'合作伙伴',href:'/404.html'}]}]);
  const ecosystem=menus.find(item=>navigationLabel(item)==='生态合作');
  assert.equal(composeNavigation().find(item=>navigationLabel(item)==='生态合作').label,'Partners');
  assert.equal(ecosystem.children.find(item=>navigationLabel(item)==='合作伙伴').href,'/cooperation/partner.html');
  assert.equal(menus.find(item=>navigationLabel(item)==='支持与服务').children.find(item=>navigationLabel(item)==='服务条款').href,'/terms.html');
 `],{stdio:'pipe'});
});

test('certificate, home titles and support text switch in place without changing structure or classes',()=>{
 const {window,document}=parseHTML('<html><body><div data-locale-actions></div><div data-home-blocks><a class="bannerbelow-list"></a><a class="bannerbelow-list"></a><a class="bannerbelow-list"></a><a class="bannerbelow-list"></a></div><div id="certificate"></div></body></html>');
 const originals=new Map(['window','document','location'].map(key=>[key,Object.getOwnPropertyDescriptor(globalThis,key)]));
 try{
  for(const [key,value] of Object.entries({window,document,location:{origin:'http://localhost'}}))Object.defineProperty(globalThis,key,{configurable:true,writable:true,value});
  const locale=initializeLocale(document);locale.setLocale('zh-CN');
  document.querySelector('#certificate').innerHTML=certificateHTML();
  initializeHomeBlocks(document,{safeURL:path=>new URL(path,'http://localhost'),customerLink:()=>{}});
  renderSupportBar(document,{businessQq:'123456'},path=>path);
  locale.render();
  const originalShape=shape(document.body),originalText=document.body.textContent;
  assert.equal(document.querySelector('[data-home-blocks] a').title,'查看最新活动');
  assert.match(document.querySelector('[data-lina-support-bar]').textContent,/提交工单/);
  locale.setLocale('en-US');
  assert.equal(document.querySelector('[data-home-blocks] a').title,'View latest offers');
  assert.match(document.querySelector('#certificate').textContent,/Certifications/);
  assert.match(document.querySelector('[data-lina-support-bar]').textContent,/Submit a ticket/);
  assert.deepEqual(shape(document.body),originalShape);
  locale.setLocale('zh-CN');
  assert.deepEqual(shape(document.body),originalShape);
  assert.equal(document.body.textContent,originalText);
  document.querySelector('[data-lina-support-bar]').remove();
  locale.setLocale('en-US');renderSupportBar(document,{},path=>path);locale.render();
  assert.equal(document.querySelector('.onlineServicetitle').textContent,'Contact us');
  locale.setLocale('zh-CN');
  assert.equal(document.querySelector('.onlineServicetitle').textContent,'联系我们');
 }finally{
  for(const [key,descriptor] of originals)if(descriptor)Object.defineProperty(globalThis,key,descriptor);else delete globalThis[key];
 }
});


test('every newly localized static copy value has a catalog entry and restores its exact original',async()=>{
 const root=new URL('../../frontend/public/lina/',import.meta.url);
 const keys=new Set();
 for(const file of ['activity.js','certificate-content.js','home-template.js','navigation-content.js','navigation.js','support-bar.js']){
  const source=await readFile(new URL(file,root),'utf8');
  for(const match of source.matchAll(/translate\('([^']+)'\)/g))keys.add(match[1]);
  if(file==='navigation-content.js')for(const match of source.matchAll(/\bentry\('([^']+)'(?:,'([^']+)')?/g))if(match[2])keys.add(match[2]);
 }
 const {document}=parseHTML('<html><body><div data-locale-actions></div><main></main></body></html>');
 const locale=initializeLocale(document);locale.setLocale('en-US');
 const main=document.querySelector('main');
 for(const key of keys){const paragraph=document.createElement('p');paragraph.textContent=translate(key);assert.notEqual(paragraph.textContent,key,'untranslated copy: '+key);main.append(paragraph);}
 locale.render();locale.setLocale('zh-CN');
 assert.deepEqual([...main.children].map(node=>node.textContent),[...keys]);
});
