const SCRIPT_URL = "https://checkout.razorpay.com/v1/checkout.js";

declare global {
  interface Window {
    Razorpay?: new (opts: unknown) => { open: () => void };
  }
}

let loadPromise: Promise<void> | null = null;

export function loadRazorpay(): Promise<void> {
  if (typeof window === "undefined") return Promise.reject(new Error("ssr"));
  if (window.Razorpay) return Promise.resolve();
  if (loadPromise) return loadPromise;
  loadPromise = new Promise((resolve, reject) => {
    const s = document.createElement("script");
    s.src = SCRIPT_URL;
    s.async = true;
    s.onload = () => resolve();
    s.onerror = () => {
      loadPromise = null;
      reject(new Error("razorpay_load_failed"));
    };
    document.head.appendChild(s);
  });
  return loadPromise;
}

export type RazorpaySuccess = {
  razorpay_payment_id: string;
  razorpay_order_id: string;
  razorpay_signature: string;
};

export type OpenInput = {
  keyID: string;
  orderID: string;
  amount: number;
  name: string;
  prefill?: { name?: string; email?: string; contact?: string };
  onSuccess: (resp: RazorpaySuccess) => void;
  onDismiss: () => void;
};

export function openRazorpayCheckout(input: OpenInput): void {
  if (typeof window === "undefined" || !window.Razorpay) {
    throw new Error("razorpay_not_loaded");
  }
  const rzp = new window.Razorpay({
    key: input.keyID,
    order_id: input.orderID,
    amount: input.amount,
    currency: "INR",
    name: input.name,
    handler: (resp: RazorpaySuccess) => input.onSuccess(resp),
    modal: { ondismiss: () => input.onDismiss() },
    prefill: input.prefill,
    theme: { color: "#0f766e" },
  });
  rzp.open();
}
