package sms

import "context"

type Sender interface {
	SendOTP(ctx context.Context, phone, code string) error
}

type SentMsg struct{ Phone, Code string }
