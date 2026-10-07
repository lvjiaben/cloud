import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { inflateRawSync } from 'node:zlib';
import { validateSiteBrandConfig, validateSitePackageManifest, SITE_MANIFEST_FILE, type SiteBrandConfig, type SitePackageManifest } from '@lina/site-contracts';

const MAX_FILES = 20_000;
const MAX_UNCOMPRESSED = 512 * 1024 * 1024;

interface Entry { path: string; data: Buffer; mode: number; }

function readZip(file: string): Entry[] {
  const input = readFileSync(file);
  const eocd = input.lastIndexOf(Buffer.from([0x50, 0x4b, 0x05, 0x06]));
  if (eocd < 0) throw new Error('not a zip archive');
  const count = input.readUInt16LE(eocd + 10);
  const centralSize = input.readUInt32LE(eocd + 12);
  const centralOffset = input.readUInt32LE(eocd + 16);
  if (count > MAX_FILES || centralOffset + centralSize > input.length) throw new Error('zip limits exceeded');
  const entries: Entry[] = [];
  const names = new Set<string>();
  let cursor = centralOffset;
  let total = 0;
  for (let i = 0; i < count; i++) {
    if (input.readUInt32LE(cursor) !== 0x02014b50) throw new Error('invalid zip central directory');
    const flags = input.readUInt16LE(cursor + 8);
    const method = input.readUInt16LE(cursor + 10);
    const compressedSize = input.readUInt32LE(cursor + 20);
    const size = input.readUInt32LE(cursor + 24);
    const nameLen = input.readUInt16LE(cursor + 28);
    const extraLen = input.readUInt16LE(cursor + 30);
    const commentLen = input.readUInt16LE(cursor + 32);
    const external = input.readUInt32LE(cursor + 38);
    const name = input.subarray(cursor + 46, cursor + 46 + nameLen).toString('utf8');
    if (flags & 0x1) throw new Error(`encrypted entry rejected: ${name}`);
    if (!name || name.startsWith('/') || name.includes('\\') || name.split('/').includes('..')) throw new Error(`unsafe path: ${name}`);
    if (names.has(name)) throw new Error(`duplicate entry rejected: ${name}`);
    names.add(name);
    const mode = external >>> 16;
    if ((mode & 0o170000) === 0o120000) throw new Error(`symbolic link rejected: ${name}`);
    const localOffset = input.readUInt32LE(cursor + 42);
    if (localOffset + 30 > input.length || input.readUInt32LE(localOffset) !== 0x04034b50) throw new Error(`invalid local entry: ${name}`);
    const localNameLen = input.readUInt16LE(localOffset + 26);
    const localExtraLen = input.readUInt16LE(localOffset + 28);
    const start = localOffset + 30 + localNameLen + localExtraLen;
    const end = start + compressedSize;
    if (end > input.length) throw new Error(`truncated entry: ${name}`);
    if (size > MAX_UNCOMPRESSED || (total += size) > MAX_UNCOMPRESSED) throw new Error('uncompressed size limit exceeded');
    let data: Buffer;
    if (method === 0) data = input.subarray(start, end);
    else if (method === 8) data = inflateRawSync(input.subarray(start, end));
    else throw new Error(`compression method not supported: ${method}`);
    if (data.length !== size) throw new Error(`size mismatch: ${name}`);
    entries.push({ path: name, data, mode });
    cursor += 46 + nameLen + extraLen + commentLen;
  }
  return entries;
}

function validateEntries(entries: Entry[]): { manifest: SitePackageManifest; files: Entry[] } {
  const manifestEntry = entries.find((entry) => entry.path === SITE_MANIFEST_FILE);
  if (!manifestEntry) throw new Error(`missing ${SITE_MANIFEST_FILE}`);
  const parsed = validateSitePackageManifest(JSON.parse(manifestEntry.data.toString('utf8')));
  if (!parsed.ok) throw new Error(`manifest rejected: ${parsed.errors.join('; ')}`);
  const byPath = new Map(entries.map((entry) => [entry.path, entry]));
  for (const file of parsed.value.files) {
    const entry = byPath.get(file.path);
    if (!entry) throw new Error(`manifest file missing: ${file.path}`);
    if (entry.data.length !== file.size) throw new Error(`manifest size mismatch: ${file.path}`);
    const digest = createHash('sha256').update(entry.data).digest('hex');
    if (digest !== file.sha256) throw new Error(`manifest digest mismatch: ${file.path}`);
  }
  if (!parsed.value.publisher.signature || !parsed.value.publisher.keyId) throw new Error('publisher signature metadata is required');
  return { manifest: parsed.value, files: entries };
}

function storage(root = process.env.LINA_SITE_STORAGE ?? '.site-runtime') { return resolve(root); }
function writeAtomic(path: string, value: string) { const tmp = `${path}.tmp-${process.pid}`; writeFileSync(tmp, value, 'utf8'); renameSync(tmp, path); }

