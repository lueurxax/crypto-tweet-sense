package windowlimiter

import (
	"sync/atomic"
	"time"
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
	return atomic.LoadUint64(l.threshold) >= l.windowCounter.GetCurrent()
}

func NewLimiter(counter windowCounter, duration time.Duration, delay int64) WindowLimiter {
	threshold := uint64(duration.Seconds() / float64(delay))

	return &limiter{
		duration:      duration,
		windowCounter: counter,
		threshold:     &threshold,
	}
}
