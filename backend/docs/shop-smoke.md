# B2C Shop — Backend Smoke

Prereqs: `SHOP_ENABLED=true`, `SHOP_ORG_ID=<uuid>`, server up at :8080.
MSG91 not set → MockSender logs OTPs to stdout. Tail the backend logs with `Mock OTP sent` to find the code.

## Setup

Export the database URL and organization ID:
```bash
DATABASE_URL="postgres://ims:yourpassword@localhost:5432/ims_db?sslmode=disable"
SHOP_ORG_ID=$(psql "$DATABASE_URL" -tAc "SELECT id FROM organizations LIMIT 1;")
```

## 1. OTP login
```bash
OTP=$(curl -s -X POST http://localhost:8080/api/shop/auth/otp/send \
  -H 'Content-Type: application/json' \
  -d '{"phone":"+919999900000"}')
OTP_ID=$(echo "$OTP" | jq -r .otp_id)
# read the code from server logs
CODE=111111  # whatever MockSender printed
TOKEN=$(curl -s -X POST http://localhost:8080/api/shop/auth/otp/verify \
  -H 'Content-Type: application/json' \
  -d "{\"otp_id\":\"$OTP_ID\",\"code\":\"$CODE\"}" | jq -r .token)
echo "TOKEN=$TOKEN"
```

## 2. Add address
```bash
ADDR=$(curl -s -X POST http://localhost:8080/api/shop/addresses \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"line1":"Some St","city":"Mumbai","state":"MH","postal_code":"400001","is_default":true}')
ADDR_ID=$(echo "$ADDR" | jq -r .id)
```

## 3. Add to cart (use a real product_id from your `products` table where `shop_visible=true`)
```bash
curl -s -X POST http://localhost:8080/api/shop/cart/items \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"product_id":"<UUID>","qty":2}' | jq
```

## 4. Checkout summary
```bash
curl -s "http://localhost:8080/api/shop/checkout/summary?address_id=$ADDR_ID" \
  -H "Authorization: Bearer $TOKEN" | jq
```

## 5. Place order (COD)
```bash
curl -s -X POST http://localhost:8080/api/shop/checkout/place \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Idempotency-Key: smoke-$(date +%s)" \
  -H 'Content-Type: application/json' \
  -d "{\"address_id\":\"$ADDR_ID\",\"payment_method\":\"cod\"}" | jq
```
Expected response contains `order_id`, `invoice_number`, `payable_paise`.

## 6. Verify order row
```bash
psql "$DATABASE_URL" -c "SELECT id, customer_id, status, invoice_number, total_amount FROM orders WHERE customer_id IS NOT NULL ORDER BY created_at DESC LIMIT 1;"
```

## 7. Verify stock decremented
```bash
psql "$DATABASE_URL" -c "SELECT product_id, quantity FROM inventory WHERE product_id = '<UUID>';"
```

## 8. Browse the catalog (Plan 2a)

```bash
# Categories (cached 60s, gzip if Accept-Encoding: gzip).
curl -s -H 'Accept-Encoding: gzip' http://localhost:8080/api/shop/categories | gunzip | jq .

# Filter + sort.
curl -s 'http://localhost:8080/api/shop/products?category=snacks&sort=price_asc&in_stock=true&limit=12' | jq .

# Search.
curl -s 'http://localhost:8080/api/shop/products?search=biskut' | jq '.items[].name'

# Cursor pagination — copy next_cursor from prior response.
NEXT=$(curl -s 'http://localhost:8080/api/shop/products?sort=newest&limit=10' | jq -r .next_cursor)
curl -s "http://localhost:8080/api/shop/products?sort=newest&limit=10&cursor=$NEXT" | jq .

# Detail with ETag.
ETAG=$(curl -s -D - -o /dev/null http://localhost:8080/api/shop/products/<SLUG> | awk '/^ETag/ {print $2}' | tr -d '\r')
curl -s -o /dev/null -w '%{http_code}\n' -H "If-None-Match: $ETAG" http://localhost:8080/api/shop/products/<SLUG>
# → expect 304

# Infinite feed.
curl -s 'http://localhost:8080/api/shop/feed?seed_category=snacks&limit=12' | jq '{tier:.page_info.tier, count:(.items|length)}'
```

## 9. Banner CMS (Plan 2b)

```bash
# B2B admin token (assume B2B login already done; needs banners:manage permission for write ops).
ADMIN_TOKEN="<paste B2B JWT>"

# Upload banner image.
curl -s -X POST http://localhost:8080/api/admin/banners/upload \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -F "file=@/path/to/diwali.jpg"
# → {"image_url":"/uploads/banners/<uuid>.jpg"}

# Create banner.
curl -s -X POST http://localhost:8080/api/admin/banners \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Diwali Mega Sale",
    "subtitle": "Up to 50% off",
    "image_url": "/uploads/banners/<uuid>.jpg",
    "cta_label": "Shop now",
    "cta_link": "/snacks",
    "event_key": "diwali_2026",
    "starts_at": "2026-11-01T00:00:00Z",
    "ends_at":   "2026-11-09T23:59:59Z",
    "sort_order": 0,
    "is_hero": true,
    "audience_filter": "all"
  }' | jq .

# Publish it.
curl -s -X POST http://localhost:8080/api/admin/banners/<id>/publish \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# B2C consumer view.
curl -s http://localhost:8080/api/shop/banners/active | jq .

# Per-category filter.
curl -s 'http://localhost:8080/api/shop/banners/active?category=snacks' | jq .

# ETag round-trip.
ETAG=$(curl -s -D - -o /dev/null http://localhost:8080/api/shop/banners/active | awk '/^ETag/ {print $2}' | tr -d '\r')
curl -s -o /dev/null -w '%{http_code}\n' -H "If-None-Match: $ETAG" http://localhost:8080/api/shop/banners/active
# → expect 304
```

## 10. Order tracking (Plan 2c)

```bash
# Customer JWT from Plan 1 OTP flow (TOKEN already set).
# List my orders.
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/shop/orders | jq .

# Paginate.
NEXT=$(curl -s -H "Authorization: Bearer $TOKEN" 'http://localhost:8080/api/shop/orders?limit=10' | jq -r .next_cursor)
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/shop/orders?limit=10&cursor=$NEXT" | jq .

# Detail.
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/shop/orders/$ORDER_ID" | jq '{status, payment_status, items_count: (.items|length), cancellable}'

# Cancel (pending COD).
curl -s -X POST -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/shop/orders/$ORDER_ID/cancel" | jq .
# → {"status":"cancelled","refund_queued":false}

# Cancel (paid Razorpay).
curl -s -X POST -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/shop/orders/$PAID_ORDER_ID/cancel" | jq .
# → {"status":"cancelling","refund_queued":true,"estimated_days":7}

# After Razorpay refund webhook fires, status should flip:
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/shop/orders/$PAID_ORDER_ID" | jq '{status, payment_status}'
# → {"status":"cancelled","payment_status":"refunded"}
```
