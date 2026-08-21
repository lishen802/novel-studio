package agent

import (
	"errors"
	"testing"
)

func TestRevisionDirectiveSeparatesQualityAndAuthorModes(t *testing.T) {
	hash := SHA256Hex([]byte("base"))
	quality := RevisionDirective{Version: 1, Mode: RevisionQualityFindings, BaseArtifactID: "base", BaseDigest: hash, TargetRefs: []string{"paragraph:1"}, FindingIDs: []string{"finding-1"}}
	if err := quality.Validate(); err != nil {
		t.Fatalf("quality directive: %v", err)
	}
	author := RevisionDirective{Version: 1, Mode: RevisionAuthorDirected, BaseArtifactID: "base", BaseDigest: hash, TargetRefs: []string{"paragraph:1"}, PreserveRefs: []string{"paragraph:2"}, AuthorInstructionRef: "instruction-1"}
	if err := author.Validate(); err != nil {
		t.Fatalf("author directive: %v", err)
	}
	author.FindingIDs = []string{"finding-1"}
	if err := author.Validate(); !errors.Is(err, ErrInvalidRevisionDirective) {
		t.Fatalf("mixed modes error = %v", err)
	}
}

func TestRevisionDirectiveRejectsOverlappingTargetsAndPreserves(t *testing.T) {
	directive := RevisionDirective{Version: 1, Mode: RevisionAuthorDirected, BaseArtifactID: "base", BaseDigest: SHA256Hex([]byte("base")), TargetRefs: []string{"paragraph:1"}, PreserveRefs: []string{"paragraph:1"}, AuthorInstructionRef: "instruction"}
	if err := directive.Validate(); !errors.Is(err, ErrInvalidRevisionDirective) {
		t.Fatalf("Validate() error = %v", err)
	}
}
