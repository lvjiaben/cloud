import { test, expect } from '@host-tests/fixtures/auth';
import { pluginApiPath } from '@host-tests/fixtures/config';
import { createAdminApiContext, expectSuccess } from '@host-tests/support/api/job';
const path = (suffix: string) => pluginApiPath('content-notice', suffix);

test.describe('TC-3 公告发布生命周期', () => {
  test('TC-3a: 草稿不可读、发布可读、撤回与删除后旧地址失效', async () => {
    const api = await createAdminApiContext();
    let id = 0;
    try {
      const title = `发布公告_${Date.now()}`;
      id = (await expectSuccess<{id: number}>(await api.post(path('notice'), {data: {title, content: '<p>发布正文</p>', status: 0, sortOrder: 0}}))).id;
      const read = () => api.get(path(`public/announcements/${id}`));
      expect(String((await (await read()).json()).code)).not.toBe('0');
      await expectSuccess(await api.put(path(`notice/${id}`), {data: {status: 1}}));
      expect((await expectSuccess<{title: string}>(await read())).title).toBe(title);
      await expectSuccess(await api.put(path(`notice/${id}`), {data: {status: 0}}));
      expect(String((await (await read()).json()).code)).not.toBe('0');
      await expectSuccess(await api.delete(path(`notice?ids[]=${id}`)));
      expect(String((await (await read()).json()).code)).not.toBe('0');
      id = 0;
    } finally {
      if (id) await expectSuccess(await api.delete(path(`notice?ids[]=${id}`)));
      await api.dispose();
    }
  });
});
