package common

import (
	"time"
)

type RequestLimits struct {
	WindowSeconds uint64
	Requests      *Requests `json:"requests_v2,omitempty"`
	Threshold     uint64
}

type Requests struct {
	Data  []int32   `json:"data"`
	Start time.Time `json:"start"`
}

type RequestLimitData struct {
	RequestsCount uint64
	Threshold     uint64
}
