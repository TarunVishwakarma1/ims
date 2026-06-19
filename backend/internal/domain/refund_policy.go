package domain

// CancellationRefundPercent returns the percentage of the captured payment
// that should be refunded when an order in `status` is cancelled. Models
// real-world platform policies (Zomato/Swiggy/Blinkit/Zepto/Flipkart):
// the further into fulfillment, the smaller the refund. Statuses not
// covered (shipped/delivered/completed) are not user-cancellable from the
// UI — they require the return-merchandise flow we haven't built yet.
//
//	pending     → 100%  — supplier hasn't even seen it
//	confirmed   → 100%
//	accepted    → 100%
//	processing  →  75%  — supplier started picking/packing; 25% restocking
//	ready       →  50%  — packed and waiting for handoff; sunk cost
//
// Returns 0 for any status outside the policy (caller decides what to do).
func CancellationRefundPercent(status OrderStatus) int {
	switch status {
	case OrderStatusPending, OrderStatusConfirmed, OrderStatusAccepted:
		return 100
	case OrderStatusProcessing:
		return 75
	case OrderStatusReady:
		return 50
	default:
		return 0
	}
}

// CancellationPolicyReason returns a human-readable explanation for the
// refund percent — used in toasts / dialogs / audit logs.
func CancellationPolicyReason(status OrderStatus) string {
	switch status {
	case OrderStatusPending, OrderStatusConfirmed, OrderStatusAccepted:
		return "full refund — order not yet started"
	case OrderStatusProcessing:
		return "75% refund — supplier began preparing (25% restocking fee)"
	case OrderStatusReady:
		return "50% refund — order packed and ready for dispatch"
	default:
		return "no refund policy for status " + string(status)
	}
}

// ApplyRefundPercent computes the refund amount in the smallest currency
// unit (paise) for a captured payment of `paidAmount`. Rounds DOWN to
// favor the platform on fractional paise (consistent with RazorPay).
func ApplyRefundPercent(paidAmount int64, percent int) int64 {
	if percent <= 0 {
		return 0
	}
	if percent >= 100 {
		return paidAmount
	}
	return (paidAmount * int64(percent)) / 100
}

// CancelPreview is the canonical answer to "what happens if I cancel
// this order right now?" — returned to the UI so the cancel dialog can
// surface the exact refund amount the backend will apply.
type CancelPreview struct {
	Eligible     bool        `json:"eligible"`      // false → button should be disabled
	Status       OrderStatus `json:"status"`        // current order status
	PaymentPaid  bool        `json:"payment_paid"`  // was the order ever paid
	RefundPercent int        `json:"refund_percent"`
	RefundAmount int64       `json:"refund_amount"` // 0 if unpaid
	Reason       string      `json:"reason"`        // human-readable policy line
	Blocked      string      `json:"blocked,omitempty"` // reason cancel not allowed
}
