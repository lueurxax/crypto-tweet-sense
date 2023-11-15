package tweetfinder

import (
	"context"
	"sync"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

type pool struct {
	finders       []Finder
	releaseSignal chan struct{}

	mu           sync.RWMutex
	finderDelays []int64

	log log.Logger
}

func (p *pool) CurrentDelay() int64 {
	sum := int64(0)

	p.mu.RLock()
	for _, d := range p.finderDelays {
		sum += d
	}
	p.mu.RUnlock()

	return sum / int64(len(p.finderDelays))
}

func (p *pool) FindAll(ctx context.Context, start, end *time.Time, search string) ([]common.TweetSnapshot, error) {
	f, index, err := p.getFinder(ctx)
	if err != nil {
		return nil, err
	}

	defer p.releaseFinder(index)

	return f.FindAll(ctx, start, end, search)
}

func (p *pool) Find(ctx context.Context, id string) (*common.TweetSnapshot, error) {
	f, index, err := p.getFinder(ctx)
	if err != nil {
		return nil, err
	}

	defer p.releaseFinder(index)

	return f.Find(ctx, id)
}

func (p *pool) getFinder(ctx context.Context) (Finder, int, error) {
	index, ok := p.getFinderIndex()
	for !ok {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-p.releaseSignal:
			break
		}

		index, ok = p.getFinderIndex()
	}

	return p.finders[index], index, nil
}

func (p *pool) getFinderIndex() (int, bool) {
	p.mu.Lock()
	minimal := p.finderDelays[0]
	index := 0

	for i, d := range p.finderDelays {
		if d == 0 {
			continue
		}

		if d < minimal || minimal == 0 {
			minimal = d
			index = i
		}
	}

	if minimal != 0 {
		p.finderDelays[index] = 0
	}

	p.mu.Unlock()

	return index, minimal != 0
}

func (p *pool) releaseFinder(index int) {
	p.mu.Lock()
	p.finderDelays[index] = p.finders[index].CurrentDelay()
	p.mu.Unlock()

	select {
	case p.releaseSignal <- struct{}{}:
	default:
	}
}

func NewPool(finders []Finder, logger log.Logger) Finder {
	delays := make([]int64, len(finders))
	for i, f := range finders {
		delays[i] = f.CurrentDelay()
	}

	return &pool{
		finders:      finders,
		finderDelays: delays,
		log:          logger,
	}
}
