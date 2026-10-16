// Install and build solely from this plugin's own locked workspace.
import './sync-locales.mjs';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
const app = resolve(dirname(fileURLToPath(import.meta.url)), '../frontend/customer');
const pnpm = process.platform === 'win32' ? 'pnpm.cmd' : 'pnpm';
for (const args of [['install', '--frozen-lockfile'], ['run', 'build']]) {
 const result=spawnSync(pnpm,args,{cwd:app,stdio:'inherit',env:process.env,shell:process.platform==='win32'});
 if(result.status!==0)process.exit(result.status??1);
}
