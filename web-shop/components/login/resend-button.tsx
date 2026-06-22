"use client";
import { useEffect, useState } from "react";

type Props = {
  onResend: () => Promise<void> | void;
  cooldownSec?: number;
};

export function ResendButton({ onResend, cooldownSec = 30 }: Props) {
  const [remaining, setRemaining] = useState(cooldownSec);

  useEffect(() => {
    if (remaining <= 0) return;
    const t = setInterval(() => setRemaining((r) => Math.max(0, r - 1)), 1000);
    return () => clearInterval(t);
  }, [remaining]);

  if (remaining > 0) {
    return <p className="text-sm text-muted">Resend code in {remaining}s</p>;
  }

  return (
    <button
      type="button"
      onClick={async () => {
        await onResend();
        setRemaining(cooldownSec);
      }}
      className="text-sm text-brand-600 hover:underline"
    >
      Resend code
    </button>
  );
}
