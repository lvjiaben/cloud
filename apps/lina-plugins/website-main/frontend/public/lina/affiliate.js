import {getLocale} from './locale.js';

// Controlled bilingual copy. No customer API is called from this public page.
const copy = {
  title:['联盟推广计划','Affiliate Program'],
  hero:['分享云服务，连接更多可能','Share cloud services. Connect more possibilities.'],
  intro:['了解推广方式，在客户中心管理您的推广链接、推荐记录与佣金信息。','Explore ways to refer and manage links, referrals and commissions in the customer center.'],
  code:['获取推广码','Get referral code'], mine:['我的推广','My referrals'],
  tabs:[['推广优势','Benefits'],['推广流程','How it works'],['推广规则','Program rules'],['推广方式','Ways to share'],['常见问题','FAQ']],
  headings:[['让每一次分享都有迹可循','Make every referral traceable'],['简单几步，开始分享','Start sharing in a few steps'],['了解规则，再开始推广','Understand the rules before sharing'],['用合适的方式，分享真实体验','Share real experiences in a way that suits you'],['您可能还想了解','A few things you may want to know']],
  subtitles:[['从专属链接到推荐记录，在同一个客户中心查看。','View referral links and records in one customer center.'],['先了解，再分享；参与情况以您的客户中心为准。','Explore before sharing. Check your participation details in the customer center.'],['以本方正式发布的政策与您账户中的适用信息为准。','Refer to our published policies and information applicable to your account.'],['以下为分享方式介绍，具体参与条件请先咨询确认。','These are possible ways to share. Confirm participation conditions first.'],['参与之前，先把问题了解清楚。','Get the details before taking part.']],
  benefits:[
    [['专属推广入口','Your referral destination'],['登录客户中心，查看属于您的推广码与推广链接。','Sign in to view your referral code and link.'],'↗'],
    [['推荐记录可查','Track your referrals'],['在我的推广中查看推荐客户与订单归因记录。','View referred customers and order attribution in My referrals.'],'◎'],
    [['佣金信息集中查看','Commissions in one place'],['在客户中心查看佣金账户、明细与结算记录。','View commission accounts, entries and settlement history in the customer center.'],'▤'],
    [['账户资料安全隔离','Private account information'],['推广码、推荐统计与佣金信息仅在登录后的客户中心展示。','Referral codes, statistics and commissions are displayed only in the signed-in customer center.'],'◇']
  ],
  steps:[
    [['登录客户中心','Sign in'],['进入本方客户中心，登录您的账户。','Visit our customer center and sign in to your account.']],
    [['查看推广信息','View referral details'],['打开我的推广，查看账户提供的推广码与链接。','Open My referrals to view the code and link available to your account.']],
    [['分享推广链接','Share your link'],['将客户中心提供的链接分享给有需要的人。','Share the link provided by the customer center with interested people.']],
    [['查看推荐与佣金','View referrals and commissions'],['登录查看推荐记录、佣金明细及结算信息。','Sign in to view referral records, commission entries and settlement information.']]
  ],
  ruleLabel:['推广说明','Program information'], ruleTitle:['公开规则尚未发布','Public rules are not yet published'],
  rule:['当前没有可供公开展示的完整推广政策。佣金比例、适用商品、参与资格、结算期限等信息，请先联系本方确认。','A complete public referral policy is not currently available. Contact us to confirm commission rates, eligible products, participation requirements and settlement timing.'],
  guarantee:['本页不作收益保证；未确认规则前，请勿依据预期收益作出承诺。','This page makes no earnings guarantee. Do not promise expected earnings before the applicable rules are confirmed.'],
  account:['查看账户信息','View account details'], ask:['咨询推广规则','Ask about program rules'],
  start:['从您的推广链接开始','Start with your referral link'], startDesc:['推广信息与参与状态，请在登录后查看。','Sign in to view referral information and participation status.'],
  methods:[
    [['网站内容分享','Website content'],['在自己的站点介绍使用场景，提供真实、有帮助的产品信息。','Describe relevant use cases on your site with useful, accurate product information.']],
    [['社群交流分享','Community conversations'],['在允许分享的社群中交流体验，尊重社群规则与他人意愿。','Share experiences where permitted, respecting community rules and individual preferences.']],
    [['经验与教程分享','Guides and tutorials'],['通过实际使用经验与教程，帮助有需求的人了解云服务。','Use practical guides and experiences to help interested readers understand cloud services.']]
  ],
  faq:[
    [['在哪里获取推广码？','Where can I find my referral code?'],['点击获取推广码或我的推广，进入客户中心。未登录时先完成登录，再查看您的推广信息。','Select Get referral code or My referrals to open the customer center. Sign in first if needed, then view your referral information.']],
    [['佣金比例和结算时间是多少？','What are the commission rates and settlement times?'],['当前官网尚未发布完整推广政策。请通过本方联系方式确认，不以页面示意或其他平台规则作为依据。','A complete policy has not been published on this website. Contact us to confirm; examples or other platforms’ rules do not apply.']],
    [['在哪里查看推荐统计和佣金？','Where can I view referral statistics and commissions?'],['登录客户中心的我的推广，查看您账户的推荐信息，并切换佣金视图查看账户、明细与结算记录。','Sign in to My referrals to view your referral information. Switch to the commissions view for accounts, entries and settlement history.']]
  ],
  help:['分享之前还有疑问？','Questions before you share?'], helpDesc:['查看本方联系方式，确认适用规则后再参与。','Contact us to confirm the applicable rules before participating.'], contact:['联系咨询','Contact us'],
  unavailable:['用户中心地址尚未配置，请联系本方客服。','The customer center address is not configured. Please contact support.']
};
const ids=['advantages','process','rules','methods','faq'];
export function initializeAffiliate(document) {
  const view=document.defaultView, main=document.querySelector('[data-affiliate]');
  const motion=view.matchMedia('(prefers-reduced-motion: reduce)');
  // Exclude this controlled copy from the legacy shell translation observer.
  main.dataset.localeOriginal='';
  let observer, frame=0;
  const t=value=>value[getLocale()==='en-US'?1:0];
  const button=(value,secondary=false)=>'<a class="aff-button'+(secondary?' aff-secondary':'')+'" data-aff-customer>'+t(value)+'</a>';
  function bindCustomerLinks() {
    let destination;
    try {const u=new URL('/account/referrals',view.__LINA_TEMPLATE__?.customerUrl);if(['http:','https:'].includes(u.protocol)&&!u.username&&!u.password)destination=u.href;} catch {}
    main.querySelectorAll('[data-aff-customer]').forEach(a=>{if(destination){a.href=destination;a.removeAttribute('aria-disabled');a.removeAttribute('title');}else{a.removeAttribute('href');a.setAttribute('aria-disabled','true');a.title=t(copy.unavailable);}});
  }
  const nav=()=>main.querySelector('.aff-nav');
  function select(index) {
    const links=[...nav().querySelectorAll('a')];
    links.forEach((a,i)=>{if(i===index)a.setAttribute('aria-current','location');else a.removeAttribute('aria-current');});
    const a=links[index], n=nav();
    if(n.scrollWidth>n.clientWidth)n.scrollTo({left:a.offsetLeft-n.clientWidth/2+a.offsetWidth/2,behavior:motion.matches?'instant':'smooth'});
  }
  function update() {
    frame=0;let index=0;
    const edge=nav().getBoundingClientRect().bottom+24;
    ids.forEach((id,i)=>{if(document.getElementById(id).getBoundingClientRect().top<=edge)index=i;});
    select(index);
  }
  function restore() {
    const index=ids.findIndex(id=>'#'+id===view.location.hash);
    if(index>=0)document.getElementById(ids[index]).scrollIntoView({behavior:'instant',block:'start'});
    update();
  }
  function reveal() {
    observer?.disconnect();const targets=main.querySelectorAll('.aff-reveal');
    if(motion.matches||!view.IntersectionObserver){targets.forEach(e=>e.classList.add('is-visible'));return;}
    observer=new view.IntersectionObserver(entries=>entries.forEach(entry=>{if(entry.isIntersecting){entry.target.classList.add('is-visible');observer.unobserve(entry.target);}}),{threshold:.08});
    targets.forEach(e=>observer.observe(e));main.classList.add('aff-motion-ready');
  }
  function render() {
    observer?.disconnect();main.classList.remove('aff-motion-ready');
    const heading=i=>'<header class="aff-heading aff-reveal"><h2>'+t(copy.headings[i])+'</h2><p>'+t(copy.subtitles[i])+'</p></header>';
    const section=(i,body,cls='')=>'<section id="'+ids[i]+'" class="aff-section '+cls+'"><div class="aff-wrap">'+heading(i)+body+'</div></section>';
    main.innerHTML='<section class="aff-hero"><div class="aff-wrap aff-hero-inner"><div class="aff-hero-copy"><p class="aff-eyebrow">'+t(copy.title)+'</p><h1>'+t(copy.hero)+'</h1><p>'+t(copy.intro)+'</p><div class="aff-actions">'+button(copy.code)+button(copy.mine,true)+'</div></div><img class="aff-hero-art" src="/lina/assets/affiliate-network.svg" alt="" width="600" height="390"></div></section>'+
      '<nav class="aff-nav" aria-label="'+t(copy.title)+'"><div class="aff-wrap">'+ids.map((id,i)=>'<a href="#'+id+'">'+t(copy.tabs[i])+'</a>').join('')+'</div></nav>'+
      section(0,'<div class="aff-advantages">'+copy.benefits.map(([title,desc,icon])=>'<article class="aff-card aff-reveal"><span class="aff-icon" aria-hidden="true">'+icon+'</span><div><h3>'+t(title)+'</h3><p>'+t(desc)+'</p></div></article>').join('')+'</div>')+
      section(1,'<div class="aff-orb" aria-hidden="true"></div><ol class="aff-steps">'+copy.steps.map(([title,desc],i)=>'<li class="aff-step aff-reveal"><div class="aff-step-top"><span>0'+(i+1)+'</span><span aria-hidden="true">'+['↗','⌕','⌁','▥'][i]+'</span></div><h3>'+t(title)+'</h3><p>'+t(desc)+'</p></li>').join('')+'</ol>','aff-tinted')+
      section(2,'<div class="aff-rule-panel aff-reveal"><div class="aff-rule-label"><span aria-hidden="true">▤</span><h3>'+t(copy.ruleLabel)+'</h3></div><div class="aff-rule-copy"><h3>'+t(copy.ruleTitle)+'</h3><p>'+t(copy.rule)+'</p><p>'+t(copy.guarantee)+'</p><div class="aff-actions">'+button(copy.account)+'<a class="aff-text-link" href="/contact.html">'+t(copy.ask)+' →</a></div></div></div>')+
      section(3,'<div class="aff-orb aff-orb-small" aria-hidden="true"></div><div class="aff-method-banner aff-reveal"><div><h3>'+t(copy.start)+'</h3><p>'+t(copy.startDesc)+'</p></div>'+button(copy.code)+'</div><div class="aff-method-grid">'+copy.methods.map(([title,desc],i)=>'<article class="aff-method aff-reveal"><span class="aff-method-number">0'+(i+1)+'</span><h3>'+t(title)+'</h3><p>'+t(desc)+'</p></article>').join('')+'</div>')+
      section(4,'<div class="aff-faq">'+copy.faq.map(([q,a])=>'<article class="aff-question aff-reveal"><h3><span aria-hidden="true">Q</span>'+t(q)+'</h3><p>'+t(a)+'</p></article>').join('')+'</div><aside class="aff-help aff-reveal"><div><h3>'+t(copy.help)+'</h3><p>'+t(copy.helpDesc)+'</p></div><a class="aff-button aff-secondary" href="/contact.html">'+t(copy.contact)+'</a></aside>','aff-tinted');
    bindCustomerLinks();reveal();view.requestAnimationFrame(restore);
  }
  main.addEventListener('click',event=>{
    const a=event.target.closest('.aff-nav a');
    if(!a||event.button!==0||event.metaKey||event.ctrlKey||event.shiftKey||event.altKey)return;
    event.preventDefault();
    if(view.location.hash!==a.hash)view.history.pushState(null,'',a.hash);
    document.getElementById(a.hash.slice(1)).scrollIntoView({behavior:motion.matches?'instant':'smooth',block:'start'});
    select(ids.indexOf(a.hash.slice(1)));
  });
  view.addEventListener('scroll',()=>{if(!frame)frame=view.requestAnimationFrame(update);},{passive:true});
  view.addEventListener('resize',update);view.addEventListener('popstate',restore);view.addEventListener('hashchange',restore);view.addEventListener('pageshow',restore);
  document.addEventListener('lina:locale-changed',render);motion.addEventListener('change',reveal);
  render();
  // Deployment config is loaded asynchronously by the existing shared shell.
  return {bindCustomerLinks};
}
