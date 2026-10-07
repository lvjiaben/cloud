import { describe, expect, it } from 'vitest';

import {
  hasPendingQuoteConfiguration,
  quoteConfigurationAction,
} from './customer-commerce-configuration-state';

describe('server-authoritative quote configuration state', () => {
  it('does not depend on missing or forged browser storage', () => {
    const frozen = { configurationFrozen: true, configurationRequired: true };
    expect(hasPendingQuoteConfiguration([frozen])).toBe(false);
    expect(quoteConfigurationAction(frozen)).toBe('reconfigure');

    const pending = { configurationFrozen: false, configurationRequired: true };
    expect(hasPendingQuoteConfiguration([pending])).toBe(true);
    expect(quoteConfigurationAction(pending)).toBe('configure');
  });
});
