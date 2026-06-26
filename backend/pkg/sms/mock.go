package sms

import (
	"context"
	"log"
	"sync"
)

type MockSender struct {
	mu   sync.Mutex
	Sent []SentMsg
	Err  error
}

func (m *MockSender) SendOTP(_ context.Context, phone, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.Sent = append(m.Sent, SentMsg{phone, code})
	// Dev convenience: print the code so UAT can paste it from the backend logs.
	log.Printf("[MockSender] OTP for %s = %s", phone, code)
	return nil
}
