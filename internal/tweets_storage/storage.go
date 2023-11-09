package tweets_storage

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

type Storage interface {
	Save(ctx context.Context, tweet *common.Tweet) error
	GetPopularAndOlderThen(ctx context.Context, time time.Time) (*common.Tweet, error)
	Delete(ctx context.Context, id string) error
}

type storage struct {
	tweets            map[string]*Tweet
	sortedByLikesDesc []string
	mu                sync.RWMutex
}

func (s *storage) Save(_ context.Context, tweet *common.Tweet) error {
	s.mu.Lock()
	s.tweets[tweet.ID] = &Tweet{
		Tweet:     tweet,
		UpdatedAt: time.Now(),
	}
	s.sortedByLikesDesc = append(s.sortedByLikesDesc, tweet.ID)
	slices.SortFunc(s.sortedByLikesDesc, s.sort)
	s.mu.Unlock()
	return nil
}

func (s *storage) GetPopularAndOlderThen(_ context.Context, time time.Time) (*common.Tweet, error) {
	s.mu.RLock()
	for _, id := range s.sortedByLikesDesc {
		if s.tweets[id].UpdatedAt.Before(time) {
			s.mu.RUnlock()
			return s.tweets[id].Tweet, nil
		}
	}
	return nil, common.ErrAllTweetsAreFresh
}

func (s *storage) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	for i, el := range s.sortedByLikesDesc {
		if el == id {
			s.sortedByLikesDesc = append(s.sortedByLikesDesc[:i], s.sortedByLikesDesc[i+1:]...)
			break
		}
	}
	delete(s.tweets, id)
	s.mu.Unlock()
	return nil
}

func (s *storage) sort(i string, j string) int {
	if s.tweets[i].Likes > s.tweets[j].Likes {
		return -1
	} else if s.tweets[i].Likes < s.tweets[j].Likes {
		return 1
	} else {
		return 0
	}
}

func NewStorage() Storage {
	return &storage{
		tweets:            map[string]*Tweet{},
		sortedByLikesDesc: make([]string, 0),
	}
}
