package model

import (
	"bytes"
	"compress/gzip"
	"io"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

// 10000/sizeofint32 = 2500
const maxRequestsInBatch = 2500

type RequestLimits struct {
	WindowSeconds uint64
	Requests      *Requests `json:"requests_v2,omitempty"`
	Threshold     uint64
}

type Requests struct {
	Data  []int32   `json:"data"`
	Start time.Time `json:"start"`
}

func (r *RequestLimits) AddCounter(counterTime time.Time) {
	value := int32(counterTime.Sub(r.Requests.Start).Seconds())
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

func (r *RequestLimits) ToCommon() common.RequestLimitData {
	return common.RequestLimitData{
		RequestsCount: uint64(len(r.Requests.Data)),
		Threshold:     r.Threshold,
	}
}

func (r *RequestLimits) ToV2() *RequestLimitsV2 {
	requests := make([]RequestsV2, 0)

	for i := 0; i < len(r.Requests.Data); i += maxRequestsInBatch {
		start := r.Requests.Start.Add(time.Duration(r.Requests.Data[i]) * time.Second)

		nextLen := min(i+maxRequestsInBatch, len(r.Requests.Data)) - i
		data := make([]uint32, 0, nextLen)
		for _, v := range r.Requests.Data[i : i+nextLen] {
			data = append(data, uint32(v+r.Requests.Data[i]))
		}

		requests = append(requests, RequestsV2{
			Data:  data,
			Start: start,
		})
	}

	return &RequestLimitsV2{
		WindowSeconds: r.WindowSeconds,
		RequestsCount: uint32(len(r.Requests.Data)),
		Requests:      requests,
		Threshold:     r.Threshold,
	}
}

func (r *RequestLimits) CleanCounters() *Requests {
	window := time.Duration(r.WindowSeconds) * time.Second
	requestData := make([]int32, 0, len(r.Requests.Data))
	newStart := time.Now().Add(-window)

	counter := int32(0)

	for i, key := range r.Requests.Data {
		tt := r.Requests.Start.Add(time.Duration(key+counter) * time.Second)

		if time.Since(tt) < window {
			value := int32(tt.Sub(newStart).Seconds())
			requestData = append(requestData, value)
			if i != len(r.Requests.Data)-1 {
				requestData = append(requestData, r.Requests.Data[i+1:]...)
			}
			break
		}

		counter += key
	}

	return &Requests{
		Data:  requestData,
		Start: newStart,
	}
}

type RequestLimitsV2 struct {
	WindowSeconds uint64       `json:"window_seconds"`
	RequestsCount uint32       `json:"requests_count"`
	Requests      []RequestsV2 `json:"-"`
	Threshold     uint64       `json:"threshold"`
}

func (r *RequestLimitsV2) Marshal() ([]byte, error) {
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

func (r *RequestLimitsV2) Unmarshal(data []byte) error {
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

type RequestsV2 struct {
	Data  []uint32  `json:"data"`
	Start time.Time `json:"start"`
}

func (r *RequestsV2) Marshal() ([]byte, error) {
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

func (r *RequestsV2) Unmarshal(data []byte) error {
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
