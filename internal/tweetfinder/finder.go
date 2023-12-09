package tweetfinder

import (
	"context"
	"fmt"
	"strings"
	"time"

	twitterscraper "github.com/lueurxax/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

const (
	limit           = 10000
	format          = "2006-01-02"
	tooManyRequests = "429 Too Many Requests"
	notFound        = "not found"
)

type Finder interface {
	FindNext(ctx context.Context, start, end *time.Time, search, cursor string) ([]common.TweetSnapshot, string, error)
	Find(ctx context.Context, id string) (*common.TweetSnapshot, error)
	CurrentDelay() int64
	CurrentTemp(ctx context.Context) float64
	Init(ctx context.Context) error
}

type delayManager interface {
	TooManyRequests(ctx context.Context)
	AfterRequest()
	CurrentTemp(ctx context.Context) float64
	CurrentDelay() int64
}

type finder struct {
	scraper *twitterscraper.Scraper
	delayManager

	log log.Logger
}

func (f *finder) CurrentTemp(ctx context.Context) float64 {
	return f.delayManager.CurrentTemp(ctx)
}

func (f *finder) Init(context.Context) error {
	return nil
}

func (f *finder) Find(ctx context.Context, id string) (*common.TweetSnapshot, error) {
	tweet, err := f.scraper.GetTweet(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), tooManyRequests) {
			f.delayManager.TooManyRequests(ctx)
		}

		if strings.Contains(err.Error(), notFound) {
			return nil, ErrNotFound
		}

		f.log.WithField("id", id).WithError(err).Error("error while getting tweet")

		return nil, err
	}

	f.delayManager.AfterRequest()

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

func (f *finder) FindNext(ctx context.Context, start, end *time.Time, search, cursor string) ([]common.TweetSnapshot, string, error) {
	query := fmt.Sprintf("%s -filter:retweets", search)
	if start != nil {
		query = fmt.Sprintf("%s since:%s", search, start.Format(format))
	}

	if end != nil {
		query = fmt.Sprintf("%s until:%s", query, end.Format(format))
	}

	tweets, nextCursor, err := f.scraper.FetchSearchTweets(ctx, query, limit, cursor)
	if err != nil {
		if strings.Contains(err.Error(), tooManyRequests) {
			f.delayManager.TooManyRequests(ctx)
		}

		return nil, "", err
	}

	response := make([]common.TweetSnapshot, 0)

	for _, tweet := range tweets {
		syncTime := time.Now()

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
	}

	f.delayManager.AfterRequest()

	return response, nextCursor, nil
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
