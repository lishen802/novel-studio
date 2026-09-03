package storyasset

import (
	"errors"
	"testing"
	"time"
)

func TestNewEnvelopeBindsSourceAndTypedRow(t *testing.T) {
	now := time.Unix(400, 0)
	asset, err := New(
		"asset-1", "project-1", KindCharacter, OriginModel,
		"cand-1", "run-1", "art-1", "characters", "char-1", now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if asset.Status != StatusCanonical || asset.CurrentRevision != 1 {
		t.Fatalf("asset = %#v", asset)
	}
}

func TestProposalEnvelopeAllowsEmptyTypedTable(t *testing.T) {
	now := time.Unix(400, 0)
	asset, err := New("asset-1", "project-1", KindProposal, OriginModel, "cand-1", "run-1", "art-1", "", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if asset.TypedTable != "" || asset.TypedID != "" {
		t.Fatalf("proposal must not require typed row: %#v", asset)
	}
}

func TestCharacterEnvelopeRequiresTypedRow(t *testing.T) {
	now := time.Unix(400, 0)
	if _, err := New("asset-1", "project-1", KindCharacter, OriginModel, "cand-1", "run-1", "art-1", "", "", now); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("missing typed row error = %v", err)
	}
}

func TestMarkStaleDoesNotChangeRevision(t *testing.T) {
	asset := mustCharacter(t)
	stale, err := asset.MarkStale("upstream_proposal_changed", time.Unix(401, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !stale.Stale || stale.CurrentRevision != 1 || stale.StaleReason != "upstream_proposal_changed" {
		t.Fatalf("stale = %#v", stale)
	}
}

func mustCharacter(t *testing.T) Asset {
	t.Helper()
	asset, err := New("asset-1", "project-1", KindCharacter, OriginModel, "cand-1", "run-1", "art-1", "characters", "char-1", time.Unix(400, 0))
	if err != nil {
		t.Fatal(err)
	}
	return asset
}
