package tweetfinder

import (
	"context"
	"fmt"
	"strings"
	"time"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

const (
	limit           = 10000
	format          = "2006-01-02"
	tooManyRequests = "429 Too Many Requests"
)

type Finder interface {
	FindAll(ctx context.Context, start, end *time.Time, search string) ([]common.TweetSnapshot, error)
	Find(ctx context.Context, id string) (*common.TweetSnapshot, error)
}

type delayManager interface {
	TooManyRequests()
	ProcessedBatchOfTweets()
	ProcessedQuery()
}

type finder struct {
	scraper *twitterscraper.Scraper
	delayManager

	log log.Logger
}

func (f *finder) Find(_ context.Context, id string) (*common.TweetSnapshot, error) {
	tweet, err := f.scraper.GetTweet(id)
	if err != nil {
		if strings.Contains(err.Error(), tooManyRequests) {
			f.delayManager.TooManyRequests()
		}

		return nil, err
	}

	return &common.TweetSnapshot{
		Tweet: &common.Tweet{
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
			Photos:       scrapperPhotosToCommon(tweet.Photos),
			Videos:       scrapperVideosToCommon(tweet.Videos),
		},
		CheckedAt: time.Now(),
	}, nil
}

func (f *finder) FindAll(ctx context.Context, start, end *time.Time, search string) ([]common.TweetSnapshot, error) {
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

	response := make([]common.TweetSnapshot, 0)
	counter := 0
	likesMap := map[int]int{}
	retweetsMap := map[int]int{}
	replyMap := map[int]int{}
	lastTweetTime := time.Now()

	for tweet := range tweetsCh {
		if tweet.Error != nil {
			if strings.Contains(tweet.Error.Error(), tooManyRequests) {
				f.delayManager.TooManyRequests()
				return response, nil
			}

			return nil, tweet.Error
		}
		syncTime := time.Now()

		const debugInterval = 100
		if counter%debugInterval == 0 {
			f.log.
				WithField("created", tweet.TimeParsed).
				WithField("count", counter).
				Debug("processed tweets")
			f.delayManager.ProcessedBatchOfTweets()
		}

		if start != nil && tweet.TimeParsed.Sub(*start).Seconds() < 0 {
			cancel()
			break
		}

		counter++
		likesMap[tweet.Likes]++
		retweetsMap[tweet.Retweets]++
		replyMap[tweet.Retweets]++

		response = append(response, common.TweetSnapshot{
			Tweet: &common.Tweet{
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
				Photos:       scrapperPhotosToCommon(tweet.Photos),
				Videos:       scrapperVideosToCommon(tweet.Videos),
			},
			CheckedAt: syncTime,
		})

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

func scrapperPhotosToCommon(photos []twitterscraper.Photo) []common.Photo {
	res := make([]common.Photo, len(photos))
	for i, photo := range photos {
		res[i] = common.Photo{
			ID:  photo.ID,
			URL: photo.URL,
		}
	}

	return res
}

func scrapperVideosToCommon(videos []twitterscraper.Video) []common.Video {
	res := make([]common.Video, len(videos))
	for i, video := range videos {
		res[i] = common.Video{
			ID:      video.ID,
			Preview: video.Preview,
			URL:     video.URL,
		}
	}

	return res
}

func NewFinder(scraper *twitterscraper.Scraper, delayManager delayManager, logger log.Logger) Finder {
	return &finder{
		scraper:      scraper,
		delayManager: delayManager,
		log:          logger,
	}
}