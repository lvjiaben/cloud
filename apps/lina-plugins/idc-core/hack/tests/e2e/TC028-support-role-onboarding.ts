// Live API acceptance: roles and organization ownership are created through
// public APIs; only this test's plugin-owned ticket fixtures use SQL cleanup.
import { randomBytes } from 'node:crypto';
import { expect, test } from '@host-tests/fixtures/auth';
import {
  createAdminApiContext, createApiContext, expectSuccess,
  getMenuIdsByPermsWithAncestors, type APIRequestContext,
} from '@host-tests/support/api/job';
import { execPgSQLStatements, queryPgScalar } from '@host-tests/support/postgres';

const support = '/x/idc-core/api/v1/admin/support';
const org = '/x/org-core/api/v1/dept';
type Ticket = { ticketNo: string; version: number; assigneeUserId: number; baseStage: string };
type Detail = { ticket: Ticket; availableActions: string[] };

async function denied(api: APIRequestContext, url: string) {
  const response = await api.get(url);
  expect(response.status(), 'route must exist and fail authorization').not.toBe(404);
  if (response.status() === 403) return;
  expect((await response.json()).code).not.toBe(0);
}

// Quotes are restricted by server-issued business-number syntax, never arbitrary SQL.
function sqlNo(value: string) {
  expect(value).toMatch(/^[A-Za-z0-9_-]+$/);
  return `'${value}'`;
}

test.describe('TC-28 仅工单角色真实入职闭环', () => {
  test('TC-28a: 新员工直接获得部门队列，领取后调部门立即失去原工单权限', async () => {
    const stamp = Date.now().toString();
    const username = `tkt${stamp}`;
    const password = `E2e!${randomBytes(12).toString('hex')}`;
    const admin = await createAdminApiContext();
    let employee: APIRequestContext | undefined;
    let userID = 0;
    let roleID = 0;
    const departments: number[] = [];
    const queues: string[] = [];
    const tickets: string[] = [];
    try {
      for (const suffix of ['A', 'B']) {
        const department = await expectSuccess<{ id: number }>(await admin.post(org, {
          data: { name: `工单验收${stamp}${suffix}`, code: `TC28_${stamp}_${suffix}`, status: 1 },
        }));
        departments.push(department.id);
        const queue = await expectSuccess<{ item: { departmentNo: string; orgDeptId: number } }>(await admin.post(`${support}/settings/departments`, {
          data: { orgDeptId: department.id, nameZhCN: `验收队列${stamp}${suffix}`, nameEnUS: `Acceptance ${stamp} ${suffix}`, assignmentMode: 'manual_claim' },
        }));
        expect(queue.item.orgDeptId).toBe(department.id);
        queues.push(queue.item.departmentNo);
      }
      const menuIds = await getMenuIdsByPermsWithAncestors(admin, ['list', 'query', 'reply', 'claim', 'resolve'].map(action => `idc:admin:support:ticket:${action}`));
      const role = await expectSuccess<{ id: number }>(await admin.post('role', {
        data: { name: `工单客服${stamp}`, key: `ticket_${stamp}`, dataScope: 3, status: 1, menuIds },
      }));
      roleID = role.id;
      const user = await expectSuccess<{ id: number }>(await admin.post('user', {
        data: { username, password, nickname: `验收客服${stamp}`, status: 1, roleIds: [roleID], deptId: departments[0] },
      }));
      userID = user.id;
      const savedUser=await expectSuccess<{deptId:number}>(await admin.get(`user/${userID}`));
      expect(savedUser.deptId).toBe(departments[0]);
      for (const [index, departmentNo] of queues.entries()) {
        const created = await expectSuccess<{ item: Detail }>(await admin.post(`${support}/tickets`, {
          data: { departmentNo, subject: `角色入职验收${stamp}-${index}`, body: '验证工单角色和直属组织归属', priority: 'normal' },
        }));
        tickets.push(created.item.ticket.ticketNo);
        expect(created.item.ticket.assigneeUserId ?? 0).toBe(0);
      }
      // No first-login principal seed or standalone employee qualification is created.
      employee = await createApiContext(username, password);
      const listing = await expectSuccess<{ tickets: Ticket[] }>(await employee.get(`${support}/tickets`, { params: { query: `角色入职验收${stamp}` } }));
      expect(listing.tickets.map(row => row.ticketNo)).toEqual([tickets[0]]);
      await denied(employee, `${support}/tickets/${tickets[1]}`);
      await denied(employee, '/x/idc-core/api/v1/admin/billing/transactions');
      const claimed = await expectSuccess<{ item: Detail }>(await employee.post(`${support}/tickets/${tickets[0]}/claims`, { data: { ticketVersion: 1 } }));
      expect(claimed.item.ticket.assigneeUserId).toBe(userID);
      expect(claimed.item.ticket.baseStage).toBe('in_progress');
      expect((await (await employee.post(`${support}/tickets/${tickets[0]}/claims`, { data: { ticketVersion: 1 } })).json()).code).not.toBe(0);
      await expectSuccess(await admin.put(`user/${userID}`, { data: { nickname: `验收客服${stamp}`, deptId: departments[1] } }));
      // Refresh login because host user updates revoke the old session by policy.
      await employee.dispose();
      employee = await createApiContext(username, password);
      const moved = await expectSuccess<{ tickets: Ticket[] }>(await employee.get(`${support}/tickets`, { params: { query: `角色入职验收${stamp}` } }));
      expect(moved.tickets.map(row => row.ticketNo)).toEqual([tickets[1]]);
      await denied(employee, `${support}/tickets/${tickets[0]}`);
      const reply = await employee.post(`${support}/tickets/${tickets[0]}/public-replies`, { data: { ticketVersion: claimed.item.ticket.version, body: '调部门后不得访问' } });
      expect((await reply.json()).code).not.toBe(0);
      await denied(employee, '/x/idc-core/api/v1/admin/billing/transactions');
    } finally {
      await employee?.dispose();
      // These records have no delete business action. Delete only exact numbers
      // returned to this test, in child-before-parent order.
      if (tickets.length) {
        const selected = tickets.map(sqlNo).join(',');
        execPgSQLStatements([
          ...['custom_field_value_option','custom_field_value','assignment_history','status_history','message','sla_round'].map(table => `DELETE FROM plugin_idc_core_support_ticket_${table} WHERE ticket_no IN (${selected});`),
          `DELETE FROM plugin_idc_core_support_ticket WHERE ticket_no IN (${selected});`,
        ]);
      }
      if (queues.length) execPgSQLStatements([`DELETE FROM plugin_idc_core_support_department WHERE department_no IN (${queues.map(sqlNo).join(',')});`]);
      if (userID) {
        execPgSQLStatements([`DELETE FROM plugin_idc_core_principal_audience WHERE sys_user_id=${userID};`]);
        await expectSuccess(await admin.delete(`user/${userID}`));
      }
      if (roleID) await expectSuccess(await admin.delete(`role/${roleID}`));
      for (const id of departments.reverse()) await expectSuccess(await admin.delete(`${org}/${id}`));
      if (tickets.length) expect(Number(queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_support_ticket WHERE ticket_no IN (${tickets.map(sqlNo).join(',')});`))).toBe(0);
      if (queues.length) expect(Number(queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_support_department WHERE department_no IN (${queues.map(sqlNo).join(',')});`))).toBe(0);
      await admin.dispose();
    }
  });
});
