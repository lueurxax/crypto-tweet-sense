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

func (b builder) EditingTweet(id string) []byte {
	return append(editingTweetPrefix[:], []byte(id)...)
}

func (b builder) EditingTweets() []byte {
	return editingTweetPrefix[:]
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

func (b builder) RequestLimits(id string, window time.Duration) []byte {
	slice := append(requestLimitPrefix[:], []byte(id)...)
	return binary.LittleEndian.AppendUint16(slice, uint16(window.Seconds()))
}

func (b builder) RequestLimitsV2(id string, window time.Duration) []byte {
	slice := append(requestLimitV2Prefix[:], []byte(id)...)
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

func (b builder) TweetCreationIndex(createdAt time.Time) []byte {
	return append(tweetCreationIndexPrefix[:], []byte(fmt.Sprintf("%d", createdAt.UTC().Unix()))...)
}

func (b builder) TweetCreationIndexV2(createdAt time.Time, id string) []byte {
	return append(
		append(tweetCreationIndexV2Prefix[:], []byte(fmt.Sprintf("%d", createdAt.UTC().Unix()))...),
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

func (b builder) TweetUntilV2(createdAt time.Time) fdb.KeyRange {
	return fdb.KeyRange{
		Begin: fdb.Key(tweetCreationIndexV2Prefix[:]),
		End: fdb.Key(append(
			tweetCreationIndexV2Prefix[:],
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
