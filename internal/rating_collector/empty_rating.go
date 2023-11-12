package rating_collector

import (
	"context"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/rating_collector/models"
)

type emptyRating struct {
	topCount int
}

func (e *emptyRating) CurrentTop() float64 {
	//TODO implement me
	panic("implement me")
}

func (e *emptyRating) Check(_ context.Context, tweet *common.TweetSnapshot) (bool, float64, error) {
	return tweet.Likes > e.topCount, 0, nil
}

func (e *emptyRating) CollectRatings(ratings <-chan *models.UsernameRating) {
	//TODO implement me
	panic("implement me")
}

func NewEmptyRating(topCount int) RatingChecker {
	return &emptyRating{topCount: topCount}
}
