package metering

import (
	"testing"
	"time"
)

func TestMeteringUtilFallbacksAndBounds(t *testing.T) {
	stamp := time.Date(2026, 6, 7, 12, 34, 56, 0, time.FixedZone("T", 8*3600))
	if got := dayStart(stamp); got.Hour() != 0 || got.Minute() != 0 || got.Location() != stamp.Location() {
		t.Fatalf("unexpected dayStart: %v", got)
	}
	if ternaryInt64(true, 1, 2) != 1 || ternaryInt64(false, 1, 2) != 2 {
		t.Fatalf("ternaryInt64 mismatch")
	}
	if firstNonEmpty("", "b", "c") != "b" || firstNonEmpty("", "") != "" {
		t.Fatalf("firstNonEmpty mismatch")
	}
	if minInt64(3, 4) != 3 || maxInt64(3, 4) != 4 {
		t.Fatalf("min/max mismatch")
	}
	if defaultString("", "fallback") != "fallback" || defaultString("value", "fallback") != "value" {
		t.Fatalf("defaultString mismatch")
	}
}
