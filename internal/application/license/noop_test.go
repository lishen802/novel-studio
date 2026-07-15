package license_test

import (
	"context"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/license"
)

func TestNoopServiceAllowsEveryV1Capability(t *testing.T) {
	service := license.NoopService{}
	for _, capability := range []license.Capability{
		license.CreateProject,
		license.EditManuscript,
		license.ImportManuscript,
		license.RunAI,
	} {
		if err := service.Require(context.Background(), capability); err != nil {
			t.Fatalf("Require(%q) = %v", capability, err)
		}
	}
}
