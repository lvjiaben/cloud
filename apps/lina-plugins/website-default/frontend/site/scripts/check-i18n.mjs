// This cross-platform check keeps public bilingual components structurally paired.
import { readdir, readFile } from 'node:fs/promises';
import { extname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../app/components/public/', import.meta.url));
const failures = [];

async function visit(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      await visit(path);
      continue;
    }
    if (!['.ts', '.vue'].includes(extname(entry.name))) continue;
    const source = await readFile(path, 'utf8');
    const chineseSections = source.match(/["']zh-CN["']\s*:\s*\{/gu)?.length || 0;
    const englishSections = source.match(/["']en-US["']\s*:\s*\{/gu)?.length || 0;
    if (chineseSections !== englishSections) {
      failures.push(`${entry.name}: zh-CN=${chineseSections}, en-US=${englishSections}`);
    }
  }
}

await visit(root);
if (failures.length) {
  throw new Error(`Bilingual section mismatch:\n${failures.join('\n')}`);
}
console.log('default website bilingual section check passed');
