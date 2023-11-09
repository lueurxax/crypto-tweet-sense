package common

import "time"

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
}
