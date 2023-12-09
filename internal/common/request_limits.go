package common

import (
	"bytes"
	"compress/gzip"
	"io"
	"time"

	jsoniter "github.com/json-iterator/go"
)

type RequestLimits struct {
	WindowSeconds uint64
	Requests      *Requests `json:"requests_v2,omitempty"`
	Threshold     uint64
}

type Requests struct {
	Data  []uint32  `json:"data"`
	Start time.Time `json:"start"`
}

type RequestLimitData struct {
	RequestsCount uint64
	Threshold     uint64
}

func (r *RequestLimits) AddCounter(counterTime time.Time) {
	value := uint32(counterTime.Sub(r.Requests.Start).Seconds())
	if len(r.Requests.Data) > 0 {
		value -= r.Requests.Data[len(r.Requests.Data)-1]
	}

	r.Requests.Data = append(r.Requests.Data, value)
}

func (r *RequestLimits) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := jsoniter.NewEncoder(buf).Encode(r); err != nil {
		return nil, err
	}

	out := new(bytes.Buffer)
	gzipWriter := gzip.NewWriter(out)

	if _, err := io.Copy(gzipWriter, buf); err != nil {
		return nil, err
	}

	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func (r *RequestLimits) Unmarshal(data []byte) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if _, err = io.Copy(buf, gzipReader); err != nil {
		return err
	}

	return jsoniter.NewDecoder(buf).Decode(r)

}
