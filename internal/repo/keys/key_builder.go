package keys

import (
	"encoding/binary"
	"time"
)

type Builder interface {
	Version() []byte
	Tweets() []byte
	Tweet(id string) []byte
	RequestLimits(id string, window time.Duration) []byte
}

type builder struct {
}

func (b builder) RequestLimits(id string, window time.Duration) []byte {
	slice := append([]byte{requestLimit}, []byte(id)...)
	binary.LittleEndian.AppendUint64(slice, uint64(window.Seconds()))

	return slice
}

func (b builder) Version() []byte {
	return []byte{version}
}

func (b builder) Tweets() []byte {
	return []byte{tweet}
}

func (b builder) Tweet(id string) []byte {
	return append([]byte{tweet}, []byte(id)...)
}

func NewBuilder() Builder {
	return &builder{}
}
