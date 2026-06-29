package shop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
)

// ShopNotifier builds and enqueues customer-facing emails for shop order
// events. Emails are queued (status=pending) for the notification worker to
// deliver with retries; enqueue failures are logged, never surfaced to the
// customer (a missing email must not fail an order).
//
// All methods no-op silently when the customer has no email on file.
type ShopNotifier struct {
	notif     repository.NotificationRepository
	customers repository.CustomerRepository
	orders    repository.OrderRepository
	webAppURL string
}

// NewShopNotifier wires the repositories the notifier needs. Any nil dependency
// disables notifications (methods become no-ops via the nil receiver guard).
func NewShopNotifier(
	notif repository.NotificationRepository,
	customers repository.CustomerRepository,
	orders repository.OrderRepository,
	webAppURL string,
) *ShopNotifier {
	return &ShopNotifier{notif: notif, customers: customers, orders: orders, webAppURL: webAppURL}
}

// rupees formats a paise amount as an Indian-rupee string, e.g. 12345 → "₹123.45".
func rupees(paise int64) string {
	return fmt.Sprintf("₹%d.%02d", paise/100, paise%100)
}

// recipientEmail returns the customer's email, or "" if absent/blank.
func (n *ShopNotifier) recipientEmail(ctx context.Context, customerID uuid.UUID) string {
	c, err := n.customers.GetByID(ctx, customerID)
	if err != nil || c == nil || c.Email == nil {
		return ""
	}
	return strings.TrimSpace(*c.Email)
}

// enqueue persists a pending email notification.
func (n *ShopNotifier) enqueue(ctx context.Context, to, subject, body string) {
	now := time.Now().UTC()
	err := n.notif.Enqueue(ctx, &domain.Notification{
		ID:          uuid.New(),
		Channel:     "email",
		Recipient:   to,
		Subject:     subject,
		BodyText:    body,
		Status:      domain.NotificationStatusPending,
		Attempts:    0,
		NextAttempt: now,
		CreatedAt:   now,
	})
	if err != nil {
		zap.L().Error("shop notifier enqueue failed",
			zap.String("recipient", to),
			zap.String("subject", subject),
			zap.Error(err))
	}
}

// orderLink returns a deep link to the order detail page, or "" if no web URL.
func (n *ShopNotifier) orderLink(orderID uuid.UUID) string {
	if n.webAppURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/orders/%s", strings.TrimRight(n.webAppURL, "/"), orderID)
}

// orderSummaryLines renders the item list + total for an order body.
func (n *ShopNotifier) orderSummaryLines(ctx context.Context, customerID, orderID uuid.UUID) (invoice string, total int64, items string, ok bool) {
	row, itemRows, err := n.orders.GetByCustomerAndID(ctx, customerID, orderID)
	if err != nil || row == nil {
		return "", 0, "", false
	}
	var b strings.Builder
	for _, it := range itemRows {
		fmt.Fprintf(&b, "  • %s × %d — %s\n", it.ProductName, it.Quantity, rupees(int64(it.Quantity)*it.UnitPrice))
	}
	inv := row.InvoiceNumber
	if inv == "" {
		inv = orderID.String()[:8]
	}
	return inv, row.TotalAmount, b.String(), true
}

