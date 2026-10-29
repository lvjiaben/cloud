import {test} from 'node:test';
import assert from 'node:assert/strict';
import {buildFailure} from '../../scripts/build-result.mjs';

test('reports executable lookup failures without calling them compilation failures',()=>{
 const error=Object.assign(new Error('spawnSync go ENOENT'),{code:'ENOENT',syscall:'spawnSync go',path:'go'});
 assert.match(buildFailure({error,status:null,signal:null},'go build','plugin-root'),/Failed to start.*ENOENT.*spawnSync go.*path: go/);
});
test('preserves denied launch details without guessing the policy cause',()=>{
 const error=Object.assign(new Error('access denied'),{code:'EACCES'});
 assert.match(buildFailure({error,status:null},'go build','plugin-root'),/Failed to start.*EACCES.*access denied/);
});
test('distinguishes process signal, nonzero exit and success',()=>{
 assert.match(buildFailure({status:null,signal:'SIGTERM'},'go build','plugin-root'),/terminated by signal SIGTERM/);
 assert.match(buildFailure({status:2},'go build','plugin-root'),/exited with status 2/);
 assert.equal(buildFailure({status:0},'go build','plugin-root'),'');
});
