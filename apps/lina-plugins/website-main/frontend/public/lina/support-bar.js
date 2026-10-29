import {translate} from './locale.js';
const text = (value) => typeof value === 'string' ? value.trim() : '';

function safeImage(value) {
  const raw = text(value);
  if (!raw || raw.length > 2048 || raw.startsWith('//') || raw.includes('\\')) return '';
  try {
    const url = new URL(raw, location.origin);
    return ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password
      ? url.href
      : '';
  } catch {
    return '';
  }
}

function icon(name) {
  const i = document.createElement('i');
  i.className = `${name.startsWith('sk-') ? 'develop' : 'universal'} ${name}`;
  i.setAttribute('aria-hidden', 'true');
  return i;
}

function item({ iconName, title, description, href, external = false }) {
  const a = document.createElement('a');
  a.className = 'onlineService_Title';
  a.href = href;
  if (external) {
    a.target = '_blank';
    a.rel = 'noopener noreferrer';
  }
  const iconBox = document.createElement('div');
  iconBox.className = 'onlineService_Title_Icon';
  iconBox.append(icon(iconName));
  const copy = document.createElement('div');
  copy.className = 'onlineService_Title_Text';
  const heading = document.createElement('p');
  heading.className = 'onlineServicetitle';
  // Preserve bilingual source copy for the observer when English labels have aliases.
  heading.textContent = typeof title === 'string' ? title : title.zhCN;
  const detail = document.createElement('span');
  detail.className = 'onlineServicedesc';
  detail.textContent = description;
  copy.append(heading, detail);
  a.append(iconBox, copy);
  return a;
}

function panel(trigger, content, label, last = false) {
  const wrapper = document.createElement('div');
  wrapper.className = 'online_navItem';
  const button = document.createElement('button');
  button.className = 'onlineService';
  button.type = 'button';
  button.setAttribute('aria-expanded', 'false');
  button.setAttribute('aria-label', label);
  button.append(trigger);
  const hover = document.createElement('div');
  hover.className = 'hoverWindow';
  hover.hidden = true;
  if (last) hover.classList.add('online-last-panel');
  const triangle = document.createElement('div');
  triangle.className = 'onlineservice_triangle';
  const body = document.createElement('div');
  body.className = 'onlineService_Body';
  body.append(content);
  hover.append(triangle, body);
  wrapper.append(button, hover);
  const open = (value) => {
    button.setAttribute('aria-expanded', String(value));
    hover.hidden = !value;
    hover.style.display = value ? 'block' : 'none';
  };
  wrapper.addEventListener('mouseenter', () => open(true));
  wrapper.addEventListener('mouseleave', () => { if (!wrapper.contains(document.activeElement)) open(false); });
  wrapper.addEventListener('focusin', () => open(true));
  wrapper.addEventListener('focusout', event => { if (!wrapper.contains(event.relatedTarget)) open(false); });
  button.addEventListener('click', () => open(true));
  wrapper.addEventListener('keydown', event => { if (event.key === 'Escape') { button.focus(); open(false); } });
  return wrapper;
}

function makeContactPanel(brand) {
  const content = document.createElement('div');
  content.className = 'onlineservice_content';
  const phone = text(brand.contactPhone);
  const qq = /^\d{1,20}$/.test(text(brand.businessQq)) ? text(brand.businessQq) : '';
  if (qq) content.append(item({
    iconName: 'icon-dianhua', title: translate('商务 QQ'), description: translate('售前咨询与售后服务'),
    href: `https://wpa.qq.com/msgrd?v=3&uin=${encodeURIComponent(qq)}&site=qq&menu=yes`, external: true,
  }));
  if (phone) content.append(item({ iconName: 'icon-dianhua', title: translate('服务热线'), description: `拨打：${phone}`, href: `tel:${encodeURIComponent(phone)}` }));
  if (!content.childElementCount) content.append(item({ iconName: 'icon-dianhua', title: { zhCN: '联系我们', enUS: 'Contact us' }, description: translate('查看本方联系方式'), href: '/contact.html' }));
  return content;
}

