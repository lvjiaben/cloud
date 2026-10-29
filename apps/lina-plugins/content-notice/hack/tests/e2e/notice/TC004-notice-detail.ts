import { test, expect } from '@host-tests/fixtures/auth';
import { pluginApiPath } from '@host-tests/fixtures/config';
import { createAdminApiContext, expectSuccess } from '@host-tests/support/api/job';
const path = (suffix: string) => pluginApiPath('content-notice', suffix);
import { NoticePage } from '../../pages/NoticePage';

test.describe('TC-4 公告预览', () => {
  test('TC-4a: 标题正文图片来自公告详情', async ({adminPage}) => {
    const api = await createAdminApiContext();
    let id = 0;
    const title = `预览公告_${Date.now()}`;
    try {
      id = (await expectSuccess<{id:number}>(await api.post(path('notice'), {data:{title, content:'<p>公告预览正文</p><img src="/favicon.ico" alt="公告图片">', status:1}}))).id;
      const notice = new NoticePage(adminPage);
      await notice.goto();
      await notice.previewNotice(title);
      await expect(notice.modal.getByText(title, {exact:true})).toBeVisible();
      await expect(notice.modal.getByText('公告预览正文', {exact:true})).toBeVisible();
      await expect(notice.modal.getByAltText('公告图片')).toBeVisible();
    } finally {
      if (id) await expectSuccess(await api.delete(path(`notice?ids[]=${id}`)));
      await api.dispose();
    }
  });
});
