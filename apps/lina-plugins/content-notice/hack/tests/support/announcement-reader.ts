import type { APIRequestContext } from "@host-tests/support/api/job";

import {
  createAdminApiContext,
  createApiContext,
  createUser,
  deleteUser,
  expectSuccess,
} from "@host-tests/support/api/job";

const recipientPassword = "test123456";

export type AnnouncementReader = {
  api: APIRequestContext;
  cleanup: () => Promise<void>;
  password: string;
  username: string;
};

function normalizeLabel(label: string) {
  return label
    .replaceAll(/[^a-z0-9]+/gi, "_")
    .replace(/^_+|_+$/g, "")
    .toLowerCase();
}

export async function createAnnouncementReader(
  label: string,
): Promise<AnnouncementReader> {
  const adminApi = await createAdminApiContext();
  const username = `e2e_notice_${normalizeLabel(label)}_${Date.now()}_${Math.random()
    .toString(36)
    .slice(2, 8)}`;
  let userID = 0;
  let userApi: APIRequestContext | undefined;

  try {
    userID = (
      await createUser(adminApi, {
        username,
        password: recipientPassword,
        nickname: "E2E Notice Recipient",
      })
    ).id;
    userApi = await createApiContext(username, recipientPassword);
  } catch (error) {
    await userApi?.dispose().catch(() => {});
    if (userID > 0) {
      await deleteUser(adminApi, userID).catch(() => {});
    }
    await adminApi.dispose();
    throw error;
  }

  let cleaned = false;

  return {
    api: userApi,
    password: recipientPassword,
    username,
    cleanup: async () => {
      if (cleaned) {
        return;
      }
      cleaned = true;
      await userApi.post("auth/logout").catch(() => {});
      await userApi.dispose().catch(() => {});
      if (userID > 0) {
        await deleteUser(adminApi, userID).catch(() => {});
      }
      await adminApi.dispose();
    },
  };
}

