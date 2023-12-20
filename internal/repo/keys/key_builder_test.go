package keys

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_builder_RequestLimits(t *testing.T) {
	t.Run("minute", func(t *testing.T) {
		b := builder{}
		got := b.RequestLimits("test", time.Minute)
		assert.Equal(t, append(requestLimitPrefix[:], []byte{'t', 'e', 's', 't', 0x3c, 0}...), got)
	})
}

func Test_builder_Tweet(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want []byte
	}{
		{
			name: "simple",
			id:   "1729960929805099056",
			want: []byte{0x0, 0x1, 0x31, 0x37, 0x32, 0x39, 0x39, 0x36, 0x30, 0x39, 0x32, 0x39, 0x38, 0x30, 0x35, 0x30, 0x39, 0x39, 0x30, 0x35, 0x36},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder()
			got := b.Tweet(tt.id)
			assert.Equalf(t, tt.want, got, "Tweet(%v)", tt.id)
			t.Log(string(got))
		})
	}
}
