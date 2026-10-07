import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

function findRepositoryRoot() {
  let current = dirname(fileURLToPath(import.meta.url));
  while (dirname(current) !== current) {
    if (
      statSafe(join(current, 'AGENTS.md')) &&
      statSafe(join(current, 'apps', 'lina-plugins'))
    ) {
      return current;
    }
    current = dirname(current);
  }
  throw new Error('repository root not found');
}

function statSafe(path: string) {
  try {
    return statSync(path);
  } catch {
    return undefined;
  }
}

function collectVueFiles(root: string): string[] {
  if (!statSafe(root)) return [];
  return readdirSync(root, { withFileTypes: true }).flatMap((entry) => {
    const path = join(root, entry.name);
    if (entry.isDirectory()) return collectVueFiles(path);
    return entry.isFile() && entry.name.endsWith('.vue') ? [path] : [];
  });
}

describe('business table governance', () => {
  it('keeps every reachable business view on the wrapped Vben grid', () => {
    const repositoryRoot = findRepositoryRoot();
    const pluginRoot = join(repositoryRoot, 'apps', 'lina-plugins');
    const pluginPageRoots = readdirSync(pluginRoot, { withFileTypes: true })
      .filter((entry) => entry.isDirectory())
      .map((entry) => join(pluginRoot, entry.name, 'frontend', 'pages'));
    const roots = [
      ...pluginPageRoots,
      resolve(repositoryRoot, 'apps/lina-vben/apps/web-antd/src/views'),
      resolve(repositoryRoot, 'apps/lina-vben/apps/web-customer/src/views'),
    ];
    const violations = roots
      .flatMap(collectVueFiles)
      .filter((path) =>
        /<(?:table\b|a-table\b|Table\b|ATable\b)/i.test(
          readFileSync(path, 'utf8'),
        ),
      )
      .map((path) =>
        path.slice(repositoryRoot.length + 1).replaceAll('\\', '/'),
      );

    expect(violations).toEqual([]);
  });
});
