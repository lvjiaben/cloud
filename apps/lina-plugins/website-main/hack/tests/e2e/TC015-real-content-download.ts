import {test,expect} from '@playwright/test';
import {createHash} from 'node:crypto';
import {readFile} from 'node:fs/promises';
import {WebsitePage} from '../pages/WebsitePage';

test.describe('TC-15 真实附件上传发布与下载',()=>{
 test('TC-15a: 后台真实文件经官网票据下载，字节哈希一致，下线后拒绝旧入口',async({page})=>{
  test.skip(process.env.WEBSITE_LOCAL_CHECKOUT!=='1','仅授权本地隔离测试');
  const backend=process.env.WEBSITE_API_ORIGIN!;
  expect(['localhost','127.0.0.1']).toContain(new URL(backend).hostname);
  expect(['16630','16634'],'只允许既有隔离后台，禁止当前6030业务库').toContain(new URL(backend).port);
  const login=await (await page.request.post(backend+'/api/v1/auth/login',{data:{username:process.env.WEBSITE_QA_ADMIN_USER,password:process.env.WEBSITE_QA_ADMIN_PASSWORD,clientType:'web'}})).json();
  expect(login.code).toBe(0);const headers={Authorization:'Bearer '+login.data.accessToken};
  async function call(path:string,method:string,data?:unknown){const result=await (await page.request.fetch(backend+'/x/idc-core/api/v1/admin/content/'+path,{method,headers,data})).json();expect(result.code,result.message).toBe(0);return result.data;}
  const fields={type:'download',visibility:'public',title:'隔离下载验收（测试专用）'+Date.now(),summary:'仅用于本地上传和下载哈希核对。',body:'<p>本地测试附件，不作正式内容。</p>',targetCustomerNos:[],targetServiceNos:[],targetCustomerGroupNos:[]};
  let item=(await call('items','POST',fields)).item;
  // A valid one-pixel PNG, uploaded to the real storage API; never packaged.
  const bytes=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a1ZkAAAAASUVORK5CYII=','base64');
  const upload=(await call('items/'+item.contentNo+'/upload-sessions','POST',{fileName:'local-download-test.png',sizeBytes:bytes.length,contentVersion:item.version})).item;
  let completed=false;
  try{
   const chunk=await (await page.request.put(backend+'/x/idc-core/api/v1/admin/content/upload-sessions/'+upload.uploadNo+'/chunks/0',{headers:{...headers,'Content-Type':'application/octet-stream'},data:bytes})).json();expect(chunk.code,chunk.message).toBe(0);
   const candidate=(await call('upload-sessions/'+upload.uploadNo+'/completion','POST',{expectedSizeBytes:bytes.length,version:chunk.data.version})).item;
   completed=true;
   expect(candidate.canEnable).toBe(true);
   item=(await call('items/'+item.contentNo,'GET')).item;
   item=(await call('items/'+item.contentNo+'/file-candidates/'+candidate.fileNo+'/enablements','POST',{contentVersion:item.version,fileVersion:candidate.version,confirmed:true})).item;
   item=(await call('items/'+item.contentNo+'/publications','POST',{version:item.version,confirmed:true})).item;
   const site=new WebsitePage(page);await site.open('/downloads.html?query='+encodeURIComponent(fields.title));
   await expect(page.locator('[data-content-list]')).toContainText(fields.title);
   await site.open('/content.html?id='+item.contentNo);
   await expect(page.getByRole('button',{name:'下载 local-download-test.png',exact:true})).toBeVisible();
   const downloadButton=page.getByRole('button',{name:'下载 local-download-test.png',exact:true});
   await expect(downloadButton).toHaveClass('fancy-button bg-gradient1');
   await expect(downloadButton.locator('span')).toHaveCSS('padding-left','28px');
   await expect(downloadButton.locator('span')).toHaveCSS('background-image',/linear-gradient/);
   await site.capture('real-download-ready',false);
   const pending=page.waitForEvent('download');await site.download();const download=await pending;
   const downloaded=await readFile((await download.path())!);
   expect(createHash('sha256').update(downloaded).digest('hex')).toBe(createHash('sha256').update(bytes).digest('hex'));
   expect(download.suggestedFilename()).toBe('local-download-test.png');
   item=(await call('items/'+item.contentNo+'/unpublications','POST',{version:item.version,confirmed:true})).item;
   await site.open('/content.html?id='+item.contentNo);
   await expect(page.locator('[data-status]')).toHaveText('内容加载失败，请重试。');
   await expect(page.getByRole('button',{name:'下载 local-download-test.png',exact:true})).toHaveCount(0);
  }finally{
   if(item.status==='published')await call('items/'+item.contentNo+'/unpublications','POST',{version:item.version,confirmed:true});
   if(!completed){const current=(await call('upload-sessions/'+upload.uploadNo,'GET')).item;await call('upload-sessions/'+upload.uploadNo+'/cancellation','POST',{version:current.version,confirmed:true});}
  }
 });
});
