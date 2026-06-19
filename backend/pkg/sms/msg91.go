package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultMSG91URL = "https://control.msg91.com/api/v5/flow/"

type msg91 struct {
	authKey, templateID, senderID, url string
	hc                                 *http.Client
}

func NewMSG91(authKey, templateID, senderID string, hc *http.Client) Sender {
	return newMSG91WithURL(authKey, templateID, senderID, defaultMSG91URL, hc)
}

func newMSG91WithURL(authKey, templateID, senderID, url string, hc *http.Client) *msg91 {
	if hc == nil { hc = http.DefaultClient }
	return &msg91{authKey, templateID, senderID, url, hc}
}

func (m *msg91) SendOTP(ctx context.Context, phone, code string) error {
	mobile := strings.TrimPrefix(phone, "+")
	body := map[string]any{
		"template_id": m.templateID,
		"short_url":   "0",
		"mobile":      mobile,
		"otp":         code,
		"sender":      m.senderID,
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, m.url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authkey", m.authKey)

	resp, err := m.hc.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("msg91: status %d body %s", resp.StatusCode, string(b))
	}
	return nil
}
