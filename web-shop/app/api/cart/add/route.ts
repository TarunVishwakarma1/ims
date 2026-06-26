import { NextResponse } from "next/server";

/**
 * Cart add stub. Plan 3e replaces this with a real proxy to backend
 * POST /api/shop/cart/items.
 */
export async function POST() {
  return NextResponse.json({ error: "not_implemented" }, { status: 501 });
}
