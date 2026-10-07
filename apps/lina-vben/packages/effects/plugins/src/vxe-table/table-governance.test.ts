import { describe, expect, it } from 'vitest';

import { normalizeVbenGridUsage } from './table-governance';

describe('normalizeVbenGridUsage', () => {
  it('keeps one business column flexible and fixes the action column right', () => {
    const source = {
      gridOptions: {
        columns: [
          { field: 'name', width: 180 },
          { field: 'status', width: 120 },
          { field: 'action', slots: { default: 'action' }, width: 140 },
        ],
      },
    };

    const normalized = normalizeVbenGridUsage(source);
    expect(normalized.gridOptions.columns).toEqual([
      { field: 'name', width: 180 },
      { field: 'status', minWidth: 120 },
      {
        field: 'action',
        fixed: 'right',
        slots: { default: 'action' },
        width: 140,
      },
    ]);
    expect(normalized.gridOptions.maxHeight).toBe(680);
    expect(normalized.gridOptions.scrollX).toEqual({ enabled: true, gt: 0 });
    expect(source.gridOptions.columns[1]).toEqual({
      field: 'status',
      width: 120,
    });
  });

  it('does not override an explicit height or a deferred search schema', () => {
    const normalized = normalizeVbenGridUsage({
      formOptions: { schema: [] },
      gridOptions: { columns: [{ field: 'name', minWidth: 120 }], height: 420 },
      showSearchForm: true,
    });

    expect(normalized.formOptions).toEqual({ schema: [] });
    expect(normalized.showSearchForm).toBe(true);
    expect(normalized.gridOptions.height).toBe(420);
    expect(normalized.gridOptions.maxHeight).toBeUndefined();
  });
});
