import {describe,it,expect} from 'vitest';
import {catalogPurchaseLinks} from './catalog-purchase-links';
describe('Configured product purchase links',()=>{
 it('keeps the website origin and uses the sales category independently of member navigation',()=>{
   expect(catalogPurchaseLinks({websiteurl:'https://shop.example',websiteconfigurepath:'/configure.html?product={product}',websitecatalogpath:'/services.html?lang=zh-CN'},'cloud/one','ctg_1')).toEqual({product:'https://shop.example/configure.html?product=cloud%2Fone',group:'https://shop.example/services.html?lang=zh-CN&category=ctg_1'});
 });
 it('rejects incomplete configuration, credentials and cross-origin paths',()=>{
   for(const websiteurl of ['', 'javascript:alert(1)','https://user:secret@shop.example','https://shop.example/subpath'])expect(()=>catalogPurchaseLinks({websiteurl,websiteconfigurepath:'/configure.html?product={product}',websitecatalogpath:'/products.html'},'cloud','ctg')).toThrow();
   for(const websitecatalogpath of ['//evil.example/products','https://evil.example','/\\evil.example'])expect(()=>catalogPurchaseLinks({websiteurl:'https://shop.example',websiteconfigurepath:'/configure.html?product={product}',websitecatalogpath},'cloud','ctg')).toThrow();
 });
});
