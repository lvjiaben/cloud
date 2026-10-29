// Standard Node/pnpm build; all inputs belong to this independent plugin.
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
const cwd = fileURLToPath(new URL('..', import.meta.url));
for (const args of [['install', '--ignore-workspace', '--frozen-lockfile'], ['run', 'build']]) {
  const child = spawnSync(process.platform === 'win32' ? 'pnpm.cmd' : 'pnpm', args, {cwd, stdio: 'inherit', shell: process.platform === 'win32'});
  if (child.status !== 0) process.exit(child.status ?? 1);
}
