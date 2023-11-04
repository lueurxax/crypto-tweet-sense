package rating_collector

import (
	"context"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/rating_collector/models"
)

type emptyRating struct {
	topCount int
}

func (e *emptyRating) CollectRatings(ratings <-chan *models.UsernameRating) {
	//TODO implement me
	panic("implement me")
}

func (e *emptyRating) Check(ctx context.Context, tweet *twitterscraper.Tweet) (bool, error) {
	return tweet.Likes > e.topCount, nil
}

func NewEmptyRating(topCount int) RatingChecker {
	return &emptyRating{topCount: topCount}
}
