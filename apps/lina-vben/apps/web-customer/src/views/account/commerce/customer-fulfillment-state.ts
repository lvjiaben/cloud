export type CustomerFulfillmentStatus =
  | 'awaiting_payment'
  | 'awaiting_review'
  | 'cancelled'
  | 'completed'
  | 'not_completed'
  | 'processing';

const safeStatuses = new Set<CustomerFulfillmentStatus>([
  'awaiting_payment',
  'awaiting_review',
  'processing',
  'completed',
  'not_completed',
  'cancelled',
]);

export function customerFulfillmentLabelKey(status: string) {
  return safeStatuses.has(status as CustomerFulfillmentStatus)
    ? `commerce.fulfillment.status.${status}`
    : 'commerce.fulfillment.statusUnavailable';
}

export function customerFulfillmentColor(status: string) {
  switch (status) {
    case 'completed':
      return 'success';
    case 'not_completed':
      return 'error';
    case 'awaiting_review':
      return 'warning';
    case 'processing':
      return 'processing';
    default:
      return 'default';
  }
}

export function customerDeliveryRows<T>(detail?: {
  deliveries?: readonly T[];
}) {
  return detail?.deliveries ?? [];
}

const safeServiceStatuses = new Set([
  'active',
  'expired',
  'suspended',
  'terminated',
]);

export function customerServiceLabelKey(status: string) {
  return safeServiceStatuses.has(status)
    ? `commerce.fulfillment.serviceStatus.${status}`
    : 'commerce.fulfillment.serviceStatusUnavailable';
}
