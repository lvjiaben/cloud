import { describe, expect, it } from 'vitest';
import { mapPurchaseOptions } from './purchase-options';

describe('mapPurchaseOptions', () => {
  it('keeps a four-level address and a three-level tree intact', () => {
    const branch = { labelEnUS: '1', labelZhCN: '一', optionToken: '1', children: [{ labelEnUS: '2', labelZhCN: '二', optionToken: '2', children: [{ labelEnUS: '3', labelZhCN: '三', optionToken: '3', children: [{ labelEnUS: '4', labelZhCN: '四', optionToken: '4' }] }] }] };
    const result = mapPurchaseOptions([branch], true);
    expect(result[0]?.children?.[0]?.children?.[0]?.children?.[0]).toEqual({ children: undefined, label: '四', value: '4' });
    expect(result[0]?.children?.[0]?.children?.[0]?.label).toBe('三');
  });
});
