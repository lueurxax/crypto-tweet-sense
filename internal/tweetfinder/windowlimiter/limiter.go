package windowlimiter

import (
	"sync/atomic"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

type WindowLimiter interface {
	Inc()
	TrySetThreshold(startTime time.Time)
	Duration() time.Duration
	TooFast() bool
}

type windowCounter interface {
	Inc()
	GetCurrent() uint64
}

type limiter struct {
	duration time.Duration
	windowCounter
	threshold *uint64

	log log.Logger
}

func (l *limiter) TrySetThreshold(startTime time.Time) {
	if time.Since(startTime) > l.duration {
		atomic.StoreUint64(l.threshold, l.windowCounter.GetCurrent())
	}
}

func (l *limiter) Duration() time.Duration {
	return l.duration
}

func (l *limiter) TooFast() bool {
	t := atomic.LoadUint64(l.threshold)
	l.log.WithField("threshold", t).
		WithField("counter", l.windowCounter.GetCurrent()).
		WithField("duration", l.duration).
		Debug("checking if too fast")

	return t <= l.windowCounter.GetCurrent()
}

func NewLimiter(counter windowCounter, duration time.Duration, delay int64, logger log.Logger) WindowLimiter {
	threshold := uint64(duration.Seconds() / float64(delay))

	return &limiter{
		duration:      duration,
		windowCounter: counter,
		threshold:     &threshold,
		log:           logger,
	}
}
