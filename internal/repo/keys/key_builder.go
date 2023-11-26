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
	TweetUsernameRatingKey(username string) []byte
	TweetRatings() []byte
	SentTweet(link string) []byte
	TelegramSessionStorage() []byte
}

type builder struct {
}

func (b builder) TelegramSessionStorage() []byte {
	return []byte(telegramSessionStorage)
}

func (b builder) SentTweet(link string) []byte {
	return append([]byte{sentTweet}, []byte(link)...)
}

func (b builder) TweetRatings() []byte {
	return []byte{tweetRating}
}

func (b builder) TweetUsernameRatingKey(username string) []byte {
	return append([]byte{tweetRating}, []byte(username)...)
}

func (b builder) RequestLimits(id string, window time.Duration) []byte {
	slice := append([]byte{requestLimit}, []byte(id)...)
	return binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds()))
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