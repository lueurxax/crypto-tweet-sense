package tweetfinder

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

const (
	finderIndexKey = "finder_index"
	finderErrorMsg = "finder error"
)

type pool struct {
	finders []Finder

	mu           sync.RWMutex
	finderDelays []*int64

	log log.Logger
}

func (p *pool) CurrentDelay() int64 {
	sum := int64(0)

	for i := range p.finderDelays {
		sum += atomic.LoadInt64(p.finderDelays[i])
	}

	return sum / int64(len(p.finderDelays))
}

func (p *pool) FindAll(ctx context.Context, start, end *time.Time, search string) ([]common.TweetSnapshot, error) {
	f, index := p.getFinder()

	res, err := f.FindAll(ctx, start, end, search)

	p.releaseFinder(index)

	if err != nil {
		p.log.WithField(finderIndexKey, index).WithError(err).Error(finderErrorMsg)
		return nil, err
	}

	return res, nil
}

func (p *pool) Find(ctx context.Context, id string) (*common.TweetSnapshot, error) {
	f, index := p.getFinder()

	res, err := f.Find(ctx, id)

	p.releaseFinder(index)

	if err != nil {
		p.log.WithField(finderIndexKey, index).WithError(err).Error(finderErrorMsg)
		return nil, err
	}

	return res, nil
}

func (p *pool) getFinder() (Finder, int) {
	p.mu.Lock()
	index := 0

	for i := range p.finderDelays {
		if atomic.LoadInt64(p.finderDelays[i]) < atomic.LoadInt64(p.finderDelays[index]) {
			index = i
		}
	}

	atomic.AddInt64(p.finderDelays[index], p.finders[index].CurrentDelay())

	return p.finders[index], index
}

func (p *pool) releaseFinder(index int) {
	atomic.StoreInt64(p.finderDelays[index], p.finders[index].CurrentDelay())
}

func NewPool(finders []Finder, logger log.Logger) Finder {
	delays := make([]*int64, len(finders))
	for i, f := range finders {
		delay := f.CurrentDelay()
		delays[i] = &delay
	}

	return &pool{
		finders:      finders,
		finderDelays: delays,
		log:          logger,
	}
}
