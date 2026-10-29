import {spawnSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {buildFailure} from './build-result.mjs';
const result=spawnSync('go',['test','./...','-count=1'],{cwd:fileURLToPath(new URL('..',import.meta.url)),env:{...process.env,GOWORK:'off'},stdio:'inherit'});
const failure=buildFailure(result,'go test ./... -count=1',fileURLToPath(new URL('..',import.meta.url)));
if(failure){console.error(failure);process.exit(Number.isInteger(result.status)&&result.status>0?result.status:1);}
process.exit(result.status??1);