function install(archive: string, root = storage()): SitePackageManifest {
  const checked = validateEntries(readZip(archive));
  const versionKey = `${checked.manifest.packageId}/${checked.manifest.version}-${createHash('sha256').update(readFileSync(archive)).digest('hex').slice(0, 16)}`;
  const target = join(root, 'versions', versionKey);
  if (existsSync(target)) return checked.manifest;
  mkdirSync(target, { recursive: true });
  for (const entry of checked.files) {
    const out = join(target, entry.path);
    mkdirSync(resolve(out, '..'), { recursive: true });
    writeFileSync(out, entry.data, { mode: entry.mode & 0o777 || 0o644 });
  }
  writeAtomic(join(target, '.verified'), JSON.stringify({ verifiedAt: Date.now(), manifest: checked.manifest }));
  return checked.manifest;
}

function pointer(root: string) { return join(root, 'current.json'); }
function publish(packageId: string, version: string, root = storage()) {
  const versionsRoot = join(root, 'versions', packageId);
  if (!existsSync(versionsRoot)) throw new Error('package is not installed');
  const candidate = resolve(versionsRoot, version);
  if (!existsSync(join(candidate, '.verified'))) throw new Error('only verified packages can be published');
  const current = existsSync(pointer(root)) ? JSON.parse(readFileSync(pointer(root), 'utf8')) : null;
  writeAtomic(pointer(root), JSON.stringify({ packageId, version, previous: current, publishedAt: Date.now() }, null, 2));
}
function rollback(root = storage()) {
  if (!existsSync(pointer(root))) throw new Error('no published package');
  const current = JSON.parse(readFileSync(pointer(root), 'utf8'));
  if (!current.previous) throw new Error('no previous package to roll back to');
  writeAtomic(pointer(root), JSON.stringify(current.previous, null, 2));
}
function uninstall(packageId: string, root = storage()) {
  const current = existsSync(pointer(root)) ? JSON.parse(readFileSync(pointer(root), 'utf8')) : null;
  if (current?.packageId === packageId) throw new Error('cannot uninstall the current package; publish another version or roll back first');
  rmSync(join(root, 'versions', packageId), { recursive: true, force: true });
}

function brandPath(root: string) { return join(root, 'brand', 'current.json'); }
function saveBrand(file: string, root = storage()): SiteBrandConfig {
  const parsed = validateSiteBrandConfig(JSON.parse(readFileSync(file, 'utf8')));
  if (!parsed.ok) throw new Error(`brand config rejected: ${parsed.errors.join('; ')}`);
  const revisionDir = join(root, 'brand', 'revisions');
  mkdirSync(revisionDir, { recursive: true });
  const target = join(revisionDir, `${parsed.value.revision}.json`);
  if (!existsSync(target)) writeFileSync(target, JSON.stringify(parsed.value, null, 2));
  return parsed.value;
}
function publishBrand(revision: string, root = storage()) {
  const source = join(root, 'brand', 'revisions', `${revision}.json`);
  if (!existsSync(source)) throw new Error('brand revision is not staged');
  const previous = existsSync(brandPath(root)) ? JSON.parse(readFileSync(brandPath(root), 'utf8')) : null;
  writeAtomic(brandPath(root), JSON.stringify({ ...JSON.parse(readFileSync(source, 'utf8')), previous }, null, 2));
}
function rollbackBrand(root = storage()) {
  if (!existsSync(brandPath(root))) throw new Error('no published brand');
  const current = JSON.parse(readFileSync(brandPath(root), 'utf8'));
  if (!current.previous) throw new Error('no previous brand revision');
  writeAtomic(brandPath(root), JSON.stringify(current.previous, null, 2));
}

function main(args: string[]) {
  const root = storage();
  mkdirSync(root, { recursive: true });
  const [command, ...rest] = args;
  if (command === 'install') return console.log(JSON.stringify(install(rest[0], root)));
  if (command === 'publish') { publish(rest[0], rest[1], root); return console.log('published'); }
  if (command === 'rollback') { rollback(root); return console.log('rolled back'); }
  if (command === 'uninstall') { uninstall(rest[0], root); return console.log('uninstalled'); }
  if (command === 'brand') {
    const action = rest[0];
    if (action === 'save') return console.log(JSON.stringify(saveBrand(rest[1], root)));
    if (action === 'preview') return console.log(readFileSync(join(root, 'brand', 'revisions', `${rest[1]}.json`), 'utf8'));
    if (action === 'publish') { publishBrand(rest[1], root); return console.log('brand published'); }
    if (action === 'rollback') { rollbackBrand(root); return console.log('brand rolled back'); }
  }
  if (command === 'preview') { const current = JSON.parse(readFileSync(pointer(root), 'utf8')); return console.log(JSON.stringify(current)); }
  if (command === 'self-test') {
    const rejected = validateSitePackageManifest({ schemaVersion: 1, routes: { public: ['/admin'] } });
    if (rejected.ok) throw new Error('self-test expected unsafe manifest to be rejected');
    if (!rejected.errors.some((error) => error.includes('route is not allowed'))) throw new Error('self-test did not report unsafe route');
    return console.log('site-packager validation ready');
  }
  throw new Error('usage: lina-site install <zip> | publish <packageId> <version> | rollback | uninstall <packageId> | preview | brand save|preview|publish|rollback');
}
try { main(process.argv.slice(2)); } catch (error) { console.error(error instanceof Error ? error.message : error); process.exitCode = 1; }
