import { test, expect } from '@host-tests/fixtures/auth';
import { ensureSourcePluginEnabled } from '@host-tests/fixtures/plugin';
import { PostPage } from '../../pages/PostPage';

test.describe('TC001 岗位管理 CRUD', () => {
  test.beforeEach(async ({ adminPage }) => {
    await ensureSourcePluginEnabled(adminPage, 'org-core');
  });

  const testPostCode = `TEST_POST_${Date.now()}`;
  const testPostName = '测试岗位';
  const testPostRenamed = '测试岗位修改';
  const testDept = 'LinaPro.AI';

  test('TC001a: 创建新岗位', async ({ adminPage }) => {
    const postPage = new PostPage(adminPage);
    await postPage.goto();
    await postPage.createPost(testDept, testPostCode, testPostName);

    await expect(adminPage.getByText(/创建成功|success/i)).toBeVisible({
      timeout: 5000,
    });
  });

  test('TC001b: 岗位列表中可见新创建的岗位', async ({ adminPage }) => {
    const postPage = new PostPage(adminPage);
    await postPage.goto();

    const hasPost = await postPage.hasPost(testPostCode);
    expect(hasPost).toBeTruthy();
  });

  test('TC001c: 编辑岗位', async ({ adminPage }) => {
    const postPage = new PostPage(adminPage);
    await postPage.goto();
    await postPage.editPost(testPostCode, testPostRenamed);

    await expect(adminPage.getByText(/更新成功|success/i)).toBeVisible({
      timeout: 5000,
    });
  });

  test('TC001d: 删除岗位', async ({ adminPage }) => {
    const postPage = new PostPage(adminPage);
    await postPage.goto();
    await postPage.deletePost(testPostCode);

    await expect(adminPage.getByText(/删除成功|success/i)).toBeVisible({
      timeout: 5000,
    });
  });
});

