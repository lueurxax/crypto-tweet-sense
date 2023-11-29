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
	EditingTweet(id string) []byte
	EditingTweets() []byte
	TelegramSessionStorage() []byte
	TwitterAccount(login string) []byte
	Cookie(login string) []byte
}

type builder struct {
	prefixesCache map[string][]byte
}

func (b builder) Cookie(login string) []byte {
	prefix := b.getPrefix(twitterAccountsCookie)

	return append(prefix, []byte(login)...)
}

func (b builder) TwitterAccount(login string) []byte {
	prefix := b.getPrefix(twitterAccounts)

	return append(prefix, []byte(login)...)
}

func (b builder) EditingTweet(id string) []byte {
	return append([]byte{editingTweet}, []byte(id)...)
}

func (b builder) EditingTweets() []byte {
	return []byte{editingTweet}
}

func (b builder) TelegramSessionStorage() []byte {
	return []byte(telegramSessionStorageOld)
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

func (b builder) getPrefix(key string) []byte {
	prefix, ok := b.prefixesCache[key]
	if !ok {
		prefix = []byte(key)
		b.prefixesCache[key] = prefix
	}

	return prefix
}

func NewBuilder() Builder {
	return &builder{prefixesCache: make(map[string][]byte)}
}
