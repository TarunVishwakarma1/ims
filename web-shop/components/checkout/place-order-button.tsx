"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { useCartStore } from "@/lib/cart-store";
import { placeOrder, verifyRazorpayPayment } from "@/lib/shop-api";
import { loadRazorpay, openRazorpayCheckout } from "@/lib/razorpay";

type Props = {
  addressID: string;
  paymentMethod: "razorpay" | "cod";
  customerName?: string;
  customerPhone?: string;
  disabled?: boolean;
};

function newIdemKey(): string {
  return crypto.randomUUID();
}

export function PlaceOrderButton({ addressID, paymentMethod, customerName, customerPhone, disabled }: Props) {
  const [busy, setBusy] = useState(false);
  const clear = useCartStore((s) => s.clear);
  const router = useRouter();

  const onClick = async () => {
    if (!addressID) {
      toast.error("Select an address");
      return;
    }
    setBusy(true);
    try {
      const idem = newIdemKey();
      const res = await placeOrder({ address_id: addressID, payment_method: paymentMethod }, idem);

      if (paymentMethod === "cod") {
        clear();
        router.push(`/orders/${res.order_id}?placed=1`);
        return;
      }

      // Razorpay branch
      if (!res.razorpay_order_id || !res.razorpay_key_id) {
        toast.error("Payment setup failed");
        return;
      }

      await loadRazorpay();
      openRazorpayCheckout({
        keyID: res.razorpay_key_id,
        orderID: res.razorpay_order_id,
        amount: res.payable_paise,
        name: "Shop",
        prefill: { name: customerName, contact: customerPhone },
        onSuccess: async (rzp) => {
          try {
            await verifyRazorpayPayment({
              order_id: res.order_id,
              razorpay_order_id: rzp.razorpay_order_id,
              razorpay_payment_id: rzp.razorpay_payment_id,
              razorpay_signature: rzp.razorpay_signature,
            });
          } catch (e) {
            const code = (e as { code?: string }).code;
            if (code !== "already_paid") {
              toast.warning("Payment recorded — confirmation pending");
            }
          }
          clear();
          router.push(`/orders/${res.order_id}?placed=1`);
        },
        onDismiss: () => {
          setBusy(false);
          toast.warning("Payment incomplete — order pending in My Orders");
        },
      });
    } catch (e) {
      const code = (e as { code?: string }).code;
      if (code === "stock_unavailable") toast.error("Item out of stock");
      else if (code === "cod_ineligible") toast.error("COD not available for this order total");
      else if (code === "address_required") toast.error("Address not valid");
      else toast.error("Could not place order");
    } finally {
      // Only reset busy here for non-Razorpay or error paths; Razorpay handler resets itself.
      if (paymentMethod === "cod") setBusy(false);
    }
  };

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled || busy}
      className="w-full h-11 rounded bg-brand-600 text-white font-medium disabled:opacity-60 mt-3"
    >
      {busy ? "Processing…" : paymentMethod === "cod" ? "Place order (COD)" : "Pay & place order"}
    </button>
  );
}
