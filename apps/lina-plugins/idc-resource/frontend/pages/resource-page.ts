/** Keep a confirmed numeric page while failed or late reads cannot replace it. */
export interface ResourcePageState<T> {
  items: T[];
  total: number;
  currentPage: number;
  pageSize: number;
}

export function isResourceAccessDenied(error: unknown) {
  const status = (error as { response?: { status?: number } } | undefined)
    ?.response?.status;
  return status === 401 || status === 403;
}

export function createResourcePage<T>(options: {
  onPage: (state: ResourcePageState<T>) => void;
  onDenied: () => void;
  onFailure: (error: unknown) => void;
}) {
  let generation = 0;
  let signature = "";
  let state: ResourcePageState<T> = {
    items: [], total: 0, currentPage: 1, pageSize: 10,
  };
  return {
    get current() { return state; },
    invalidate() {
      ++generation;
      signature = "";
      state = { items: [], total: 0, currentPage: 1, pageSize: state.pageSize };
      options.onPage(state);
    },
    async query<R extends { items: T[]; total: number }>(
      requested: { currentPage: number; pageSize: number },
      filterSignature: string,
      load: (page: number, limit: number) => Promise<R>,
      commit?: (result: R) => void,
    ): Promise<ResourcePageState<T>> {
      const attempt = ++generation;
      const currentPage = requested.pageSize !== state.pageSize || filterSignature !== signature
        ? 1 : requested.currentPage;
      try {
        const result = await load(currentPage, requested.pageSize);
        if (attempt !== generation) return state;
        if (!Number.isSafeInteger(result.total) || result.total < 0 ||
            !Array.isArray(result.items) || result.items.length > requested.pageSize || result.items.length > result.total) {
          throw new Error("Invalid resource directory page");
        }
        state = { items: result.items, total: result.total, currentPage, pageSize: requested.pageSize };
        signature = filterSignature;
        commit?.(result);
      } catch (error) {
        if (attempt !== generation) return state;
        if (isResourceAccessDenied(error)) {
          ++generation;
          state = { items: [], total: 0, currentPage: 1, pageSize: state.pageSize };
          signature = "";
          options.onDenied();
        }
        options.onFailure(error);
      } finally {
        options.onPage(state);
      }
      return state;
    },
  };
}
