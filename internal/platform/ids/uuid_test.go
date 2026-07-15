package ids

import (
	"testing"

	"github.com/google/uuid"
)

func TestUUIDGeneratorReturnsDistinctValidUUIDs(t *testing.T) {
	generator := UUIDGenerator{}

	first := generator.NewID()
	second := generator.NewID()
	if first == second {
		t.Fatalf("generated duplicate ID %q", first)
	}
	for _, value := range []string{first, second} {
		if _, err := uuid.Parse(value); err != nil {
			t.Fatalf("Parse(%q): %v", value, err)
		}
	}
}
