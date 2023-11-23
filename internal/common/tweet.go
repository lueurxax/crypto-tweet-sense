package common

import (
	"fmt"
	"time"
)

type Tweet struct {
	ID           string
	Likes        int
	Name         string
	PermanentURL string
	Replies      int
	Retweets     int
	Text         string
	TimeParsed   time.Time
	Timestamp    int64
	UserID       string
	Username     string
	Views        int
	Photos       []Photo
	Videos       []Video
}

// Photo type.
type Photo struct {
	ID  string
	URL string
}

// Video type.
type Video struct {
	ID      string
	Preview string
	URL     string
}

type TweetSnapshot struct {
	*Tweet
	RatingGrowSpeed float64
	CheckedAt       time.Time
}

func (t TweetSnapshot) String() string {
	return fmt.Sprintf(
		"TweetSnapshot{ID: %s, CreatedAt: %s, RatingGrowSpeed: %0.3f, CheckedAt: %s}",
		t.ID, t.TimeParsed.Format(time.RFC3339), t.RatingGrowSpeed, t.CheckedAt.Format(time.RFC3339),
	)
}
