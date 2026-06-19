package sms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMSG91_SendOTP_PostsExpectedBody(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authkey") != "AUTH" { t.Fatalf("missing authkey: %v", r.Header) }
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"type":"success"}`))
	}))
	defer srv.Close()

	s := newMSG91WithURL("AUTH", "TPL", "IMSHOP", srv.URL, &http.Client{Timeout: 5 * time.Second})
	if err := s.SendOTP(context.Background(), "+919999900401", "123456"); err != nil { t.Fatal(err) }
	if gotBody["template_id"] != "TPL" { t.Fatalf("unexpected body: %v", gotBody) }
	if gotBody["mobile"] != "919999900401" { t.Fatalf("mobile should be plain digits: %v", gotBody["mobile"]) }
	if gotBody["otp"] != "123456" { t.Fatalf("expected otp passthrough: %v", gotBody["otp"]) }
}
