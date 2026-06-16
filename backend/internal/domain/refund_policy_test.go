package domain

import "testing"

func TestCancellationRefundPercent(t *testing.T) {
	tests := []struct {
		name   string
		status OrderStatus
		want   int
	}{
		{"pending is full refund", OrderStatusPending, 100},
		{"confirmed is full refund", OrderStatusConfirmed, 100},
		{"accepted is full refund", OrderStatusAccepted, 100},
		{"processing has restocking fee", OrderStatusProcessing, 75},
		{"ready is half refund", OrderStatusReady, 50},
		{"shipped not covered", OrderStatusShipped, 0},
		{"delivered not covered", OrderStatusDelivered, 0},
		{"cancelled not covered", OrderStatusCancelled, 0},
		{"unknown not covered", OrderStatus("garbage"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CancellationRefundPercent(tt.status); got != tt.want {
				t.Errorf("CancellationRefundPercent(%q) = %d, want %d", tt.status, got, tt.want)
			}
		})
	}
}

func TestApplyRefundPercent(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		percent int
		want    int64
	}{
		{"100% of nothing", 0, 100, 0},
		{"100% returns full amount", 100_00, 100, 100_00},
		{"75% rounds down", 100_00, 75, 75_00},
		{"50% half", 200_00, 50, 100_00},
		{"non-divisible rounds down", 9999, 75, 7499}, // 9999*75/100 = 7499.25 → 7499
		{"negative percent → 0", 100_00, -10, 0},
		{"zero percent → 0", 100_00, 0, 0},
		{"over 100 clamped to full", 100_00, 150, 100_00},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ApplyRefundPercent(tt.amount, tt.percent); got != tt.want {
				t.Errorf("ApplyRefundPercent(%d, %d) = %d, want %d", tt.amount, tt.percent, got, tt.want)
			}
		})
	}
}

func TestCancellationPolicyReason_NonEmpty(t *testing.T) {
	// Spot-check that every recognized status maps to a non-empty reason.
	// Avoids regressions where someone adds a status but forgets a case.
	covered := []OrderStatus{
		OrderStatusPending, OrderStatusConfirmed, OrderStatusAccepted,
		OrderStatusProcessing, OrderStatusReady,
	}
	for _, s := range covered {
		if msg := CancellationPolicyReason(s); msg == "" {
			t.Errorf("CancellationPolicyReason(%q) returned empty", s)
		}
	}
}
