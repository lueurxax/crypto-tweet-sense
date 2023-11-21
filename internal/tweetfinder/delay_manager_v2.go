package tweetfinder

import (
	"context"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder/windowlimiter"
)

type WindowLimiter interface {
	Inc()
	TrySetThreshold(ctx context.Context, startTime time.Time) error
	Duration() time.Duration
	TooFast(ctx context.Context) (uint64, error)
	SetResetLimiter(resetLimiter windowlimiter.ResetLimiter)
	Threshold(ctx context.Context) uint64
	Start(ctx context.Context, delay int64) error
}

type managerV2 struct {
	setter func(seconds int64)
	delay  int64

	windowLimiters   []WindowLimiter
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
			if err := m.recalculate(ctx, 5); err != nil {
				m.log.WithError(err).Error("error while recalculate")
			}
		case <-ticker.C:
			if err := m.recalculate(ctx, 1); err != nil {
				m.log.WithError(err).Error("error while recalculate")
			}
		}
	}
}

func (m *managerV2) recalculate(ctx context.Context, factor int) error {
	var (
		recommendedDelay uint64
		err              error
	)

	shouldDecrease := true

	for _, limiter := range m.windowLimiters {
		recommendedDelay, err = limiter.TooFast(ctx)
		if err != nil {
			return err
		}

		delay := int64(recommendedDelay) * int64(factor)

		if recommendedDelay > 0 {
			shouldDecrease = false
		}

		if recommendedDelay > 0 && delay != m.delay {
			m.delay = delay
			m.log.
				WithField("limiter_duration", limiter.Duration()).
				WithField(delayKey, m.delay).
				Debug("delay increased")

			break
		}
	}

	if shouldDecrease && m.delay > 1 {
		if m.delay < 6 {
			m.delay--
		} else {
			m.delay /= 2
		}

		m.log.WithField(delayKey, m.delay).Debug("delay decreased")
	}

	m.setter(m.delay)

	return nil
}

func NewDelayManagerV2(
	setter func(seconds int64),
	windowLimiters []WindowLimiter,
	minimalDelay int64,
	log log.Logger,
) Manager {

	return &managerV2{
		forceRecalculate: make(chan struct{}, 1000),
		setter:           setter,
		delay:            minimalDelay,
		windowLimiters:   windowLimiters,
		startTime:        time.Now(),
		log:              log,
	}
}
