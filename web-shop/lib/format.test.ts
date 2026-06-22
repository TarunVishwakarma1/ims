import { describe, expect, it } from "vitest";
import { paiseToINR } from "./format";

describe("paiseToINR", () => {
  it("formats rupees with Indian grouping", () => {
    expect(paiseToINR(129900)).toBe("₹1,299");
    expect(paiseToINR(10000000)).toBe("₹1,00,000");
  });
  it("rounds to whole rupees (no paise visible)", () => {
    expect(paiseToINR(150)).toBe("₹2");
    expect(paiseToINR(149)).toBe("₹1");
  });
  it("zero paise renders as ₹0", () => {
    expect(paiseToINR(0)).toBe("₹0");
  });
});