function makeWorkPanel(customerLink) {
  const content = document.createElement('div');
  content.className = 'onlineservice_content workOrder';
  content.append(
    item({ iconName: 'icon_edit', title: translate('提交工单'), description: translate('我们会第一时间处理您的需求'), href: customerLink('/account/support/new') }),
    item({ iconName: 'sk-tousujianyi', title: translate('建议反馈'), description: translate('真诚期待您的宝贵意见'), href: customerLink('/account/support/new') }),
    item({ iconName: 'sk-jubao', title: translate('违法举报'), description: translate('“违法有害信息”举报专区'), href: customerLink('/account/support/new') }),
  );
  return content;
}

function makeQrPanel(brand) {
  const content = document.createElement('div');
  content.className = 'onlineservice_content';
  for (const [value, label] of [[brand.enterpriseWechatQrCodeUrl, '企业微信'], [brand.wechatOfficialQrCodeUrl, '微信公众号']]) {
    const qr = safeImage(value);
    if (!qr) continue;
    const card = document.createElement('div');
    const title = document.createElement('div');
    title.className = 'onlineService_Title';
    const iconBox = document.createElement('div');
    iconBox.className = 'onlineService_Title_Icon';
    const copy = document.createElement('div');
    copy.className = 'onlineService_Title_Text';
    const image = document.createElement('img');
    image.className = 'qrCode';
    image.src = qr;
    image.alt = label + '二维码';
    copy.append(image);
    title.append(iconBox, copy);
    card.append(title);
    const caption = document.createElement('div');
    caption.className = 'onlineService_Close';
    caption.textContent = label;
    card.append(caption);
    content.append(card);
  }
  if (!content.childElementCount) {
    content.append(item({ iconName: 'sk-31erweima', title: translate('微信客服'), description: translate('暂未配置二维码'), href: '/contact.html' }));
  }
  return content;
}

export function renderSupportBar(document, brand, customerLink) {
  const signature = JSON.stringify([brand.contactPhone, brand.businessQq, brand.enterpriseWechatQrCodeUrl, brand.wechatOfficialQrCodeUrl, customerLink('/account/support/new')]);
  const previous = document.querySelector('[data-lina-support-bar]');
  if (previous?.dataset.signature === signature) return;
  previous?.dispatchEvent(new Event('support-bar-dispose'));
  previous?.remove();
  const root = document.createElement('aside');
  root.className = 'online_Nav lina-support-bar';
  root.dataset.linaSupportBar = '';
  root.dataset.signature = signature;
  root.style.setProperty('--servicewidth', '40px');
  root.setAttribute('aria-label', '客户服务');
  const consultation = document.createElement('div');
  consultation.className = 'phone-container';
  const consultationBox = document.createElement('div');
  consultationBox.className = 'consultation-box';
  for (const suffix of ['one', 'two', 'three', 'four']) {
    const decorator = document.createElement('div');
    decorator.className = 'border-decorator decorator-' + suffix;
    consultationBox.append(decorator);
  }
  const avatar = document.createElement('div');
  avatar.className = 'consultation-image consultation-placeholder';
  avatar.append(icon('sk-tubiao_kefu'));
  const link = document.createElement('a');
  link.className = 'consultation-button';
  link.href = '/contact.html';
  const label = document.createElement('p');
  label.textContent = '在线咨询';
  link.append(label);
  consultationBox.append(avatar, link);
  consultation.append(consultationBox);
  const body = document.createElement('div');
  body.className = 'online_body';
  body.append(panel(icon('sk-tubiao_kefu'), makeContactPanel(brand), '联系客服'));
  if (customerLink('/account/support/new')) body.append(panel(icon('sk-bianji1'), makeWorkPanel(customerLink), '工单与反馈'));
  const qr = panel(icon('sk-31erweima'), makeQrPanel(brand), '微信二维码', true);
  body.append(qr);
  root.append(consultation, body);
  const top = document.createElement('button');
  top.className = 'onlineTop';
  top.type = 'button';
  top.title = '返回顶部';
  top.append(icon('icon-arrow-11'));
  root.append(top);
  root.addEventListener('click', (event) => {
    if (event.target.closest('.onlineTop')) window.scrollTo({ top: 0, behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth' });
  });
  const updateTop = () => { top.hidden = window.scrollY <= 240; top.style.opacity = top.hidden ? '0' : '1'; };
  window.addEventListener('scroll', updateTop, { passive: true });
  root.addEventListener('support-bar-dispose', () => window.removeEventListener('scroll', updateTop), { once: true });
  updateTop();
  document.body.append(root);
}
