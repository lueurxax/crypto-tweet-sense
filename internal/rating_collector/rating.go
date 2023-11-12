package rating_collector

import (
	"context"
	"sync"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/rating_collector/models"
)

type RatingChecker interface {
	Check(ctx context.Context, tweet *common.TweetSnapshot) (bool, float64, error)
	CurrentTop() float64
	CollectRatings(<-chan *models.UsernameRating)
}

type checker struct {
	rating   map[string]*models.Rating
	mu       *sync.RWMutex
	topCount int
}

func (c *checker) CurrentTop() float64 {
	return float64(c.topCount)
}

func (c *checker) CollectRatings(ratings <-chan *models.UsernameRating) {
	go c.loop(ratings)
}

func (c *checker) loop(ratings <-chan *models.UsernameRating) {
	for rating := range ratings {
		c.mu.Lock()
		c.rating[rating.Username] = rating.Rating
		c.mu.Unlock()
	}
}

func (c *checker) Check(_ context.Context, tweet *common.TweetSnapshot) (bool, float64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	liveDuration := tweet.CheckedAt.Sub(tweet.TimeParsed).Seconds()

	likes := float64(tweet.Likes)

	rating, ok := c.rating[tweet.Username]
	if !ok {
		return tweet.Likes > c.topCount, likes / liveDuration, nil
	}

	raiting := likes * (1.0 + float64(rating.Likes-rating.Dislikes)/10.0)

	return raiting > float64(c.topCount), raiting / liveDuration, nil
}

func NewChecker(rating map[string]*models.Rating, topCount int) RatingChecker {
	return &checker{
		rating:   rating,
		mu:       &sync.RWMutex{},
		topCount: topCount,
	}
}
