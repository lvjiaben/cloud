import { describe, expect, it } from 'vitest';
import type { PurchaseField } from '#/api/idc/purchase-configuration';
import {
  mergeAuthoritativeFileTokens,
  submissionSnapshotUnchanged,
} from './purchase-configuration-flow';

const fileField = (visible: boolean): PurchaseField => ({
  descriptionEnUS: '',
  descriptionZhCN: '',
  disabled: false,
  fieldToken: 'opaque-field',
  labelEnUS: 'File',
  labelZhCN: '文件',
  required: false,
  type: 'file',
  visible,
});

describe('purchase configuration authoritative flow', () => {
  it('preserves a hidden file when a hidden request becomes visible without returning the omitted token', () => {
    const hidden = mergeAuthoritativeFileTokens(
      [fileField(false)],
      [fileField(true)],
      [],
      {
        'opaque-field': ['CFU_A'],
      },
    );
    expect(hidden['opaque-field']).toEqual(['CFU_A']);
    const visible = mergeAuthoritativeFileTokens(
      [fileField(true)],
      [fileField(false)],
      [],
      hidden,
    );
    expect(visible['opaque-field']).toEqual(['CFU_A']);
  });
  it('removes a token rejected by the authoritative server while continuously visible', () =>
    expect(
      mergeAuthoritativeFileTokens([fileField(true)], [fileField(true)], [], {
        'opaque-field': ['CFU_A'],
      })['opaque-field'],
    ).toEqual([]));
  it('rejects a submit result after the draft changes in flight', () =>
    expect(submissionSnapshotUnchanged('r1', [], 'r1', [], true)).toBe(false));
});
