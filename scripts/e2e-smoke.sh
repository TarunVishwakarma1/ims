#!/usr/bin/env bash
# End-to-end smoke test for the Kirana seller + shop-directory flow.
#
# Zero interaction: run it against a live backend and it drives the whole
# seller path over HTTP, asserting the important functionality shipped in the
# P4 shop work — onboarding, product storefront overlay, go-live, the shop
# directory (pincode + geofencing), business-hours open/closed, and analytics.
#
#   ./scripts/e2e-smoke.sh                      # against http://localhost:8080
#   BASE_URL=http://host:8080 ./scripts/e2e-smoke.sh
#
# Exit code 0 = all checks passed, 1 = at least one failed.

set -uo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
TS=$(date +%s)
PASS=0
FAIL=0

say()  { printf '\n\033[1m== %s\033[0m\n' "$1"; }
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; PASS=$((PASS+1)); }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$1"; FAIL=$((FAIL+1)); }

# check <desc> <actual> <expected>
check() { if [ "$2" = "$3" ]; then ok "$1 ($2)"; else bad "$1 — got '$2', want '$3'"; fi; }

# JSON scalar by key (string or number), first match. No jq dependency.
jget() { grep -o "\"$2\":[^,}]*" | head -1 | sed 's/^[^:]*://;s/^"//;s/"$//'; }

# 1-minute business-hours window one hour from now (IST) → shop is CLOSED now.
CLOSED_OPEN=$(TZ=Asia/Kolkata date -v+1H +%H:%M 2>/dev/null || TZ=Asia/Kolkata date -d '+1 hour' +%H:%M)
CLOSED_CLOSE=$(TZ=Asia/Kolkata date -v+1H -v+1M +%H:%M 2>/dev/null || TZ=Asia/Kolkata date -d '+1 hour 1 minute' +%H:%M)

say "Backend reachable"
HEALTH=$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/health")
check "GET /health" "$HEALTH" "200"
[ "$HEALTH" = "200" ] || { echo "backend not up at $BASE_URL"; exit 1; }

say "Seller onboarding — register"
SLUG="smoke-$TS"
PIN=$(printf '5%05d' $(( TS % 100000 )))         # a 6-digit pincode
REG=$(curl -s -X POST "$BASE_URL/api/auth/register" -H 'Content-Type: application/json' \
  -d "{\"org_name\":\"Smoke Shop $TS\",\"org_slug\":\"$SLUG\",\"user_name\":\"Smoke Seller\",\"email\":\"smoke-$TS@example.com\",\"password\":\"Smoke@Pass12345\"}")
TOKEN=$(printf '%s' "$REG" | jget x access_token)
ORG_ID=$(printf '%s' "$REG" | jget x org_id)
EMAIL="smoke-$TS@example.com"
if [ -n "$TOKEN" ]; then ok "registered seller, got access token"; else bad "no access token: $REG"; exit 1; fi
AUTH=(-H "Authorization: Bearer $TOKEN")

say "Create a category + product"
CAT=$(curl -s -X POST "$BASE_URL/api/categories" "${AUTH[@]}" -H 'Content-Type: application/json' \
  -d '{"name":"Smoke Snacks","description":"smoke"}')
CAT_ID=$(printf '%s' "$CAT" | jget x id)
[ -n "$CAT_ID" ] && ok "category created" || bad "category create failed: $CAT"

PROD=$(curl -s -X POST "$BASE_URL/api/products" "${AUTH[@]}" -H 'Content-Type: application/json' \
  -d "{\"category_id\":\"$CAT_ID\",\"name\":\"Smoke Chips\",\"sku\":\"SMOKE-$TS\",\"price\":5000,\"gst_rate\":5}")
PROD_ID=$(printf '%s' "$PROD" | jget x id)
[ -n "$PROD_ID" ] && ok "product created" || bad "product create failed: $PROD"

say "Product storefront overlay (visibility + shop price)"
PS_CODE=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$BASE_URL/api/products/$PROD_ID/storefront" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d "{\"shop_visible\":true,\"shop_slug\":\"smoke-chips-$TS\",\"shop_price_paise\":4500,\"shop_description\":\"crispy\",\"shop_image_urls\":[\"https://picsum.photos/seed/smoke/400\"]}")
check "PUT product storefront" "$PS_CODE" "200"
PS_GET=$(curl -s "$BASE_URL/api/products/$PROD_ID/storefront" "${AUTH[@]}")
check "GET reflects shop_visible" "$(printf '%s' "$PS_GET" | jget x shop_visible)" "true"
check "GET reflects shop_price_paise" "$(printf '%s' "$PS_GET" | jget x shop_price_paise)" "4500"

say "Product storefront — visible requires a slug (422)"
PS_BAD=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$BASE_URL/api/products/$PROD_ID/storefront" "${AUTH[@]}" \
  -H 'Content-Type: application/json' -d '{"shop_visible":true,"shop_slug":""}')
check "PUT visible w/o slug rejected" "$PS_BAD" "422"

