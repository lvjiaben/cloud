import { getSession } from "#/api/request";

type InitialLoad<T> = {
  promise: Promise<void>;
  data?: T;
  error: string;
  settled: boolean;
};
// Only bridges an uncommitted Suspense render. It is not a business-data cache.
const pending = new Map<string, InitialLoad<unknown>>();
let session: string | undefined;
let navigation: string | undefined;
if (typeof window !== "undefined")
  window.addEventListener("customer-session", () => pending.clear());
export function initialLoad<T>(
  key: string,
  load: () => Promise<T>,
  navigationKey: string,
): InitialLoad<T> {
  const token = getSession()?.accessToken;
  if (token !== session || navigation !== navigationKey) {
    pending.clear();
    session = token;
    navigation = navigationKey;
  }
  const existing = pending.get(key);
  if (existing) return existing as InitialLoad<T>;
  const entry: InitialLoad<T> = {
    error: "",
    settled: false,
    promise: Promise.resolve(),
  };
  entry.promise = Promise.resolve()
    .then(load)
    .then(
      (value) => {
        entry.data = value;
      },
      (error) => {
        entry.error =
          error instanceof Error ? error.message : "加载失败，请重试";
      },
    )
    .finally(() => {
      entry.settled = true;
      // Discard abandoned navigation results, including errors, after a short grace period.
      setTimeout(() => forgetInitialLoad(key, entry), 30000);
    });
  pending.set(key, entry);
  return entry;
}
export function forgetInitialLoad(key: string, entry: InitialLoad<unknown>) {
  if (pending.get(key) === entry) pending.delete(key);
}
