import {translate} from './locale.js';
// The current public campaign contract is product-scoped. Until a bounded
// public activity feed exists, keep the page truthful with the same empty state
// as the approved activity presentation instead of querying staff-only data.
export function initializeActivityPage(document){
 const page=document.querySelector('[data-activity-page]');if(!page)return;
 const tabs=[...page.querySelectorAll('[data-activity-filter]')];
 const list=page.querySelector('[data-activity-list]');
 const empty=page.querySelector('[data-activity-empty]');
 let items=[];
 const render=key=>{
  if(!list)return;
  list.replaceChildren();
  const visible=key==='all'?items:[];
  for(const item of visible){
   const card=document.createElement('article');card.className='activity-item';
   const content=document.createElement('div');content.className='activity-item-content';
   const title=document.createElement('h2');title.className='activity-item-title';title.textContent=item.title||translate('最新活动');
   const summary=document.createElement('p');summary.className='activity-item-desc';summary.textContent=item.summary||'';
   content.append(title,summary);card.append(content);list.append(card);
  }
  if(empty)empty.hidden=visible.length>0;
 };
 const select=key=>{
  tabs.forEach(tab=>{
   const selected=tab.dataset.activityFilter===key;
   tab.classList.toggle('selected',selected);tab.setAttribute('aria-selected',String(selected));
  });
  render(key);
 };
 tabs.forEach(tab=>{
  tab.setAttribute('role','tab');tab.tabIndex=0;
  tab.addEventListener('click',()=>select(tab.dataset.activityFilter||'all'));
  tab.addEventListener('keydown',event=>{if(['Enter',' '].includes(event.key)){event.preventDefault();select(tab.dataset.activityFilter||'all');}});
 });
 select('all');
}
