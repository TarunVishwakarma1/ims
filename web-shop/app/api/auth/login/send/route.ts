import { NextResponse } from "next/server";
import { z } from "zod";

const reqSchema = z.object({
  phone: z.string().regex(/^\d{10}$/, "phone must be 10 digits"),
});

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  const parsed = reqSchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json({ error: "invalid_phone" }, { status: 400 });
  }

  const base = process.env.BACKEND_URL || "http://localhost:8080";
  let backendRes: Response;
  try {
    backendRes = await fetch(`${base}/api/shop/auth/otp/send`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ phone: parsed.data.phone, purpose: "login" }),
    });
  } catch {
    return NextResponse.json({ error: "send_failed" }, { status: 502 });
  }

  const payload = await backendRes.json().catch(() => ({}));
  return NextResponse.json(payload, { status: backendRes.status });
}
