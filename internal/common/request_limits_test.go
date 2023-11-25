package common

import (
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequest_MarshalJSON(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		tt := time.Now()
		request := RequestLimits{
			WindowSeconds: 1000,
			Requests: []RequestTime{
				RequestTime(tt),
			},
			Threshold: 1010,
		}

		data, err := jsoniter.Marshal(&request)
		require.NoError(t, err)
		assert.Equal(t, `{"WindowSeconds":1000,"requests":["`+tt.UTC().Format(requestTimeFormat)+`"],"Threshold":1010}`, string(data))
		reqU := new(RequestLimits)
		assert.NoError(t, jsoniter.Unmarshal(data, &reqU))
		assert.Equal(t, tt.Unix(), time.Time(reqU.Requests[0]).Unix())
	})
	t.Run("old", func(t *testing.T) {
		tt := time.Now()
		data := []byte(`{"WindowSeconds":1000,"requests":["` + tt.UTC().Format(time.RFC3339Nano) + `"],"Threshold":1010}`)
		reqU := new(RequestLimits)
		assert.NoError(t, jsoniter.Unmarshal(data, &reqU))
		assert.Equal(t, tt.Unix(), time.Time(reqU.Requests[0]).Unix())
	})
}
