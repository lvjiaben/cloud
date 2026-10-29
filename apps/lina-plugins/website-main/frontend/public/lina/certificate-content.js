import {translate} from './locale.js';
// Original hero copy is isolated from user-authorized additional sections.
export const certificateSource={source:'user-attachment:d73aa243-90af-4cf3-9600-8b091db8c490',copy:[
 {section:'hero',order:0,parent:'section.globalBanner/div.comp_container/div.pagebanner-relative/div.pagebanner/div.pagebanner-text',tag:'h1',headingLevel:1,text:'资质荣誉',interaction:'visible'},
 {section:'hero',order:1,parent:'section.globalBanner/div.comp_container/div.pagebanner-relative/div.pagebanner/div.pagebanner-text',tag:'p',headingLevel:null,text:'我们是一家拥有丰富资质荣誉的云服务商，为您提供卓越的技术支持和可靠的服务，助您在竞争激烈的市场中脱颖而出。',interaction:'visible'},
]};
export const certificateAdditions={source:'user-authorized-additions',sections:[
 {id:'principle',get title(){return translate('以规范为基石，以信任见长');},get description(){return translate('每一份信任，都值得认真回应。我们希望通过清晰的信息展示、审慎的服务表达与持续完善的服务体验，让客户在了解、选择与合作过程中更加从容。');}},
 {id:'qualifications',get title(){return translate('资质展示');},get description(){return translate('让资质信息清晰呈现，让合作判断有据可循。');}},
 {id:'values',get title(){return translate('值得信赖，始于每一处认真');},items:[
  {get title(){return translate('让信息更清晰');},get description(){return translate('以易于理解的方式组织重要信息，让客户更方便地了解服务内容与合作边界。');}},
  {get title(){return translate('让沟通更顺畅');},get description(){return translate('重视每一次咨询与反馈，让需求表达更直接，让沟通过程更有条理。');}},
  {get title(){return translate('让体验持续向前');},get description(){return translate('以长期视角看待服务，持续关注使用体验与实际需求，让每一次改进都更贴近客户。');}},
 ]},
 {id:'consult',get title(){return translate('进一步了解我们，开启合作沟通');},get description(){return translate('如需了解服务内容及相关资料，欢迎与我们联系。');},get button(){return translate('联系咨询');}},
 ]};
// No qualification API exists in the inspected public contract. Add verified records here only.
export const certificateRecords=[];
export const escapeCertificateText=text=>String(text).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');
export function certificateHTML(){
 const esc=escapeCertificateText;
 const hero=certificateSource.copy.map(item=>`<${item.tag} class="pagebanner-${item.tag==='h1'?'title':'description'}">${esc(item.text)}</${item.tag}>`).join('');
 return `<section class="globalBanner leftSide black"><div class="comp_container comp_container_auto comp_container_100"><div class="pagebanner-relative"><div class="pagebanner center-content"><div class="pagebanner-text">${hero}</div></div></div></div></section><main class="et-main certificate-main" data-certificate-additions>${certificateAdditions.sections.map((section,i)=>`<section id="certificate-${section.id}" class="certificate-section certificate-${section.id}" aria-labelledby="certificate-heading-${i}"><div class="comp_container comp_container_auto"><div class="certificate-section-heading"><h2 id="certificate-heading-${i}">${esc(section.title)}</h2>${section.description?`<p>${esc(section.description)}</p>`:''}</div>${section.id==='qualifications'?'<div class="certificate-grid" data-certificate-grid></div>':''}${section.items?`<div class="certificate-values-grid">${section.items.map((item,index)=>`<article class="certificate-value"><span class="certificate-value-number" aria-hidden="true">0${index+1}</span><h3>${esc(item.title)}</h3><p>${esc(item.description)}</p></article>`).join('')}</div>`:''}${section.button?`<a class="certificate-consult-button" data-certificate-consult href="/contact.html">${esc(section.button)}<span aria-hidden="true"> →</span></a>`:''}</div></section>`).join('')}</main>`;
}
