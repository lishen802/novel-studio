package timeutil

import (
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

var _ ports.Clock = SystemClock{}
