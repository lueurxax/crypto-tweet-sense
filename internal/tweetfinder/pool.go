package tweetfinder

import (
	"context"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

type pool struct {
	freeFinders chan *finderWrapper
	log         log.Logger
}

func (p *pool) FindAll(ctx context.Context, start, end *time.Time, search string) ([]common.TweetSnapshot, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case f := <-p.freeFinders:
		res, err := f.finder.FindAll(ctx, start, end, search)

		p.freeFinders <- f

		if err != nil {
			p.log.WithField("finder_index", f.index).WithError(err).Error("finder error")
			return nil, err
		}

		return res, nil
	}
}

func (p *pool) Find(ctx context.Context, id string) (*common.TweetSnapshot, error) {
	f := <-p.freeFinders

	res, err := f.finder.Find(ctx, id)

	p.freeFinders <- f

	if err != nil {
		p.log.WithField("finder_index", f.index).WithError(err).Error("finder error")
		return nil, err
	}

	return res, nil
}

func NewPool(finders []Finder, logger log.Logger) Finder {
	ch := make(chan *finderWrapper, len(finders))
	for i, f := range finders {
		ch <- &finderWrapper{
			finder: f,
			index:  i,
		}
	}

	return &pool{
		freeFinders: ch,
		log:         logger,
	}
}

type finderWrapper struct {
	finder Finder
	index  int
}
