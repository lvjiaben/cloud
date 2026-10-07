// A bounded slice of the full fixture, with the total computed before slicing.
export function numberedMockPage<T>(rows: T[], query: URLSearchParams) {
  const page = Number(query.get("page") || 1);
  const pageSize = Number(query.get("pageSize") || 10);
  return { items: rows.slice((page - 1) * pageSize, page * pageSize), page, pageSize, total: rows.length, nextCursor: "" };
}
