package common

import "time"

type RequestLimits struct {
	WindowSeconds   uint64
	CurrentRequests map[time.Time]struct{}
	Requests        []time.Time `json:"requests,omitempty"`
	Threshold       uint64
}
