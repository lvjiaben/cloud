// Keep each payment directory's confirmed rows and official pager atomic.
import { shallowReactive } from "vue";

export interface PaymentPageRequest { currentPage: number; pageSize: number }
export interface PaymentPageResult<T> { items: T[]; total: number }

export function paymentAccessDenied(error: unknown) {
  const value = error as { status?: number; code?: number | string; response?: { status?: number; data?: { code?: number | string } } };
  const status = value?.response?.status ?? value?.status;
  const code = String(value?.response?.data?.code ?? value?.code ?? "");
  return status === 401 || status === 403 || ["401", "403", "61", "NotAuthorized"].includes(code);
}

export function usePaymentPage<T, Extra extends object = object>(
  read: (page: number, pageSize: number) => Promise<PaymentPageResult<T> & Extra>,
  failed: () => void,
  denied?: () => void,
  committed?: (result: PaymentPageResult<T> & Extra) => void,
) {
  const state = shallowReactive({ items: [] as T[], total: 0, currentPage: 1, pageSize: 20, loading: false });
  let sequence = 0;
  let requested: PaymentPageRequest = { currentPage: 1, pageSize: 20 };
  function reset() {
    sequence += 1;
    requested = { currentPage: 1, pageSize: state.pageSize };
    Object.assign(state, requested, { items: [], total: 0, loading: false });
  }
  async function load(page = requested.currentPage, pageSize = requested.pageSize) {
    const current = ++sequence;
    requested = { currentPage: pageSize === state.pageSize ? page : 1, pageSize };
    const target = { ...requested };
    state.loading = true;
    try {
      const result = await read(target.currentPage, target.pageSize);
      if (current !== sequence) return false;
      if (!Number.isSafeInteger(result.total) || result.total < result.items.length) throw new Error("Invalid payment page total");
      Object.assign(state, { items: result.items, total: result.total }, target);
      committed?.(result);
      return true;
    } catch (error) {
      if (current !== sequence) return false;
      if (paymentAccessDenied(error)) { reset(); denied?.(); }
      failed();
      return false;
    } finally {
      if (current === sequence) state.loading = false;
    }
  }
  return { state, load, reset, invalidate: () => { sequence += 1; } };
}
