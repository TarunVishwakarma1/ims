import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { push, placeOrder, verifyRazorpayPayment, loadRazorpay, openRazorpayCheckout } = vi.hoisted(() => ({
  push: vi.fn(),
  placeOrder: vi.fn(),
  verifyRazorpayPayment: vi.fn(),
  loadRazorpay: vi.fn().mockResolvedValue(undefined),
  openRazorpayCheckout: vi.fn(),
}));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));
vi.mock("@/lib/shop-api", () => ({ placeOrder, verifyRazorpayPayment }));
vi.mock("@/lib/razorpay", () => ({ loadRazorpay, openRazorpayCheckout }));

import { PlaceOrderButton } from "./place-order-button";
import { useCartStore } from "@/lib/cart-store";

beforeEach(() => {
  push.mockReset();
  placeOrder.mockReset();
  verifyRazorpayPayment.mockReset();
  loadRazorpay.mockClear();
  loadRazorpay.mockResolvedValue(undefined);
  openRazorpayCheckout.mockReset();
  useCartStore.setState({ items: [], serverHydrated: true, loading: false });
});

describe("PlaceOrderButton", () => {
  it("COD: places + clears cart + navigates", async () => {
    placeOrder.mockResolvedValue({ order_id: "ord1", payable_paise: 12345, invoice_number: "INV-1" });
    render(<PlaceOrderButton addressID="a1" paymentMethod="cod" />);
    fireEvent.click(screen.getByText(/Place order \(COD\)/i));
    await waitFor(() => expect(push).toHaveBeenCalledWith("/orders/ord1?placed=1"));
    expect(useCartStore.getState().items).toHaveLength(0);
  });

  it("Razorpay: places + opens + verifies + navigates", async () => {
    placeOrder.mockResolvedValue({
      order_id: "ord2", payable_paise: 50000, invoice_number: "INV-2",
      razorpay_order_id: "rzp_o", razorpay_key_id: "rzp_k",
    });
    verifyRazorpayPayment.mockResolvedValue({ order_id: "ord2", status: "confirmed", payment_status: "paid", invoice_number: "INV-2" });
    openRazorpayCheckout.mockImplementation((args) => {
      args.onSuccess({ razorpay_payment_id: "p1", razorpay_order_id: "rzp_o", razorpay_signature: "sig" });
    });
    render(<PlaceOrderButton addressID="a1" paymentMethod="razorpay" />);
    fireEvent.click(screen.getByText(/Pay & place order/i));
    await waitFor(() => expect(verifyRazorpayPayment).toHaveBeenCalled());
    expect(push).toHaveBeenCalledWith("/orders/ord2?placed=1");
  });

  it("Razorpay dismiss: no verify, no nav", async () => {
    placeOrder.mockResolvedValue({
      order_id: "ord3", payable_paise: 50000, invoice_number: "INV-3",
      razorpay_order_id: "rzp_o", razorpay_key_id: "rzp_k",
    });
    openRazorpayCheckout.mockImplementation((args) => args.onDismiss());
    render(<PlaceOrderButton addressID="a1" paymentMethod="razorpay" />);
    fireEvent.click(screen.getByText(/Pay & place order/i));
    await waitFor(() => expect(openRazorpayCheckout).toHaveBeenCalled());
    expect(verifyRazorpayPayment).not.toHaveBeenCalled();
    expect(push).not.toHaveBeenCalled();
  });
});