// OrderConfirmed emails the customer that their order was placed.
func (n *ShopNotifier) OrderConfirmed(ctx context.Context, customerID, orderID uuid.UUID) {
	if n == nil {
		return
	}
	to := n.recipientEmail(ctx, customerID)
	if to == "" {
		return
	}
	inv, total, items, ok := n.orderSummaryLines(ctx, customerID, orderID)
	if !ok {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Thanks for your order!\n\nInvoice: %s\n\nItems:\n%s\nTotal: %s\n", inv, items, rupees(total))
	if link := n.orderLink(orderID); link != "" {
		fmt.Fprintf(&b, "\nTrack your order: %s\n", link)
	}
	n.enqueue(ctx, to, fmt.Sprintf("Order confirmed — %s", inv), b.String())
}

// OrderCancelled emails the customer that their order was cancelled.
func (n *ShopNotifier) OrderCancelled(ctx context.Context, customerID, orderID uuid.UUID) {
	if n == nil {
		return
	}
	to := n.recipientEmail(ctx, customerID)
	if to == "" {
		return
	}
	row, _, err := n.orders.GetByCustomerAndID(ctx, customerID, orderID)
	if err != nil || row == nil {
		return
	}
	inv := row.InvoiceNumber
	if inv == "" {
		inv = orderID.String()[:8]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Your order %s has been cancelled.\n\n", inv)
	if row.PaymentStatus == "paid" || row.PaymentStatus == "refunded" {
		fmt.Fprintf(&b, "A refund of %s will be processed to your original payment method within 5–7 business days.\n", rupees(row.TotalAmount))
	} else {
		b.WriteString("No payment was taken, so there is nothing to refund.\n")
	}
	n.enqueue(ctx, to, fmt.Sprintf("Order cancelled — %s", inv), b.String())
}

// PaymentReceived emails the customer a payment receipt.
func (n *ShopNotifier) PaymentReceived(ctx context.Context, customerID, orderID uuid.UUID) {
	if n == nil {
		return
	}
	to := n.recipientEmail(ctx, customerID)
	if to == "" {
		return
	}
	inv, total, _, ok := n.orderSummaryLines(ctx, customerID, orderID)
	if !ok {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "We've received your payment of %s for order %s.\n\nYour order is confirmed and being prepared.\n", rupees(total), inv)
	if link := n.orderLink(orderID); link != "" {
		fmt.Fprintf(&b, "\nView order: %s\n", link)
	}
	n.enqueue(ctx, to, fmt.Sprintf("Payment received — %s", inv), b.String())
}

// PaymentFailed emails the customer that a payment attempt failed, with a link
// back to retry. Fired from the Razorpay webhook on payment.failed.
func (n *ShopNotifier) PaymentFailed(ctx context.Context, customerID, orderID uuid.UUID) {
	if n == nil {
		return
	}
	to := n.recipientEmail(ctx, customerID)
	if to == "" {
		return
	}
	row, _, err := n.orders.GetByCustomerAndID(ctx, customerID, orderID)
	if err != nil || row == nil {
		return
	}
	inv := row.InvoiceNumber
	if inv == "" {
		inv = orderID.String()[:8]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "We couldn't process the payment for order %s.\n\nYour order is on hold. You can retry payment from your orders page.\n", inv)
	if link := n.orderLink(orderID); link != "" {
		fmt.Fprintf(&b, "\nRetry payment: %s\n", link)
	}
	n.enqueue(ctx, to, fmt.Sprintf("Payment failed — %s", inv), b.String())
}

// OrderStatusChanged emails the customer on shipped/delivered transitions.
// Wired for future use once an order status-update flow exists.
func (n *ShopNotifier) OrderStatusChanged(ctx context.Context, customerID, orderID uuid.UUID, status string) {
	if n == nil {
		return
	}
	to := n.recipientEmail(ctx, customerID)
	if to == "" {
		return
	}
	row, _, err := n.orders.GetByCustomerAndID(ctx, customerID, orderID)
	if err != nil || row == nil {
		return
	}
	inv := row.InvoiceNumber
	if inv == "" {
		inv = orderID.String()[:8]
	}
	var subject, line string
	switch status {
	case "shipped":
		subject, line = "Order shipped", "Your order is on its way."
	case "delivered":
		subject, line = "Order delivered", "Your order has been delivered. Enjoy!"
	default:
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nOrder: %s\n", line, inv)
	if link := n.orderLink(orderID); link != "" {
		fmt.Fprintf(&b, "\nView order: %s\n", link)
	}
	n.enqueue(ctx, to, fmt.Sprintf("%s — %s", subject, inv), b.String())
}
