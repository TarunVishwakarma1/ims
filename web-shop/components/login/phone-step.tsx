"use client";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { toast } from "sonner";
import { useState } from "react";
import { phoneSchema, type PhoneInput } from "@/lib/login-schemas";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Props = {
  onSent: (otpId: string, phone: string) => void;
};

export function PhoneStep({ onSent }: Props) {
  const [submitting, setSubmitting] = useState(false);
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<PhoneInput>({
    resolver: zodResolver(phoneSchema),
    mode: "onSubmit",
  });

  async function onSubmit(data: PhoneInput) {
    setSubmitting(true);
    try {
      const res = await fetch("/api/auth/login/send", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ phone: data.phone }),
      });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) {
        if (body.error === "rate_limit") toast.error("Too many requests. Try later.");
        else if (body.error === "invalid_phone") toast.error("Invalid phone number.");
        else toast.error("Could not send code. Try again.");
        return;
      }
      onSent(body.otp_id, data.phone);
    } catch {
      toast.error("Network error. Try again.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <div>
        <label className="block text-sm font-medium mb-1">Phone number</label>
        <Input
          type="tel"
          inputMode="numeric"
          autoComplete="tel"
          placeholder="10-digit mobile"
          {...register("phone")}
        />
        {errors.phone && (
          <p className="text-sm text-red-600 mt-1">{errors.phone.message}</p>
        )}
      </div>
      <Button type="submit" disabled={submitting} className="w-full">
        {submitting ? "Sending…" : "Send OTP"}
      </Button>
    </form>
  );
}
