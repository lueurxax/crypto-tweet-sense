package common

import (
	"strings"
	"time"
)

const requestTimeFormat = "02T15:04:05"

type RequestLimits struct {
	WindowSeconds uint64
	Requests      []RequestTime `json:"requests,omitempty"`
	RequestsV2    *Requests     `json:"requests_v2,omitempty"`
	Threshold     uint64
}

type Requests struct {
	Data  []int32   `json:"data"`
	Start time.Time `json:"start"`
}

type RequestTime time.Time

func (t *RequestTime) MarshalJSON() ([]byte, error) {
	return []byte(strings.Join([]string{"\"", time.Time(*t).UTC().Format(requestTimeFormat), "\""}, "")), nil
}

func (t *RequestTime) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")

	tt, err := time.Parse(requestTimeFormat, str)
	if err == nil {
		now := time.Now().UTC()

		tt = tt.AddDate(now.Year(), int(now.Month()-1), 0)
	} else {
		tt, err = time.Parse(time.RFC3339Nano, str)
		if err != nil {
			return err
		}
	}

	*t = RequestTime(tt)

	return nil
}

type RequestLimitData struct {
	RequestsCount uint64
	Threshold     uint64
}
