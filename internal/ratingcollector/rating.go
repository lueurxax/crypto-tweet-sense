package ratingcollector

import (
	"context"
	"errors"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

type RatingChecker interface {
	Check(ctx context.Context, tweet *common.TweetSnapshot) (bool, float64, error)
	CurrentTop() float64
}

type checker struct {
	repo
	topCount int
}

func (c *checker) CurrentTop() float64 {
	return float64(c.topCount)
}

func (c *checker) Check(ctx context.Context, tweet *common.TweetSnapshot) (bool, float64, error) {
	liveDuration := tweet.CheckedAt.Sub(tweet.TimeParsed).Seconds()

	likes := float64(tweet.Likes)

	rating, err := c.repo.GetRating(ctx, tweet.Username)
	if err != nil {
		if errors.Is(err, common.ErrRatingNotFound) {
			return tweet.Likes > c.topCount, likes / liveDuration, nil
		}

		return false, 0, err
	}

	rate := likes * (1.0 + float64(rating.Likes-rating.Dislikes)/10.0)

	return rate > float64(c.topCount), rate / liveDuration, nil
}

func NewChecker(db repo, topCount int) RatingChecker {
	return &checker{
		repo:     db,
		topCount: topCount,
	}
}
