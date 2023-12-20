package keys

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
)

type Builder interface {
	Version() []byte
	Tweets() []byte
	Tweet(id string) []byte
	TweetRatingIndexes() []byte
	TweetRatingPositiveIndexes() fdb.KeyRange
	TweetRatingIndex(ratingGrowSpeed float64, id string) []byte
	RequestLimits(id string, window time.Duration) []byte
	RequestLimitsV2(id string, window time.Duration) []byte
	Requests(id string, window time.Duration, start time.Time) []byte
	RequestsByRequestLimits(id string, window time.Duration) []byte
	TweetUsernameRatingKey(username string) []byte
	TweetRatings() []byte
	SentTweet(link string) []byte
	EditingTweet(id string) []byte
	EditingTweets() []byte
	TelegramSessionStorage() []byte
	TwitterAccount(login string) []byte
	TwitterAccounts() []byte
	Cookie(login string) []byte
	TweetCreationIndex(createdAt time.Time) []byte
	TweetCreationIndexV2(createdAt time.Time, id string) []byte
	TweetUntil(createdAt time.Time) fdb.KeyRange
	TweetUntilV2(createdAt time.Time) fdb.KeyRange
}

type builder struct {
	prefixesCache map[string][]byte
}

func (b builder) TwitterAccounts() []byte {
	return b.getPrefix(twitterAccounts)
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
	slice := append(b.getPrefix(requestLimit), []byte(id)...)
	return binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds()))
}

func (b builder) RequestLimitsV2(id string, window time.Duration) []byte {
	slice := append(b.getPrefix(requestLimitV2), []byte(id)...)
	return binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds()))
}

func (b builder) Requests(id string, window time.Duration, start time.Time) []byte {
	slice := append(b.getPrefix(requests), []byte(id)...)
	return binary.LittleEndian.AppendUint64(
		binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds())),
		uint64(start.Unix()),
	)
}

func (b builder) RequestsByRequestLimits(id string, window time.Duration) []byte {
	slice := append(b.getPrefix(requests), []byte(id)...)
	return binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds()))
}

func (b builder) Version() []byte {
	return []byte{version}
}

func (b builder) Tweets() []byte {
	return []byte{tweet}
}

func (b builder) TweetRatingIndexes() []byte {
	return b.getPrefix(tweetRatingIndex)
}

func (b builder) TweetRatingPositiveIndexes() fdb.KeyRange {
	// "_tweetRatingIndex0.00001" "_tweetRatingIndex9"
	key, err := fdb.Strinc(b.getPrefix(tweetRatingIndex))
	if err != nil {
		panic(err)
	}
	return fdb.KeyRange{
		Begin: fdb.Key(append(b.getPrefix(tweetRatingIndex), []byte("0.00001")...)),
		End:   fdb.Key(key),
	}
}

func (b builder) TweetRatingIndex(ratingGrowSpeed float64, id string) []byte {
	return append(append(b.getPrefix(tweetRatingIndex), []byte(fmt.Sprintf("%.5f", ratingGrowSpeed))...), []byte(id)...)
}

func (b builder) TweetCreationIndex(createdAt time.Time) []byte {
	return append(b.getPrefix(tweetCreationIndex), []byte(fmt.Sprintf("%d", createdAt.UTC().Unix()))...)
}

func (b builder) TweetCreationIndexV2(createdAt time.Time, id string) []byte {
	return append(
		append(b.getPrefix(tweetCreationIndexV2), []byte(fmt.Sprintf("%d", createdAt.UTC().Unix()))...),
		[]byte(id)...,
	)
}

func (b builder) TweetUntil(createdAt time.Time) fdb.KeyRange {
	return fdb.KeyRange{
		Begin: fdb.Key(b.getPrefix(tweetCreationIndex)),
		End: fdb.Key(append(
			b.getPrefix(tweetCreationIndex),
			[]byte(fmt.Sprintf("%d", createdAt.UTC().Unix()))...),
		),
	}
}

func (b builder) TweetUntilV2(createdAt time.Time) fdb.KeyRange {
	return fdb.KeyRange{
		Begin: fdb.Key(b.getPrefix(tweetCreationIndexV2)),
		End: fdb.Key(append(
			b.getPrefix(tweetCreationIndexV2),
			[]byte(fmt.Sprintf("%d", createdAt.UTC().Unix()))...),
		),
	}
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
