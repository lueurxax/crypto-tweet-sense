package tweet_finder

import (
	"context"
	"fmt"
	"strings"
	"time"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

const (
	limit  = 10000
	format = "2006-01-02"
)

type Finder interface {
	Find(ctx context.Context, start, end time.Time, search string) (string, error)
	FindAll(ctx context.Context, start, end *time.Time, search string) ([]twitterscraper.Tweet, error)
	FindAllByUser(ctx context.Context, username string) ([]twitterscraper.Tweet, error)
}

type delayManager interface {
	TooManyRequests()
	ProcessedBatchOfTweets()
	ProcessedQuery()
}

type ratingChecker interface {
	Check(ctx context.Context, tweet *twitterscraper.Tweet) (bool, error)
}

type finder struct {
	scraper *twitterscraper.Scraper
	ratingChecker
	delayManager

	log log.Logger
}

func (f *finder) FindAll(ctx context.Context, start, end *time.Time, search string) ([]twitterscraper.Tweet, error) {
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

	response := make([]twitterscraper.Tweet, 0)
	counter := 0
	likesMap := map[int]int{}
	retweetsMap := map[int]int{}
	replyMap := map[int]int{}
	lastTweetTime := time.Now()
	until := time.Now().UTC().Add(-time.Hour * 24)

	for tweet := range tweetsCh {
		if tweet.Error != nil {
			if strings.Contains(tweet.Error.Error(), "429 Too Many Requests") {
				f.delayManager.TooManyRequests()
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
			f.delayManager.ProcessedBatchOfTweets()
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
			response = append(response, tweet.Tweet)
			f.log.
				WithField("ts", tweet.TimeParsed).
				WithField("text", tweet.Text).
				Debug("found tweet")
		}

		lastTweetTime = tweet.TimeParsed
	}

	f.delayManager.ProcessedQuery()

	f.log.WithField("created", lastTweetTime).Debug("last tweet")
	f.log.WithField("count", counter).Debug("tweets found")
	f.log.WithField("map", likesMap).Debug("likes count")
	f.log.WithField("map", retweetsMap).Debug("retweet count")
	f.log.WithField("map", replyMap).Debug("reply count")

	return response, nil
}

func (f *finder) Find(ctx context.Context, start, end time.Time, search string) (string, error) {
	query := fmt.Sprintf("%s -filter:retweets since:%s until:%s", search, start.Format(format), end.Format(format))
	tweetsCh := f.scraper.SearchTweets(ctx, query, limit)
	maxTweets := 4000

	var (
		maxTweet *twitterscraper.Tweet
	)

	for tweet := range tweetsCh {
		if tweet.Error != nil {
			return "", tweet.Error
		}

		if maxTweets < tweet.Likes {
			maxTweets = tweet.Likes
			maxTweet = &tweet.Tweet
		}
	}

	if maxTweet == nil {
		return "", NoTops
	}

	return maxTweet.PermanentURL, nil
}

func (f *finder) FindAllByUser(ctx context.Context, username string) ([]twitterscraper.Tweet, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tweetsCh := f.scraper.GetTweets(ctx, username, limit)

	response := make([]twitterscraper.Tweet, 0)
	counter := 0
	likesMap := map[int]int{}
	retweetsMap := map[int]int{}
	replyMap := map[int]int{}
	lastTweetTime := time.Now()

	for tweet := range tweetsCh {
		if tweet.Error != nil {
			return nil, tweet.Error
		}

		if counter == 0 {
			f.log.WithField("created", tweet.TimeParsed).Debug("first tweet")
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
			response = append(response, tweet.Tweet)
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

func NewFinder(scraper *twitterscraper.Scraper, ratingChecker ratingChecker, delayManager delayManager, logger log.Logger) Finder {
	return &finder{
		scraper:       scraper,
		delayManager:  delayManager,
		ratingChecker: ratingChecker,
		log:           logger,
	}
}
