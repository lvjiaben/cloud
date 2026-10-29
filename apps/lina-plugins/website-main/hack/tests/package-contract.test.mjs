import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile,readdir} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {WASI} from 'node:wasi';
import {parse} from 'yaml';

const root=new URL('../../',import.meta.url);
const bytes=await readFile(new URL('dist/website-main.wasm',root));
const compiled=await WebAssembly.compile(bytes);
function section(name){const values=WebAssembly.Module.customSections(compiled,name);assert.equal(values.length,1,name);return JSON.parse(Buffer.from(values[0]).toString());}
const hash=value=>createHash('sha256').update(value).digest('hex');

test('managed package preserves existing identity, dependencies and actual source assets',async()=>{
 const manifest=section('lina.plugin.manifest'),source=parse(await readFile(new URL('plugin.yaml',root),'utf8'));
 assert.equal(manifest.id,'website-main');assert.equal(manifest.name,'官网');assert.match(manifest.description,/官网/);assert.doesNotMatch(manifest.description,/模[板版]/);assert.equal(manifest.version,source.version);
 assert.equal(manifest.type,'dynamic');assert.equal(manifest.distribution,'managed');
 assert.deepEqual(manifest.dependencies.plugins.map(item=>item.id),['idc-portal','idc-core']);
 const extensions=manifest.business_extensions;assert.equal(extensions.length,1);
 assert.equal(extensions[0].point,'website-template');assert.equal(extensions[0].owner,'idc-portal');
 assert.deepEqual(extensions[0].methods,['capabilities.describe']);
 // Homepage settings and the seven independent solution bindings stay template-owned.
 assert.deepEqual(extensions[0].interfaceConfiguration.fields.map(field=>field.key),['herotitle','herodescription','heroimage','hero1href',...['electricity','web','game','goingsea','financial','government','safety'].map(key=>'relatedproducts'+key)]);
 assert.equal(extensions[0].interfaceConfigurationCardinality,'singleton');
 assert.equal(manifest.menus,undefined);assert.equal(manifest.admin_surfaces,undefined);
 const assets=section('lina.plugin.frontend.assets');
 // A successful upgrade must include newly added pages and modules as well as old assets.
 const sourcePaths=[];
 async function inventory(directory){for(const entry of await readdir(new URL(directory,root),{withFileTypes:true})){const path=directory+entry.name;if(entry.isDirectory())await inventory(path+'/');else sourcePaths.push(path);}}
 await inventory('frontend/public/');
 assert.deepEqual(assets.map(asset=>asset.path).sort(),sourcePaths.sort(),'release must contain every current public source asset');
 assert.equal(section('lina.plugin.dynamic').frontendAssetCount,assets.length);
 const paths=new Set();
 for(const asset of assets){assert.equal(paths.has(asset.path),false);paths.add(asset.path);assert.match(asset.path,/^frontend\/public\//);assert.doesNotMatch(asset.path,/\.\./);
  assert.equal(hash(Buffer.from(asset.contentBase64,'base64')),hash(await readFile(new URL(asset.path,root))),asset.path);
 }
 assert.ok(paths.has('frontend/public/lina/server-hero.js'),'dynamic server hero must ship');
 assert.ok(paths.has('frontend/public/lina/brand-refresh.js'),'public refresh lifecycle must ship');
 for(const name of ['newspaper','megaphone','book-open','download'])assert.ok(paths.has('frontend/public/static/picture/sidebar-'+name+'.svg'));
 assert.ok(paths.has('frontend/public/static/picture/sidebar-icons-LICENSE.txt'),'sidebar license must ship in public assets');
 const pages=extensions[0].providerMetadata.values.pages;
 assert.deepEqual(pages,source.business_extensions[0].provider_metadata.values.pages);
 assert.ok(pages.some(page=>page.path==='/newestactivity.html'),'the new activity destination must be declared and shipped');
 for(const page of pages)assert.ok(paths.has('frontend/public/'+(page.path==='/'?'index.html':page.path.slice(1))),page.path);
 const exports=WebAssembly.Module.exports(compiled).map(item=>item.name);
 for(const name of ['memory','_initialize','lina_dynamic_route_alloc','lina_dynamic_route_execute'])assert.ok(exports.includes(name),name);
});

function varint(value){const bytes=[];do{let byte=value&127;value=Math.floor(value/128);if(value)byte|=128;bytes.push(byte);}while(value);return Buffer.from(bytes);}
function field(id,value){const bytes=Buffer.from(value);return Buffer.concat([varint(id*8+2),varint(bytes.length),bytes]);}

test('actual WASI guest executes descriptor and rejects forged invocation without host or database',async()=>{
 const wasi=new WASI({version:'preview1',args:[],env:{},preopens:{},returnOnExit:true});
 const instance=await WebAssembly.instantiate(compiled,{wasi_snapshot_preview1:wasi.wasiImport});wasi.initialize(instance);
 const base={version:'v1',ownerPluginId:'idc-portal',providerPluginId:'website-main',point:'website-template',contractVersion:'v1',method:'capabilities.describe',releaseVersion:section('lina.plugin.manifest').version,releaseDigest:'a'.repeat(64),declarationDigest:'b'.repeat(64),purpose:'new-business',runtimeContext:{contextId:'package-validation-only'},payload:{}};
 function invoke(value){
  const envelope=Buffer.concat([field(1,'website-main'),field(2,field(7,'BusinessExtensionBridgeRequest')),field(3,field(13,JSON.stringify(value)))]);
  const offset=instance.exports.lina_dynamic_route_alloc(envelope.length);assert.ok(offset>0);
  new Uint8Array(instance.exports.memory.buffer,offset,envelope.length).set(envelope);
  const result=instance.exports.lina_dynamic_route_execute(envelope.length);
  const pointer=Number(result>>32n),length=Number(result&0xffffffffn);
  return Buffer.from(new Uint8Array(instance.exports.memory.buffer,pointer,length));
 }
 const good=invoke(base);assert.match(good.toString(),/"pluginId":"website-main"/);assert.deepEqual(good.subarray(0,3),Buffer.from([8,0xc8,1]));
 for(const altered of [{...base,ownerPluginId:'forged'},{...base,providerPluginId:'other'},{...base,method:'delete'},{...base,runtimeContext:null}]){
  const response=invoke(altered);assert.doesNotMatch(response.toString(),/"pluginId"/);assert.match(response.toString(),/TEMPLATE_DESCRIPTOR_INVALID/);
 }
});
