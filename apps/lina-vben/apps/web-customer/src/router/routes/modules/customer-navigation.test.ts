import type { RouteRecordRaw } from 'vue-router';

import { describe, expect, it } from 'vitest';

import accountRoutes from './account';
import customerServiceRoutes from './customer-service';

function routeNames(routes: RouteRecordRaw[]): string[] {
  return routes.flatMap((route) => [
    String(route.name || ''),
    ...routeNames(route.children || []),
  ]);
}

describe('customer navigation planning', () => {
  it('keeps referrals and commissions under finance', () => {
    const finance = accountRoutes.find(
      (route) => route.name === 'CustomerFinanceGroup',
    );
    const names = routeNames(finance?.children || []);

    expect(names).toContain('CustomerReferrals');
    expect(names).toContain('CustomerCommissions');
    expect(routeNames(accountRoutes)).not.toContain(
      'CustomerSalesChannelGroup',
    );
  });

  it('keeps the assistant under technical support', () => {
    const support = customerServiceRoutes.find(
      (route) => route.name === 'CustomerServiceGroup',
    );

    expect(routeNames(support?.children || [])).toContain(
      'CustomerAIAssistant',
    );
    expect(routeNames(accountRoutes)).not.toContain('CustomerAIAssistant');
  });

  it('uses semantic Lucide icons for every visible first-level entry', () => {
    const visibleRoutes = [...accountRoutes, ...customerServiceRoutes].filter(
      (route) => !route.meta?.hideInMenu,
    );

    for (const route of visibleRoutes) {
      expect(String(route.meta?.icon || '')).toMatch(/^lucide:[a-z0-9-]+$/u);
    }
  });
});
