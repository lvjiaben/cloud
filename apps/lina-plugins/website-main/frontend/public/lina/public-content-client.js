// Read the current public owners without introducing product or content fields.
const contentBase='/x/idc-core/api/v1/public/content';
const announcementsBase='/x/content-notice/api/v1/public/announcements';
const announcementPrefix='announcement:';
function announcementItem(item){
 if(!item||!Number.isSafeInteger(item.id)||item.id<=0)throw new Error('invalid_public_announcement');
 return {contentNo:announcementPrefix+item.id,type:'announcement',title:item.title||'',summary:'',body:item.content||'',publishedAt:item.createdAt??item.updatedAt??0,updatedAt:item.updatedAt??0};
}
export async function readPublicContentList(request,{type,query='',cursor='',limit=20}={}){
 if(!['news','knowledge','download','announcement'].includes(type))throw new Error('invalid_public_content_type');
 const size=Math.min(100,Math.max(1,Number(limit)||20));
 if(type==='announcement'){
  // The announcement owner exposes pagination only; do not pretend to search a truncated page.
  if(String(query).trim())throw new Error('announcement_search_unavailable');
  const page=cursor===''?1:Number(cursor);
  if(!Number.isSafeInteger(page)||page<1)throw new Error('invalid_announcement_page');
  const data=await request(announcementsBase+'?'+new URLSearchParams({pageNum:String(page),pageSize:String(size)}));
  if(!Array.isArray(data.list)||!Number.isSafeInteger(data.total)||data.total<0)throw new Error('invalid_public_announcement_page');
  return {items:data.list.map(announcementItem),nextCursor:(page-1)*size+data.list.length<data.total?String(page+1):''};
 }
 const params=new URLSearchParams({type,limit:String(size)});
 if(query)params.set('query',query);if(cursor)params.set('cursor',cursor);
 return request(contentBase+'/items?'+params);
}
export async function readPublicContentDetail(request,contentNo){
 const key=String(contentNo||'');
 if(key.startsWith(announcementPrefix)){
  const id=key.slice(announcementPrefix.length);
  if(!/^[1-9]\d*$/u.test(id))throw new Error('invalid_public_announcement');
  return {item:announcementItem(await request(announcementsBase+'/'+encodeURIComponent(id)))};
 }
 return request(contentBase+'/items/'+encodeURIComponent(key));
}
