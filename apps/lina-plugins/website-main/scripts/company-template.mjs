// Keep the offline company-page component tree without source business data.
// Only class names survive: URLs, IDs, inline styles, handlers and labels do not.
function clean(node) {
  for (const child of [...node.childNodes]) {
    if (child.nodeType === 3 || child.nodeType === 8) child.remove();
    else if (child.nodeType === 1) {
      if (child.matches('script,style,iframe,object,embed')) child.remove();
      else clean(child);
    }
  }
  for (const attribute of [...node.attributes]) {
    if (attribute.name !== 'class') node.removeAttribute(attribute.name);
  }
}

export function companyTemplate(source, kind) {
  if (!['about', 'contact'].includes(kind)) throw new Error('Unsupported company page');
  const clone = selector => {
    const original = source.querySelector(selector);
    if (!original) throw new Error('Missing company template: ' + selector);
    const result = original.cloneNode(true);
    clean(result);
    return result;
  };
  const banner = clone('.globalBanner');
  const tabs = clone('.et-container');
  const main = clone('.et-main');
  // There is no public contract for banners or the remaining tab destinations.
  // Retain their templates without presenting empty or invented business actions.
  banner.hidden = true;
  tabs.hidden = true;
  const sections = [...main.querySelectorAll('.et-slide')];
  sections.forEach(section => { section.hidden = true; });
  const first = sections[0];
  first.hidden = false;
  if (kind === 'about') {
    main.querySelector('.companyIntroduction-left').hidden = true;
    main.querySelector('.companyIntroduction-right-title').textContent = '关于我们';
    main.querySelector('.companyIntroduction-right-content').insertAdjacentHTML('beforeend',
      '<article data-content-body></article><p data-status role="status">正在读取网站资料…</p>');
  } else {
    // The source banner previously supplied the fixed-header offset.
    main.style.paddingTop = 'calc(var(--navheight,62px) + 20px)';
    first.querySelector('.currency-title-a').textContent = '联系我们';
    // Preserve every original contact card, including unavailable channels.
    main.querySelectorAll('.currency-content-item').forEach(item => { item.hidden = true; });
    const items = first.querySelectorAll('.currency-content-item');
    for (const [index, key, label] of [[0, 'phone', '电话'], [1, 'email', '电子邮箱']]) {
      const item = items[index];
      item.setAttribute('data-content-contact', key);
      const icon = item.querySelector('.currency-content-item-img');
      icon.hidden = false;
      const image = icon.querySelector('img');
      image.src = 'static/picture/' + (key === 'phone' ? 'telephone.svg' : 'mailbox.svg');
      image.alt = '';
      image.setAttribute('aria-hidden', 'true');
      item.querySelector('.label').textContent = label;
      item.querySelector('.consultationButton a span').textContent = label;
    }
    first.querySelector('.currency-content').insertAdjacentHTML('afterend',
      '<p data-status role="status">正在读取联系方式…</p>');
  }
  return {banner, tabs, main};
}
