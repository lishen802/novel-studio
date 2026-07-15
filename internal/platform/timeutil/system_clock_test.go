package timeutil

import (
	"testing"
	"time"
)

func TestSystemClockReturnsCurrentUTCTime(t *testing.T) {
	before := time.Now().UTC()
	got := (SystemClock{}).Now()
	after := time.Now().UTC()

	if got.Before(before) || got.After(after) {
		t.Fatalf("Now() = %v, want between %v and %v", got, before, after)
	}
	if got.Location() != time.UTC {
		t.Fatalf("Location = %v, want UTC", got.Location())
	}
}
