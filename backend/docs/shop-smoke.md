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
