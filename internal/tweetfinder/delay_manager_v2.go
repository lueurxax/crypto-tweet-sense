package tweetfinder

import (
	"context"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder/windowlimiter"
)

type windowLimiter interface {
	Inc()
	TrySetThreshold(ctx context.Context, startTime time.Time) error
	Duration() time.Duration
	TooFast(ctx context.Context) (uint64, error)
	Start(ctx context.Context, delay int64) error
}

type repo interface {
	AddCounter(ctx context.Context, id string, window time.Duration, counterTime time.Time) error
	CleanCounters(ctx context.Context, id string, window time.Duration) error
	GetCounters(ctx context.Context, id string, window time.Duration) (uint64, error)
	SetThreshold(ctx context.Context, id string, window time.Duration) error
	GetThreshold(ctx context.Context, id string, window time.Duration) (uint64, error)
	CheckIfExist(ctx context.Context, id string, window time.Duration) (bool, error)
	Create(ctx context.Context, id string, window time.Duration, threshold uint64) error
}

type managerV2 struct {
	setter func(seconds int64)
	delay  int64

	windowLimiters   []windowLimiter
	forceRecalculate chan struct{}

	startTime time.Time

	log log.Logger
}

func (m *managerV2) TooManyRequests(ctx context.Context) {
	m.log.WithField(delayKey, m.delay).Error("too many requests")

	for _, limiter := range m.windowLimiters {
		if err := limiter.TrySetThreshold(ctx, m.startTime); err != nil {
			m.log.WithError(err).Error("error while setting threshold")
			return
		}
	}
}

func (m *managerV2) AfterRequest() {
	for _, limiter := range m.windowLimiters {
		limiter.Inc()
	}
	m.forceRecalculate <- struct{}{}
}

func (m *managerV2) ProcessedQuery() {}

func (m *managerV2) SetSetterFn(f func(seconds int64)) {
	m.setter = f
}

func (m *managerV2) CurrentDelay() int64 {
	return m.delay
}

func (m *managerV2) Start(ctx context.Context) error {
	for _, limiter := range m.windowLimiters {
		if err := limiter.Start(ctx, m.delay); err != nil {
			return err
		}
	}

	go m.loop(ctx)

	return nil
}

func (m *managerV2) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 10)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.forceRecalculate:
			if err := m.recalculate(ctx, 10); err != nil {
				m.log.WithError(err).Error("error while recalculate")
			}
		case <-ticker.C:
			if err := m.recalculate(ctx, 10); err != nil {
				m.log.WithError(err).Error("error while recalculate")
			}
		}
	}
}

func (m *managerV2) recalculate(ctx context.Context, factor int) error {
	var (
		recomendedDelay uint64
		err             error
	)

	for _, limiter := range m.windowLimiters {
		recomendedDelay, err = limiter.TooFast(ctx)
		if err != nil {
			return err
		}

		if recomendedDelay > 0 {
			m.delay += int64(factor)
			if uint64(m.delay) >= recomendedDelay*2 {
				m.delay = int64(recomendedDelay * 2)
			} else {
				m.log.WithField("limiter_duration", limiter.Duration()).WithField(delayKey, m.delay).Debug("delay increased")
				break
			}
		}
	}

	if recomendedDelay == 0 && m.delay > 1 {
		m.delay--
		m.log.WithField(delayKey, m.delay).Debug("delay decreased")
	}

	m.setter(m.delay)

	return nil
}

func NewDelayManagerV2(setter func(seconds int64), id string, minimalDelay int64, repo repo, log log.Logger) Manager {
	limiterIntervals := []time.Duration{
		time.Minute,
		time.Hour,
		time.Hour * 24,
	}

	windowLimiters := make([]windowLimiter, len(limiterIntervals))

	for i, duration := range limiterIntervals {
		windowLimiters[i] = windowlimiter.NewLimiter(duration, id, repo, log)
	}

	return &managerV2{
		forceRecalculate: make(chan struct{}, 1000),
		setter:           setter,
		delay:            minimalDelay,
		windowLimiters:   windowLimiters,
		startTime:        time.Now(),
		log:              log,
	}
}
