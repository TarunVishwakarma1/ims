"use client";
import { Suspense, useState } from "react";
import { toast } from "sonner";
import { PhoneStep } from "@/components/login/phone-step";
import { CodeStep } from "@/components/login/code-step";

function LoginInner() {
  const [otpId, setOtpId] = useState<string | null>(null);
  const [phone, setPhone] = useState<string>("");

  async function resend(): Promise<boolean> {
    if (!phone) return false;
    try {
      const res = await fetch("/api/auth/login/send", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone }),
      });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) {
        if (body.error === "rate_limit") toast.error("Too many requests. Try later.");
        else if (body.error === "invalid_phone") toast.error("Invalid phone number.");
        else toast.error("Could not resend code. Try again.");
        return false;
      }
      // Backend issues a fresh otp_id on resend — adopt it.
      if (body.otp_id) setOtpId(body.otp_id);
      toast.success("Code resent.");
      return true;
    } catch {
      toast.error("Network error. Try again.");
      return false;
    }
  }

  return (
    <div className="max-w-sm mx-auto space-y-6 py-12">
      <h1 className="text-2xl font-semibold">Sign in</h1>
      {otpId === null ? (
        <PhoneStep
          onSent={(id, p) => {
            setOtpId(id);
            setPhone(p);
          }}
        />
      ) : (
        <CodeStep
          otpId={otpId}
          phone={phone}
          onResend={resend}
          onChangeNumber={() => setOtpId(null)}
        />
      )}
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={<div className="max-w-sm mx-auto py-12">Loading…</div>}>
      <LoginInner />
    </Suspense>
  );
}
