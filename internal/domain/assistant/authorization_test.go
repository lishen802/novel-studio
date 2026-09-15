package assistant

import (
	"errors"
	"testing"
	"time"
)

func TestAuthorizationActivateAndBudgetLifecycle(t *testing.T) {
	now := time.Unix(400, 0)
	auth := mustAuthorization(t, now)
	if auth.State != AuthPendingConfirmation {
		t.Fatalf("state = %q", auth.State)
	}
	active, err := auth.Activate(now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	reserved, err := active.ReserveTurn(now.Add(2 * time.Minute))
	if err != nil || reserved.ReservedCalls != 1 || reserved.ReservedTokens != 400 {
		t.Fatalf("reserve = %#v err=%v", reserved, err)
	}
	settled, err := reserved.SettleTurn(120, now.Add(3*time.Minute))
	if err != nil || settled.UsedCalls != 1 || settled.UsedTokens != 120 || settled.ReservedCalls != 0 {
		t.Fatalf("settle = %#v err=%v", settled, err)
	}
	if settled.RemainingCalls() != 1 {
		t.Fatalf("remaining calls = %d", settled.RemainingCalls())
	}
}

func TestAuthorizationDoesNotRefundUnknownOrProviderFailure(t *testing.T) {
	now := time.Unix(400, 0)
	active, err := mustAuthorization(t, now).Activate(now)
	if err != nil {
		t.Fatal(err)
	}
	reserved, err := active.ReserveTurn(now)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := reserved.KeepReservation(now.Add(time.Second))
	if err != nil || kept.ReservedCalls != 1 || kept.UsedCalls != 0 {
		t.Fatalf("unknown must keep reservation: %#v err=%v", kept, err)
	}
	released, err := reserved.ReleaseTurn(now)
	if err != nil || released.ReservedCalls != 0 {
		t.Fatalf("local failure may release: %#v err=%v", released, err)
	}
}

func TestAuthorizationRejectsStaleExpiredAndRevokedTurns(t *testing.T) {
	now := time.Unix(400, 0)
	active, err := mustAuthorization(t, now).Activate(now)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := active.MarkStale(now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stale.ReserveTurn(now); !errors.Is(err, ErrArgumentMismatch) {
		t.Fatalf("stale error = %v", err)
	}
	expired, err := mustAuthorization(t, now).Activate(now)
	if err != nil {
		t.Fatal(err)
	}
	expired, err = expired.Expire(now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := expired.ReserveTurn(now); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired error = %v", err)
	}
	revoked, err := mustAuthorization(t, now).Activate(now)
	if err != nil {
		t.Fatal(err)
	}
	revoked, err = revoked.Revoke(now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := revoked.ReserveTurn(now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("revoked error = %v", err)
	}
}

func TestAuthorizationExhaustsWhenBudgetCannotCoverNextTurn(t *testing.T) {
	now := time.Unix(400, 0)
	auth := mustAuthorization(t, now)
	auth.MaxCalls = 1
	active, err := auth.Activate(now)
	if err != nil {
		t.Fatal(err)
	}
	reserved, err := active.ReserveTurn(now)
	if err != nil {
		t.Fatal(err)
	}
	settled, err := reserved.SettleTurn(10, now)
	if err != nil || settled.State != AuthExhausted || settled.RemainingCalls() != 0 {
		t.Fatalf("exhausted = %#v err=%v", settled, err)
	}
	if _, err := settled.ReserveTurn(now); err == nil {
		t.Fatal("exhausted authorization must not start another turn")
	}
}

func TestAuthorizationExhaustsAsSoonAsCallsAreReserved(t *testing.T) {
	now := time.Unix(400, 0)
	auth := mustAuthorization(t, now)
	auth.MaxCalls = 1
	auth.MaxTotalTokens = 1000
	active, err := auth.Activate(now)
	if err != nil {
		t.Fatal(err)
	}
	reserved, err := active.ReserveTurn(now)
	if err != nil || reserved.State != AuthExhausted || reserved.RemainingCalls() != 0 {
		t.Fatalf("reserved authorization = %#v err=%v", reserved, err)
	}
}

func TestAuthorizationRecordsProviderOverageTruthfully(t *testing.T) {
	now := time.Unix(400, 0)
	active, err := mustAuthorization(t, now).Activate(now)
	if err != nil {
		t.Fatal(err)
	}
	reserved, err := active.ReserveTurn(now)
	if err != nil {
		t.Fatal(err)
	}
	settled, err := reserved.SettleTurn(450, now)
	if err != nil || settled.UsedTokens != 450 {
		t.Fatalf("settled authorization = %#v err=%v", settled, err)
	}
}

func TestAuthorizationMatchesFrozenTargetAndProfile(t *testing.T) {
	auth := mustAuthorization(t, time.Unix(400, 0))
	if !auth.MatchesTarget("cand-1", "abc", 2) || auth.MatchesTarget("cand-1", "abc", 3) {
		t.Fatal("candidate match mismatch")
	}
	if !auth.MatchesProfile("profile@3", "conn-1", "planner-model") || auth.MatchesProfile("profile@4", "conn-1", "planner-model") {
		t.Fatal("profile match mismatch")
	}
}

func mustAuthorization(t *testing.T, now time.Time) Authorization {
	t.Helper()
	auth, err := NewAuthorization(
		"auth-1", "session-1", "project-1", "branch-1", "action-1",
		"cand-1", "abc", "people_world", "character:hero", 2, []string{"/characters/@hero/motivation"},
		"profile@3", "conn-1", "test-provider", "planner-model", "low",
		2, 800, 400, now, now.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	return auth
}
