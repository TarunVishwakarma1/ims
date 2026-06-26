import { describe, expect, it } from "vitest";
import { paiseToINR } from "./format";

describe("paiseToINR", () => {
  it("formats rupees with Indian grouping", () => {
    expect(paiseToINR(129900)).toBe("₹1,299");
    expect(paiseToINR(10000000)).toBe("₹1,00,000");
  });
  it("shows paise precision when amount has a fractional rupee", () => {
    expect(paiseToINR(150)).toBe("₹1.50");
    expect(paiseToINR(2240)).toBe("₹22.40");
  });
  it("zero paise renders as ₹0", () => {
    expect(paiseToINR(0)).toBe("₹0");
  });
});
