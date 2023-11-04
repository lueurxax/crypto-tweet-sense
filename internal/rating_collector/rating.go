package rating_collector

import (
	"context"
	"sync"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/rating_collector/models"
)

type RatingChecker interface {
	Check(ctx context.Context, tweet *twitterscraper.Tweet) (bool, error)
	CollectRatings(<-chan *models.UsernameRating)
}

type checker struct {
	rating   map[string]*models.Rating
	mu       *sync.RWMutex
	topCount int
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

func (c *checker) Check(_ context.Context, tweet *twitterscraper.Tweet) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rating, ok := c.rating[tweet.Username]
	if !ok {
		return tweet.Likes > c.topCount, nil
	}
	return float32(tweet.Likes)*(1.0+float32(rating.Likes-rating.Dislikes)/10.0) > float32(c.topCount), nil
}

func NewChecker(rating map[string]*models.Rating, topCount int) RatingChecker {
	return &checker{
		rating:   rating,
		mu:       &sync.RWMutex{},
		topCount: topCount,
	}
}
