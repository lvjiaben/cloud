import {localizeAssetImage} from './locale.js';
// Display-only assets extracted from the supplied safety-page HTML.
// Identity, QR, support, and filing assets remain owned by the brand adapter.
const base = '/lina/safety-assets/';
export const safetyImages = {
  advantages: [
    {src: base + 'remoteL17138728737820.png', alt: '方案灵活'},
    {src: base + 'remoteL17139321972692.png', alt: '部署简单'},
    {src: base + 'remoteL17139319668888.png', alt: '成本效益'},
  ],
  architecture: {src: base + 'remoteL17139281467723.png', enSrc: base + 'remoteL17139281467723.en.svg', alt: ''},
  partners: [
    'remoteL17063681798362.png','local65b51d7304309.jpg','local65b51d681ab80.jpeg','remoteL17063681802376.png',
    'local65b51d6d7b96f.jpeg','remoteL17063681807597.png','remoteL17063681819770.png','remoteL17063681816166.png',
    'remoteL17063681811685.png','remoteL17063681815858.png','local65b51d70398c6.jpg','remoteL17063681823466.png',
  ].map(src => ({src: base + src, alt: ''})),
  more: [
    'remoteL17138728737820.png','remoteL17138729853995.png','remoteL17138729852306.png','remoteL17138729502939.png','remoteL17138730145615.png',
  ].map(src => ({src: base + src, alt: ''})),
  backgrounds: {
    problemAndMore: base + 'remoteL17063678634493.jpg',
    partners: base + 'remoteL17138735335796.png',
  },
};

export function renderSafetyImages(root) {
  function image(value) {
    const node = document.createElement('img');
    node.src = value.src; node.alt = value.alt; node.decoding = 'async';
    if (value.enSrc) localizeAssetImage(node, value.src, value.enSrc);
    return node;
  }
  root.querySelectorAll('.advantage-Content-list-item-icon .safety-image-slot').forEach((slot, index) => slot.replaceWith(image(safetyImages.advantages[index])));
  root.querySelector('.architecture-item-img .safety-image-slot')?.replaceWith(image(safetyImages.architecture));
  root.querySelectorAll('.partner-Content-logo-item .safety-image-slot').forEach((slot, index) => slot.replaceWith(image(safetyImages.partners[index])));
  root.querySelectorAll('.more-item-img .safety-image-slot').forEach((slot, index) => slot.replaceWith(image(safetyImages.more[index])));
}
