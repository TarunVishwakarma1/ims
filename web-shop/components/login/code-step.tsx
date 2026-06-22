"use client";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter, useSearchParams } from "next/navigation";
import { toast } from "sonner";
import { useState } from "react";
import { codeSchema, type CodeInput } from "@/lib/login-schemas";
import { safeNext } from "@/lib/safe-next";
import { Button } from "@/components/ui/button";
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from "@/components/ui/input-otp";
import { ResendButton } from "./resend-button";

type Props = {
  otpId: string;
  phone: string;
  onResend: () => Promise<boolean>;
  onChangeNumber: () => void;
};

export function CodeStep({ otpId, phone, onResend, onChangeNumber }: Props) {
  const router = useRouter();
  const params = useSearchParams();
  const [submitting, setSubmitting] = useState(false);
  const {
    control,
    handleSubmit,
    formState: { errors },
  } = useForm<CodeInput>({
    resolver: zodResolver(codeSchema),
    mode: "onSubmit",
  });

  async function onSubmit(data: CodeInput) {
    setSubmitting(true);
    try {
      const res = await fetch("/api/auth/login/verify", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ otp_id: otpId, code: data.code }),
      });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) {
        if (body.error === "invalid_code") toast.error("Wrong code. Try again.");
        else if (body.error === "otp_expired") toast.error("Code expired. Resend.");
        else if (body.error === "otp_locked")
          toast.error("Too many attempts. Wait 5 minutes.");
        else toast.error("Verification failed. Try again.");
        return;
      }
      router.replace(safeNext(params.get("next")));
    } catch {
      toast.error("Network error. Try again.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <p className="text-sm text-muted">
        Code sent to +91 {phone.slice(0, 2)}******{phone.slice(-2)}
      </p>

      <div>
        <label className="block text-sm font-medium mb-2">Enter 6-digit code</label>
        <Controller
          control={control}
          name="code"
          render={({ field }) => (
            <InputOTP
              maxLength={6}
              inputMode="numeric"
              autoComplete="one-time-code"
              value={field.value ?? ""}
              onChange={(v: string) => {
                field.onChange(v);
                if (v.length === 6) {
                  // Auto-submit when 6 digits entered.
                  void handleSubmit(onSubmit)();
                }
              }}
            >
              <InputOTPGroup>
                {[0, 1, 2, 3, 4, 5].map((i) => (
                  <InputOTPSlot key={i} index={i} />
                ))}
              </InputOTPGroup>
            </InputOTP>
          )}
        />
        {errors.code && (
          <p className="text-sm text-red-600 mt-1">{errors.code.message}</p>
        )}
      </div>

      <Button type="submit" disabled={submitting} className="w-full">
        {submitting ? "Verifying…" : "Verify"}
      </Button>

      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={onChangeNumber}
          className="text-sm text-muted hover:underline"
        >
          Change number
        </button>
        <ResendButton onResend={onResend} />
      </div>
    </form>
  );
}
