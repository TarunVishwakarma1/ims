import { describe, it, expect, beforeEach, vi } from "vitest";
import type { CartItem } from "./shop-types";

const mockAdd = vi.fn();
const mockRemove = vi.fn();
const mockMerge = vi.fn();
const mockFetchCart = vi.fn();

vi.mock("./shop-api", () => {
  // Declared inside the factory: vi.mock is hoisted above module-level
  // bindings, so a top-level class would be in the TDZ when the mock runs.
  class CartShopConflictError extends Error {
    code = "cart_other_shop" as const;
    currentSlug: string;
    currentName: string;
    constructor(slug: string, name: string) {
      super("cart_other_shop");
      this.currentSlug = slug;
      this.currentName = name;
    }
  }
  return {
    addCartItem: (...args: unknown[]) => mockAdd(...args),
    removeCartItem: (...args: unknown[]) => mockRemove(...args),
    mergeCart: (...args: unknown[]) => mockMerge(...args),
    fetchCart: () => mockFetchCart(),
    CartShopConflictError,
  };
});

const SHOP = "sharma-kirana";

import { useCartStore, selectItemCount, selectSubtotalPaise } from "./cart-store";

const item = (overrides: Partial<CartItem> = {}): CartItem => ({
  product_id: "p1",
  slug: "p-one",
  name: "Product One",
  image: "/img.jpg",
  qty: 1,
  unit_price_paise: 50000,
  max_qty: 10,
  ...overrides,
});

describe("cart-store anonymous", () => {
  beforeEach(() => {
    localStorage.clear();
    useCartStore.setState({ items: [], shopSlug: null, shopName: null, serverHydrated: false });
    mockAdd.mockReset();
    mockRemove.mockReset();
    mockMerge.mockReset();
    mockFetchCart.mockReset();
  });

  it("adds new item", async () => {
    await useCartStore.getState().add(item(), 1, SHOP);
    expect(selectItemCount(useCartStore.getState())).toBe(1);
    expect(mockAdd).not.toHaveBeenCalled();
  });

  it("merges qty when adding duplicate", async () => {
    await useCartStore.getState().add(item(), 2, SHOP);
    await useCartStore.getState().add(item(), 3, SHOP);
    const items = useCartStore.getState().items;
    expect(items).toHaveLength(1);
    expect(items[0].qty).toBe(5);
  });

  it("clamps qty to max_qty", async () => {
    await useCartStore.getState().add(item({ max_qty: 4 }), 10, SHOP);
    expect(useCartStore.getState().items[0].qty).toBe(4);
  });

  it("setQty replaces qty", async () => {
    await useCartStore.getState().add(item(), 1, SHOP);
    await useCartStore.getState().setQty("p1", 3);
    expect(useCartStore.getState().items[0].qty).toBe(3);
  });

  it("setQty <= 0 removes", async () => {
    await useCartStore.getState().add(item(), 1, SHOP);
    await useCartStore.getState().setQty("p1", 0);
    expect(useCartStore.getState().items).toHaveLength(0);
  });

  it("subtotal selector", async () => {
    await useCartStore.getState().add(item({ unit_price_paise: 10000 }), 2, SHOP);
    await useCartStore.getState().add(item({ product_id: "p2", unit_price_paise: 5000 }), 1, SHOP);
    expect(selectSubtotalPaise(useCartStore.getState())).toBe(25000);
  });
});

describe("cart-store hydrated", () => {
  beforeEach(() => {
    localStorage.clear();
    useCartStore.setState({ items: [], shopSlug: null, shopName: null, serverHydrated: true });
    mockAdd.mockReset();
    mockRemove.mockReset();
  });

  it("calls server addCartItem on add", async () => {
    mockAdd.mockResolvedValue({
      items: [{ ...item(), qty: 1 }],
      subtotal_paise: 50000,
      item_count: 1,
    });
    await useCartStore.getState().add(item(), 1, SHOP);
    expect(mockAdd).toHaveBeenCalledWith(SHOP, "p1", 1);
    expect(useCartStore.getState().items[0].qty).toBe(1);
  });

  it("rolls back on server error", async () => {
    mockAdd.mockRejectedValue(new Error("nope"));
    await expect(useCartStore.getState().add(item(), 1, SHOP)).rejects.toThrow();
    expect(useCartStore.getState().items).toHaveLength(0);
  });

  it("mergeOnLogin pushes local items to server", async () => {
    // anonymous-mode add (serverHydrated:false initially)
    useCartStore.setState({ items: [], shopSlug: null, shopName: null, serverHydrated: false });
    await useCartStore.getState().add(item(), 2, SHOP);
    mockMerge.mockResolvedValue({
      items: [{ ...item(), qty: 2 }],
      subtotal_paise: 100000,
      item_count: 2,
    });
    await useCartStore.getState().mergeOnLogin();
    expect(mockMerge).toHaveBeenCalledWith(SHOP, [{ product_id: "p1", qty: 2 }]);
    expect(useCartStore.getState().serverHydrated).toBe(true);
  });
});
