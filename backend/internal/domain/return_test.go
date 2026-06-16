package domain

import "testing"

func TestCanTransitionReturn(t *testing.T) {
	type tc struct {
		name string
		from ReturnStatus
		to   ReturnStatus
		want bool
	}
	tests := []tc{
		// Happy path: requested → approved → in_transit → received → refunded
		{"requested → approved", ReturnStatusRequested, ReturnStatusApproved, true},
		{"requested → rejected", ReturnStatusRequested, ReturnStatusRejected, true},
		{"approved → in_transit", ReturnStatusApproved, ReturnStatusInTransit, true},
		{"approved → rejected", ReturnStatusApproved, ReturnStatusRejected, true},
		{"in_transit → received", ReturnStatusInTransit, ReturnStatusReceived, true},
		{"received → refunded", ReturnStatusReceived, ReturnStatusRefunded, true},

		// Illegal jumps
		{"requested → in_transit (skip approval)", ReturnStatusRequested, ReturnStatusInTransit, false},
		{"requested → received", ReturnStatusRequested, ReturnStatusReceived, false},
		{"requested → refunded", ReturnStatusRequested, ReturnStatusRefunded, false},
		{"approved → received (skip transit)", ReturnStatusApproved, ReturnStatusReceived, false},
		{"approved → refunded", ReturnStatusApproved, ReturnStatusRefunded, false},
		{"in_transit → refunded", ReturnStatusInTransit, ReturnStatusRefunded, false},
		{"received → approved (regress)", ReturnStatusReceived, ReturnStatusApproved, false},

		// Terminal states
		{"rejected → anything", ReturnStatusRejected, ReturnStatusApproved, false},
		{"refunded → anything", ReturnStatusRefunded, ReturnStatusReceived, false},

		// Self-transition is a no-op, treated as illegal
		{"requested → requested", ReturnStatusRequested, ReturnStatusRequested, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanTransitionReturn(tt.from, tt.to); got != tt.want {
				t.Errorf("CanTransitionReturn(%q, %q) = %v, want %v",
					tt.from, tt.to, got, tt.want)
			}
		})
	}
}
