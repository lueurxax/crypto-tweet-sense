package tweetfinder

import (
	"context"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder/windowcounter"
	"github.com/lueurxax/crypto-tweet-sense/internal/tweetfinder/windowlimiter"
)

type windowLimiter interface {
	Inc()
	TrySetThreshold(startTime time.Time)
	Duration() time.Duration
	TooFast() bool
}

type managerV2 struct {
	setter func(seconds int64)
	delay  int64

	windowLimiters []windowLimiter

	startTime time.Time

	log log.Logger
}

func (m *managerV2) TooManyRequests() {
	m.log.WithField(delayKey, m.delay).Error("too many requests")

	for _, limiter := range m.windowLimiters {
		limiter.TrySetThreshold(m.startTime)
	}
}

func (m *managerV2) ProcessedBatchOfTweets() {
	for _, limiter := range m.windowLimiters {
		limiter.Inc()
	}
}

func (m *managerV2) ProcessedQuery() {}

func (m *managerV2) SetSetterFn(f func(seconds int64)) {
	m.setter = f
}

func (m *managerV2) CurrentDelay() int64 {
	return m.delay
}

func (m *managerV2) start() {
	ctx := context.Background()
	go m.loop(ctx)
}

func (m *managerV2) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.recalculate()
		}
	}
}

func (m *managerV2) recalculate() {
	for _, limiter := range m.windowLimiters {
		if limiter.TooFast() {
			m.delay++
			m.log.WithField("limiter_duration", limiter.Duration()).WithField(delayKey, m.delay).Debug("delay increased")
		}
	}

	m.setter(m.delay)
}

func NewDelayManagerV2(setter func(seconds int64), minimalDelay int64, log log.Logger) Manager {
	windowLimiters := make([]windowLimiter, 3)
	for i, duration := range []time.Duration{
		time.Minute,
		time.Hour,
		time.Hour * 24,
	} {
		windowLimiters[i] = windowlimiter.NewLimiter(windowcounter.NewCounter(duration), duration, minimalDelay, log)
	}

	m := &managerV2{
		setter:         setter,
		delay:          minimalDelay,
		windowLimiters: windowLimiters,
		startTime:      time.Now(),
		log:            log,
	}
	m.start()

	return m
}
