import type {
  PublicCatalogCategory,
  PublicCatalogProduct,
} from '#/api/idc/purchase-configuration';

export interface PurchaseCatalogState {
  categories: PublicCatalogCategory[];
  initialFailed: boolean;
  moreFailed: boolean;
  nextCursor: string;
  products: PublicCatalogProduct[];
}

export function createPurchaseCatalogState(): PurchaseCatalogState {
  return {
    categories: [],
    initialFailed: false,
    moreFailed: false,
    nextCursor: '',
    products: [],
  };
}

export function applyPurchaseCatalogPage(
  state: PurchaseCatalogState,
  reset: boolean,
  page: {
    categories?: PublicCatalogCategory[];
    nextCursor?: string;
    products?: PublicCatalogProduct[];
  },
) {
  return {
    categories: page.categories ?? state.categories,
    initialFailed: false,
    moreFailed: false,
    nextCursor: page.nextCursor || '',
    products: reset
      ? (page.products ?? [])
      : [...state.products, ...(page.products ?? [])],
  } satisfies PurchaseCatalogState;
}

export function applyPurchaseCatalogFailure(
  state: PurchaseCatalogState,
  reset: boolean,
) {
  return {
    ...state,
    initialFailed: reset,
    moreFailed: !reset,
  } satisfies PurchaseCatalogState;
}
