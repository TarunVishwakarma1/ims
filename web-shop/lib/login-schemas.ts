import { z } from "zod";

export const phoneSchema = z.object({
  phone: z
    .string()
    .trim()
    .transform((v) => v.replace(/[^\d]/g, ""))
    .pipe(
      z
        .string()
        .regex(/^(?:91)?\d{10}$/, "Enter a valid 10-digit Indian mobile number")
        .transform((v) => (v.length === 12 ? v.slice(2) : v)),
    ),
});
export type PhoneInput = z.infer<typeof phoneSchema>;

export const codeSchema = z.object({
  code: z.string().regex(/^\d{6}$/, "Enter the 6-digit code"),
});
export type CodeInput = z.infer<typeof codeSchema>;
