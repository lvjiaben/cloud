import {localized} from './locale.js';
import {webmasterContent as data} from './webmaster-content.js';

export function initializeWebmaster(document){
 const main=document.querySelector('[data-webmaster]');if(!main)return;
 // Register both languages with the existing locale adapter before it starts observing.
 const register=value=>{if(Array.isArray(value))value.forEach(register);else if(value&&typeof value==='object'){if(value.zhCN&&value.enUS)localized(value);else Object.values(value).forEach(register);}};
 register(data);
 const text=value=>localized(value);
 const element=(tag,cls,value)=>{const node=document.createElement(tag);if(cls)node.className=cls;if(value)node.textContent=text(value);return node;};
 const image=(name,cls)=>{const img=element('img',cls);img.src='/lina/webmaster-assets/'+name+'.svg';img.alt='';img.setAttribute('aria-hidden','true');img.width=480;img.height=360;return img;};
 const link=(label,target,cls='wm-button')=>{const node=element('a',cls,label);node.href=target;return node;};
 const section=(id,title,description)=>{const node=element('section','wm-section wm-reveal');node.id=id;const heading=element('header','wm-heading');heading.append(element('h2','',title));if(description)heading.append(element('p','',description));node.append(heading);main.append(node);return node;};
 const hero=element('section','wm-hero');const wrap=element('div','wm-wrap');
 const intro=element('div','wm-hero-copy');intro.append(element('h1','',data.title),element('p','',data.description));
 wrap.append(intro,image('cloud-gift','wm-hero-art'));hero.append(wrap);main.append(hero);
 const cards=element('div','wm-cards wm-wrap');
 data.cards.forEach((item,index)=>{const card=element('article','wm-card'+(index===2?' is-active':''));card.append(element('span','wm-accent',item.accent),element('h2','',item.title),element('p','',item.description));
  card.append(link(item.link,item.target,'wm-text-button'));
  card.addEventListener('pointerenter',()=>{for(const sibling of cards.children)sibling.classList.toggle('is-active',sibling===card);});cards.append(card);
 });
 cards.addEventListener('pointerleave',()=>{[...cards.children].forEach((card,index)=>card.classList.toggle('is-active',index===2));});main.append(cards);
 const content=element('div','wm-wrap wm-content');main.append(content);
 const addSection=(...args)=>{const node=section(...args);content.append(node);return node;};
 const application=addSection('wm-application',data.applicationTitle);
 const panel=element('div','wm-application-panel');const header=element('div','wm-application-header');const lead=element('div');lead.append(element('h3','',data.applicationHeading),element('p','',data.applicationDescription));header.append(lead,link(data.apply,'/contact.html'));panel.append(header);
 const requirements=element('div','wm-requirements');data.requirements.forEach(item=>{const row=element('article','wm-requirement');const copy=element('div');copy.append(element('h3','',item.title),element('p','',item.description));row.append(image(item.icon),copy);requirements.append(row);});panel.append(requirements);application.append(panel);
 const plans=addSection('wm-plans',data.plansTitle,data.plansDescription);const vouchers=element('div','wm-vouchers');
 data.plans.forEach(item=>{const voucher=element('article','wm-voucher wm-reveal');const left=element('div','wm-plan-left');const info=element('div','wm-plan-copy');info.append(element('h3','',item.title),element('p','',item.configuration),link(data.submit,'/contact.html'));left.append(image('server','wm-server-art'),info);
  const right=element('div','wm-plan-right');const duration=element('div','wm-duration');duration.append(element('strong','',item.duration),element('p','',item.durationDescription));const details=element('div','wm-plan-details');for(const [label,value] of [[data.regionLabel,item.region],[data.usageLabel,item.usage]]){const p=element('p');p.append(element('span','',label),element('span','',value));details.append(p);}right.append(duration,details);voucher.append(left,right);vouchers.append(voucher);});plans.append(vouchers);
 const rules=addSection('wm-rules',data.rulesTitle);const rulePanel=element('div','wm-rules-panel');const ruleHeading=element('h3');ruleHeading.append(image('materials'),document.createTextNode(text(data.rulesHeading)));const list=element('ol');const appendRules=(items,parent)=>items.forEach(rule=>{const li=element('li','',rule.text||rule);if(rule.children?.length){const nested=element('ul');appendRules(rule.children,nested);li.append(nested);}parent.append(li);});appendRules(data.rules,list);rulePanel.append(ruleHeading,list);rules.append(rulePanel);
 const cases=addSection('wm-cases',data.casesTitle,data.casesDescription);const caseGrid=element('div','wm-case-grid');data.cases.forEach(item=>{const card=element('article','wm-case');card.append(image('community'),element('h3','',item.title),element('p','',item.label));caseGrid.append(card);});if(data.cases.length)cases.append(caseGrid);
 const motion=document.defaultView.matchMedia('(prefers-reduced-motion: reduce)');let observer;
 const reveal=()=>{observer?.disconnect();main.classList.remove('wm-motion');for(const item of main.querySelectorAll('.wm-reveal'))item.classList.remove('is-visible');if(motion.matches||!document.defaultView.IntersectionObserver)return;
  main.classList.add('wm-motion');observer=new document.defaultView.IntersectionObserver(entries=>{for(const entry of entries)if(entry.isIntersecting){entry.target.classList.add('is-visible');observer.unobserve(entry.target);}},{threshold:0.08});main.querySelectorAll('.wm-reveal').forEach(item=>observer.observe(item));};
 reveal();motion.addEventListener('change',reveal);
}
