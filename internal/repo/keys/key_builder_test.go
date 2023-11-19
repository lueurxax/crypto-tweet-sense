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
		assert.Equal(t, []byte{requestLimit, 't', 'e', 's', 't', 0x3c, 0}, got)
	})
}
