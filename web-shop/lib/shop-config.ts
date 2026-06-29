// Display-only shop constants. The backend remains authoritative for the
// actual charges applied at checkout; these drive cart-page nudges where no
// server summary is available (no address selected yet).
//
// Keep defaults in sync with backend config:
//   SHOP_FREE_SHIP_THRESHOLD_PAISE (default 50000 = ₹500)
//   SHOP_SHIPPING_FEE_PAISE        (default 4000  = ₹40)
export const FREE_SHIP_THRESHOLD_PAISE = Number(
  process.env.NEXT_PUBLIC_FREE_SHIP_THRESHOLD_PAISE ?? 50000,
);

export const SHIPPING_FEE_PAISE = Number(
  process.env.NEXT_PUBLIC_SHIPPING_FEE_PAISE ?? 4000,
);
