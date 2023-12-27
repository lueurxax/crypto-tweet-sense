package tweetfinder

import (
	"context"
	"sync"
	"time"

	twitterscraper "github.com/lueurxax/twitter-scraper"
	"github.com/prometheus/client_golang/prometheus"

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
	config        ConfigProxies
	finders       []Finder
	releaseSignal chan struct{}

	manager accountManager
	repo    repo

	mu           sync.RWMutex
	finderDelays []int64
	finderTemp   []float64

	log log.Logger

	metricsOne   *prometheus.HistogramVec
	metricsNext  *prometheus.HistogramVec
	metricsDelay *prometheus.GaugeVec
}

func (p *pool) IsHot() bool {
	hotCounter := 0

	for _, temp := range p.finderTemp {
		if skipFinder(temp) {
			hotCounter++
		}
	}

	return hotCounter > len(p.finderTemp)/2
}

func (p *pool) CurrentTemp(context.Context) float64 {
	sum := 0.0

	p.mu.RLock()
	for _, d := range p.finderTemp {
		sum += d
	}
	p.mu.RUnlock()

	return sum / float64(len(p.finderTemp))
}

func (p *pool) FindNext(ctx context.Context, start, end *time.Time, search, cursor string) ([]common.TweetSnapshot, string, error) {
	f, index, err := p.getFinder(ctx)
	if err != nil {
		return nil, "", err
	}

	defer p.releaseFinder(index)

	return f.FindNext(ctx, start, end, search, cursor)
}

func (p *pool) Init(ctx context.Context) error {
	if err := p.init(ctx); err != nil {
		return err
	}

	go p.reinit(ctx)

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

func (p *pool) Find(ctx context.Context, id string) (*common.TweetSnapshot, error) {
	f, index, err := p.getFinder(ctx)
	if err != nil {
		return nil, err
	}

	defer p.releaseFinder(index)

	return f.Find(ctx, id)
}

func (p *pool) getFinder(ctx context.Context) (Finder, int, error) {
	index, ok := p.getFinderIndex(ctx)

	ticker := time.NewTicker(time.Second)

	timer := time.NewTimer(time.Minute)
	defer timer.Stop()

	for !ok {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-timer.C:
			return nil, 0, ErrTimeoutSelectFinder
		case <-p.releaseSignal:
			break
		case <-ticker.C:
			break
		}

		index, ok = p.getFinderIndex(ctx)
	}

	return p.finders[index], index, nil
}

func (p *pool) getFinderIndex(ctx context.Context) (int, bool) {
	p.mu.Lock()
	minimal := 0.0
	index := 0

	for i := range p.finderTemp {
		if p.finderTemp[i] != 0 {
			p.finderDelays[i] = p.finders[i].CurrentDelay()
			p.finderTemp[i] = p.finders[i].CurrentTemp(ctx)
		}
	}

	for i, d := range p.finderTemp {
		if skipFinder(d) {
			continue
		}

		if d < minimal || minimal == 0 {
			minimal = d
			index = i
		}
	}

	if minimal != 0 {
		p.finderDelays[index] = 0
		p.finderTemp[index] = 0
	}

	p.mu.Unlock()

	return index, minimal != 0
}

func skipFinder(d float64) bool {
	return d == 0 || d > 4
}

func (p *pool) releaseFinder(i int) {
	p.mu.Lock()
	p.finderDelays[i] = p.finders[i].CurrentDelay()
	p.finderTemp[i] = p.finders[i].CurrentTemp(context.Background())
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

	proxies := p.config.GetProxies()

	for i, account := range accounts {
		scraper := twitterscraper.New().WithDelay(startDelay).SetSearchMode(twitterscraper.SearchLatest)

		if len(proxies) > len(p.finders)+i {
			if err = scraper.SetProxy(proxies[len(p.finders)+i]); err != nil {
				return err
			}
		}

		if err = p.manager.AuthScrapper(ctx, account, scraper); err != nil {
			return err
		}

		limiterIntervals := []time.Duration{
			time.Minute * 10,
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
				limiterLogger.WithField(finderLogin, account.Login),
			)

			if j != len(limiterIntervals)-1 {
				windowLimiters[j].SetResetLimiter(windowLimiters[j+1])
			}
		}

		ds := newDelaySetter(func(seconds int64) { scraper.WithDelay(seconds) }, p.metricsDelay, account.Login)

		delayManager = NewDelayManagerV2(
			ds.Set,
			windowLimiters,
			startDelay,
			delayManagerLogger.WithField(finderLogin, account.Login),
		)

		if err = delayManager.Start(ctx); err != nil {
			return err
		}

		f := NewMetricMiddleware(
			p.metricsOne, p.metricsNext,
			account.Login,
			NewFinder(scraper, delayManager, finderLogger.WithField(finderLogin, account.Login)),
		)

		p.mu.Lock()
		p.finders = append(p.finders, f)
		p.finderDelays = append(p.finderDelays, f.CurrentDelay())
		p.finderTemp = append(p.finderTemp, f.CurrentTemp(ctx))
		p.mu.Unlock()

		select {
		case p.releaseSignal <- struct{}{}:
		default:
		}
	}

	return nil
}

func (p *pool) reinit(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		if err := p.init(ctx); err != nil {
			p.log.WithError(err).Error("error while reinit pool")
		}
	}
}

func NewPool(metricsOne, metricsNext *prometheus.HistogramVec, metricsDelay *prometheus.GaugeVec,
	config ConfigProxies, manager accountManager, db repo, logger log.Logger) Finder {
	return &pool{
		config:       config,
		finders:      make([]Finder, 0),
		manager:      manager,
		repo:         db,
		mu:           sync.RWMutex{},
		finderDelays: make([]int64, 0),
		finderTemp:   make([]float64, 0),
		log:          logger,
		metricsOne:   metricsOne,
		metricsNext:  metricsNext,
		metricsDelay: metricsDelay,
	}
}
