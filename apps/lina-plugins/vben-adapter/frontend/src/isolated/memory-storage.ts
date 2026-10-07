/** Component preferences only: opaque frames cannot use browser storage. */
export class StorageManager {
  private readonly values = new Map<string, { expiry?: number; value: unknown }>();

  constructor(_options: { prefix?: string; storageType?: string } = {}) {}

  clear() { this.values.clear(); }
  clearExpiredItems() {
    for (const key of this.values.keys()) this.getItem(key);
  }
  getItem<T>(key: string, fallback: T | null = null): T | null {
    const entry = this.values.get(key);
    if (!entry) return fallback;
    if (entry.expiry !== undefined && entry.expiry <= Date.now()) {
      this.values.delete(key);
      return fallback;
    }
    return entry.value as T;
  }
  removeItem(key: string) { this.values.delete(key); }
  setItem<T>(key: string, value: T, ttl?: number) {
    this.values.set(key, { expiry: ttl ? Date.now() + ttl : undefined, value });
  }
}
