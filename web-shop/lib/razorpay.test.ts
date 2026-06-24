import { describe, it, expect, vi, beforeEach } from "vitest";
import { openRazorpayCheckout } from "./razorpay";

beforeEach(() => {
  // @ts-expect-error
  delete window.Razorpay;
});

describe("openRazorpayCheckout", () => {
  it("throws when script not loaded", () => {
    expect(() =>
      openRazorpayCheckout({
        keyID: "k", orderID: "o", amount: 100, name: "shop",
        onSuccess: () => {}, onDismiss: () => {},
      }),
    ).toThrow("razorpay_not_loaded");
  });

  it("calls Razorpay constructor + open when loaded", () => {
    const open = vi.fn();
    const ctor = vi.fn().mockReturnValue({ open });
    // @ts-expect-error
    window.Razorpay = ctor;
    openRazorpayCheckout({
      keyID: "k", orderID: "o", amount: 100, name: "shop",
      onSuccess: () => {}, onDismiss: () => {},
    });
    expect(ctor).toHaveBeenCalledOnce();
    expect(open).toHaveBeenCalledOnce();
  });
});
