import type { CustomerSummary } from "./customer-client";

import { customerList } from "./customer-client";

export interface CustomerSelectOption { label: string; value: string }

export function customerDisplay(input: {
  customerCode: string;
  customerDisplayName: string;
  customerPrimaryPhone: string;
}) {
  return [input.customerDisplayName, input.customerPrimaryPhone, input.customerCode ? `#${input.customerCode}` : ""]
    .filter(Boolean).join(" · ");
}

function option(customer: CustomerSummary): CustomerSelectOption {
  return {
    label: customerDisplay({ customerCode: customer.customerCode, customerDisplayName: customer.displayName, customerPrimaryPhone: customer.primaryPhone }),
    value: customer.customerNo,
  };
}

export async function searchCustomerOptions(keyword: string) {
  const value = keyword.trim();
  const filters = value
    ? /^\d+$/.test(value)
      ? [{ customerCode: value }, ...(value.length >= 6 ? [{ phone: value }] : []), { namePrefix: value }]
      : [{ namePrefix: value }]
    : [{}];
  const pages = await Promise.all(filters.map((filter) => customerList({ ...filter, limit: 20 })));
  const unique = new Map<string, CustomerSummary>();
  for (const page of pages) for (const customer of page.customers || []) unique.set(customer.customerNo, customer);
  return [...unique.values()].slice(0, 20).map(option);
}
