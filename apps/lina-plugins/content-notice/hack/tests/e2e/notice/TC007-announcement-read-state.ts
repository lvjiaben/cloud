import { test, expect } from '@host-tests/fixtures/auth';
import { pluginApiPath } from '@host-tests/fixtures/config';
import { createAdminApiContext, expectSuccess } from '@host-tests/support/api/job';
const path = (suffix: string) => pluginApiPath('content-notice', suffix);
import { createAnnouncementReader } from '../../support/announcement-reader';

test.describe('TC-7 公告已读状态', () => {
  test('TC-7a: 阅读幂等持久化且不影响其他用户', async () => {
    const api = await createAdminApiContext();
    let first: Awaited<ReturnType<typeof createAnnouncementReader>> | undefined;
    let second: Awaited<ReturnType<typeof createAnnouncementReader>> | undefined;
    let id = 0;
    try {
      first = await createAnnouncementReader('first');
      second = await createAnnouncementReader('second');
      id = (await expectSuccess<{id:number}>(await api.post(path('notice'), {data:{title:`阅读公告_${Date.now()}`,content:'<p>阅读正文</p>',status:1}}))).id;
      const detail = async (reader: Awaited<ReturnType<typeof createAnnouncementReader>>) => expectSuccess<{read:boolean}>(await reader.api.get(path(`notice/announcements/${id}`)));
      expect((await detail(first)).read).toBe(false);
      await expectSuccess(await first.api.put(path(`notice/announcements/${id}/read`)));
      await expectSuccess(await first.api.put(path(`notice/announcements/${id}/read`)));
      expect((await detail(first)).read).toBe(true);
      expect((await detail(second)).read).toBe(false);
    } finally {
      if (id) await expectSuccess(await api.delete(path(`notice?ids[]=${id}`)));
      await first?.cleanup();
      await second?.cleanup();
      await api.dispose();
    }
  });
});
