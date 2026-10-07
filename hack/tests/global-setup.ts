import type { FullConfig } from '@playwright/test';

import { writeAdminStorageState } from './fixtures/auth-state';

export default async function globalSetup(config: FullConfig) {
  if (process.env.E2E_REUSE_AUTH_STATE === '1') {
    return;
  }
  const projectBaseURL = config.projects[0]?.use?.baseURL;
  const baseURL = typeof projectBaseURL === 'string' ? projectBaseURL : undefined;
  await writeAdminStorageState(baseURL);
}
