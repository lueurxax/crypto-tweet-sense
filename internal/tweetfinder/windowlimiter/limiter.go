package windowlimiter

import (
	"context"
	"time"

	"github.com/lueurxax/crypto-tweet-sense/internal/common"
	"github.com/lueurxax/crypto-tweet-sense/internal/log"
)

const (
	queueLen    = 10
	durationKey = "duration"
)

type WindowLimiter interface {
	Start(ctx context.Context, delay int64) error
	Inc()
	TrySetThreshold(ctx context.Context, startTime time.Time) error
	Duration() time.Duration
	RecommendedDelay(ctx context.Context) (uint64, error)
	Threshold(ctx context.Context) uint64
	Temp(ctx context.Context) float64
	SetResetLimiter(resetLimiter ResetLimiter)
}

type ResetLimiter interface {
	Threshold(ctx context.Context) uint64
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

type limiter struct {
	id       string
	duration time.Duration

	fire             uint
	putOutFireTicker *time.Ticker

	resetLimiter  ResetLimiter
	resetDuration time.Duration
	resetTicker   *time.Ticker

	count chan time.Time
	repo

	log log.Logger
}

func (l *limiter) Threshold(ctx context.Context) uint64 {
	rl, err := l.repo.GetRequestLimit(ctx, l.id, l.duration)
	if err != nil {
		l.log.WithError(err).Error("error while getting threshold")
		return 0
	}

	return rl.Threshold
}

func (l *limiter) Temp(ctx context.Context) float64 {
	rl, err := l.repo.GetRequestLimit(ctx, l.id, l.duration)
	if err != nil {
		l.log.WithError(err).Error("error while getting temp")
		return 0
	}

	temp := (float64(rl.RequestsCount) + 0.1) / float64(rl.Threshold)

	if l.fire > 0 {
		temp += float64(l.fire)
	}

	return temp
}

func (l *limiter) TrySetThreshold(ctx context.Context, startTime time.Time) error {
	l.resetTicker.Reset(l.resetDuration)

	if time.Since(startTime) > l.duration {
		if err := l.SetThreshold(ctx, l.id, l.duration); err != nil {
			return err
		}
	}

	rl, err := l.repo.GetRequestLimit(ctx, l.id, l.duration)
	if err == nil {
		l.log.WithField(durationKey, l.duration).
			WithField("threshold", rl.Threshold).
			WithField("current", rl.RequestsCount).
			Debug("set threshold")
	}

	l.putOutFireTicker.Reset(l.duration / 10)
	l.fire++

	return nil
}

func (l *limiter) Duration() time.Duration {
	return l.duration
}

func (l *limiter) RecommendedDelay(ctx context.Context) (uint64, error) {
	rl, err := l.repo.GetRequestLimit(ctx, l.id, l.duration)
	if err != nil {
		return 0, err
	}

	if rl.Threshold == 0 {
		return 0, nil
	}

	isFast := rl.Threshold <= rl.RequestsCount

	if !isFast {
		return 0, nil
	}

	l.log.WithField("threshold", rl.Threshold).
		WithField("counter", rl.RequestsCount).
		WithField(durationKey, l.duration).
		Trace("checking if too fast")

	return uint64(l.duration.Seconds()) / 5 / rl.Threshold, nil
}

func (l *limiter) Start(ctx context.Context, delay int64) error {
	go l.loop(ctx)

	isExist, err := l.repo.CheckIfExist(ctx, l.id, l.duration)
	if err != nil {
		return err
	}

	if isExist {
		return nil
	}

	threshold := uint64(l.duration.Seconds() / float64(delay))

	return l.repo.Create(ctx, l.id, l.duration, threshold)
}

func (l *limiter) Inc() {
	l.count <- time.Now()
}

func (l *limiter) GetCurrent(ctx context.Context) (uint64, error) {
	rl, err := l.repo.GetRequestLimit(ctx, l.id, l.duration)
	if err != nil {
		return 0, err
	}

	return rl.RequestsCount, nil
}

func (l *limiter) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	l.resetTicker = time.NewTicker(l.resetDuration)

	l.log.WithField(durationKey, l.duration).Info("start loop")

	for {
		requestctx, cancel := context.WithCancel(context.Background())
		select {
		case <-ctx.Done():
			cancel()
			return
		case t := <-l.count:
			l.log.WithField("time", t).Trace("inc counter")

			if err := l.repo.AddCounter(requestctx, l.id, l.duration, t); err != nil {
				panic(err)
			}
		case <-l.putOutFireTicker.C:
			l.fire = 0
		case <-ticker.C:
			l.log.WithField(durationKey, l.duration).Trace("clean counters")

			if err := l.repo.CleanCounters(requestctx, l.id, l.duration); err != nil {
				l.log.WithError(err).Error("error while cleaning counters")
			}
		case <-l.resetTicker.C:
			if l.resetLimiter == nil {
				continue
			}

			threshold := uint64(l.duration.Seconds()) * l.resetLimiter.Threshold(requestctx) / uint64(l.resetDuration.Seconds())
			if threshold == 0 {
				continue
			}

			if err := l.repo.IncreaseThresholdTo(requestctx, l.id, l.duration, threshold); err != nil {
				panic(err)
			}

			l.log.WithField(durationKey, l.duration).Trace("reset threshold")
		}
		cancel()
	}
}

func (l *limiter) SetResetLimiter(resetLimiter ResetLimiter) {
	l.resetLimiter = resetLimiter
}

func NewLimiter(duration, resetDuration time.Duration, id string, repo repo, logger log.Logger) WindowLimiter {
	return &limiter{
		putOutFireTicker: time.NewTicker(duration),
		duration:         duration,
		count:            make(chan time.Time, queueLen),
		id:               id,
		resetDuration:    resetDuration,
		repo:             repo,
		log:              logger,
	}
}
