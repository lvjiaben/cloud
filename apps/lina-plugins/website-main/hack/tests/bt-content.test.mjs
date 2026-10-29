import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {parseHTML} from 'linkedom';
import {btContent,btHTML} from '../../frontend/public/lina/bt-content.js';
import {collectBtCopy,canonicalBtCopy} from './support/bt-copy.mjs';
const source=await readFile(new URL('../../docs/bt-reference/attachment.html',import.meta.url));
const attachment=parseHTML('<!doctype html><html>'+source.toString().split(String.fromCharCode(13)).join('')+'</html>').document;
const actual=parseHTML('<html><body><div data-bt>'+btHTML+'</div></body></html>').document;
test('BT: attachment hash, 69 text records, paths, order, levels and states',()=>{
 // Git normalizes the original Windows attachment to LF on Unix checkouts.
 const originalWindowsBytes=Buffer.from(source.toString().replace(/\r?\n/g,'\r\n'));
 assert.ok([source,originalWindowsBytes].some(bytes=>createHash('sha256').update(bytes).digest('hex')===btContent.source.sha256));
 const expected=collectBtCopy(attachment);assert.equal(expected.length,69);assert.deepEqual(canonicalBtCopy(btContent.copy),canonicalBtCopy(expected));assert.deepEqual(canonicalBtCopy(collectBtCopy(actual)),canonicalBtCopy(expected));
});
test('BT: seven exact display and clipboard payloads, including newlines',()=>{
 const expected=[...attachment.querySelectorAll('main .copyBtn')].map((button,order)=>({order,title:button.parentElement.querySelector('h2').textContent,displayText:button.previousElementSibling.textContent,copyText:button.getAttribute('data-clipboard-text')}));
 assert.equal(expected.length,7);assert.deepEqual(btContent.commands,expected);
 assert.deepEqual([...actual.querySelectorAll('.copyBtn')].map(b=>b.getAttribute('data-clipboard-text')),expected.map(c=>c.copyText));
 assert.equal(expected.filter(c=>c.copyText.includes('59d9cde77')).length,5);
});
test('BT: local display assets, permitted technical links and section structure',async()=>{
 const assets=JSON.parse(await readFile(new URL('../../docs/bt-images.json',import.meta.url)));
 for(const asset of assets){const data=await readFile(new URL('../../frontend/public'+asset.local,import.meta.url));assert.equal(createHash('sha256').update(data).digest('hex'),asset.sha256);}
 assert.equal(assets.length,9);assert.equal(actual.querySelectorAll('main>.et-slide').length,5);
 assert.equal(actual.querySelectorAll('.command .container-item>h2').length,7);
 assert.ok([...actual.querySelectorAll('img')].every(img=>img.getAttribute('src').startsWith('/lina/bt-assets/')));
 assert.ok(actual.querySelector('a[href="https://demo.bt.cn/login"]'));
 assert.equal(actual.querySelectorAll('a[href^="https://docs.qq.com/"]').length,2);
 assert.doesNotMatch(btHTML,/xiaobaiyun|小白云|白猿|400-8876675/i);
});
