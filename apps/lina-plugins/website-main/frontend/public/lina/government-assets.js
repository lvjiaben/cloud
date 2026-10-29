import {localizeAssetImage} from './locale.js';
// Display-only assets extracted from the supplied government-page HTML.
// Brand, QR, support, and filing assets remain owned by the public brand adapter.
const base='/lina/government-assets/';
export const governmentImages={
 advantages:[
  {src:base+'remoteL17138730125602.png',alt:'提升电子政务效率'},
  {src:base+'remoteL17138729496462.png',alt:'建立全方位安全体系'},
  {src:base+'remoteL17139321972692.png',alt:'提高服务供给质量'},
 ],
 architecture:{src:base+'remoteL17139346523543.png',enSrc:base+'remoteL17139346523543.en.svg',alt:''},
 partners:['remoteL17063681798362.png','local65b51d7304309.jpg','local65b51d681ab80.jpeg','remoteL17063681802376.png','local65b51d6d7b96f.jpeg','remoteL17063681807597.png','remoteL17063681819770.png','remoteL17063681816166.png','remoteL17063681811685.png','remoteL17063681815858.png','local65b51d70398c6.jpg','remoteL17063681823466.png'].map(src=>({src:base+src,alt:''})),
 more:['remoteL17138728737820.png','remoteL17138729853995.png','remoteL17138729852306.png','remoteL17138729502939.png','remoteL17138730145615.png'].map(src=>({src:base+src,alt:''})),
};
export function renderGovernmentImages(root){
 const image=value=>{const node=document.createElement('img');node.src=value.src;node.alt=value.alt;node.decoding='async';if(value.enSrc)localizeAssetImage(node,value.src,value.enSrc);return node;};
 root.querySelectorAll('.advantage-Content-list-item-icon .government-image-slot').forEach((slot,index)=>slot.replaceWith(image(governmentImages.advantages[index])));
 root.querySelector('.architecture-item-img .government-image-slot')?.replaceWith(image(governmentImages.architecture));
 root.querySelectorAll('.partner-Content-logo-item .government-image-slot').forEach((slot,index)=>slot.replaceWith(image(governmentImages.partners[index])));
 root.querySelectorAll('.more-item-img .government-image-slot').forEach((slot,index)=>slot.replaceWith(image(governmentImages.more[index])));
}
