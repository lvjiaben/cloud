import { describe, expect, it } from 'vitest';

import {
  customerFulfillmentColor,
  customerDeliveryRows,
  customerFulfillmentLabelKey,
  customerServiceLabelKey,
} from './customer-fulfillment-state';

describe('customer-safe fulfillment projection', () => {
  it.each([
    'awaiting_payment',
    'awaiting_review',
    'processing',
    'completed',
    'not_completed',
    'cancelled',
  ])('maps the governed status %s without exposing internals', (status) => {
    expect(customerFulfillmentLabelKey(status)).toBe(
      `commerce.fulfillment.status.${status}`,
    );
  });

  it.each([
    'queued',
    'manual_pending',
    'review_pending',
    'pending_query',
    'running',
    'retry_wait',
    'succeeded',
    'failed',
    'dead_letter',
    'result_unknown',
    'manual_takeover',
    'plugin_error',
  ])(
    'does not render internal status %s',
    (status) => {
      expect(customerFulfillmentLabelKey(status)).toBe(
        'commerce.fulfillment.statusUnavailable',
      );
      expect(customerFulfillmentColor(status)).toBe('default');
    },
  );

  it('uses only the one-record-per-item delivery projection', () => {
    const safe = [{ statusCode: 'processing' }];
    expect(
      customerDeliveryRows({
        deliveries: safe,
        productFulfillments: [{ statusCode: 'internal-running' }],
        fulfillments: [{ statusCode: 'internal-running' }],
      } as { deliveries: typeof safe }),
    ).toBe(safe);
    expect(customerDeliveryRows({})).toEqual([]);
  });

  it('does not expose unknown service state codes', () => {
    expect(customerServiceLabelKey('active')).toBe(
      'commerce.fulfillment.serviceStatus.active',
    );
    expect(customerServiceLabelKey('provider_error')).toBe(
      'commerce.fulfillment.serviceStatusUnavailable',
    );
  });
});
