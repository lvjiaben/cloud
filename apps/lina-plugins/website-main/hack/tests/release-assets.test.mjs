import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';

test('official website package excludes isolated QA products, accounts and test assets',async()=>{
 const bytes=await readFile(new URL('../../dist/website-main.wasm',import.meta.url));
 let offset=8;let assets;
 function leb(){let value=0,shift=0,byte;do{byte=bytes[offset++];value+=(byte&127)*2**shift;shift+=7;}while(byte&128);return value;}
 while(offset<bytes.length){const id=bytes[offset++];const size=leb();const end=offset+size;
  if(id===0){const length=leb();const name=bytes.subarray(offset,offset+length).toString();offset+=length;
   if(name==='lina.plugin.frontend.assets')assets=JSON.parse(bytes.subarray(offset,end).toString());
  }offset=end;
 }
 assert.ok(Array.isArray(assets)&&assets.length>0,'must inspect actual embedded assets');
 for(const asset of assets){
  assert.doesNotMatch(asset.path,/(?:^|\/)(?:hack|tests?|temp)(?:\/|$)/);
  const content=Buffer.from(asset.contentBase64,'base64');
  for(const marker of ['isolated-integration-only','隔离联调商品','website-qa@project1.test','test-customer-private.json','local-download-test.png','controlled-alpha','受控测试专用网站','controlled-ticket','controlled@example.test','preview-product-','模拟预览 · 官网模板','preview-category-'])assert.equal(content.includes(Buffer.from(marker)),false,asset.path+': '+marker);
 }
});
