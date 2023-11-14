package tweetfinder

import (
	"context"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
)

type pool struct {
	freeFinders chan Finder
}

func (p *pool) FindAll(ctx context.Context, start, end *time.Time, search string) ([]common.TweetSnapshot, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case f := <-p.freeFinders:
		defer func() {
			p.freeFinders <- f
		}()
		return f.FindAll(ctx, start, end, search)
	}
}

func (p *pool) Find(ctx context.Context, id string) (*common.TweetSnapshot, error) {
	f := <-p.freeFinders
	defer func() {
		p.freeFinders <- f
	}()
	return f.Find(ctx, id)
}

func NewPool(finders []Finder) Finder {
	ch := make(chan Finder, len(finders))
	for _, f := range finders {
		ch <- f
	}
	return &pool{freeFinders: ch}
}
