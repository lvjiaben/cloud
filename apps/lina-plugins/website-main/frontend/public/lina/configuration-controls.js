import {localizedControlLabel} from './locale.js';
// Visual grouping only. Values and availability always come from the original select.
const families=[['CentOS','21.svg',/^centos(?=\b|\d|[-_])/i],['Ubuntu','31.svg',/^ubuntu(?:\b|[-_])/i],['Windows','11.svg',/^(?:microsoft\s+)?windows(?:\b|[-_])/i],['Debian','41.svg',/^debian(?:\b|[-_])/i],['Rocky','9.svg',/^rocky(?:\b|[-_])/i],['AlmaLinux','9.svg',/^alma\s*linux(?:\b|[-_])/i]];
const activeGroups=new WeakMap(),installedDocuments=new WeakSet();
function closeOpen(root){for(const details of root.querySelectorAll('details[open]'))details.open=false;}
function familyFor(text){
 const known=families.find(([, ,pattern])=>pattern.test(text));if(known)return {name:known[0],icon:known[1]};
 const prefix=text.match(/^[^\d]+/)?.[0].trim().replace(/[-_]+/g,' ').replace(/\s+/g,' ');
 return {name:prefix||'其他系统',icon:''};
}
export function enhanceOperatingSystem(input,field,label){
 const title=label(field).trim().toLowerCase(),optionLabels=(field.options||[]).map(option=>label(option).trim());
 const looksLikeOS=optionLabels.filter(text=>families.some(([, ,pattern])=>pattern.test(text))).length>=2;
 if(field.type!=='single_select'||!(field.presentation==='operating_system'||/操作系统|作业系统|系统镜像|系统版本|^os(?:\s*version)?$|operating\s*system/.test(title)||looksLikeOS)||field.options?.some(o=>o.children?.length))return false;
 const options=[...input.options].filter(o=>o.value);if(!options.length)return false;
 const doc=input.ownerDocument,make=(tag,cls,text)=>{const el=doc.createElement(tag);if(cls)el.className=cls;if(text!==undefined)el.textContent=text;return el;};
 const container=make('div','os-container lina-os-controls');container.setAttribute('role','group');container.setAttribute('aria-label',localizedControlLabel(label(field),'选项'));
 const groups=new Map();for(const option of options){const family=familyFor(option.textContent.trim());if(!groups.has(family.name))groups.set(family.name,{family,options:[]});groups.get(family.name).options.push(option);}
 const cards=[];
 const ordered=[...families.map(([name])=>name),...Array.from(groups.keys()).filter(name=>!families.some(([known])=>known===name))];
 for(const name of ordered){const group=groups.get(name);if(!group)continue;
  const card=make('div','os-card'),details=make('details','version-dropdown'),summary=make('summary'),header=make('div','os-header');card.dataset.os=name;
  if(group.family.icon){const img=make('img','img_os');img.src=new URL('static/picture/'+group.family.icon,doc.baseURI).href;img.alt='';img.height=20;img.width=20;img.decoding='async';header.append(img);}
  header.append(make('span','os-name unselectable',name));const current=make('div','dropdown-btn maximumRows1 unselectable','选择版本');summary.append(header,current);summary.setAttribute('aria-label',localizedControlLabel(name,'版本'));
  const list=make('ul','version-list');
  for(const option of group.options){const li=make('li'),button=make('button','lina-os-version',option.textContent);button.type='button';button.id=input.id+'-os-'+options.indexOf(option);button.dataset.configEnhanced='true';button.disabled=input.disabled||option.disabled;button.addEventListener('click',()=>{input.value=option.value;details.open=false;sync();summary.focus();input.dispatchEvent(new Event('change',{bubbles:true}));});li.append(button);list.append(li);}
  details.append(summary,list);card.append(details);container.append(card);cards.push({card,details,summary,current,options:group.options,list});
  details.addEventListener('toggle',()=>{card.style.zIndex=details.open?'3':'';if(details.open)for(const other of cards)if(other.details!==details)other.details.open=false;});
  details.addEventListener('focusout',event=>{if(!details.contains(event.relatedTarget))details.open=false;});
  if(input.disabled){summary.setAttribute('aria-disabled','true');summary.tabIndex=-1;details.addEventListener('click',event=>event.preventDefault());}
 }
 function sync(){for(const {card,current,options,list}of cards){const selected=options.find(o=>o.value===input.value);card.classList.toggle('active',!!selected);current.textContent=selected?.textContent||'选择版本';for(const [index,li]of [...list.children].entries()){const active=options[index].value===input.value;li.classList.toggle('active',active);li.firstElementChild.setAttribute('aria-pressed',String(active));}}}
 input.classList.add('lina-option-source');input.addEventListener('change',sync);input.addEventListener('focus',()=>{(cards.find(c=>c.card.classList.contains('active'))||cards[0]).summary.focus();});
 if(!installedDocuments.has(doc)){doc.addEventListener('pointerdown',event=>{const root=activeGroups.get(event.currentTarget);if(root&&event.target instanceof Node&&!root.contains(event.target))closeOpen(root);});doc.addEventListener('keydown',event=>{if(event.key!=='Escape')return;const root=activeGroups.get(event.currentTarget);if(!root)return;const opened=root.querySelector('details[open]');if(opened){const summary=opened.querySelector('summary');closeOpen(root);summary?.focus();}});installedDocuments.add(doc);}
 activeGroups.set(doc,container);sync();input.after(container);return true;
}
