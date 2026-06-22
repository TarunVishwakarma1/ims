"use client";
import { Suspense, useState } from "react";
import { PhoneStep } from "@/components/login/phone-step";
import { CodeStep } from "@/components/login/code-step";

function LoginInner() {
  const [otpId, setOtpId] = useState<string | null>(null);
  const [phone, setPhone] = useState<string>("");

  async function resend() {
    if (!phone) return;
    await fetch("/api/auth/login/send", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ phone }),
    });
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
