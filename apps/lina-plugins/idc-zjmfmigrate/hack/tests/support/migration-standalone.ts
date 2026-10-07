import { readFileSync, readdirSync } from "node:fs";
import type { Page } from "@host-tests/support/playwright";

/** Deliver unchanged production bytes inside an opaque iframe; only the host bridge is a fixture. */
export async function openMigrationStandalone(page: Page) {
  const runtime = new URL("../../../../vben-adapter/frontend/dist/isolated/", import.meta.url);
  const runtimeNames = new Set(readdirSync(runtime).filter((name) => /^[\w.-]+\.(css|js)$/.test(name)));
  await page.route("**/x-assets/vben-adapter/v0.1.0/isolated/*", (route) => {
    const name = new URL(route.request().url()).pathname.split("/").at(-1) ?? "";
    if (!runtimeNames.has(name)) return route.abort();
    return route.fulfill({ headers: { "Access-Control-Allow-Origin": "*" }, contentType: name.endsWith(".css") ? "text/css" : "text/javascript", body: readFileSync(new URL(name, runtime)) });
  });
  const entry = new URL("../../../frontend/dist/admin/migration/", import.meta.url);
  await page.route("**/x-assets/idc-zjmfmigrate/v0.2.1/admin/migration/*", (route) => {
    const name = new URL(route.request().url()).pathname.split("/").at(-1) ?? "";
    if (!["index.html", "app.js", "style.css"].includes(name)) return route.abort();
    return route.fulfill({ headers: { "Access-Control-Allow-Origin": "*" }, contentType: name.endsWith(".html") ? "text/html" : name.endsWith(".css") ? "text/css" : "text/javascript", body: readFileSync(new URL(name, entry)) });
  });
  await page.route("**/__migration-standalone-test", (route) => route.fulfill({ contentType: "text/html", body: `<!doctype html><html><body style="margin:0"><iframe id="migration" style="width:100%;height:1200px;border:0" sandbox="allow-downloads allow-forms allow-modals allow-popups allow-scripts" src="/x-assets/idc-zjmfmigrate/v0.2.1/admin/migration/index.html"></iframe><script>
    const frame=document.getElementById('migration');
    const reply=(message,result)=>frame.contentWindow.postMessage({...message,direction:'host-to-plugin',...result},'*');
    window.addEventListener('message',async(event)=>{
      if(event.source!==frame.contentWindow)return;
      const message=event.data;
      if(message?.protocol!=='linapro.plugin-page-bridge'||message.direction!=='plugin-to-host')return;
      if(message.method==='ready'){reply({...message,capability:'host.context.v1'},{payload:{locale:'zh-CN'}});return;}
      if(message.capability!=='idc-migration.admin.v1'||message.method!=='request')return;
      const payload=message.payload;
      const url=new URL('/x/idc-zjmfmigrate/api/v1/'+payload.path,location.origin);
      Object.entries(payload.params||{}).forEach(([key,value])=>url.searchParams.set(key,String(value)));
      try{const response=await fetch(url,{method:payload.method,headers:{'Content-Type':'application/json'},body:payload.body?JSON.stringify(payload.body):undefined});
        const result=await response.json(); if(!response.ok)throw Error('bridge_request_failed'); reply(message,{ok:true,result:result.data});
      }catch(error){reply(message,{ok:false,error:{code:'bridge_request_failed'}});}
    });
  </script></body></html>` }));
  await page.goto("/__migration-standalone-test", { waitUntil: "domcontentloaded" });
}
