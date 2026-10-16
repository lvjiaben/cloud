// Validate every published navigation page against a real Nuxt page implementation.
import {readFile,access,writeFile} from 'node:fs/promises';
const manifest=await readFile(new URL('../../../plugin.yaml',import.meta.url),'utf8');
const declared=[...manifest.matchAll(/- \{ id: ([^,]+), nameZh: ([^,]+), nameEn: ([^,]+), path: ([^ }]+) \}/gu)].map(match=>({id:match[1],nameZh:match[2],nameEn:match[3],path:match[4]}));
if(!declared.length)throw new Error('No published template pages');
for(const page of declared){
 const route=page.path==='/'?'index':page.path.slice(1)+'/index';
 await access(new URL('../app/pages/'+route+'.vue',import.meta.url)).catch(()=>{throw new Error('Declared navigation page has no implementation: '+page.path);});
}
await writeFile(new URL('../public-pages.json',import.meta.url),JSON.stringify(declared,null,2)+'\n');
console.log('Verified '+declared.length+' implemented navigation pages');
