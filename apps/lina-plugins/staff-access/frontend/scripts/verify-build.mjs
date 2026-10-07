import { access, readFile } from 'node:fs/promises';
const entry = new URL('../dist/mount.js', import.meta.url); await access(entry);
const source = await readFile(entry, 'utf8');
if (/\bprocess(?:\.|\[)/u.test(source)) throw new Error('Browser bundle must not depend on process.');
const module = await import(`${entry.href}?verify=${Date.now()}`);
if (typeof module.mount !== 'function') throw new Error('Plugin bundle must export mount(context).');
