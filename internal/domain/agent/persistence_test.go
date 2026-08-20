package agent

import (
	"strings"
	"testing"
	"time"
)

func TestBoundedStepPreviewPreservesUnicodeBoundary(t *testing.T) {
	content := strings.Repeat("章", MaxStepPreviewRunes+20)
	preview := BoundedStepPreview(content)
	if got := len([]rune(preview)); got != MaxStepPreviewRunes {
		t.Fatalf("preview runes = %d, want %d", got, MaxStepPreviewRunes)
	}
}

func TestAgentArtifactValidateVerifiesContentHash(t *testing.T) {
	artifact := AgentArtifact{
		ID: "artifact-1", RunID: "run-1", Kind: ArtifactDraft,
		Content: "正文", SHA256: SHA256Hex([]byte("其他正文")), CreatedAt: time.Now().UTC(),
	}
	if err := artifact.Validate(); err != ErrInvalidAgentPersistence {
		t.Fatalf("Validate error = %v, want ErrInvalidAgentPersistence", err)
	}
}

func TestContextManifestRequiresExplicitOmissionReason(t *testing.T) {
	item := ContextManifestItem{
		ManifestID: "manifest-1", SourceKind: ManifestStoryAsset, SourceID: "asset-1",
		SourceVersion: "1", ContentHash: SHA256Hex([]byte("设定")), Position: 1,
		Priority: 100, SelectionReason: "与章节目标相关", Included: false,
	}
	if err := item.Validate(); err != ErrInvalidAgentPersistence {
		t.Fatalf("Validate error = %v, want ErrInvalidAgentPersistence", err)
	}
	item.OmissionReason = "超出上下文预算"
	if err := item.Validate(); err != nil {
		t.Fatalf("Validate with omission reason: %v", err)
	}
}
