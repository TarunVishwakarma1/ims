import { describe, expect, it } from "vitest";
import { isProductSort, type ProductSort } from "./shop-types";

describe("isProductSort", () => {
  it("accepts the three valid sorts", () => {
    const valid: ProductSort[] = ["newest", "price_asc", "price_desc"];
    for (const s of valid) expect(isProductSort(s)).toBe(true);
  });
  it("rejects unknown sorts", () => {
    expect(isProductSort("popularity")).toBe(false);
    expect(isProductSort("")).toBe(false);
    expect(isProductSort("PRICE_ASC")).toBe(false);
  });
});
