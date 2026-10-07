import { test, expect } from '@host-tests/fixtures/auth';
import { DictPage } from '@host-tests/pages/DictPage';
import {
  createAdminApiContext,
  expectSuccess,
} from '@host-tests/support/api/job';
import type { APIRequestContext } from '@host-tests/support/playwright';

type DictDataItem = {
  id: number;
  dictType: string;
  label: string;
  value: string;
  sort: number;
  tagStyle: string;
  cssClass: string;
  status: number;
  remark: string;
};

const dictType = 'sys_normal_disable';
const normalValue = '1';

async function getNormalStatusData(api: APIRequestContext) {
  const result = await expectSuccess<{ list: DictDataItem[]; total: number }>(
    await api.get(
      `dict/data?pageNum=1&pageSize=100&dictType=${encodeURIComponent(dictType)}`,
    ),
  );
  const item = result.list.find((entry) => entry.value === normalValue);
  expect(item, `missing ${dictType} value ${normalValue}`).toBeTruthy();
  return item!;
}

async function setNormalStatusLabel(api: APIRequestContext, label: string) {
  const item = await getNormalStatusData(api);
  await expectSuccess<unknown>(
    await api.put(`dict/data/${item.id}`, {
      data: { label },
    }),
  );
}

test.describe('TC001 字典修改全局生效', () => {
  test.describe.configure({ mode: 'serial' });

  const originalLabel = '正常';
  const modifiedLabel = `测试状态_${Date.now()}`;
  let adminApi: APIRequestContext;

  test.beforeAll(async () => {
    adminApi = await createAdminApiContext();
    await setNormalStatusLabel(adminApi, originalLabel);
  });

  test.afterAll(async () => {
    if (!adminApi) {
      return;
    }
    try {
      await setNormalStatusLabel(adminApi, originalLabel);
    } finally {
      await adminApi.dispose();
    }
  });

  test('TC001a: 修改字典标签后部门管理页面同步更新', async ({ adminPage }) => {
    await setNormalStatusLabel(adminApi, originalLabel);

    const dictPage = new DictPage(adminPage);
    await dictPage.goto();

    // 选择 sys_normal_disable 字典类型
    await dictPage.clickTypeRow(dictType);

    // 修改"正常"标签为临时名称
    await dictPage.editData(originalLabel, { label: modifiedLabel });
    await expect(adminPage.getByText(/更新成功/)).toBeVisible({ timeout: 5000 });

    // 导航到部门管理页面
    await adminPage.goto('/system/dept');
    await adminPage.waitForLoadState('networkidle');
    await adminPage.locator('.vxe-table').first().waitFor({ state: 'visible', timeout: 10000 });

    // 验证表格状态列中显示修改后的标签（DictTag 渲染为 ant-tag）
    const modifiedTag = adminPage.locator('.ant-tag', { hasText: modifiedLabel });
    await expect(modifiedTag.first()).toBeVisible({ timeout: 5000 });
  });

  test('TC001b: 修改字典标签后用户管理页面查询表单同步更新', async ({ adminPage }) => {
    await setNormalStatusLabel(adminApi, modifiedLabel);

    // 导航到用户管理页面
    await adminPage.goto('/system/user');
    await adminPage.waitForLoadState('networkidle');
    await adminPage.locator('.vxe-table').first().waitFor({ state: 'visible', timeout: 10000 });

    // 点击用户状态下拉框
    const statusSelect = adminPage.getByLabel('用户状态', { exact: true }).first();
    await statusSelect.click();

    // 验证下拉选项中包含修改后的标签
    const dropdown = adminPage.locator('.ant-select-dropdown');
    await expect(dropdown.getByText(modifiedLabel)).toBeVisible({ timeout: 5000 });

    // 按 Escape 关闭下拉
    await adminPage.keyboard.press('Escape');
  });

  test('TC001c: 还原字典标签确保测试环境干净', async ({ adminPage }) => {
    await setNormalStatusLabel(adminApi, modifiedLabel);

    const dictPage = new DictPage(adminPage);
    await dictPage.goto();

    // 选择 sys_normal_disable 字典类型
    await dictPage.clickTypeRow(dictType);

    // 还原标签
    await dictPage.editData(modifiedLabel, { label: originalLabel });
    await expect(adminPage.getByText(/更新成功/)).toBeVisible({ timeout: 5000 });

    // 验证还原成功 - 导航到部门页面检查
    await adminPage.goto('/system/dept');
    await adminPage.waitForLoadState('networkidle');
    await adminPage.locator('.vxe-table').first().waitFor({ state: 'visible', timeout: 10000 });

    const originalTag = adminPage.locator('.ant-tag', { hasText: originalLabel });
    await expect(originalTag.first()).toBeVisible({ timeout: 5000 });
  });
});

