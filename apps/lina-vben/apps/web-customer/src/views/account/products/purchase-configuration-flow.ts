import type {
  PurchaseField,
  PurchaseValue,
} from '#/api/idc/purchase-configuration';

export function mergeAuthoritativeFileTokens(
  fields: PurchaseField[],
  previousFields: PurchaseField[],
  rows: PurchaseValue[],
  current: Record<string, string[]>,
) {
  const incoming = new Map(
    rows.map((row) => [row.fieldToken, row.fileTokens ?? []]),
  );
  const previousVisibility = new Map(
    previousFields.map((field) => [field.fieldToken, field.visible]),
  );
  const next: Record<string, string[]> = {};
  for (const field of fields.filter((item) => item.type === 'file')) {
    const preserveLocal =
      !field.visible ||
      (previousVisibility.get(field.fieldToken) === false &&
        !incoming.has(field.fieldToken));
    next[field.fieldToken] = preserveLocal
      ? [...(current[field.fieldToken] ?? [])]
      : [...(incoming.get(field.fieldToken) ?? [])];
  }
  return next;
}

export function submissionSnapshotUnchanged(
  initialRevision: string,
  initialRows: PurchaseValue[],
  currentRevision: string,
  currentRows: PurchaseValue[],
  changed: boolean,
) {
  return (
    !changed &&
    initialRevision === currentRevision &&
    JSON.stringify(initialRows) === JSON.stringify(currentRows)
  );
}
