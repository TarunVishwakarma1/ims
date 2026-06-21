import { NextResponse } from "next/server";

/**
 * POST /api/auth/login — OTP verify proxy. Plan 3b implements the body:
 *  1. Accept { otp_id, code }.
 *  2. POST /api/shop/auth/otp/verify on backend.
 *  3. On success, Set-Cookie shop_session=<jwt> httpOnly + return { customer }.
 *  4. On failure, return backend error verbatim.
 */
export async function POST() {
  return NextResponse.json({ error: "not_implemented" }, { status: 501 });
}
