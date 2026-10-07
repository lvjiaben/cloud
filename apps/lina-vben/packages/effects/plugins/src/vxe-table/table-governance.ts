const DEFAULT_BUSINESS_TABLE_MAX_HEIGHT = 680;

type GridColumn = Record<string, any>;
type GridUsageOptions = Record<string, any>;

function isActionColumn(column: GridColumn) {
  const field = String(column.field ?? '').toLowerCase();
  const defaultSlot = String(column.slots?.default ?? '').toLowerCase();
  return (
    ['action', 'actions', 'operation', 'operations'].includes(field) ||
    defaultSlot === 'action'
  );
}

function normalizeColumnGroup(columns: GridColumn[]) {
  const normalized = columns.map((column) => {
    if (!column || typeof column !== 'object') {
      return column;
    }
    const next = { ...column };
    if (Array.isArray(next.children)) {
      next.children = normalizeColumnGroup(next.children);
    }
    if (isActionColumn(next) && next.fixed === undefined) {
      next.fixed = 'right';
    }
    return next;
  });

  const businessColumns = normalized.filter(
    (column) =>
      column &&
      typeof column === 'object' &&
      !column.type &&
      !column.fixed &&
      !isActionColumn(column),
  );
  const hasFlexibleBusinessColumn = businessColumns.some(
    (column) => column.width === undefined || column.minWidth !== undefined,
  );

  if (!hasFlexibleBusinessColumn && businessColumns.length > 0) {
    const fillColumn = businessColumns.at(-1)!;
    fillColumn.minWidth = fillColumn.width;
    delete fillColumn.width;
  }
  return normalized;
}

/**
 * Applies the project-wide table layout baseline without changing page data or
 * query semantics. Business columns always leave one flexible fill column,
 * action columns stay visible on the right and large local datasets are
 * bounded. Query semantics and deferred search schemas remain page-owned.
 */
export function normalizeVbenGridUsage<T extends GridUsageOptions>(options: T) {
  const next: GridUsageOptions = { ...options };

  if (!next.gridOptions) {
    return next as T;
  }

  const gridOptions = { ...next.gridOptions };
  if (Array.isArray(gridOptions.columns)) {
    gridOptions.columns = normalizeColumnGroup(gridOptions.columns);
  }
  if (gridOptions.height === undefined && gridOptions.maxHeight === undefined) {
    gridOptions.maxHeight = DEFAULT_BUSINESS_TABLE_MAX_HEIGHT;
  }
  gridOptions.scrollX ??= { enabled: true, gt: 0 };
  gridOptions.showHeaderOverflow ??= 'tooltip';
  gridOptions.showOverflow ??= 'tooltip';

  return { ...next, gridOptions } as unknown as T;
}
