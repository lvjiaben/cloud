// Keep confirmed rows and pager state together across failed or delayed reads.
// Business readers supply an authoritative total; this module never infers it.
export interface NumberedPageState<T> {
  currentPage: number;
  items: T[];
  pageSize: number;
  total: number;
}

export function createNumberedPage<T>(
  publish: (state: NumberedPageState<T>) => void,
  denied?: () => void,
) {
  let sequence = 0;
  let confirmed: NumberedPageState<T> = {
    currentPage: 1,
    items: [],
    pageSize: 10,
    total: 0,
  };
  return {
    reset() {
      sequence += 1;
      confirmed = { currentPage: 1, items: [], pageSize: 10, total: 0 };
      publish(confirmed);
    },
    invalidate() {
      sequence += 1;
    },
    async query(
      requested: { currentPage: number; pageSize: number },
      read: (
        page: number,
        pageSize: number,
      ) => Promise<{ items: T[]; total: number }>,
    ) {
      const currentSequence = ++sequence;
      const currentPage =
        requested.pageSize === confirmed.pageSize ? requested.currentPage : 1;
      try {
        const result = await read(currentPage, requested.pageSize);
        if (currentSequence === sequence) {
          if (
            !Number.isSafeInteger(result.total) ||
            result.total < result.items.length
          ) {
            throw new Error("Invalid authoritative page total");
          }
          confirmed = { ...result, currentPage, pageSize: requested.pageSize };
        }
      } catch (failure) {
        const status = (failure as { response?: { status?: number } })?.response
          ?.status;
        if (
          currentSequence === sequence &&
          (status === 401 || status === 403)
        ) {
          confirmed = {
            currentPage: 1,
            items: [],
            pageSize: requested.pageSize,
            total: 0,
          };
          denied?.();
        }
        // The existing request interceptor reports the error. A temporary
        // outage must not replace a confirmed page with a fabricated empty one.
      }
      if (currentSequence === sequence) publish(confirmed);
      return { items: confirmed.items, total: confirmed.total };
    },
  };
}
