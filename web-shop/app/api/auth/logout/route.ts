import { NextResponse } from "next/server";
import { SHOP_SESSION_COOKIE } from "@/lib/cookies";

export async function POST() {
  const res = NextResponse.json({ ok: true });
  res.cookies.delete(SHOP_SESSION_COOKIE);
  return res;
}
