package tweets_storage

import (
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

type Tweet struct {
	*common.Tweet
	UpdatedAt time.Time
}
