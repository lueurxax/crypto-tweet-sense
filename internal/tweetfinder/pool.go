package tweetfinder

import (
	"context"
	"sync"
	"time"

	twitterscraper "github.com/n0madic/twitter-scraper"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder/windowlimiter"
)

const (
	startDelay  = 15
	finderLogin = "finder_login"
	pkgKey      = "pkg"
)

type accountManager interface {
	AuthScrapper(ctx context.Context, account common.TwitterAccount, scraper *twitterscraper.Scraper) error
	SearchUnAuthAccounts(ctx context.Context) ([]common.TwitterAccount, error)
}

type repo interface {
	AddCounter(ctx context.Context, id string, window time.Duration, counterTime time.Time) error
	CleanCounters(ctx context.Context, id string, window time.Duration) error
	SetThreshold(ctx context.Context, id string, window time.Duration) error
	GetRequestLimit(ctx context.Context, id string, window time.Duration) (common.RequestLimitData, error)
	CheckIfExist(ctx context.Context, id string, window time.Duration) (bool, error)
	Create(ctx context.Context, id string, window time.Duration, threshold uint64) error
	IncreaseThresholdTo(ctx context.Context, id string, duration time.Duration, threshold uint64) error
}

type pool struct {
	config        ConfigPool
	finders       []Finder
	releaseSignal chan struct{}

	manager accountManager
	repo    repo

	mu           sync.RWMutex
	finderDelays []int64

	log log.Logger
}

func (p *pool) Init(ctx context.Context) error {
	if err := p.init(ctx); err != nil {
		return err
	}

	go p.reinit()

	return nil
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
	ticker := time.NewTicker(time.Second)

	for !ok {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-p.releaseSignal:
			break
		case <-ticker.C:
			break
		}

		index, ok = p.getFinderIndex()
	}

	return p.finders[index], index, nil
}

func (p *pool) getFinderIndex() (int, bool) {
	p.mu.Lock()
	minimal := int64(0)
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

func (p *pool) releaseFinder(i int) {
	p.mu.Lock()
	for index := range p.finderDelays {
		if index == i || p.finderDelays[index] != 0 {
			p.finderDelays[index] = p.finders[index].CurrentDelay()
		}
	}
	p.mu.Unlock()

	select {
	case p.releaseSignal <- struct{}{}:
	default:
	}
}

func (p *pool) init(ctx context.Context) error {
	delayManagerLogger := p.log.WithField(pkgKey, "delay_manager")
	limiterLogger := p.log.WithField(pkgKey, "window_limiter")
	finderLogger := p.log.WithField(pkgKey, "finder")

	var delayManager Manager

	accounts, err := p.manager.SearchUnAuthAccounts(ctx)
	if err != nil {
		return err
	}

	for i, account := range accounts {
		scraper := twitterscraper.New().WithDelay(startDelay).SetSearchMode(twitterscraper.SearchLatest)

		if len(p.config.Proxies) > len(p.finders)+i {
			if err = scraper.SetProxy(p.config.Proxies[len(p.finders)+i]); err != nil {
				return err
			}
		}

		if err = p.manager.AuthScrapper(ctx, account, scraper); err != nil {
			return err
		}

		limiterIntervals := []time.Duration{
			time.Minute,
			time.Hour,
			time.Hour * 24,
			time.Hour * 24 * 30,
		}

		windowLimiters := make([]WindowLimiter, len(limiterIntervals))

		for j := len(limiterIntervals) - 1; j >= 0; j-- {
			resetInterval := limiterIntervals[j]
			if j != len(limiterIntervals)-1 {
				resetInterval = limiterIntervals[j+1]
			}

			windowLimiters[j] = windowlimiter.NewLimiter(
				limiterIntervals[j],
				resetInterval,
				account.Login,
				p.repo,
				limiterLogger.WithField(finderLogin, account),
			)

			if j != len(limiterIntervals)-1 {
				windowLimiters[j].SetResetLimiter(windowLimiters[j+1])
			}
		}

		delayManager = NewDelayManagerV2(
			func(seconds int64) { scraper.WithDelay(seconds) },
			windowLimiters,
			startDelay,
			delayManagerLogger.WithField(finderLogin, account),
		)

		if err = delayManager.Start(ctx); err != nil {
			return err
		}

		f := NewFinder(scraper, delayManager, finderLogger.WithField(finderLogin, account))

		p.mu.Lock()
		p.finders = append(p.finders, f)
		p.finderDelays = append(p.finderDelays, f.CurrentDelay())
		p.mu.Unlock()

		select {
		case p.releaseSignal <- struct{}{}:
		default:
		}
	}

	return nil
}

func (p *pool) reinit() {
	for range time.After(time.Minute) {
		if err := p.init(context.Background()); err != nil {
			p.log.WithError(err).Error("error while reinit pool")
		}
	}
}

func NewPool(config ConfigPool, manager accountManager, db repo, logger log.Logger) Finder {
	return &pool{
		config:       config,
		finders:      make([]Finder, 0),
		manager:      manager,
		repo:         db,
		finderDelays: make([]int64, 0),
		log:          logger,
	}
}
