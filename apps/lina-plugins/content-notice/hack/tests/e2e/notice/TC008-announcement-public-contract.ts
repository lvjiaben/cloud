import { test, expect } from '@host-tests/fixtures/auth';
import { pluginApiPath } from '@host-tests/fixtures/config';
import { createAdminApiContext, expectSuccess } from '@host-tests/support/api/job';
const path = (suffix: string) => pluginApiPath('content-notice', suffix);

test.describe('TC-8 公告公共投影', () => {
  test('TC-8a: 公开详情不含废弃字段，危险HTML被清除', async ({request}) => {
    const api = await createAdminApiContext();
    let id=0;
    try {
      id=(await expectSuccess<{id:number}>(await api.post(path('notice'), {data:{title:`公告结构_${Date.now()}`,content:'<p>正文</p><img src="/favicon.ico" onerror="alert(1)"><script>alert(1)</script>',status:1}}))).id;
      const response=await request.get(path(`public/announcements/${id}`));
      const item=await expectSuccess<Record<string,unknown>>(response);
      expect(item).not.toHaveProperty('type');
      expect(item).not.toHaveProperty('fileIds');
      expect(item).not.toHaveProperty('visibility');
      expect(item).not.toHaveProperty('read');
      expect(String(item.content)).toContain('<img');
      expect(String(item.content)).not.toMatch(/script|onerror|alert/);
      expect((await request.get(path(`notice/announcements/${id}`))).status()).toBe(401);
    } finally {
      if(id) await expectSuccess(await api.delete(path(`notice?ids[]=${id}`)));
      await api.dispose();
    }
  });
});
