package consensus

import (
	"time"
)

type status struct {
	from     uint64
	latest   uint64
	lastChat time.Time
}
