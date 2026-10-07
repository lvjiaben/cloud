import { describe, expect, it } from 'vitest';

import {
  applyPurchaseCatalogFailure,
  applyPurchaseCatalogPage,
  createPurchaseCatalogState,
} from './purchase-catalog-state';

const product = (slug: string) => ({ publicSlug: slug }) as never;

describe('purchase catalog pagination state', () => {
  it('keeps the first page and retry cursor when loading more fails', () => {
    const loaded = applyPurchaseCatalogPage(
      createPurchaseCatalogState(),
      true,
      {
        nextCursor: 'opaque-next-page',
        products: [product('first')],
      },
    );
    const failed = applyPurchaseCatalogFailure(loaded, false);

    expect(failed.products).toEqual(loaded.products);
    expect(failed.nextCursor).toBe('opaque-next-page');
    expect(failed.initialFailed).toBe(false);
    expect(failed.moreFailed).toBe(true);

    const retried = applyPurchaseCatalogPage(failed, false, {
      products: [product('second')],
    });
    expect(retried.products).toHaveLength(2);
    expect(retried.moreFailed).toBe(false);
  });

  it('uses a full-page failure only before any first page is available', () => {
    const failed = applyPurchaseCatalogFailure(
      createPurchaseCatalogState(),
      true,
    );
    expect(failed.initialFailed).toBe(true);
    expect(failed.products).toEqual([]);
  });
});
