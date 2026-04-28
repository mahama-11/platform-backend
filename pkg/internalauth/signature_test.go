package internalauth

import (
	"testing"

	"platform-service/pkg/platformconst"
)

func TestBuildSignVerifyAndHeaders(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	msg := BuildMessage("svc", "POST", "/internal", "123", body)
	if msg == "" {
		t.Fatalf("expected message")
	}
	sig := Sign("secret", "svc", "POST", "/internal", "123", body)
	if sig == "" {
		t.Fatalf("expected signature")
	}
	if !Verify("secret", sig, "svc", "POST", "/internal", "123", body) {
		t.Fatalf("expected signature verification to pass")
	}
	if Verify("bad-secret", sig, "svc", "POST", "/internal", "123", body) {
		t.Fatalf("expected signature verification to fail for bad secret")
	}
	headers := BuildHeaders("secret", "svc", "POST", "/internal", body)
	if headers[platformconst.HeaderInternalService] != "svc" || headers[platformconst.HeaderInternalSignature] == "" {
		t.Fatalf("unexpected headers: %+v", headers)
	}
}
