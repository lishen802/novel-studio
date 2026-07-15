package license

import (
	"context"
)

type NoopService struct{}

func (NoopService) Require(context.Context, Capability) error {
	return nil
}
