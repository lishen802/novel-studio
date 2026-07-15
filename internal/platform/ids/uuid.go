package ids

import (
	"github.com/google/uuid"
	"github.com/lishen802/novel-studio/internal/application/ports"
)

type UUIDGenerator struct{}

func (UUIDGenerator) NewID() string {
	return uuid.NewString()
}

var _ ports.IDGenerator = UUIDGenerator{}
