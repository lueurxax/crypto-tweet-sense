package tweet_finder

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

const (
	limit  = 10000
	format = "2006-01-02"
)

type Finder interface {
	FindAll(ctx context.Context, start, end *time.Time, search string) ([]common.Tweet, error)
	GetTweet(ctx context.Context, id string) (*common.Tweet, error)
}

type ratingChecker interface {
	Check(ctx context.Context, tweet *twitterscraper.Tweet) (bool, error)
}

type finder struct {
	scraper *twitterscraper.Scraper
	ratingChecker
	delay int64

	log log.Logger
}

func (f *finder) GetTweet(_ context.Context, id string) (*common.Tweet, error) {
	tweet, err := f.scraper.GetTweet(id)
	if err != nil {
		return nil, err
	}
	return &common.Tweet{
		ID:           tweet.ID,
		Likes:        tweet.Likes,
		Name:         tweet.Name,
		PermanentURL: tweet.PermanentURL,
		Replies:      tweet.Replies,
		Retweets:     tweet.Retweets,
		Text:         tweet.Text,
		TimeParsed:   tweet.TimeParsed,
		Timestamp:    tweet.Timestamp,
		UserID:       tweet.UserID,
		Username:     tweet.Username,
		Views:        tweet.Views,
	}, nil
}

func (f *finder) FindAll(ctx context.Context, start, end *time.Time, search string) ([]common.Tweet, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	query := fmt.Sprintf("%s -filter:retweets", search)
	if start != nil {
		query = fmt.Sprintf("%s since:%s", search, start.Format(format))
	}
	if end != nil {
		query = fmt.Sprintf("%s until:%s", query, end.Format(format))
	}
	f.log.WithField("query", query).Debug("searching")
	tweetsCh := f.scraper.SearchTweets(ctx, query, limit)

	response := make([]common.Tweet, 0)
	counter := 0
	likesMap := map[int]int{}
	retweetsMap := map[int]int{}
	replyMap := map[int]int{}
	lastTweetTime := time.Now()
	until := time.Now().UTC().Add(-time.Hour * 24)
	for tweet := range tweetsCh {
		if tweet.Error != nil {
			if strings.Contains(tweet.Error.Error(), "429 Too Many Requests") {
				f.log.WithError(tweet.Error).WithField("delay", f.delay).Error("too many requests")
				f.incRandomDelay()
				return response, nil
			}
			return nil, tweet.Error
		}
		const debugInterval = 100
		if counter%debugInterval == 0 {
			f.log.
				WithField("created", tweet.TimeParsed).
				WithField("count", counter).
				Debug("processed tweets")
			f.decDelay()
		}
		if tweet.TimeParsed.Sub(until).Seconds() < 0 {
			cancel()
			break
		}
		counter++
		likesMap[tweet.Likes]++
		retweetsMap[tweet.Retweets]++
		replyMap[tweet.Retweets]++
		ok, err := f.ratingChecker.Check(ctx, &tweet.Tweet)
		if err != nil {
			return nil, err
		}
		if ok {
			response = append(response, common.Tweet{
				ID:           tweet.Tweet.ID,
				Likes:        tweet.Likes,
				Name:         tweet.Name,
				PermanentURL: tweet.PermanentURL,
				Replies:      tweet.Replies,
				Retweets:     tweet.Retweets,
				Text:         tweet.Text,
				TimeParsed:   tweet.TimeParsed,
				Timestamp:    tweet.Timestamp,
				UserID:       tweet.UserID,
				Username:     tweet.Username,
				Views:        tweet.Views,
			})
			f.log.
				WithField("ts", tweet.TimeParsed).
				WithField("text", tweet.Text).
				Debug("found tweet")
		}
		lastTweetTime = tweet.TimeParsed
	}
	f.log.WithField("created", lastTweetTime).Debug("last tweet")
	f.log.WithField("count", counter).Debug("tweets found")
	f.log.WithField("map", likesMap).Debug("likes count")
	f.log.WithField("map", retweetsMap).Debug("retweet count")
	f.log.WithField("map", replyMap).Debug("reply count")

	return response, nil
}

func (f *finder) incRandomDelay() {
	if f.delay == 0 {
		f.delay = 1
	}
	f.delay += rand.Int63n(f.delay) + 1
	f.scraper.WithDelay(f.delay)
}

func (f *finder) decDelay() {
	if f.delay <= 0 {
		return
	}
	f.delay--
	f.scraper.WithDelay(f.delay)
}

func NewFinder(scraper *twitterscraper.Scraper, ratingChecker ratingChecker, delay int64, logger log.Logger) Finder {
	return &finder{
		scraper:       scraper,
		delay:         delay,
		ratingChecker: ratingChecker,
		log:           logger,
	}
}
