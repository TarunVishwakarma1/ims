"use client";

import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { useCartStore } from "@/lib/cart-store";
import { fetchCart } from "@/lib/shop-api";
import { toast } from "sonner";

type Props = {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
};

type Stage = "phone" | "otp";

const PHONE_RE = /^[6-9]\d{9}$/;
const OTP_RE = /^\d{6}$/;

export function LoginModal({ open, onClose, onSuccess }: Props) {
  const [stage, setStage] = useState<Stage>("phone");
  const [phone, setPhone] = useState("");
  const [otpId, setOtpId] = useState("");
  const [otp, setOtp] = useState("");
  const [loading, setLoading] = useState(false);
  const phoneRef = useRef<HTMLInputElement>(null);
  const verifyInFlight = useRef(false);

  useEffect(() => {
    if (open) {
      setStage("phone");
      setPhone("");
      setOtpId("");
      setOtp("");
      requestAnimationFrame(() => phoneRef.current?.focus());
    }
  }, [open]);

  if (!open) return null;

  const sendOtp = async () => {
    if (!PHONE_RE.test(phone)) {
      toast.error("Enter a valid 10-digit phone number starting with 6–9");
      return;
    }
    setLoading(true);
    try {
      const r = await fetch("/api/auth/login/send", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone }),
      });
      if (!r.ok) {
        const body = await r.json().catch(() => ({})) as { error?: string };
        throw new Error(body.error ?? `send_failed_${r.status}`);
      }
      const data = await r.json() as { otp_id: string };
      setOtpId(data.otp_id);
      setStage("otp");
      toast.success("OTP sent");
    } catch {
      toast.error("Could not send OTP. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  const verifyOtp = async () => {
    if (!OTP_RE.test(otp)) {
      toast.error("Enter the 6-digit OTP");
      return;
    }
    // Block duplicate submits (double Enter / click) — a single-use OTP would
    // be consumed by the first call and the second would report otp_expired.
    if (verifyInFlight.current) return;
    verifyInFlight.current = true;
    setLoading(true);
    try {
      const r = await fetch("/api/auth/login/verify", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ otp_id: otpId, code: otp }),
      });
      if (!r.ok) {
        const body = await r.json().catch(() => ({})) as { error?: string };
        throw new Error(body.error ?? `verify_failed_${r.status}`);
      }

      // Cookie is now set by the proxy route. Sync cart.
      try {
        const localItems = useCartStore.getState().items;
        if (localItems.length > 0) {
          await useCartStore.getState().mergeOnLogin();
        } else {
          const cart = await fetchCart();
          useCartStore.getState().hydrateFromServer(cart);
        }
      } catch {
        // Cart sync failure is recoverable — user is logged in, continue.
      }

      onSuccess();
    } catch {
      toast.error("Invalid or expired OTP. Please try again.");
    } finally {
      setLoading(false);
      verifyInFlight.current = false;
    }
  };

  const resetToPhone = () => {
    setStage("phone");
    setOtpId("");
    setOtp("");
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Sign in"
      className="fixed inset-0 z-50 grid place-items-center bg-black/50"
    >
      <div className="bg-bg w-full max-w-sm rounded-lg shadow-xl p-6 relative">
        <button
          type="button"
          onClick={onClose}
          aria-label="Close sign in dialog"
          className="absolute top-3 right-3 size-8 grid place-items-center rounded-full hover:bg-brand-50"
        >
          <X className="size-4" />
        </button>

        <h2 className="text-lg font-semibold mb-1">Sign in to continue</h2>
        <p className="text-sm text-text-muted mb-4">
          {stage === "phone"
            ? "We'll send a one-time code to your mobile."
            : `Code sent to +91 ${phone}`}
        </p>

        {stage === "phone" ? (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              sendOtp();
            }}
            className="space-y-3"
          >
            <label className="block text-sm">
              Phone
              <input
                ref={phoneRef}
                type="tel"
                inputMode="numeric"
                maxLength={10}
                value={phone}
                onChange={(e) => setPhone(e.target.value.replace(/\D/g, ""))}
                placeholder="9876543210"
                className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg"
                required
              />
            </label>
            <button
              type="submit"
              disabled={loading}
              className="w-full h-10 rounded bg-brand-600 text-white disabled:opacity-60"
            >
              {loading ? "Sending…" : "Send OTP"}
            </button>
          </form>
        ) : (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              verifyOtp();
            }}
            className="space-y-3"
          >
            <label className="block text-sm">
              OTP
              <input
                type="text"
                inputMode="numeric"
                maxLength={6}
                value={otp}
                onChange={(e) => setOtp(e.target.value.replace(/\D/g, ""))}
                placeholder="123456"
                className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg text-center tracking-widest"
                autoFocus
                required
              />
            </label>
            <button
              type="submit"
              disabled={loading}
              className="w-full h-10 rounded bg-brand-600 text-white disabled:opacity-60"
            >
              {loading ? "Verifying…" : "Verify"}
            </button>
            <button
              type="button"
              onClick={resetToPhone}
              className="w-full text-sm text-text-muted hover:text-brand-600"
            >
              Use a different number
            </button>
          </form>
        )}
      </div>
    </div>
  );
}
