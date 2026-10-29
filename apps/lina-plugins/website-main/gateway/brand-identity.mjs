// Shared, bounded public first-paint hints. The browser still checks the authoritative API.
import {readFile,writeFile,mkdir,rename,unlink,stat,open} from 'node:fs/promises';
import {resolve,join} from 'node:path';
import {createHash,randomUUID} from 'node:crypto';

const maxAge=30000;
const fields=['siteName','footer','companyName','icpNumber','icpUrl','policeNumber','policeUrl','telecomLicenseNumber','telecomLicenseUrl','logoUrl','headerLogoUrl','footerLogoUrl','copyright','businessQq','companyAddress','contactPhone','contactEmail','enterpriseWechatQrCodeUrl','wechatOfficialQrCodeUrl'];
const text=value=>typeof value==='string'&&value.length<=4096?value:'';
const project=brand=>Object.fromEntries(fields.map(key=>[key,['siteName','footer'].includes(key)?{zhCN:text(brand[key]?.zhCN),enUS:text(brand[key]?.enUS)}:text(brand[key])]));

export function createBrandIdentityCache({directory,backendUrl,now=Date.now,report=error=>console.warn('Public brand prewarm:',error.message)}){
 const root=resolve(directory),key=createHash('sha256').update(new URL(backendUrl).origin).digest('hex').slice(0,20);
 const file=join(root,key+'.json'),lock=join(root,key+'.lock');
 let pending;
 async function locked(path,action){
  await mkdir(root,{recursive:true});let handle;
  try{
   try{handle=await open(path,'wx');}catch(error){
    if(error.code!=='EEXIST')throw error;
    try{if(now()-(await stat(path)).mtimeMs>60000)await unlink(path);}catch(error){if(error.code!=='ENOENT')throw error;}
    return;
   }
   await action();
  }finally{if(handle){await handle.close();await unlink(path).catch(error=>{if(error.code!=='ENOENT')report(error);});}}
 }
 async function stored(){try{return JSON.parse(await readFile(file,'utf8'));}catch(error){if(error.code!=='ENOENT'&&!(error instanceof SyntaxError))report(error);return null;}}
 async function read(){const item=await stored();return item?.brand&&Number.isFinite(item.savedAt)&&now()>=item.savedAt&&now()-item.savedAt<=maxAge?item:null;}
 async function publish(brand,startedAt){
  let accepted=false;
  await locked(lock,async()=>{
   const previous=await stored();if(previous?.startedAt>startedAt)return;
   const identity=brand?project(brand):null;
   const item={startedAt,savedAt:now(),brand:identity};
   await save(item);
   accepted=true;
  });
  if(!accepted)return;
 }
 async function save(item){const temporary=file+'.'+randomUUID()+'.tmp';try{await writeFile(temporary,JSON.stringify(item));await rename(temporary,file);}finally{await unlink(temporary).catch(error=>{if(error.code!=='ENOENT')report(error);});}}
 async function refresh(){
  if(pending)return pending;
  pending=locked(join(root,key+'.refresh.lock'),async()=>{
   const existing=await stored();if(existing&&now()>=existing.savedAt&&now()-existing.savedAt<5000)return;
   const startedAt=now();
   const response=await fetch(new URL('/x/idc-portal/api/v1/public/brand',backendUrl),{headers:{Accept:'application/json'},signal:AbortSignal.timeout(10000),redirect:'error'});
   if(!response.ok)throw new Error('Brand authority returned HTTP '+response.status);
   const body=await response.text();if(body.length>2097152)throw new Error('Brand response too large');
   const envelope=JSON.parse(body);if(String(envelope.code??0)!=='0'||!envelope.data)throw new Error('Invalid brand response');
   await publish(envelope.data.brand||null,startedAt);
  }).catch(report).finally(()=>{pending=null;});return pending;
 }
 return {read,refresh,publish};
}