say "Storefront profile + go live (open hours, location, radius)"
LIVE_CODE=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$BASE_URL/api/admin/storefront" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d "{\"slug\":\"$SLUG\",\"display_name\":\"Smoke Shop\",\"tagline\":\"t\",\"logo_url\":\"\",\"area\":\"Koregaon Park\",\"city\":\"Pune\",\"pincodes\":[\"$PIN\"],\"lat\":18.5204,\"lng\":73.8567,\"delivery_radius_km\":5,\"opens_at\":null,\"closes_at\":null,\"is_live\":true}")
check "PUT storefront go-live" "$LIVE_CODE" "200"

say "Go-live guard — reject when missing pincode (422)"
GUARD=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$BASE_URL/api/admin/storefront" "${AUTH[@]}" \
  -H 'Content-Type: application/json' \
  -d "{\"slug\":\"$SLUG\",\"display_name\":\"Smoke Shop\",\"tagline\":\"\",\"logo_url\":\"\",\"area\":\"\",\"city\":\"\",\"pincodes\":[],\"lat\":18.5204,\"lng\":73.8567,\"delivery_radius_km\":5,\"opens_at\":null,\"closes_at\":null,\"is_live\":true}")
check "go-live w/o pincode rejected" "$GUARD" "422"

say "Directory — shop appears by pincode, open now"
DIR=$(curl -s "$BASE_URL/api/shop/shops?pincode=$PIN")
printf '%s' "$DIR" | grep -q "\"slug\":\"$SLUG\"" && ok "shop listed for pincode $PIN" || bad "shop missing from pincode directory: $DIR"
printf '%s' "$DIR" | grep -q '"is_open":true' && ok "shop is_open=true (no hours set)" || bad "is_open not true: $DIR"

say "Geofencing — near me finds it, far does not"
NEAR=$(curl -s "$BASE_URL/api/shop/shops?lat=18.5250&lng=73.8600")
printf '%s' "$NEAR" | grep -q "\"slug\":\"$SLUG\"" && ok "shop found ~0.6km away (within 5km radius)" || bad "near-me missed the shop: $NEAR"
printf '%s' "$NEAR" | grep -q '"distance_km"' && ok "distance_km present on near-me result" || bad "no distance_km"
FAR=$(curl -s "$BASE_URL/api/shop/shops?lat=19.5000&lng=73.8567")
if printf '%s' "$FAR" | grep -q "\"slug\":\"$SLUG\""; then bad "shop wrongly returned ~108km away"; else ok "shop excluded far outside its radius"; fi

say "Business hours — closed window flips is_open to false"
curl -s -o /dev/null -X PUT "$BASE_URL/api/admin/storefront" "${AUTH[@]}" -H 'Content-Type: application/json' \
  -d "{\"slug\":\"$SLUG\",\"display_name\":\"Smoke Shop\",\"tagline\":\"\",\"logo_url\":\"\",\"area\":\"\",\"city\":\"\",\"pincodes\":[\"$PIN\"],\"lat\":18.5204,\"lng\":73.8567,\"delivery_radius_km\":5,\"opens_at\":\"$CLOSED_OPEN\",\"closes_at\":\"$CLOSED_CLOSE\",\"is_live\":true}"
DIR2=$(curl -s "$BASE_URL/api/shop/shops?pincode=$PIN")
printf '%s' "$DIR2" | grep -q '"is_open":false' && ok "shop is_open=false during closed window" || bad "is_open not false with closed hours: $DIR2"

say "Analytics — endpoint returns a summary"
AN=$(curl -s "$BASE_URL/api/admin/shop/analytics?days=30" "${AUTH[@]}")
check "analytics orders (fresh shop)" "$(printf '%s' "$AN" | jget x orders)" "0"
printf '%s' "$AN" | grep -q '"by_day"' && ok "analytics has by_day series" || bad "no by_day: $AN"

# ── Money path (checkout, closed-shop block, seller alert) ───────────────────
# Needs a minted shop-customer JWT + local psql to seed a customer/inventory.
# Gated: skips cleanly (no failure) when JWT_SECRET or docker psql are absent.
JWT_SECRET="${JWT_SECRET:-$(grep -m1 '^JWT_SECRET=' backend/.env.local 2>/dev/null | cut -d= -f2- | tr -d '"'"'"'')}"
psql_x() { docker compose exec -T postgres psql -U ims -d ims_db -tA -c "$1" 2>/dev/null; }

if [ -z "$JWT_SECRET" ] || ! command -v python3 >/dev/null || ! psql_x "SELECT 1" >/dev/null 2>&1; then
  say "Money path — SKIPPED (needs JWT_SECRET + local docker psql + python3)"
