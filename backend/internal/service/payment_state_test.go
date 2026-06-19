package service

import (
	"testing"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
)

// canTransition is the payment state machine. These tests pin its current
// behavior; changes here must be intentional (review whether the captured
// state machine in payment_service.go still matches).
func TestCanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		want     bool
	}{
		// Same-state is a no-op (idempotency dedupe catches it anyway, but
		// the machine should refuse to "transition" to itself).
		{"created → created", domain.PaymentStatusCreated, domain.PaymentStatusCreated, false},
		{"captured → captured", domain.PaymentStatusCaptured, domain.PaymentStatusCaptured, false},

		// created can go anywhere forward
		{"created → authorized", domain.PaymentStatusCreated, domain.PaymentStatusAuthorized, true},
		{"created → captured", domain.PaymentStatusCreated, domain.PaymentStatusCaptured, true},
		{"created → failed", domain.PaymentStatusCreated, domain.PaymentStatusFailed, true},

		// authorized → only captured or failed
		{"authorized → captured", domain.PaymentStatusAuthorized, domain.PaymentStatusCaptured, true},
		{"authorized → failed", domain.PaymentStatusAuthorized, domain.PaymentStatusFailed, true},
		{"authorized → refunded (illegal)", domain.PaymentStatusAuthorized, domain.PaymentStatusRefunded, false},

		// captured → only refunded
		{"captured → refunded", domain.PaymentStatusCaptured, domain.PaymentStatusRefunded, true},
		{"captured → failed (regress)", domain.PaymentStatusCaptured, domain.PaymentStatusFailed, false},
		{"captured → authorized (regress)", domain.PaymentStatusCaptured, domain.PaymentStatusAuthorized, false},

		// terminal states cannot transition
		{"failed → anything", domain.PaymentStatusFailed, domain.PaymentStatusCaptured, false},
		{"refunded → anything", domain.PaymentStatusRefunded, domain.PaymentStatusCaptured, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canTransition(tt.from, tt.to); got != tt.want {
				t.Errorf("canTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestIsAlreadyRefundedErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"unrelated error", &fakeErr{"network timeout"}, false},
		{"fully refunded variant", &fakeErr{"The payment has been fully refunded already"}, true},
		{"already refunded variant", &fakeErr{"refund: payment already refunded"}, true},
		{"already been refunded variant", &fakeErr{"This payment has already been refunded."}, true},
		{"case-insensitive match", &fakeErr{"PAYMENT FULLY REFUNDED"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAlreadyRefundedErr(tt.err); got != tt.want {
				t.Errorf("isAlreadyRefundedErr(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// fakeErr is a trivial error type for testing string-matching helpers
// without dragging in RazorPay's error shapes.
type fakeErr struct{ msg string }

func (e *fakeErr) Error() string { return e.msg }
