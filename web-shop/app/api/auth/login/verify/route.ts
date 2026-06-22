import { NextResponse } from "next/server";
import { z } from "zod";
import { shopSessionCookieOptions } from "@/lib/cookies";

const reqSchema = z.object({
  otp_id: z.string().uuid(),
  code: z.string().regex(/^\d{6}$/),
});

export async function POST(req: Request) {
  const body = await req.json().catch(() => ({}));
  const parsed = reqSchema.safeParse(body);
  if (!parsed.success) {
    return NextResponse.json({ error: "invalid_body" }, { status: 400 });
  }

  const base = process.env.BACKEND_URL || "http://localhost:8080";
  let backendRes: Response;
  try {
    backendRes = await fetch(`${base}/api/shop/auth/otp/verify`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(parsed.data),
    });
  } catch {
    return NextResponse.json({ error: "verify_failed" }, { status: 502 });
  }

  const payload = await backendRes.json().catch(() => ({}));
  if (!backendRes.ok) {
    return NextResponse.json(payload, { status: backendRes.status });
  }

  const token = (payload as { token?: string }).token;
  if (!token) {
    return NextResponse.json({ error: "verify_failed" }, { status: 500 });
  }

  const { name, ...cookieOpts } = shopSessionCookieOptions();
  const customer = (payload as { customer?: unknown }).customer ?? null;
  const res = NextResponse.json({ customer });
  res.cookies.set(name, token, cookieOpts);
  return res;
}
