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
	Requests(id string, window time.Duration, start time.Time) []byte
	RequestsByRequestLimits(id string, window time.Duration) []byte
	RequestLimitsUnzip(id string, window time.Duration) []byte
	RequestsUnzip(id string, window time.Duration, start time.Time) []byte
	RequestsByRequestLimitsUnzip(id string, window time.Duration) []byte
	TweetUsernameRatingKey(username string) []byte
	TweetRatings() []byte
	SentTweet(link string) []byte
	EditingTweetShort(id string) []byte
	EditingTweetLong(id string) []byte
	EditingTweetsShort() []byte
	EditingTweetsLong() []byte
	TelegramSessionStorage() []byte
	TwitterAccount(login string) []byte
	TwitterAccounts() []byte
	Cookie(login string) []byte
	TweetCreationIndex(createdAt time.Time, id string) []byte
	TweetUntil(createdAt time.Time) fdb.KeyRange
}

type builder struct {
}

func (b builder) TwitterAccounts() []byte {
	return twitterAccountsPrefix[:]
}

func (b builder) Cookie(login string) []byte {
	return append(twitterAccountsCookiePrefix[:], []byte(login)...)
}

func (b builder) TwitterAccount(login string) []byte {
	return append(twitterAccountsPrefix[:], []byte(login)...)
}

func (b builder) EditingTweetShort(id string) []byte {
	return append(editingTweetShortPrefix[:], []byte(id)...)
}

func (b builder) EditingTweetsShort() []byte {
	return editingTweetShortPrefix[:]
}

func (b builder) EditingTweetLong(id string) []byte {
	return append(editingTweetLongPrefix[:], []byte(id)...)
}

func (b builder) EditingTweetsLong() []byte {
	return editingTweetLongPrefix[:]
}

func (b builder) TelegramSessionStorage() []byte {
	return telegramSessionStoragePrefix[:]
}

func (b builder) SentTweet(link string) []byte {
	return append(sentTweetPrefix[:], []byte(link)...)
}

func (b builder) TweetRatings() []byte {
	return tweetRatingPrefix[:]
}

func (b builder) TweetUsernameRatingKey(username string) []byte {
	return append(tweetRatingPrefix[:], []byte(username)...)
}

func (b builder) RequestLimitsUnzip(id string, window time.Duration) []byte {
	slice := append(requestLimitUnzipPrefix[:], []byte(id)...)
	return binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds()))
}

func (b builder) RequestsUnzip(id string, window time.Duration, start time.Time) []byte {
	slice := requestsUnzipPrefix[:]
	slice = append(slice, []byte(id)...)

	return binary.LittleEndian.AppendUint64(
		binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds())),
		uint64(start.Unix()),
	)
}

func (b builder) RequestsByRequestLimitsUnzip(id string, window time.Duration) []byte {
	slice := append(requestsUnzipPrefix[:], []byte(id)...)
	return binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds()))
}

func (b builder) RequestLimits(id string, window time.Duration) []byte {
	slice := append(requestLimitPrefix[:], []byte(id)...)
	return binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds()))
}

func (b builder) Requests(id string, window time.Duration, start time.Time) []byte {
	slice := requestsPrefix[:]
	slice = append(slice, []byte(id)...)

	return binary.LittleEndian.AppendUint64(
		binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds())),
		uint64(start.Unix()),
	)
}

func (b builder) RequestsByRequestLimits(id string, window time.Duration) []byte {
	slice := append(requestsPrefix[:], []byte(id)...)
	return binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds()))
}

func (b builder) Version() []byte {
	return versionPrefix[:]
}

func (b builder) Tweets() []byte {
	return tweetPrefix[:]
}

func (b builder) TweetRatingIndexes() []byte {
	return tweetRatingIndexPrefix[:]
}

func (b builder) TweetRatingPositiveIndexes() fdb.KeyRange {
	// "0120.00001" "0129"
	key, err := fdb.Strinc(tweetRatingIndexPrefix[:])
	if err != nil {
		panic(err)
	}

	return fdb.KeyRange{
		Begin: fdb.Key(append(tweetRatingIndexPrefix[:], []byte("0.00001")...)),
		End:   fdb.Key(key),
	}
}

func (b builder) TweetRatingIndex(ratingGrowSpeed float64, id string) []byte {
	return append(append(tweetRatingIndexPrefix[:], []byte(fmt.Sprintf("%.5f", ratingGrowSpeed))...), []byte(id)...)
}

func (b builder) TweetCreationIndex(createdAt time.Time, id string) []byte {
	return append(
		append(tweetCreationIndexPrefix[:], []byte(fmt.Sprintf("%d", createdAt.UTC().Unix()))...),
		[]byte(id)...,
	)
}

func (b builder) TweetUntil(createdAt time.Time) fdb.KeyRange {
	return fdb.KeyRange{
		Begin: fdb.Key(tweetCreationIndexPrefix[:]),
		End: fdb.Key(append(
			tweetCreationIndexPrefix[:],
			[]byte(fmt.Sprintf("%d", createdAt.UTC().Unix()))...),
		),
	}
}

func (b builder) Tweet(id string) []byte {
	return append(tweetPrefix[:], []byte(id)...)
}

func NewBuilder() Builder {
	return &builder{}
}
