import { expect, test } from '@host-tests/fixtures/auth';
import { prepareSourcePluginsBaseline } from '@host-tests/fixtures/plugin';
import { DeptPage } from '../../pages/DeptPage';

test.describe('TC004 部门空树创建顶级部门', () => {
  test.beforeAll(async () => {
    await prepareSourcePluginsBaseline(['org-core']);
  });

  test('TC004a: 空部门树下可创建首个顶级部门', async ({ adminPage }) => {
    const suffix = Date.now();
    const deptName = `空树根部门_${suffix}`;
    const deptCode = `empty_root_${suffix}`;
    let createdParentId: number | undefined;

    await adminPage.route('**/x/org-core/api/v1/dept/tree', async (route) => {
      await route.fulfill({
        contentType: 'application/json',
        json: { code: 0, data: { list: [] }, message: 'success' },
        status: 200,
      });
    });
    await adminPage.route('**/x/org-core/api/v1/dept?**', async (route) => {
      await route.fulfill({
        contentType: 'application/json',
        json: { code: 0, data: { list: [] }, message: 'success' },
        status: 200,
      });
    });
    await adminPage.route('**/x/org-core/api/v1/dept', async (route, request) => {
      if (request.method() === 'GET') {
        await route.fulfill({
          contentType: 'application/json',
          json: { code: 0, data: { list: [] }, message: 'success' },
          status: 200,
        });
        return;
      }
      if (request.method() !== 'POST') {
        await route.continue();
        return;
      }
      const body = request.postDataJSON() as { parentId?: number };
      createdParentId = body.parentId;
      await route.fulfill({
        contentType: 'application/json',
        json: { code: 0, data: { id: 900001 }, message: 'success' },
        status: 200,
      });
    });

    const deptPage = new DeptPage(adminPage);
    await deptPage.goto();

    await deptPage.expectTopLevelParentOption();
    await deptPage.createRootDept(deptName, { code: deptCode });

    expect(createdParentId).toBe(0);
  });
});

