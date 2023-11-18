package common

import "time"

type RequestLimits struct {
	WindowSeconds   uint64
	CurrentRequests map[time.Time]struct{}
	Threshold       uint64
}
