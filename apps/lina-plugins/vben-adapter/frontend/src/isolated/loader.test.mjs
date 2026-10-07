import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { createContext, SourceTextModule } from 'node:vm';
import ts from 'typescript';

// Execute the actual loader source with only its remote module boundary replaced.
// No browser/Vben behavior is simulated or claimed by this lifecycle test.
async function fixture() {
  const events = [];
  const imports = [];
  let resolveImport;
  let rejectImport;
  const context = createContext({ events });
  const runtime = new SourceTextModule(`
    function mount(kind, container, options) {
      events.push({kind, options});
      return { ready:Promise.resolve(), update(next){events.push({next});}, close(){events.push({closed:true});}, destroy(){events.push({destroyed:true});} };
    }
    export function mountIsolatedVbenTable(container,options){return mount('table',container,options);}
    export function mountIsolatedVbenFormDrawer(container,options){return mount('drawer',container,options);}
  `, { context });
  await runtime.link(() => { throw new Error('unexpected stub import'); });
  await runtime.evaluate();
  const source = readFileSync(new URL('./loader.ts', import.meta.url), 'utf8');
  const code = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } }).outputText;
  const loader = new SourceTextModule(code, { context, importModuleDynamically(url) {
    imports.push(url);
    return new Promise((resolve, reject) => { resolveImport = () => resolve(runtime); rejectImport = reject; });
  } });
  await loader.link(() => { throw new Error('public loader must have no static runtime dependency'); });
  await loader.evaluate();
  return { api: loader.namespace, events, imports, resolve: () => resolveImport(), reject: () => rejectImport(new Error('network unavailable')) };
}

test('table and drawer share one import, and updates before ready use latest values', async () => {
  const f = await fixture();
  const table = f.api.mountIsolatedVbenTable({}, { rows: [] });
  const drawer = f.api.mountIsolatedVbenFormDrawer({}, { title: 'old', values: {} });
  drawer.update({ title: 'new', values: { name: 'new input' } });
  assert.equal(f.imports.length, 1);
  f.resolve();
  await Promise.all([table.ready, drawer.ready]);
  assert.equal(f.events[1].options.title, 'new');
  assert.equal(f.events[1].options.values.name, 'new input');
  drawer.close();
  table.destroy();
  assert.equal(f.events.at(-2).closed, true);
  assert.equal(f.events.at(-1).destroyed, true);
});

test('closed or destroyed drawer never mounts when a delayed runtime arrives', async () => {
  const f = await fixture();
  let closed = 0;
  const first = f.api.mountIsolatedVbenFormDrawer({}, { onClosed: () => closed++ });
  const second = f.api.mountIsolatedVbenFormDrawer({}, {});
  first.close();
  second.destroy();
  f.resolve();
  await Promise.all([first.ready, second.ready]);
  assert.equal(closed, 1);
  assert.equal(f.events.length, 0);
});

test('failed shared import rejects both surfaces and a later open retries once', async () => {
  const f = await fixture();
  const first = f.api.mountIsolatedVbenFormDrawer({}, {});
  const second = f.api.mountIsolatedVbenTable({}, {});
  const rejected = Promise.all([assert.rejects(first.ready, /network unavailable/), assert.rejects(second.ready, /network unavailable/)]);
  f.reject();
  await rejected;
  const third = f.api.mountIsolatedVbenFormDrawer({}, { title: 'retry' });
  assert.equal(f.imports.length, 2);
  assert.equal(f.imports[1], '/x-assets/vben-adapter/v0.1.0/isolated/runtime.js?retry=1');
  f.resolve();
  await third.ready;
  assert.equal(f.events.length, 1);
  assert.equal(f.events[0].options.title, 'retry');
});