else
  say "Money path — checkout, closed-shop block, seller alert"

  # Seed inventory + a guest customer; mint a shop JWT for it.
  CUST_PHONE="+919$(printf '%09d' $(( TS % 1000000000 )))"
  psql_x "INSERT INTO inventory (org_id, product_id, quantity, low_stock_threshold) VALUES ('$ORG_ID','$PROD_ID',50,5) ON CONFLICT (product_id) DO UPDATE SET quantity=50" >/dev/null
  CID=$(psql_x "INSERT INTO customers (name, phone, is_guest) VALUES ('Smoke Cust','$CUST_PHONE',true) RETURNING id" | head -1 | tr -d '[:space:]')
  [ -n "$CID" ] && ok "seeded customer + inventory" || bad "customer seed failed"

  CTOKEN=$(python3 - "$CID" "$JWT_SECRET" <<'PY'
import sys, hmac, hashlib, base64, json, time
cid, secret = sys.argv[1], sys.argv[2]
b64 = lambda b: base64.urlsafe_b64encode(b).rstrip(b'=')
h = b64(json.dumps({"alg":"HS256","typ":"JWT"}).encode())
p = b64(json.dumps({"sub":cid,"aud":"shop","iat":int(time.time()),"exp":int(time.time())+3600}).encode())
sig = b64(hmac.new(secret.encode(), h+b'.'+p, hashlib.sha256).digest())
print((h+b'.'+p+b'.'+sig).decode())
PY
)
  CA=(-H "Authorization: Bearer $CTOKEN")
  ME=$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/api/shop/me" "${CA[@]}")
  check "shop JWT accepted (GET /me)" "$ME" "200"

  ADDR=$(curl -s -X POST "$BASE_URL/api/shop/addresses" "${CA[@]}" -H 'Content-Type: application/json' \
    -d "{\"name\":\"Smoke Cust\",\"phone\":\"$CUST_PHONE\",\"line1\":\"1 Test St\",\"city\":\"Pune\",\"state\":\"MH\",\"pincode\":\"$PIN\"}")
  ADDR_ID=$(printf '%s' "$ADDR" | jget x id)
  [ -n "$ADDR_ID" ] && ok "address created" || bad "address create failed: $ADDR"

  # qty 3 × ₹45 = ₹135, above the COD minimum.
  ADD=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/shop/s/$SLUG/cart/items" "${CA[@]}" \
    -H 'Content-Type: application/json' -d "{\"product_id\":\"$PROD_ID\",\"qty\":3}")
  check "add visible product to cart" "$ADD" "200"

  # Shop is still CLOSED (from the hours section) → placement must be blocked.
  PLACE_CLOSED=$(curl -s -X POST "$BASE_URL/api/shop/checkout/place" "${CA[@]}" -H 'Content-Type: application/json' \
    -d "{\"address_id\":\"$ADDR_ID\",\"payment_method\":\"cod\"}")
  printf '%s' "$PLACE_CLOSED" | grep -q '"shop_closed"' && ok "checkout blocked while shop closed (shop_closed)" || bad "closed shop did not block: $PLACE_CLOSED"

  # Reopen (clear hours) and place for real.
  curl -s -o /dev/null -X PUT "$BASE_URL/api/admin/storefront" "${AUTH[@]}" -H 'Content-Type: application/json' \
    -d "{\"slug\":\"$SLUG\",\"display_name\":\"Smoke Shop\",\"tagline\":\"\",\"logo_url\":\"\",\"area\":\"\",\"city\":\"\",\"pincodes\":[\"$PIN\"],\"lat\":18.5204,\"lng\":73.8567,\"delivery_radius_km\":5,\"opens_at\":null,\"closes_at\":null,\"is_live\":true}"
  PLACE=$(curl -s -X POST "$BASE_URL/api/shop/checkout/place" "${CA[@]}" -H 'Content-Type: application/json' \
    -d "{\"address_id\":\"$ADDR_ID\",\"payment_method\":\"cod\"}")
  ORDER_ID=$(printf '%s' "$PLACE" | jget x order_id)
  [ -n "$ORDER_ID" ] && ok "COD order placed when open" || bad "place failed: $PLACE"

  say "Order shows in analytics + seller was alerted"
  AN2=$(curl -s "$BASE_URL/api/admin/shop/analytics?days=30" "${AUTH[@]}")
  check "analytics now shows 1 order" "$(printf '%s' "$AN2" | jget x orders)" "1"
  NOTIF=$(psql_x "SELECT count(*) FROM notifications WHERE recipient='$EMAIL' AND subject LIKE 'New order%'" | head -1 | tr -d '[:space:]')
  if [ "${NOTIF:-0}" -ge 1 ] 2>/dev/null; then ok "seller new-order email queued ($NOTIF)"; else bad "no seller notification for $EMAIL (got '$NOTIF')"; fi

  # Tidy the seeded customer (its cart cascades off it). The throwaway "smoke-*"
  # org is left in place — it has orders/carts that block a plain delete, and a
  # leftover demo shop is harmless (and handy to inspect). Purge accumulated
  # ones when needed with scripts/e2e-smoke-clean.sh.
  psql_x "DELETE FROM customers WHERE id='$CID'" >/dev/null 2>&1
fi

printf '\n\033[1m== Result: %d passed, %d failed ==\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
